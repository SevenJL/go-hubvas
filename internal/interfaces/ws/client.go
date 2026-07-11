package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

const (
	// writeWait is the maximum time to wait for a write to complete.
	writeWait = 10 * time.Second

	// pongWait is the maximum time to wait for a pong response.
	pongWait = 60 * time.Second

	// pingPeriod is the interval at which pings are sent.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum allowed size of an incoming message.
	maxMessageSize = 512 * 1024 // 512 KB

	// sendBufferSize is the capacity of each client's outbound channel.
	sendBufferSize = 256
)

// Client wraps a WebSocket connection and manages its read and write pumps.
// Each Client goroutines:
//   - readPump:  reads from WebSocket → pushes to Room.inbound
//   - writePump: reads from Client.send channel → writes to WebSocket
type Client struct {
	UserID   identity.UserID
	Username string
	RoomID   collaboration.RoomID
	Role     string
	CanEdit  bool

	conn   *websocket.Conn
	send   chan []byte                   // buffered outbound channel
	hub    *Hub                          // the room registry (for deregistration)
	onRead func(collaboration.Operation) // callback to the Room's inbound

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startOnce sync.Once
}

// NewClient creates a Client. Start must be called after room registration.
func NewClient(
	conn *websocket.Conn,
	userID identity.UserID,
	username string,
	roomID collaboration.RoomID,
	role string,
	canEdit bool,
	hub *Hub,
	onRead func(collaboration.Operation),
) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		UserID:   userID,
		Username: username,
		RoomID:   roomID,
		Role:     role,
		CanEdit:  canEdit,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		hub:      hub,
		onRead:   onRead,
		ctx:      ctx,
		cancel:   cancel,
	}

	return c
}

// Start launches the client pumps once registration has wired the room callback.
func (c *Client) Start() {
	c.startOnce.Do(func() {
		c.wg.Add(2)
		go c.readPump()
		go c.writePump()
	})
}

// readPump blocks on reading messages from the WebSocket connection and
// pushes them to the Room's inbound processing queue.
//
// Supports two message formats:
//   - TEXT frame (JSON envelope): used for awareness, presence, chat, ack.
//   - BINARY frame (raw bytes): used for Yjs CRDT sync updates (efficient).
func (c *Client) readPump() {
	defer c.wg.Done()
	defer c.cancel()
	defer c.hub.Unregister(c) // Ensure cleanup on disconnect

	c.conn.SetReadLimit(maxMessageSize)

	for {
		msgType, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				log.Printf("[ws] client %d left room %d cleanly", c.UserID, c.RoomID)
			} else {
				log.Printf("[ws] read error from user %d in room %d: %v", c.UserID, c.RoomID, err)
			}
			return
		}

		switch msgType {
		case websocket.MessageBinary:
			if !canSubmitOperation(c.CanEdit, MsgTypeSync) {
				c.sendError("forbidden", "read-only clients cannot modify the canvas")
				continue
			}
			// Binary frame — raw Yjs CRDT update. Pass directly as a sync operation.
			op := collaboration.Operation{
				Type:    collaboration.OpSync,
				UserID:  c.UserID,
				Payload: data,
			}
			if c.onRead != nil {
				c.onRead(op)
			}

		case websocket.MessageText:
			// Text frame — JSON protocol envelope for all other message types.
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("[ws] invalid message from user %d: %v", c.UserID, err)
				c.sendError("bad_request", "invalid message format")
				continue
			}

			if msg.Type == MsgTypePresence {
				// Presence is generated from the server-side client registry.
				continue
			}
			if !canSubmitOperation(c.CanEdit, msg.Type) {
				c.sendError("forbidden", "read-only clients cannot modify the canvas")
				continue
			}
			if msg.Type == MsgTypeAwareness || msg.Type == MsgTypeChat {
				payload, err := authoritativePayload(msg.Payload, c.UserID, c.Username)
				if err != nil {
					c.sendError("bad_request", "invalid message payload")
					continue
				}
				msg.Payload = payload
			}

			op := ToOperation(msg, c.UserID)
			if c.onRead != nil {
				c.onRead(op)
			}

		default:
			log.Printf("[ws] unsupported message type %d from user %d", msgType, c.UserID)
		}
	}
}

func canSubmitOperation(canEdit bool, messageType string) bool {
	return messageType != MsgTypeSync || canEdit
}

func authoritativePayload(payload json.RawMessage, userID identity.UserID, username string) (json.RawMessage, error) {
	values := make(map[string]interface{})
	if len(payload) > 0 && string(payload) != "null" {
		if err := json.Unmarshal(payload, &values); err != nil {
			return nil, err
		}
	}
	values["user_id"] = int64(userID)
	values["username"] = username
	return json.Marshal(values)
}

// writePump drains the Client's send channel and writes to the WebSocket.
// It also handles ping/pong keep-alive.
func (c *Client) writePump() {
	defer c.wg.Done()
	defer c.conn.Close(websocket.StatusNormalClosure, "")

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case msg, ok := <-c.send:
			if !ok {
				// Channel closed; the client is being kicked.
				c.conn.Write(c.ctx, websocket.MessageText, []byte(`{"type":"close"}`))
				return
			}

			writeCtx, cancel := context.WithTimeout(c.ctx, writeWait)

			// Binary frame marker: msg[0] == 0 → send as binary, rest is payload.
			if len(msg) > 0 && msg[0] == 0 {
				err := c.conn.Write(writeCtx, websocket.MessageBinary, msg[1:])
				cancel()
				if err != nil {
					log.Printf("[ws] write error to user %d: %v", c.UserID, err)
					return
				}
			} else {
				err := c.conn.Write(writeCtx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					log.Printf("[ws] write error to user %d: %v", c.UserID, err)
					return
				}
			}

		case <-ticker.C:
			writeCtx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.conn.Ping(writeCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Send enqueues a text message for delivery. If the channel is full,
// the client is considered slow and gets kicked.
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		// Slow client — kick them. They'll auto-reconnect and catch up.
		log.Printf("[ws] kicking slow client user %d in room %d", c.UserID, c.RoomID)
		c.Close()
	}
}

// SendBinary enqueues a binary frame for delivery (used for CRDT sync updates).
func (c *Client) SendBinary(data []byte) {
	// Prefix with a zero byte to signal binary frame to the writePump.
	// (writePump checks if msg[0]==0 to decide text vs binary)
	frame := make([]byte, len(data)+1)
	frame[0] = 0 // binary marker
	copy(frame[1:], data)
	c.Send(frame)
}

// Close terminates the client connection.
func (c *Client) Close() {
	c.cancel()
	c.conn.Close(websocket.StatusNormalClosure, "closing")
}

// Wait blocks until both pumps have exited.
func (c *Client) Wait() {
	c.wg.Wait()
}

// sendError queues an error message to the client.
func (c *Client) sendError(code, message string) {
	msg := NewErrorMessage(code, message)
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	c.Send(data)
}
