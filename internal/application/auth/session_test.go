package auth

import (
	"bytes"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/identity"
)

func TestRefreshCredentialsAreRandomAndHashed(t *testing.T) {
	a, rawA, err := newRefreshCredential(identity.UserID(1), "", time.Hour, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	b, rawB, err := newRefreshCredential(identity.UserID(1), "", time.Hour, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if rawA == rawB || bytes.Equal(a.TokenHash, b.TokenHash) {
		t.Fatal("refresh credentials must be unique")
	}
	if bytes.Equal(a.TokenHash, []byte(rawA)) {
		t.Fatal("repository must receive only a hash")
	}
	if !bytes.Equal(a.TokenHash, hashRefreshToken(rawA)) {
		t.Fatal("stored hash does not match credential")
	}
}
