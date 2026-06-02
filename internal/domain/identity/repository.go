package identity

import "context"

// UserRepository defines the contract for persisting and retrieving User aggregates.
// Implementations live in the infrastructure layer (e.g., PostgreSQL).
type UserRepository interface {
	// Save persists a new or updated user.
	Save(ctx context.Context, user *User) error

	// FindByID retrieves a user by their unique identifier.
	FindByID(ctx context.Context, id UserID) (*User, error)

	// FindByUsername retrieves a user by their username.
	FindByUsername(ctx context.Context, username string) (*User, error)

	// FindByEmail retrieves a user by their email address.
	FindByEmail(ctx context.Context, email string) (*User, error)

	// ExistsByUsername checks whether a username is already taken.
	ExistsByUsername(ctx context.Context, username string) (bool, error)

	// ExistsByEmail checks whether an email is already registered.
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// Delete removes a user permanently.
	Delete(ctx context.Context, id UserID) error
}
