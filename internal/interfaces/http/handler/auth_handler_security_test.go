package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/pkg/config"
)

func TestTrustedCookieOrigin(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		forwarded string
		want      bool
	}{
		{name: "non browser client", want: true},
		{name: "same origin", origin: "https://hubvas.example", forwarded: "https", want: true},
		{name: "cross origin", origin: "https://evil.example", forwarded: "https", want: false},
		{name: "scheme downgrade", origin: "http://hubvas.example", forwarded: "https", want: false},
		{name: "malformed", origin: "://bad", forwarded: "https", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://hubvas.example/api/auth/refresh", nil)
			req.Host = "hubvas.example"
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}
			if got := trustedCookieOrigin(req); got != tt.want {
				t.Fatalf("trustedCookieOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshCookieSecurityAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	handler := NewAuthHandler(nil, config.AuthConfig{
		CookieName: "hubvas_refresh", CookieSecure: true, CookieSameSite: "strict", RefreshTokenTTL: time.Hour,
	})
	handler.setRefreshCookie(ctx, "opaque-token")
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/api/auth" || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("insecure refresh cookie attributes: %#v", cookie)
	}
}
