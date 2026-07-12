package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
)

type tokenValidatorStub struct{}

func (tokenValidatorStub) ValidateAccessToken(string) (identity.AccessIdentity, error) {
	return identity.AccessIdentity{}, errors.New("invalid token")
}

func TestNewRouterDoesNotRegisterWebSocketWhenGatewayIsNil(t *testing.T) {
	router := NewRouter(RouterConfig{
		AuthHandler:      &handler.AuthHandler{},
		CanvasHandler:    &handler.CanvasHandler{},
		CommunityHandler: &handler.CommunityHandler{},
		HealthHandler:    &handler.HealthHandler{},
		SnapshotHandler:  &handler.SnapshotHandler{},
		TokenSvc:         tokenValidatorStub{},
		RateLimiter:      middleware.NewRateLimiter(100, 100),
	})

	for _, route := range router.Routes() {
		if route.Path == "/ws" {
			t.Fatal("/ws must not be registered when WSGateway is nil")
		}
	}
}

func TestSharedCanvasRouteIsNotCapturedByCanvasIDWildcard(t *testing.T) {
	router := NewRouter(RouterConfig{
		AuthHandler:      &handler.AuthHandler{},
		CanvasHandler:    &handler.CanvasHandler{},
		CommunityHandler: &handler.CommunityHandler{},
		HealthHandler:    &handler.HealthHandler{},
		SnapshotHandler:  &handler.SnapshotHandler{},
		TokenSvc:         tokenValidatorStub{},
		RateLimiter:      middleware.NewRateLimiter(100, 100),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/canvases/shared", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected protected shared route to return 401, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
