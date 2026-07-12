package identity

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hubvas/internal/domain/shared"
)

type UserID int64

type User struct {
	shared.AggregateRoot
	id                            UserID
	username, email, passwordHash string
	displayName, bio, website     string
	avatarURL, avatarKey          string
	avatarVersion                 int64
	accountRole, status           string
	createdAt, updatedAt          time.Time
}

func NewUser(id UserID, username, email, passwordHash string) (*User, error) {
	username, email = strings.TrimSpace(username), strings.TrimSpace(email)
	if username == "" || utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 50 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "username must be 3-50 characters")
	}
	if email == "" || !strings.Contains(email, "@") {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "email is invalid")
	}
	if passwordHash == "" {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "password hash must not be empty")
	}
	now := time.Now()
	u := &User{id: id, username: username, email: email, passwordHash: passwordHash, displayName: username, accountRole: "user", status: "active", createdAt: now, updatedAt: now}
	u.AddEvent(UserRegisteredEvent{BaseEvent: shared.NewBaseEvent("UserRegistered"), UserID: id, Username: username, Email: email})
	return u, nil
}

// ReconstituteUser is retained for test and adapter compatibility.
func ReconstituteUser(id UserID, username, email, passwordHash, avatarURL string, createdAt time.Time) *User {
	return ReconstituteUserProfile(id, username, email, passwordHash, username, "", "", avatarURL, "", 0, "user", "active", createdAt, createdAt)
}
func ReconstituteUserProfile(id UserID, username, email, passwordHash, displayName, bio, website, avatarURL, avatarKey string, avatarVersion int64, role, status string, createdAt, updatedAt time.Time) *User {
	if displayName == "" {
		displayName = username
	}
	return &User{id: id, username: username, email: email, passwordHash: passwordHash, displayName: displayName, bio: bio, website: website, avatarURL: avatarURL, avatarKey: avatarKey, avatarVersion: avatarVersion, accountRole: role, status: status, createdAt: createdAt, updatedAt: updatedAt}
}
func (u *User) SetID(id UserID) {
	if u.id == 0 {
		u.id = id
	}
}
func (u *User) ID() UserID           { return u.id }
func (u *User) Username() string     { return u.username }
func (u *User) Email() string        { return u.email }
func (u *User) PasswordHash() string { return u.passwordHash }
func (u *User) DisplayName() string  { return u.displayName }
func (u *User) Bio() string          { return u.bio }
func (u *User) Website() string      { return u.website }
func (u *User) AvatarURL() string    { return u.avatarURL }
func (u *User) AvatarKey() string    { return u.avatarKey }
func (u *User) AvatarVersion() int64 { return u.avatarVersion }
func (u *User) AccountRole() string  { return u.accountRole }
func (u *User) Status() string       { return u.status }
func (u *User) IsAdmin() bool        { return u.accountRole == "admin" }
func (u *User) IsActive() bool       { return u.status == "active" }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }
func (u *User) ChangePassword(v string) error {
	if v == "" {
		return shared.NewDomainError(shared.ErrInvalidArgument, "password hash must not be empty")
	}
	u.passwordHash = v
	u.updatedAt = time.Now()
	return nil
}
func (u *User) SetAvatarURL(v string) error {
	if v != "" && !strings.HasPrefix(v, "http") {
		return shared.NewDomainError(shared.ErrInvalidArgument, "avatar URL must be valid")
	}
	u.avatarURL = v
	u.updatedAt = time.Now()
	return nil
}
func (u *User) SetAvatarObject(key string, version int64, publicURL string) {
	u.avatarKey = key
	u.avatarVersion = version
	u.avatarURL = publicURL
	u.updatedAt = time.Now()
}
func (u *User) ClearAvatar() {
	u.avatarKey = ""
	u.avatarVersion = 0
	u.avatarURL = ""
	u.updatedAt = time.Now()
}
func (u *User) UpdateProfile(displayName, bio, website string) error {
	displayName, bio, website = strings.TrimSpace(displayName), strings.TrimSpace(bio), strings.TrimSpace(website)
	if n := utf8.RuneCountInString(displayName); n < 1 || n > 50 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "display name must be 1-50 characters")
	}
	if utf8.RuneCountInString(bio) > 500 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "bio must be at most 500 characters")
	}
	if len(website) > 2048 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "website is too long")
	}
	if website != "" {
		parsed, err := url.ParseRequestURI(website)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return shared.NewDomainError(shared.ErrInvalidArgument, "website must be an http or https URL")
		}
	}
	u.displayName, u.bio, u.website = displayName, bio, website
	u.updatedAt = time.Now()
	return nil
}
func (u *User) VerifyPassword(hash string) error {
	if u.passwordHash != hash {
		return errors.New("password mismatch")
	}
	return nil
}
