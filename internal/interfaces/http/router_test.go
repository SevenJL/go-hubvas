package http

import (
	"errors"
	"testing"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
)

type tokenValidatorStub struct{}

func (tokenValidatorStub) ValidateAccessToken(string) (identity.UserID, error) {
	return 0, errors.New("invalid token")
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
