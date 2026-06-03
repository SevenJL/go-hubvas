package identity

import (
	"testing"
)

func TestNewUser(t *testing.T) {
	u, err := NewUser(1, "alice", "alice@example.com", "hashed_password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID() != 1 {
		t.Fatalf("expected id 1, got %d", u.ID())
	}
	if u.Username() != "alice" {
		t.Fatalf("expected username 'alice', got '%s'", u.Username())
	}
	if u.Email() != "alice@example.com" {
		t.Fatalf("expected email 'alice@example.com', got '%s'", u.Email())
	}
	if u.PasswordHash() != "hashed_password" {
		t.Fatalf("password hash mismatch")
	}
	if !u.HasEvents() {
		t.Fatal("expected domain events after registration")
	}
}

func TestNewUser_WhitespaceTrim(t *testing.T) {
	u, _ := NewUser(1, "  bob  ", "  bob@test.com  ", "hash")
	if u.Username() != "bob" {
		t.Fatalf("expected trimmed username 'bob', got '%s'", u.Username())
	}
	if u.Email() != "bob@test.com" {
		t.Fatalf("expected trimmed email 'bob@test.com', got '%s'", u.Email())
	}
}

func TestNewUser_UsernameTooShort(t *testing.T) {
	_, err := NewUser(1, "ab", "ab@test.com", "hash")
	if err == nil {
		t.Fatal("expected error for too-short username")
	}
}

func TestNewUser_UsernameTooLong(t *testing.T) {
	long := make([]byte, 51)
	for i := range long {
		long[i] = 'a'
	}
	_, err := NewUser(1, string(long), "test@test.com", "hash")
	if err == nil {
		t.Fatal("expected error for too-long username")
	}
}

func TestNewUser_UsernameBoundary(t *testing.T) {
	// Exactly 3 chars — should work.
	_, err := NewUser(1, "abc", "abc@test.com", "hash")
	if err != nil {
		t.Fatalf("unexpected error for 3-char username: %v", err)
	}

	// Exactly 50 chars — should work.
	fifty := make([]byte, 50)
	for i := range fifty {
		fifty[i] = 'a'
	}
	_, err = NewUser(1, string(fifty), "test@test.com", "hash")
	if err != nil {
		t.Fatalf("unexpected error for 50-char username: %v", err)
	}
}

func TestNewUser_InvalidEmail(t *testing.T) {
	_, err := NewUser(1, "alice", "not-an-email", "hash")
	if err == nil {
		t.Fatal("expected error for invalid email")
	}

	_, err = NewUser(1, "alice", "", "hash")
	if err == nil {
		t.Fatal("expected error for empty email")
	}
}

func TestNewUser_EmptyPasswordHash(t *testing.T) {
	_, err := NewUser(1, "alice", "alice@test.com", "")
	if err == nil {
		t.Fatal("expected error for empty password hash")
	}
}

func TestUser_ChangePassword(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "old_hash")
	err := u.ChangePassword("new_hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PasswordHash() != "new_hash" {
		t.Fatal("expected password hash to be updated")
	}
}

func TestUser_ChangePassword_Empty(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "hash")
	err := u.ChangePassword("")
	if err == nil {
		t.Fatal("expected error for empty new password hash")
	}
}

func TestUser_SetAvatarURL(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "hash")

	err := u.SetAvatarURL("https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.AvatarURL() != "https://example.com/avatar.png" {
		t.Fatalf("avatar URL mismatch")
	}
}

func TestUser_SetAvatarURL_Invalid(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "hash")

	err := u.SetAvatarURL("ftp://bad.com/pic.png")
	if err == nil {
		t.Fatal("expected error for non-http avatar URL")
	}
}

func TestUser_SetAvatarURL_Empty(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "hash")

	// Setting to empty should be allowed (removes avatar).
	err := u.SetAvatarURL("")
	if err != nil {
		t.Fatalf("unexpected error for empty avatar URL: %v", err)
	}
	if u.AvatarURL() != "" {
		t.Fatal("expected empty avatar URL")
	}
}

func TestUser_VerifyPassword(t *testing.T) {
	u, _ := NewUser(1, "alice", "alice@test.com", "hashed_password")

	err := u.VerifyPassword("hashed_password")
	if err != nil {
		t.Fatal("expected password to match")
	}

	err = u.VerifyPassword("wrong_password")
	if err == nil {
		t.Fatal("expected password mismatch error")
	}
}

func TestUser_SetID_Idempotent(t *testing.T) {
	u, _ := NewUser(0, "alice", "alice@test.com", "hash")
	if u.ID() != 0 {
		t.Fatal("expected initial id to be 0")
	}

	u.SetID(42)
	if u.ID() != 42 {
		t.Fatalf("expected id 42, got %d", u.ID())
	}

	// Setting again should be a no-op.
	u.SetID(99)
	if u.ID() != 42 {
		t.Fatalf("expected id still 42 (idempotent), got %d", u.ID())
	}
}
