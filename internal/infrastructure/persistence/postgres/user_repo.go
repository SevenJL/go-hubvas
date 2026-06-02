package postgres

import (
	"context"
	"database/sql"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// UserRepo implements identity.UserRepository using PostgreSQL.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Save persists a user. If the user's ID is 0, it INSERTs and sets the ID.
// Otherwise it UPDATEs the existing row.
func (r *UserRepo) Save(ctx context.Context, user *identity.User) error {
	// TODO: Implement with real SQL
	return shared.NewDomainError(shared.ErrNotFound, "postgres.UserRepo.Save not implemented")
}

// FindByID retrieves a user by ID.
func (r *UserRepo) FindByID(ctx context.Context, id identity.UserID) (*identity.User, error) {
	// TODO: Implement with real SQL
	return nil, shared.NewDomainError(shared.ErrNotFound, "postgres.UserRepo.FindByID not implemented")
}

// FindByUsername retrieves a user by username.
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*identity.User, error) {
	// TODO: Implement with real SQL
	return nil, shared.NewDomainError(shared.ErrNotFound, "postgres.UserRepo.FindByUsername not implemented")
}

// FindByEmail retrieves a user by email.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*identity.User, error) {
	// TODO: Implement with real SQL
	return nil, shared.NewDomainError(shared.ErrNotFound, "postgres.UserRepo.FindByEmail not implemented")
}

// ExistsByUsername checks username uniqueness.
func (r *UserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	// TODO: Implement with real SQL
	return false, nil
}

// ExistsByEmail checks email uniqueness.
func (r *UserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	// TODO: Implement with real SQL
	return false, nil
}

// Delete removes a user.
func (r *UserRepo) Delete(ctx context.Context, id identity.UserID) error {
	// TODO: Implement with real SQL
	return nil
}
