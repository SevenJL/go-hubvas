package auth

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type authUserRepoStub struct {
	user      *identity.User
	saveCalls int
}

func (r *authUserRepoStub) Save(context.Context, *identity.User) error { r.saveCalls++; return nil }
func (r *authUserRepoStub) FindByID(context.Context, identity.UserID) (*identity.User, error) {
	return r.user, nil
}
func (r *authUserRepoStub) FindByUsername(context.Context, string) (*identity.User, error) {
	return r.user, nil
}
func (r *authUserRepoStub) FindByEmail(context.Context, string) (*identity.User, error) {
	return r.user, nil
}
func (*authUserRepoStub) ExistsByUsername(context.Context, string) (bool, error) { return false, nil }
func (*authUserRepoStub) ExistsByEmail(context.Context, string) (bool, error)    { return false, nil }
func (*authUserRepoStub) Delete(context.Context, identity.UserID) error          { return nil }

type authSessionRepoStub struct {
	items          []SessionDTO
	revokedUser    identity.UserID
	revokedSession string
	rotateVersion  int64
}

func (*authSessionRepoStub) Create(context.Context, RefreshSession) error { return nil }
func (r *authSessionRepoStub) Rotate(context.Context, []byte, RefreshSession) (identity.UserID, int64, error) {
	version := r.rotateVersion
	if version <= 0 {
		version = 1
	}
	return 7, version, nil
}
func (*authSessionRepoStub) Revoke(context.Context, []byte) error { return nil }
func (r *authSessionRepoStub) RevokeAll(_ context.Context, id identity.UserID) error {
	r.revokedUser = id
	return nil
}
func (r *authSessionRepoStub) List(_ context.Context, _ identity.UserID, current []byte) ([]SessionDTO, error) {
	out := append([]SessionDTO(nil), r.items...)
	if len(out) > 0 {
		out[0].Current = bytes.Equal(current, hashRefreshToken("current"))
	}
	return out, nil
}
func (r *authSessionRepoStub) RevokeByID(_ context.Context, _ identity.UserID, id string) error {
	r.revokedSession = id
	return nil
}

type accountUOWStub struct {
	registerErr      error
	registered       bool
	passwordChanged  bool
	allAccessRevoked bool
}

func (a *accountUOWStub) Register(_ context.Context, user *identity.User, _ RefreshSession) error {
	if a.registerErr != nil {
		return a.registerErr
	}
	user.SetID(42)
	a.registered = true
	return nil
}
func (a *accountUOWStub) ChangePasswordAndRevokeSessions(context.Context, *identity.User) error {
	a.passwordChanged = true
	return nil
}
func (a *accountUOWStub) RevokeAllAccess(context.Context, identity.UserID) error {
	a.allAccessRevoked = true
	return nil
}

type passwordStub struct{}

func (passwordStub) Hash(value string) (string, error) { return "hash:" + value, nil }
func (passwordStub) Verify(value, hash string) bool    { return hash == "hash:"+value }

type tokenStub struct{}

func (tokenStub) GenerateAccessToken(id identity.UserID, _ int64) (string, int64, error) {
	return "token", int64(id), nil
}

func testUser() *identity.User {
	return identity.ReconstituteUserProfile(7, "tester", "test@example.com", "hash:old-password", "Tester", "", "", "", "", 0, 1, "user", "active", time.Now(), time.Now())
}

