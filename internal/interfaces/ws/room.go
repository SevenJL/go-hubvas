package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// Room wraps a domain Room and manages the serial processing goroutine,
// client registry, and outbound broadcast to all connected WebSocket clients.
//
// Architecture (as per the development doc §3.2):
//
//	Each Room has ONE goroutine that reads from an inbound channel,
//	processes the operation via the domain model, and broadcasts
//	the result to all connected clients' send channels.
//
//	This serial design eliminates the need for locks on canvas state.
type Room struct {
	domainRoom *collaboration.Room // domain aggregate
	domainMu   sync.Mutex

	// inbound is the serial processing queue. All client operations
	// are pushed here by readPumps and processed by the Room goroutine.
	inbound chan collaboration.Operation

	// clients is the set of connected WebSocket clients.
	clients   map[*Client]bool
	clientsMu sync.RWMutex

	// snapshot is a concurrency-safe copy used by persistence and initial replay.
	snapshotMu sync.RWMutex
	snapshot   []byte

	// snapshotRepo for periodic persistence.
	snapshotRepo collaboration.SnapshotRepository

	ctx    context.Context
	cancel context.CancelFunc
}

// NewRoom creates a new Room and starts its processing goroutine.
func NewRoom(id collaboration.RoomID, snapshotRepo collaboration.SnapshotRepository, initialSnapshot []byte) *Room {
	ctx, cancel := context.WithCancel(context.Background())

	r := &Room{
		domainRoom:   collaboration.NewRoom(id, cloneBytes(initialSnapshot)),
		snapshot:     cloneBytes(initialSnapshot),
		inbound:      make(chan collaboration.Operation, 1024),
		clients:      make(map[*Client]bool),
		snapshotRepo: snapshotRepo,
		ctx:          ctx,
		cancel:       cancel,
	}

	go r.processLoop()
	if snapshotRepo != nil {
		go startSnapshotLoop(ctx, r, snapshotRepo)
	}

	return r
}

// processLoop is the single goroutine that serializes all operations on this Room.
func (r *Room) processLoop() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case op := <-r.inbound:
			r.processOp(op)
		}
	}
}

// processOp validates, applies, and broadcasts a single operation.
func (r *Room) processOp(op collaboration.Operation) {
	r.domainMu.Lock()
	result, err := r.domainRoom.ProcessOp(op)
	r.domainMu.Unlock()
	if err != nil {
		log.Printf("[room %d] op error from user %d: %v", r.domainRoom.ID(), op.UserID, err)
		r.sendToClient(op.UserID, NewErrorMessage("invalid_op", err.Error()))
		return
	}

	if result == nil {
		return
	}
	if result.Operation.Type == collaboration.OpSync {
		r.setSnapshot(result.Operation.Payload)
	}

	// Sync operations received as binary frames (Yjs CRDT) → broadcast as binary.
	// Sync operations received as text frames (JSON snapshot) → broadcast as text.
	// We distinguish by checking if the payload looks like JSON (starts with '{').
	if result.Operation.Type == collaboration.OpSync && len(result.Operation.Payload) > 0 {
		if len(result.Operation.Payload) > 0 && result.Operation.Payload[0] == '{' {
			// JSON text sync — broadcast as text.
			msg := FromOperation(result.Operation)
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("[room %d] marshal error: %v", r.domainRoom.ID(), err)
				return
			}
			r.broadcastText(data, result)
		} else {
			// Binary CRDT sync — broadcast as binary.
			r.broadcastBinary(result.Operation.Payload, result)
		}
	} else {
		msg := FromOperation(result.Operation)
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("[room %d] marshal error: %v", r.domainRoom.ID(), err)
			return
		}
		r.broadcastText(data, result)
	}
}

// broadcastText sends a JSON text message to clients.
func (r *Room) broadcastText(data []byte, result *collaboration.BroadcastResult) {
	switch result.Target {
	case collaboration.BroadcastAll:
		r.broadcastAll(data, result.ExcludeUserID, false)
	case collaboration.BroadcastOthers:
		r.broadcastAll(data, result.ExcludeUserID, false)
	case collaboration.BroadcastSingle:
		if result.TargetUserID != nil {
			r.sendToClient(*result.TargetUserID, data)
		}
	}
}

// broadcastBinary sends a binary frame to clients (for CRDT sync updates).
func (r *Room) broadcastBinary(data []byte, result *collaboration.BroadcastResult) {
	switch result.Target {
	case collaboration.BroadcastAll:
		r.broadcastAll(data, result.ExcludeUserID, true)
	case collaboration.BroadcastOthers:
		r.broadcastAll(data, result.ExcludeUserID, true)
	case collaboration.BroadcastSingle:
		if result.TargetUserID != nil {
			r.sendBinaryToClient(*result.TargetUserID, data)
		}
	}
}

