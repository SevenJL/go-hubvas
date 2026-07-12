package postgres

import (
	"context"
	"net"
	"time"

	appauth "github.com/hubvas/internal/application/auth"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthSessionRepo struct{ pool *pgxpool.Pool }

func NewAuthSessionRepo(pool *pgxpool.Pool) *AuthSessionRepo { return &AuthSessionRepo{pool: pool} }

func nullableIP(value string) any {
	if net.ParseIP(value) == nil {
		return nil
	}
	return value
}

func (r *AuthSessionRepo) Create(ctx context.Context, s appauth.RefreshSession) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO auth_sessions(id,family_id,user_id,token_hash,user_agent,ip_address,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, s.ID, s.FamilyID, s.UserID, s.TokenHash, s.Metadata.UserAgent, nullableIP(s.Metadata.IPAddress), s.ExpiresAt)
	return mapPgError(err)
}

func (r *AuthSessionRepo) Rotate(ctx context.Context, currentHash []byte, next appauth.RefreshSession) (identity.UserID, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var id, family string
	var userID identity.UserID
	var securityVersion int64
	var expires time.Time
	var revoked *time.Time
	err = tx.QueryRow(ctx, `
		SELECT s.id,s.family_id,s.user_id,s.expires_at,s.revoked_at,u.security_version
		FROM auth_sessions s
		JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1
		FOR UPDATE OF s
	`, currentHash).Scan(&id, &family, &userID, &expires, &revoked, &securityVersion)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, shared.NewDomainError(shared.ErrUnauthorized, "invalid refresh session")
		}
		return 0, 0, mapPgError(err)
	}
	if revoked != nil {
		if _, e := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE family_id=$1`, family); e != nil {
			return 0, 0, e
		}
		if e := tx.Commit(ctx); e != nil {
			return 0, 0, e
		}
		return 0, 0, shared.NewDomainError(shared.ErrUnauthorized, "refresh token reuse detected; session family revoked")
	}
	if time.Now().After(expires) {
		_, _ = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE id=$1`, id)
		if e := tx.Commit(ctx); e != nil {
			return 0, 0, e
		}
		return 0, 0, shared.NewDomainError(shared.ErrUnauthorized, "refresh session expired")
	}
	next.UserID = userID
	next.FamilyID = family
	// A refresh rotates credentials but does not extend the session family's
	// absolute lifetime. Keeping one fixed expiry also preserves reuse-detection
	// records until every credential in the family is unusable.
	next.ExpiresAt = expires
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now(),last_used_at=now(),replaced_by_id=$1 WHERE id=$2`, next.ID, id); err != nil {
		return 0, 0, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_sessions(id,family_id,user_id,token_hash,user_agent,ip_address,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, next.ID, family, userID, next.TokenHash, next.Metadata.UserAgent, nullableIP(next.Metadata.IPAddress), next.ExpiresAt); err != nil {
		return 0, 0, mapPgError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return userID, securityVersion, nil
}

func (r *AuthSessionRepo) Revoke(ctx context.Context, hash []byte) error {
	_, err := r.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash)
	return err
}
func (r *AuthSessionRepo) RevokeAll(ctx context.Context, userID identity.UserID) error {
	_, err := r.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}

// CleanupExpired removes session rows only after their credentials can no
// longer be used. Revoked but unexpired rows are retained so refresh-token
// reuse can still revoke the complete token family.
func (r *AuthSessionRepo) CleanupExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := r.pool.Exec(ctx, `
		DELETE FROM auth_sessions
		WHERE id IN (
			SELECT id FROM auth_sessions
			WHERE expires_at < $1
			ORDER BY expires_at ASC
			LIMIT $2
		)
	`, before, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *AuthSessionRepo) List(ctx context.Context, userID identity.UserID, currentHash []byte) ([]appauth.SessionDTO, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,user_agent,COALESCE(host(ip_address),''),created_at,last_used_at,expires_at,revoked_at,CASE WHEN $2::bytea IS NULL THEN false ELSE token_hash=$2 END FROM auth_sessions WHERE user_id=$1 AND expires_at>now() ORDER BY created_at DESC`, userID, nullableBytes(currentHash))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]appauth.SessionDTO, 0)
	for rows.Next() {
		var item appauth.SessionDTO
		if err := rows.Scan(&item.ID, &item.UserAgent, &item.IPAddress, &item.CreatedAt, &item.LastUsedAt, &item.ExpiresAt, &item.RevokedAt, &item.Current); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func (r *AuthSessionRepo) RevokeByID(ctx context.Context, userID identity.UserID, sessionID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1 AND user_id=$2`, sessionID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "session not found")
	}
	return nil
}
