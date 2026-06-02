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
	Role     string // from the domain; set after auth

	conn    *websocket.Conn
	send    chan []byte       // buffered outbound channel
	hub     *Hub              // the room registry (for deregistration)
	onRead  func(collaboration.Operation) // callback to the Room's inbound

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewClient creates a Client and starts its pumps.
func NewClient(
	conn *websocket.Conn,
	userID identity.UserID,
	username string,
	roomID collaboration.RoomID,
	hub *Hub,
	onRead func(collaboration.Operation),
) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		UserID:   userID,
		Username: username,
		RoomID:   roomID,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		hub:      hub,
		onRead:   onRead,
		ctx:      ctx,
		cancel:   cancel,
	}

	c.wg.Add(2)
	go c.readPump()
	go c.writePump()

	return c
}

// readPump blocks on reading messages from the WebSocket connection and
// pushes them to the Room's inbound processing queue.
func (c *Client) readPump() {
	defer c.wg.Done()
	defer c.cancel()
	defer c.hub.Unregister(c) // Ensure cleanup on disconnect

	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				log.Printf("[ws] client %d left room %d cleanly", c.UserID, c.RoomID)
			} else {
				log.Printf("[ws] read error from user %d in room %d: %v", c.UserID, c.RoomID, err)
			}
			return
		}

		// Parse the JSON envelope.
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[ws] invalid message from user %d: %v", c.UserID, err)
			c.sendError("bad_request", "invalid message format")
			continue
		}

		op := ToOperation(msg, c.UserID)

		// Push to the Room's serial processing queue.
		// The onRead callback is wired by Room.Register to push to the room's inbound channel.
		if c.onRead != nil {
			c.onRead(op)
		}
	}
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
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				log.Printf("[ws] write error to user %d: %v", c.UserID, err)
				return
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

// Send enqueues a message for delivery. If the channel is full,
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
