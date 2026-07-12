package postgres

import (
	"context"
	"errors"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

const userColumns = `id, username, email, password_hash, display_name, bio, website, avatar_url, avatar_key, avatar_version, account_role, status, created_at, updated_at`
const saveInsertSQL = `INSERT INTO users (username,email,password_hash,display_name,bio,website,avatar_url,avatar_key,avatar_version,account_role,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`
const saveUpdateSQL = `UPDATE users SET email=$1,password_hash=$2,display_name=$3,bio=$4,website=$5,avatar_url=$6,avatar_key=$7,avatar_version=$8,account_role=$9,status=$10,updated_at=$11 WHERE id=$12`

func (r *UserRepo) Save(ctx context.Context, u *identity.User) error {
	if u.ID() == 0 {
		var id int64
		err := r.pool.QueryRow(ctx, saveInsertSQL, u.Username(), u.Email(), u.PasswordHash(), u.DisplayName(), u.Bio(), u.Website(), nullIfEmpty(u.AvatarURL()), nullIfEmpty(u.AvatarKey()), u.AvatarVersion(), u.AccountRole(), u.Status(), u.CreatedAt(), u.UpdatedAt()).Scan(&id)
		if err != nil {
			return mapPgError(err)
		}
		u.SetID(identity.UserID(id))
		return nil
	}
	tag, err := r.pool.Exec(ctx, saveUpdateSQL, u.Email(), u.PasswordHash(), u.DisplayName(), u.Bio(), u.Website(), nullIfEmpty(u.AvatarURL()), nullIfEmpty(u.AvatarKey()), u.AvatarVersion(), u.AccountRole(), u.Status(), u.UpdatedAt(), u.ID())
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found for update")
	}
	return nil
}
func (r *UserRepo) FindByID(ctx context.Context, id identity.UserID) (*identity.User, error) {
	return r.findOneBy(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id)
}
func (r *UserRepo) FindByUsername(ctx context.Context, v string) (*identity.User, error) {
	return r.findOneBy(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(username)=LOWER($1)`, v)
}
func (r *UserRepo) FindByEmail(ctx context.Context, v string) (*identity.User, error) {
	return r.findOneBy(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(email)=LOWER($1)`, v)
}
func (r *UserRepo) ExistsByUsername(ctx context.Context, v string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username)=LOWER($1))`, v).Scan(&ok)
	return ok, mapPgError(err)
}
func (r *UserRepo) ExistsByEmail(ctx context.Context, v string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email)=LOWER($1))`, v).Scan(&ok)
	return ok, mapPgError(err)
}
func (r *UserRepo) Delete(ctx context.Context, id identity.UserID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found for deletion")
	}
	return nil
}
func (r *UserRepo) findOneBy(ctx context.Context, q string, arg any) (*identity.User, error) {
	var id int64
	var username, email, hash, display, bio, website, role, status string
	var avatarURL, avatarKey *string
	var version int64
	var created, updated time.Time
	err := r.pool.QueryRow(ctx, q, arg).Scan(&id, &username, &email, &hash, &display, &bio, &website, &avatarURL, &avatarKey, &version, &role, &status, &created, &updated)
	if err != nil {
		return nil, mapPgError(err)
	}
	return identity.ReconstituteUserProfile(identity.UserID(id), username, email, hash, display, bio, website, derefString(avatarURL), derefString(avatarKey), version, role, status, created, updated), nil
}

func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NewDomainError(shared.ErrNotFound, "entity not found")
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			msg := "already exists"
			if p.ConstraintName == "uq_users_username" {
				msg = "username is already taken"
			}
			if p.ConstraintName == "uq_users_email" {
				msg = "email is already registered"
			}
			return shared.NewDomainError(shared.ErrAlreadyExists, msg)
		case "23503":
			return shared.NewDomainError(shared.ErrInvalidArgument, "referenced entity does not exist")
		case "23514":
			return shared.NewDomainError(shared.ErrInvalidArgument, "constraint violation: "+p.Message)
		}
	}
	return err
}
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
