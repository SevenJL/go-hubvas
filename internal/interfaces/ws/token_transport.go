package ws

import (
	"net/http"
	"strings"
)

const (
	websocketProtocol    = "hubvas"
	accessProtocolPrefix = "hubvas.access."
)

func accessTokenFromRequest(r *http.Request) (token, protocol string) {
	for _, candidate := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		candidate = strings.TrimSpace(candidate)
		if strings.HasPrefix(candidate, accessProtocolPrefix) {
			return strings.TrimPrefix(candidate, accessProtocolPrefix), websocketProtocol
		}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer "), ""
	}
	return "", ""
}
