package identity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hubvas/internal/domain/shared"
)

// UserID is the strongly-typed identifier for a user.
type UserID int64

// User is the aggregate root for the Identity bounded context.
// It enforces invariants around registration, login, and profile updates.
type User struct {
	shared.AggregateRoot
	id           UserID
	username     string
	email        string
	passwordHash string
	avatarURL    string
	createdAt    time.Time
}

// NewUser is the factory for creating a new User aggregate.
// It validates business rules before construction.
func NewUser(id UserID, username, email, passwordHash string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)

	if username == "" || utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "username must be 3-50 characters")
	}
	if email == "" || !strings.Contains(email, "@") {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "email is invalid")
	}
	if passwordHash == "" {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "password hash must not be empty")
	}

	u := &User{
		id:           id,
		username:     username,
		email:        email,
		passwordHash: passwordHash,
		createdAt:    time.Now(),
	}

	u.AddEvent(UserRegisteredEvent{
		BaseEvent: shared.NewBaseEvent("UserRegistered"),
		UserID:    id,
		Username:  username,
		Email:     email,
	})

	return u, nil
}

// ReconstituteUser rebuilds a User from persistence. Only the repository should call this.
func ReconstituteUser(id UserID, username, email, passwordHash, avatarURL string, createdAt time.Time) *User {
	return &User{
		id:           id,
		username:     username,
		email:        email,
		passwordHash: passwordHash,
		avatarURL:    avatarURL,
		createdAt:    createdAt,
	}
}

// ---- Accessors ----

func (u *User) ID() UserID            { return u.id }
func (u *User) Username() string      { return u.username }
func (u *User) Email() string         { return u.email }
func (u *User) PasswordHash() string  { return u.passwordHash }
func (u *User) AvatarURL() string     { return u.avatarURL }
func (u *User) CreatedAt() time.Time  { return u.createdAt }

// ---- Mutations (with invariants) ----

// ChangePassword updates the password hash. Old hash verification is the caller's responsibility.
func (u *User) ChangePassword(newHash string) error {
	if newHash == "" {
		return shared.NewDomainError(shared.ErrInvalidArgument, "password hash must not be empty")
	}
	u.passwordHash = newHash
	return nil
}

// SetAvatarURL sets the user's avatar URL.
func (u *User) SetAvatarURL(url string) error {
	if url != "" && !strings.HasPrefix(url, "http") {
		return shared.NewDomainError(shared.ErrInvalidArgument, "avatar URL must be valid")
	}
	u.avatarURL = url
	return nil
}

// VerifyPassword delegates to the hashing service. The domain model does not
// import crypto libraries — the application layer passes the hash for comparison.
func (u *User) VerifyPassword(hash string) error {
	if u.passwordHash != hash {
		return errors.New("password mismatch")
	}
	return nil
}
