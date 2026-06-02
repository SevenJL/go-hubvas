package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// JWTService implements application/auth.TokenService using HS256 JWTs.
type JWTService struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

// NewJWTService creates a JWTService with the given secrets and TTLs.
// If secrets are empty, random ones are generated (for development only).
func NewJWTService(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *JWTService {
	if accessSecret == "" {
		accessSecret = randomHex(32)
	}
	if refreshSecret == "" {
		refreshSecret = randomHex(32)
	}
	return &JWTService{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// GenerateAccessToken creates a short-lived access token.
func (s *JWTService) GenerateAccessToken(userID identity.UserID) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(s.accessSecret)
	if err != nil {
		return "", 0, err
	}
	return tokenStr, expiresAt.Unix(), nil
}

// GenerateRefreshToken creates a long-lived refresh token.
func (s *JWTService) GenerateRefreshToken(userID identity.UserID) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.refreshTTL)

	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.refreshSecret)
}

// ValidateAccessToken verifies and extracts the user ID from an access token.
func (s *JWTService) ValidateAccessToken(tokenString string) (identity.UserID, error) {
	return s.validateToken(tokenString, s.accessSecret)
}

// ValidateRefreshToken verifies and extracts the user ID from a refresh token.
func (s *JWTService) ValidateRefreshToken(tokenString string) (identity.UserID, error) {
	return s.validateToken(tokenString, s.refreshSecret)
}

func (s *JWTService) validateToken(tokenString string, secret []byte) (identity.UserID, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, shared.NewDomainError(shared.ErrUnauthorized, "unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return 0, shared.NewDomainError(shared.ErrUnauthorized, "invalid token: "+err.Error())
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, shared.NewDomainError(shared.ErrUnauthorized, "invalid token claims")
	}

	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, shared.NewDomainError(shared.ErrUnauthorized, "missing subject claim")
	}

	return identity.UserID(int64(sub)), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
