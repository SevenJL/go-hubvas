package postgres

import (
	"context"

	appauth "github.com/hubvas/internal/application/auth"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepo struct{ pool *pgxpool.Pool }

func NewAccountRepo(pool *pgxpool.Pool) *AccountRepo { return &AccountRepo{pool: pool} }

func (r *AccountRepo) Register(ctx context.Context, user *identity.User, session appauth.RefreshSession) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, saveInsertSQL, user.Username(), user.Email(), user.PasswordHash(), user.DisplayName(), user.Bio(), user.Website(), nullIfEmpty(user.AvatarURL()), nullIfEmpty(user.AvatarKey()), user.AvatarVersion(), user.SecurityVersion(), user.AccountRole(), user.Status(), user.CreatedAt(), user.UpdatedAt()).Scan(&id)
	if err != nil {
		return mapPgError(err)
	}
	user.SetID(identity.UserID(id))
	session.UserID = user.ID()
	_, err = tx.Exec(ctx, `INSERT INTO auth_sessions(id,family_id,user_id,token_hash,user_agent,ip_address,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, session.ID, session.FamilyID, session.UserID, session.TokenHash, session.Metadata.UserAgent, nullableIP(session.Metadata.IPAddress), session.ExpiresAt)
	if err != nil {
		return mapPgError(err)
	}
	return tx.Commit(ctx)
}

func (r *AccountRepo) ChangePasswordAndRevokeSessions(ctx context.Context, user *identity.User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password_hash=$1,security_version=$2,updated_at=$3 WHERE id=$4 AND security_version=$5`, user.PasswordHash(), user.SecurityVersion(), user.UpdatedAt(), user.ID(), user.SecurityVersion()-1)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrConflict, "password was changed by another request")
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL`, user.ID()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AccountRepo) RevokeAllAccess(ctx context.Context, userID identity.UserID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET security_version=security_version+1,updated_at=now() WHERE id=$1`, userID)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found")
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
