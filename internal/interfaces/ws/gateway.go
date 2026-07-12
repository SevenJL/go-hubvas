package ws

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// Gateway handles WebSocket upgrade requests and creates Client instances.
//
// Connection URL: wss://host/ws?canvas=<canvasID> (JWT via Sec-WebSocket-Protocol)
//
// Flow:
//  1. Upgrade HTTP to WebSocket
//  2. Validate JWT token → extract userID
//  3. Validate canvas access permission
//  4. Create Client and register with the Hub
type Gateway struct {
	hub           *Hub
	tokenSvc      TokenValidator
	permissionSvc collaboration.PermissionService
	userLookup    UserLookup
	throttleSvc   collaboration.ThrottleService
}

// TokenValidator is the minimal interface for JWT validation.
// It is satisfied by infrastructure/auth.JWTService.
type TokenValidator interface {
	ValidateAccessToken(tokenString string) (identity.UserID, error)
}

// UserLookup resolves the authenticated user's server-authoritative profile.
type UserLookup interface {
	FindByID(ctx context.Context, id identity.UserID) (*identity.User, error)
}

// NewGateway creates a WebSocket gateway.
func NewGateway(
	hub *Hub,
	tokenSvc TokenValidator,
	permissionSvc collaboration.PermissionService,
	userLookup UserLookup,
	throttleSvc collaboration.ThrottleService,
) *Gateway {
	return &Gateway{
		hub:           hub,
		tokenSvc:      tokenSvc,
		permissionSvc: permissionSvc,
		userLookup:    userLookup,
		throttleSvc:   throttleSvc,
	}
}

// ServeHTTP handles the WebSocket upgrade request.
// It implements http.Handler so it can be mounted directly on a route.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Extract canvas ID from query.
	canvasIDStr := r.URL.Query().Get("canvas")
	if canvasIDStr == "" {
		http.Error(w, `{"code":"missing_canvas","message":"canvas ID is required"}`, http.StatusBadRequest)
		return
	}
	canvasID, err := strconv.ParseInt(canvasIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"code":"invalid_canvas","message":"canvas ID must be a number"}`, http.StatusBadRequest)
		return
	}

	// 2. Extract and validate JWT.
	token, selectedProtocol := accessTokenFromRequest(r)
	if token == "" {
		http.Error(w, `{"code":"missing_token","message":"token is required"}`, http.StatusUnauthorized)
		return
	}

	userID, err := g.tokenSvc.ValidateAccessToken(token)
	if err != nil {
		http.Error(w, `{"code":"invalid_token","message":"invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	// 3. Validate canvas access.
	canView, err := g.permissionSvc.CanView(r.Context(), canvas.CanvasID(canvasID), userID)
	if err != nil || !canView {
		http.Error(w, `{"code":"forbidden","message":"access denied"}`, http.StatusForbidden)
		return
	}

	role, err := g.permissionSvc.GetRole(r.Context(), canvas.CanvasID(canvasID), userID)
	canEdit := false
	if err != nil {
		role = canvas.RoleViewer
	} else {
		canEdit = role.CanEdit()
	}
	// A published non-member has view access but no membership role, so remains a read-only viewer.

	user, err := g.userLookup.FindByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"code":"invalid_user","message":"authenticated user no longer exists"}`, http.StatusUnauthorized)
		return
	}
	if !user.IsActive() {
		http.Error(w, `{"code":"suspended","message":"account is suspended"}`, http.StatusForbidden)
		return
	}
	if g.throttleSvc != nil {
		allowed, throttleErr := g.throttleSvc.AllowConnection(r.Context(), userID, collaboration.RoomID(canvasID))
		if throttleErr != nil {
			http.Error(w, `{"code":"throttle_unavailable","message":"unable to validate connection rate"}`, http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			http.Error(w, `{"code":"rate_limited","message":"too many connection attempts"}`, http.StatusTooManyRequests)
			return
		}
	}

	// 4. Upgrade to WebSocket.
	options := &websocket.AcceptOptions{InsecureSkipVerify: false}
	if selectedProtocol != "" {
		options.Subprotocols = []string{selectedProtocol}
	}
	conn, err := websocket.Accept(w, r, options)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}

	// 5. Create a client with server-authoritative identity and permissions.
	client := NewClient(
		conn,
		userID,
		user.Username(),
		collaboration.RoomID(canvasID),
		role,
		user.AvatarURL(),
		canEdit,
		g.hub,
		g.throttleSvc,
		nil, // onRead will be wired by Room.Register before pumps start
	)

	// 6. Register with the Hub (which routes to the correct Room).
	g.hub.RegisterClient(client)

	log.Printf("[ws] connection established: user=%d canvas=%d", userID, canvasID)
}
