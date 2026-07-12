package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"github.com/hubvas/internal/domain/identity"
)

type SessionMetadata struct{ UserAgent, IPAddress string }

type RefreshSession struct {
	ID, FamilyID string
	UserID       identity.UserID
	TokenHash    []byte
	ExpiresAt    time.Time
	Metadata     SessionMetadata
}

type RefreshSessionRepository interface {
	Create(ctx context.Context, session RefreshSession) error
	Rotate(ctx context.Context, currentHash []byte, replacement RefreshSession) (identity.UserID, int64, error)
	Revoke(ctx context.Context, tokenHash []byte) error
	RevokeAll(ctx context.Context, userID identity.UserID) error
	List(ctx context.Context, userID identity.UserID, currentHash []byte) ([]SessionDTO, error)
	RevokeByID(ctx context.Context, userID identity.UserID, sessionID string) error
}

func newRefreshCredential(userID identity.UserID, familyID string, ttl time.Duration, meta SessionMetadata) (RefreshSession, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return RefreshSession{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if familyID == "" {
		familyID = uuid.NewString()
	}
	return RefreshSession{ID: uuid.NewString(), FamilyID: familyID, UserID: userID, TokenHash: hashRefreshToken(token), ExpiresAt: time.Now().UTC().Add(ttl), Metadata: meta}, token, nil
}

func hashRefreshToken(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }
