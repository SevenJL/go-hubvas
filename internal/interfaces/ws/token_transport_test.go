package ws

import (
	"net/http/httptest"
	"testing"
)

func TestAccessTokenSubprotocolSelectsStableProtocol(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "hubvas, hubvas.access.header.payload.signature")
	token, protocol := accessTokenFromRequest(req)
	if token != "header.payload.signature" {
		t.Fatalf("unexpected token %q", token)
	}
	if protocol != websocketProtocol {
		t.Fatalf("expected stable selected protocol %q, got %q", websocketProtocol, protocol)
	}
}

func TestAccessTokenIsNotAcceptedFromURL(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test/ws?token=leaked", nil)
	token, protocol := accessTokenFromRequest(req)
	if token != "" || protocol != "" {
		t.Fatalf("URL credentials must be ignored, got token=%q protocol=%q", token, protocol)
	}
}
