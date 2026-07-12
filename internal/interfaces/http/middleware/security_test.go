package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/domain/identity"
)

type accessValidatorStub struct{ access identity.AccessIdentity }

func (s accessValidatorStub) ValidateAccessToken(string) (identity.AccessIdentity, error) {
	return s.access, nil
}

type accountLookupStub struct{ user *identity.User }

func (s accountLookupStub) FindByID(context.Context, identity.UserID) (*identity.User, error) {
	return s.user, nil
}

func securityTestUser(version int64) *identity.User {
	now := time.Now()
	return identity.ReconstituteUserProfile(1, "tester", "t@example.com", "hash", "Tester", "", "", "", "", 0, version, "user", "active", now, now)
}

func TestMetricsAuthRequiresConfiguredBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsAuth("0123456789abcdef0123456789abcdef"), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	unauthorized := httptest.NewRecorder()
	r.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	r.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", authorized.Code)
	}

	lowercase := httptest.NewRecorder()
	lowercaseRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	lowercaseRequest.Header.Set("Authorization", "bearer 0123456789abcdef0123456789abcdef")
	r.ServeHTTP(lowercase, lowercaseRequest)
	if lowercase.Code != http.StatusNoContent {
		t.Fatalf("expected case-insensitive bearer scheme to be accepted, got %d", lowercase.Code)
	}

	malformed := httptest.NewRecorder()
	malformedRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	malformedRequest.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef extra")
	r.ServeHTTP(malformed, malformedRequest)
	if malformed.Code != http.StatusUnauthorized {
		t.Fatalf("expected malformed bearer header to return 401, got %d", malformed.Code)
	}
}

func TestActiveAccountRejectsRevokedSecurityVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(accessValidatorStub{access: identity.AccessIdentity{UserID: 1, SecurityVersion: 2}}))
	r.Use(ActiveAccountMiddleware(accountLookupStub{user: securityTestUser(3)}))
	r.GET("/private", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer token")
	r.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to return 401, got %d", response.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers missing: %#v", response.Header())
	}
}
