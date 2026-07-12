package ws

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/hubvas/internal/domain/identity"
	natsgo "github.com/nats-io/nats.go"
)

type notificationClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NotificationGateway exposes user-scoped notification WebSockets. REST remains
// the source of truth; this channel only lowers delivery latency.
type NotificationGateway struct {
	tokenSvc TokenValidator
	users    UserLookup
	mu       sync.RWMutex
	clients  map[identity.UserID]map[*notificationClient]struct{}
	sub      *natsgo.Subscription
}

func NewNotificationGateway(tokenSvc TokenValidator, users UserLookup, nc *natsgo.Conn) *NotificationGateway {
	g := &NotificationGateway{tokenSvc: tokenSvc, users: users, clients: make(map[identity.UserID]map[*notificationClient]struct{})}
	if nc != nil {
		sub, err := nc.Subscribe("notifications.user.*", func(msg *natsgo.Msg) {
			parts := strings.Split(msg.Subject, ".")
			if len(parts) != 3 {
				return
			}
			id, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return
			}
			g.broadcast(identity.UserID(id), msg.Data)
		})
		if err != nil {
			log.Printf("[notifications-ws] subscribe failed: %v", err)
		} else {
			g.sub = sub
		}
	}
	return g
}

func (g *NotificationGateway) Close() {
	if g.sub != nil {
		_ = g.sub.Unsubscribe()
	}
}

func (g *NotificationGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, selectedProtocol := accessTokenFromRequest(r)
	if token == "" {
		http.Error(w, `{"code":"missing_token","message":"token is required"}`, http.StatusUnauthorized)
		return
	}
	access, err := g.tokenSvc.ValidateAccessToken(token)
	if err != nil {
		http.Error(w, `{"code":"invalid_token","message":"invalid or expired token"}`, http.StatusUnauthorized)
		return
	}
	userID := access.UserID
	user, err := g.users.FindByID(r.Context(), userID)
	if err != nil || user == nil {
		http.Error(w, `{"code":"invalid_user","message":"user not found"}`, http.StatusUnauthorized)
		return
	}
	if !user.IsActive() {
		http.Error(w, `{"code":"suspended","message":"account is suspended"}`, http.StatusForbidden)
		return
	}
	if access.SecurityVersion != user.SecurityVersion() {
		http.Error(w, `{"code":"revoked_token","message":"access token has been revoked"}`, http.StatusUnauthorized)
		return
	}
	options := &websocket.AcceptOptions{InsecureSkipVerify: false}
	if selectedProtocol != "" {
		options.Subprotocols = []string{selectedProtocol}
	}
	conn, err := websocket.Accept(w, r, options)
	if err != nil {
		return
	}
	conn.SetReadLimit(1024)
	client := &notificationClient{conn: conn}
	g.add(userID, client)
	heartbeatCtx, cancelHeartbeat := context.WithCancel(r.Context())
	defer cancelHeartbeat()
	go g.heartbeat(heartbeatCtx, client)
	defer func() { g.remove(userID, client); conn.Close(websocket.StatusNormalClosure, "") }()
	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}

func (g *NotificationGateway) heartbeat(ctx context.Context, client *notificationClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			client.mu.Lock()
			err := client.conn.Ping(pingCtx)
			client.mu.Unlock()
			cancel()
			if err != nil {
				_ = client.conn.Close(websocket.StatusGoingAway, "heartbeat failed")
				return
			}
		}
	}
}

func (g *NotificationGateway) add(user identity.UserID, c *notificationClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.clients[user] == nil {
		g.clients[user] = make(map[*notificationClient]struct{})
	}
	g.clients[user][c] = struct{}{}
}
func (g *NotificationGateway) remove(user identity.UserID, c *notificationClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.clients[user], c)
	if len(g.clients[user]) == 0 {
		delete(g.clients, user)
	}
}
func (g *NotificationGateway) broadcast(user identity.UserID, payload []byte) {
	g.mu.RLock()
	clients := make([]*notificationClient, 0, len(g.clients[user]))
	for c := range g.clients[user] {
		clients = append(clients, c)
	}
	g.mu.RUnlock()
	for _, c := range clients {
		c.mu.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.conn.Write(ctx, websocket.MessageText, payload)
		cancel()
		c.mu.Unlock()
		if err != nil {
			_ = c.conn.Close(websocket.StatusGoingAway, "write failed")
		}
	}
}
