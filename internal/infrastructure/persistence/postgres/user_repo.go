package postgres

import (
	"context"
	"errors"

	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// UserRepo implements identity.UserRepository using PostgreSQL via pgx.
//
// It uses a pgxpool.Pool for connection pooling. All methods accept a context
// for cancellation and tracing.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a UserRepo backed by the given pgx connection pool.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// ---- Save ----

const (
	saveInsertSQL = `
		INSERT INTO users (username, email, password_hash, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	saveUpdateSQL = `
		UPDATE users
		SET username      = $1,
		    email         = $2,
		    password_hash = $3,
		    avatar_url    = $4
		WHERE id = $5
	`
)

// Save persists a user. When the User's ID is 0 it performs an INSERT and
// sets the generated ID on the aggregate. Otherwise it runs an UPDATE.
func (r *UserRepo) Save(ctx context.Context, user *identity.User) error {
	if user.ID() == 0 {
		return r.insert(ctx, user)
	}
	return r.update(ctx, user)
}

func (r *UserRepo) insert(ctx context.Context, user *identity.User) error {
	var id int64
	err := r.pool.QueryRow(ctx, saveInsertSQL,
		user.Username(),
		user.Email(),
		user.PasswordHash(),
		nullIfEmpty(user.AvatarURL()),
		user.CreatedAt(),
	).Scan(&id)
	if err != nil {
		return mapPgError(err)
	}
	user.SetID(identity.UserID(id))
	return nil
}

func (r *UserRepo) update(ctx context.Context, user *identity.User) error {
	tag, err := r.pool.Exec(ctx, saveUpdateSQL,
		user.Username(),
		user.Email(),
		user.PasswordHash(),
		nullIfEmpty(user.AvatarURL()),
		user.ID(),
	)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found for update")
	}
	return nil
}

// ---- FindByID ----

const findByIDSQL = `
	SELECT id, username, email, password_hash, avatar_url, created_at
	FROM users
	WHERE id = $1
`

// FindByID retrieves a user by primary key. Returns ErrNotFound when the row
// does not exist.
func (r *UserRepo) FindByID(ctx context.Context, id identity.UserID) (*identity.User, error) {
	var (
		uid          int64
		username     string
		email        string
		passwordHash string
		avatarURL    *string
		createdAt    time.Time
	)

	err := r.pool.QueryRow(ctx, findByIDSQL, id).Scan(
		&uid, &username, &email, &passwordHash, &avatarURL, &createdAt,
	)
	if err != nil {
		return nil, mapPgError(err)
	}

	return identity.ReconstituteUser(
		identity.UserID(uid),
		username,
		email,
		passwordHash,
		derefString(avatarURL),
		createdAt,
	), nil
}

// ---- FindByUsername ----

const findByUsernameSQL = `
	SELECT id, username, email, password_hash, avatar_url, created_at
	FROM users
	WHERE username = $1
`

// FindByUsername retrieves a user by exact username match.
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*identity.User, error) {
	return r.findOneBy(ctx, findByUsernameSQL, username)
}

// ---- FindByEmail ----

const findByEmailSQL = `
	SELECT id, username, email, password_hash, avatar_url, created_at
	FROM users
	WHERE email = $1
`

// FindByEmail retrieves a user by exact email match.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*identity.User, error) {
	return r.findOneBy(ctx, findByEmailSQL, email)
}

// ---- ExistsByUsername ----

const existsByUsernameSQL = `
	SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)
`

// ExistsByUsername returns true if the username is already registered.
func (r *UserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, existsByUsernameSQL, username).Scan(&exists)
	if err != nil {
		return false, mapPgError(err)
	}
	return exists, nil
}

// ---- ExistsByEmail ----

const existsByEmailSQL = `
	SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
`

// ExistsByEmail returns true if the email is already registered.
func (r *UserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, existsByEmailSQL, email).Scan(&exists)
	if err != nil {
		return false, mapPgError(err)
	}
	return exists, nil
}

// ---- Delete ----

const deleteSQL = `DELETE FROM users WHERE id = $1`

// Delete removes a user permanently.
func (r *UserRepo) Delete(ctx context.Context, id identity.UserID) error {
	tag, err := r.pool.Exec(ctx, deleteSQL, id)
	if err != nil {
		return mapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewDomainError(shared.ErrNotFound, "user not found for deletion")
	}
	return nil
}

// ---- helpers ----

// findOneBy executes a single-row query that returns a user row.
func (r *UserRepo) findOneBy(ctx context.Context, query string, arg any) (*identity.User, error) {
	var (
		uid          int64
		username     string
		email        string
		passwordHash string
		avatarURL    *string
		createdAt    time.Time
	)

	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&uid, &username, &email, &passwordHash, &avatarURL, &createdAt,
	)
	if err != nil {
		return nil, mapPgError(err)
	}

	return identity.ReconstituteUser(
		identity.UserID(uid),
		username,
		email,
		passwordHash,
		derefString(avatarURL),
		createdAt,
	), nil
}

// mapPgError translates pgx-level errors into domain errors so the domain
// layer never sees driver-specific types.
func mapPgError(err error) error {
	if err == nil {
		return nil
	}

	// No rows returned — map to domain "not found".
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NewDomainError(shared.ErrNotFound, "entity not found")
	}

	// Unique constraint violation (code 23505).
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			// Extract the constraint name to produce a helpful message.
			msg := "already exists"
			switch pgErr.ConstraintName {
			case "uq_users_username":
				msg = "username is already taken"
			case "uq_users_email":
				msg = "email is already registered"
			}
			return shared.NewDomainError(shared.ErrAlreadyExists, msg)
		case "23503": // foreign_key_violation
			return shared.NewDomainError(shared.ErrInvalidArgument, "referenced entity does not exist")
		case "23514": // check_violation
			return shared.NewDomainError(shared.ErrInvalidArgument, "constraint violation: "+pgErr.Message)
		}
	}

	// All other errors — connection failures, timeouts, etc.
	return err
}

// nullIfEmpty returns a *string suitable for SQL NULL. An empty string yields nil.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// derefString returns the dereferenced string, or "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