func TestRegisterUsesAtomicAccountUnit(t *testing.T) {
	users := &authUserRepoStub{}
	accounts := &accountUOWStub{}
	service := NewAuthApplicationService(users, &authSessionRepoStub{}, accounts, tokenStub{}, passwordStub{}, time.Hour)
	result, err := service.Register(context.Background(), RegisterRequest{Username: "new-user", Email: "new@example.com", Password: "password123"}, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if !accounts.registered || users.saveCalls != 0 {
		t.Fatalf("registration must only use account transaction; registered=%v saveCalls=%d", accounts.registered, users.saveCalls)
	}
	if result.User.ID != 42 {
		t.Fatalf("expected database ID in response, got %d", result.User.ID)
	}
}

func TestRegisterFailureDoesNotFallBackToNonTransactionalSave(t *testing.T) {
	users := &authUserRepoStub{}
	accounts := &accountUOWStub{registerErr: errors.New("session insert failed")}
	service := NewAuthApplicationService(users, &authSessionRepoStub{}, accounts, tokenStub{}, passwordStub{}, time.Hour)
	_, err := service.Register(context.Background(), RegisterRequest{Username: "new-user", Email: "new@example.com", Password: "password123"}, SessionMetadata{})
	if err == nil {
		t.Fatal("expected transaction failure")
	}
	if users.saveCalls != 0 {
		t.Fatal("user must not be saved outside the failed transaction")
	}
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	accounts := &accountUOWStub{}
	service := NewAuthApplicationService(&authUserRepoStub{user: testUser()}, &authSessionRepoStub{}, accounts, tokenStub{}, passwordStub{}, time.Hour)
	err := service.ChangePassword(context.Background(), 7, ChangePasswordRequest{CurrentPassword: "wrong", NewPassword: "new-password"})
	if !errors.Is(err, shared.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if accounts.passwordChanged {
		t.Fatal("password transaction must not run")
	}
}

func TestChangePasswordUsesAtomicUpdateAndSessionRevocation(t *testing.T) {
	accounts := &accountUOWStub{}
	user := testUser()
	service := NewAuthApplicationService(&authUserRepoStub{user: user}, &authSessionRepoStub{}, accounts, tokenStub{}, passwordStub{}, time.Hour)
	if err := service.ChangePassword(context.Background(), 7, ChangePasswordRequest{CurrentPassword: "old-password", NewPassword: "new-password"}); err != nil {
		t.Fatal(err)
	}
	if !accounts.passwordChanged {
		t.Fatal("expected transactional password update")
	}
	if user.PasswordHash() != "hash:new-password" {
		t.Fatalf("unexpected password hash %q", user.PasswordHash())
	}
	if user.SecurityVersion() != 2 {
		t.Fatalf("expected password change to rotate security version, got %d", user.SecurityVersion())
	}
}

func TestSessionsMarksCurrentCredentialAndRevokesByID(t *testing.T) {
	sessions := &authSessionRepoStub{items: []SessionDTO{{ID: "session-a"}}}
	service := NewAuthApplicationService(&authUserRepoStub{}, sessions, &accountUOWStub{}, tokenStub{}, passwordStub{}, time.Hour)
	items, err := service.Sessions(context.Background(), 7, "current")
	if err != nil || len(items) != 1 || !items[0].Current {
		t.Fatalf("current session not marked: %#v err=%v", items, err)
	}
	if err = service.RevokeSession(context.Background(), 7, "session-a"); err != nil {
		t.Fatal(err)
	}
	if sessions.revokedSession != "session-a" {
		t.Fatal("session ID was not delegated")
	}
}

func TestLogoutAllRevokesRefreshAndAccessCredentials(t *testing.T) {
	accounts := &accountUOWStub{}
	service := NewAuthApplicationService(&authUserRepoStub{}, &authSessionRepoStub{}, accounts, tokenStub{}, passwordStub{}, time.Hour)
	if err := service.LogoutAll(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if !accounts.allAccessRevoked {
		t.Fatal("expected transactional access revocation")
	}
}

func TestRefreshRejectsCredentialVersionChangedDuringRotation(t *testing.T) {
	now := time.Now()
	users := &authUserRepoStub{user: identity.ReconstituteUserProfile(7, "tester", "test@example.com", "hash", "Tester", "", "", "", "", 0, 2, "user", "active", now, now)}
	sessions := &authSessionRepoStub{rotateVersion: 1}
	service := NewAuthApplicationService(users, sessions, &accountUOWStub{}, tokenStub{}, passwordStub{}, time.Hour)

	if _, err := service.Refresh(context.Background(), "refresh-token", SessionMetadata{}); err == nil {
		t.Fatal("expected refresh to fail after account security version changed")
	}
	if sessions.revokedUser != 7 {
		t.Fatalf("expected replacement session to be revoked, got user %d", sessions.revokedUser)
	}
}
