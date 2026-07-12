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

func (r *AuthSessionRepo) Rotate(ctx context.Context, currentHash []byte, next appauth.RefreshSession) (identity.UserID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var id, family string
	var userID identity.UserID
	var expires time.Time
	var revoked *time.Time
	err = tx.QueryRow(ctx, `SELECT id,family_id,user_id,expires_at,revoked_at FROM auth_sessions WHERE token_hash=$1 FOR UPDATE`, currentHash).Scan(&id, &family, &userID, &expires, &revoked)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, shared.NewDomainError(shared.ErrUnauthorized, "invalid refresh session")
		}
		return 0, mapPgError(err)
	}
	if revoked != nil {
		if _, e := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE family_id=$1`, family); e != nil {
			return 0, e
		}
		if e := tx.Commit(ctx); e != nil {
			return 0, e
		}
		return 0, shared.NewDomainError(shared.ErrUnauthorized, "refresh token reuse detected; session family revoked")
	}
	if time.Now().After(expires) {
		_, _ = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE id=$1`, id)
		if e := tx.Commit(ctx); e != nil {
			return 0, e
		}
		return 0, shared.NewDomainError(shared.ErrUnauthorized, "refresh session expired")
	}
	next.UserID = userID
	next.FamilyID = family
	// A refresh rotates credentials but does not extend the session family's
	// absolute lifetime. Keeping one fixed expiry also preserves reuse-detection
	// records until every credential in the family is unusable.
	next.ExpiresAt = expires
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now(),last_used_at=now(),replaced_by_id=$1 WHERE id=$2`, next.ID, id); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_sessions(id,family_id,user_id,token_hash,user_agent,ip_address,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, next.ID, family, userID, next.TokenHash, next.Metadata.UserAgent, nullableIP(next.Metadata.IPAddress), next.ExpiresAt); err != nil {
		return 0, mapPgError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return userID, nil
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
