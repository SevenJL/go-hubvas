package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type JWTService struct {
	accessSecret []byte
	accessTTL    time.Duration
	issuer       string
	audience     string
}

// NewJWTService remains source-compatible with existing callers. Refresh tokens
// are now opaque, server-side sessions and the refresh secret is intentionally unused.
func NewJWTService(accessSecret, _ string, accessTTL, _ time.Duration) *JWTService {
	return NewJWTServiceWithClaims(accessSecret, accessTTL, "hubvas", "hubvas-web")
}

func NewJWTServiceWithClaims(accessSecret string, accessTTL time.Duration, issuer, audience string) *JWTService {
	if accessSecret == "" {
		accessSecret = randomHex(32)
	}
	return &JWTService{accessSecret: []byte(accessSecret), accessTTL: accessTTL, issuer: issuer, audience: audience}
}

type accessClaims struct {
	SecurityVersion int64 `json:"sv"`
	jwt.RegisteredClaims
}

func (s *JWTService) GenerateAccessToken(userID identity.UserID, securityVersion int64) (string, int64, error) {
	if userID <= 0 || securityVersion <= 0 {
		return "", 0, shared.NewDomainError(shared.ErrInvalidArgument, "invalid access token identity")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.accessTTL)
	claims := accessClaims{SecurityVersion: securityVersion, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: s.issuer, Subject: strconv.FormatInt(int64(userID), 10), Audience: jwt.ClaimStrings{s.audience},
		ExpiresAt: jwt.NewNumericDate(expiresAt), NotBefore: jwt.NewNumericDate(now), IssuedAt: jwt.NewNumericDate(now), ID: randomHex(16),
	}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["typ"] = "at+jwt"
	value, err := token.SignedString(s.accessSecret)
	return value, expiresAt.Unix(), err
}

func (s *JWTService) ValidateAccessToken(value string) (identity.AccessIdentity, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(value, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, shared.NewDomainError(shared.ErrUnauthorized, "unexpected signing method")
		}
		return s.accessSecret, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithAudience(s.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return identity.AccessIdentity{}, shared.NewDomainError(shared.ErrUnauthorized, "invalid or expired token")
	}
	if typ, _ := token.Header["typ"].(string); typ != "at+jwt" {
		return identity.AccessIdentity{}, shared.NewDomainError(shared.ErrUnauthorized, "invalid token type")
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return identity.AccessIdentity{}, shared.NewDomainError(shared.ErrUnauthorized, "invalid token subject")
	}
	if claims.SecurityVersion <= 0 {
		return identity.AccessIdentity{}, shared.NewDomainError(shared.ErrUnauthorized, "invalid token security version")
	}
	return identity.AccessIdentity{UserID: identity.UserID(id), SecurityVersion: claims.SecurityVersion}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