// broadcastAll sends data to all connected clients, optionally excluding one.
// If binary is true, data is sent as a binary WebSocket frame.
func (r *Room) broadcastAll(data []byte, exclude *identity.UserID, binary bool) {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()

	for client := range r.clients {
		if exclude != nil && client.UserID == *exclude {
			continue
		}
		if binary {
			client.SendBinary(data)
		} else {
			client.Send(data)
		}
	}
}

// sendBinaryToClient sends a binary frame to a specific user.
func (r *Room) sendBinaryToClient(userID identity.UserID, data []byte) {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()
	for client := range r.clients {
		if client.UserID == userID {
			client.SendBinary(data)
		}
	}
}

// sendToClient sends a text message to a specific user. If the user has
// multiple connections, all of them receive the message.
func (r *Room) sendToClient(userID identity.UserID, msg interface{}) {
	var data []byte
	switch v := msg.(type) {
	case []byte:
		data = v
	case Message:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return
		}
	case string:
		data = []byte(v)
	default:
		return
	}

	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()

	for client := range r.clients {
		if client.UserID == userID {
			client.Send(data)
		}
	}
}

// Register adds a client to the room and wires its onRead callback
// to the room's inbound channel. Broadcasts full presence after join.
func (r *Room) Register(client *Client) {
	client.onRead = func(op collaboration.Operation) {
		select {
		case r.inbound <- op:
		case <-r.ctx.Done():
		default:
			log.Printf("[room %d] inbound full, dropping op from user %d",
				r.domainRoom.ID(), op.UserID)
		}
	}

	r.clientsMu.Lock()
	r.clients[client] = true
	r.clientsMu.Unlock()

	r.domainMu.Lock()
	r.domainRoom.Join(client.UserID, client.Username)
	r.domainMu.Unlock()
	client.Start()
	r.sendInitialSnapshot(client)

	log.Printf("[room %d] user %d (%s) joined (total: %d)",
		r.domainRoom.ID(), client.UserID, client.Username, r.MemberCount())

	// Broadcast full presence list to everyone.
	r.broadcastPresence()
}

func (r *Room) sendInitialSnapshot(client *Client) {
	snapshot := r.Snapshot()
	if len(snapshot) == 0 {
		return
	}
	if json.Valid(snapshot) {
		data, err := json.Marshal(Message{Type: MsgTypeSync, Payload: snapshot})
		if err == nil {
			client.Send(data)
		}
		return
	}
	client.SendBinary(snapshot)
}

func (r *Room) setSnapshot(data []byte) {
	r.snapshotMu.Lock()
	r.snapshot = cloneBytes(data)
	r.snapshotMu.Unlock()
}

// Snapshot returns a safe copy of the latest persisted room state.
func (r *Room) Snapshot() []byte {
	r.snapshotMu.RLock()
	defer r.snapshotMu.RUnlock()
	return cloneBytes(r.snapshot)
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result
}

// Unregister removes a client from the room. Broadcasts full presence after leave.
func (r *Room) Unregister(client *Client) {
	r.clientsMu.Lock()
	delete(r.clients, client)
	r.clientsMu.Unlock()

	r.domainMu.Lock()
	r.domainRoom.Leave(client.UserID)
	r.domainMu.Unlock()

	log.Printf("[room %d] user %d left (total: %d)",
		r.domainRoom.ID(), client.UserID, r.MemberCount())

	// Broadcast updated presence list.
	r.broadcastPresence()
}

// broadcastPresence sends the full online member list to all connected clients.
func (r *Room) broadcastPresence() {
	members := r.buildPresenceList()
	payload, _ := json.Marshal(map[string]interface{}{"online": members})
	msg := Message{Type: MsgTypePresence, Payload: payload}
	data, _ := json.Marshal(msg)

	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()
	for client := range r.clients {
		client.Send(data)
	}
}

// buildPresenceList builds the full list of online PresenceMembers.
func (r *Room) buildPresenceList() []PresenceMember {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()

	members := make([]PresenceMember, 0, len(r.clients))
	for client := range r.clients {
		members = append(members, PresenceMember{
			UserID:    int64(client.UserID),
			Username:  client.Username,
			AvatarURL: "",
			Role:      client.Role,
		})
	}
	return members
}

// DomainRoom returns the underlying domain aggregate.
func (r *Room) DomainRoom() *collaboration.Room {
	return r.domainRoom
}

// MemberCount returns the number of connected clients.
func (r *Room) MemberCount() int {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()
	return len(r.clients)
}

// IsIdle returns true if the room has been inactive and has no members.
func (r *Room) IsIdle(timeout time.Duration) bool {
	if r.MemberCount() != 0 {
		return false
	}
	r.domainMu.Lock()
	defer r.domainMu.Unlock()
	return r.domainRoom.IsIdle(timeout)
}

// Shutdown stops the room's goroutine and flushes state.
func (r *Room) Shutdown() {
	r.cancel()
	// Final snapshot flush happens in startSnapshotLoop's cleanup.
}

// Inbound returns the inbound channel for testing/inspection.
func (r *Room) Inbound() chan<- collaboration.Operation {
	return r.inbound
}
