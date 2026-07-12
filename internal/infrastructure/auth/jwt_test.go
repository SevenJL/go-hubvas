package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/hubvas/internal/domain/identity"
)

func TestAccessTokenUsesIssuerAudienceAndSubject(t *testing.T) {
	svc := NewJWTServiceWithClaims("0123456789abcdef0123456789abcdef", time.Minute, "issuer-a", "audience-a")
	token, _, err := svc.GenerateAccessToken(identity.UserID(42))
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Fatalf("got user %d", id)
	}
	other := NewJWTServiceWithClaims("0123456789abcdef0123456789abcdef", time.Minute, "issuer-b", "audience-a")
	if _, err = other.ValidateAccessToken(token); err == nil {
		t.Fatal("expected issuer mismatch")
	}
}

func TestAccessTokenRejectsMissingTypeHeader(t *testing.T) {
	svc := NewJWTServiceWithClaims("0123456789abcdef0123456789abcdef", time.Minute, "issuer-a", "audience-a")
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer: "issuer-a", Subject: "42", Audience: jwt.ClaimStrings{"audience-a"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)), IssuedAt: jwt.NewNumericDate(now),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ValidateAccessToken(value); err == nil {
		t.Fatal("expected token without access-token typ header to be rejected")
	}
}
