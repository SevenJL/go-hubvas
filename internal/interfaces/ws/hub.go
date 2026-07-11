package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
)

const (
	// idleTimeout is how long a room can be empty before being unloaded.
	idleTimeout = 5 * time.Minute

	// gcInterval is how often the hub checks for idle rooms.
	gcInterval = 1 * time.Minute

	// snapshotInterval is how often active rooms flush their state.
	snapshotInterval = 30 * time.Second
)

// OperationPubSub fans room operations out to other WebSocket server nodes.
type OperationPubSub interface {
	Publish(canvasID collaboration.RoomID, op collaboration.Operation) error
	Subscribe(canvasID collaboration.RoomID, onOp func(collaboration.Operation)) error
	Unsubscribe(canvasID collaboration.RoomID) error
}

// HubOption configures optional distributed collaboration dependencies.
type HubOption func(*Hub)

func WithPubSub(pubsub OperationPubSub) HubOption {
	return func(h *Hub) { h.pubsub = pubsub }
}

func WithPresenceRepository(repo collaboration.PresenceRepository) HubOption {
	return func(h *Hub) { h.presenceRepo = repo }
}

func WithLockRepository(repo collaboration.LockRepository) HubOption {
	return func(h *Hub) { h.lockRepo = repo }
}

// Hub is the central registry of all active Rooms.
// It implements the application/collaboration.RoomManager interface.
//
// Responsibilities:
//   - Lazy room creation (first user joining)
//   - Routing connections to the correct Room
//   - Garbage collecting idle rooms
//   - Periodic snapshot persistence
type Hub struct {
	mu    sync.RWMutex
	rooms map[collaboration.RoomID]*Room

	snapshotRepo collaboration.SnapshotRepository
	presenceRepo collaboration.PresenceRepository
	lockRepo     collaboration.LockRepository
	pubsub       OperationPubSub

	// register and unregister channels for clients.
	register   chan *Client
	unregister chan *Client

	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub creates and starts the Hub.
func NewHub(snapshotRepo collaboration.SnapshotRepository, options ...HubOption) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Hub{
		rooms:        make(map[collaboration.RoomID]*Room),
		snapshotRepo: snapshotRepo,
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		ctx:          ctx,
		cancel:       cancel,
	}
	for _, option := range options {
		option(h)
	}

	go h.run()
	go h.gcLoop()

	return h
}

// run is the main event loop of the Hub. It serializes room creation/teardown
// and client registration to avoid data races.
func (h *Hub) run() {
	for {
		select {
		case <-h.ctx.Done():
			return

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)
		}
	}
}

// handleRegister adds a client to the appropriate Room, creating it if needed.
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	room, exists := h.rooms[client.RoomID]
	if !exists {
		room = NewRoom(client.RoomID, h.snapshotRepo, h.loadSnapshot(client.RoomID), h.pubsub, h.presenceRepo, h.lockRepo)
		h.rooms[client.RoomID] = room
		h.subscribeRoom(room)
		log.Printf("[hub] created room %d", client.RoomID)
	}
	h.mu.Unlock()

	room.Register(client)
}

// handleUnregister removes a client from its Room and cleans up empty rooms.
func (h *Hub) handleUnregister(client *Client) {
	h.mu.RLock()
	room, exists := h.rooms[client.RoomID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.Unregister(client)
}

// gcLoop periodically checks for idle rooms and unloads them.
func (h *Hub) gcLoop() {
	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.collectGarbage()
		}
	}
}

// collectGarbage finds and removes idle rooms.
func (h *Hub) collectGarbage() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for id, room := range h.rooms {
		if room.IsIdle(idleTimeout) {
			log.Printf("[hub] unloading idle room %d", id)
			room.Shutdown()
			h.unsubscribeRoom(id)
			delete(h.rooms, id)
		}
	}
}

func (h *Hub) subscribeRoom(room *Room) {
	if h.pubsub == nil {
		return
	}
	if err := h.pubsub.Subscribe(room.DomainRoom().ID(), room.EnqueueRemote); err != nil {
		log.Printf("[hub] failed to subscribe room %d: %v", room.DomainRoom().ID(), err)
	}
}

func (h *Hub) unsubscribeRoom(roomID collaboration.RoomID) {
	if h.pubsub == nil {
		return
	}
	if err := h.pubsub.Unsubscribe(roomID); err != nil {
		log.Printf("[hub] failed to unsubscribe room %d: %v", roomID, err)
	}
}

func (h *Hub) loadSnapshot(roomID collaboration.RoomID) []byte {
	if h.snapshotRepo == nil {
		return nil
	}
	snapshot, err := h.snapshotRepo.Load(h.ctx, roomID)
	if err != nil {
		log.Printf("[hub] failed to load snapshot for room %d: %v", roomID, err)
		return nil
	}
	return snapshot
}

// GetOrCreate returns an existing Room or creates one. Implements RoomManager.
func (h *Hub) GetOrCreate(roomID collaboration.RoomID) *collaboration.Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if r, ok := h.rooms[roomID]; ok {
		return r.DomainRoom()
	}

	room := NewRoom(roomID, h.snapshotRepo, h.loadSnapshot(roomID), h.pubsub, h.presenceRepo, h.lockRepo)
	h.rooms[roomID] = room
	h.subscribeRoom(room)
	log.Printf("[hub] created room %d", roomID)
	return room.DomainRoom()
}

// Get returns a Room if it exists, or nil. Implements RoomManager.
func (h *Hub) Get(roomID collaboration.RoomID) *collaboration.Room {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if r, ok := h.rooms[roomID]; ok {
		return r.DomainRoom()
	}
	return nil
}

// Remove unloads a Room. Implements RoomManager.
func (h *Hub) Remove(roomID collaboration.RoomID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[roomID]; ok {
		room.Shutdown()
		h.unsubscribeRoom(roomID)
		delete(h.rooms, roomID)
	}
}

// Count returns the total number of active Rooms. Implements RoomManager.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// Shutdown gracefully stops the Hub, persisting all rooms.
func (h *Hub) Shutdown() {
	log.Println("[hub] shutting down...")
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	for id, room := range h.rooms {
		log.Printf("[hub] persisting room %d before shutdown", id)
		room.Shutdown()
		h.unsubscribeRoom(id)
	}
}

// ---- Client registration API (called by the WS gateway) ----

// RegisterClient adds a client to the hub's event loop.
func (h *Hub) RegisterClient(client *Client) {
	select {
	case h.register <- client:
	case <-h.ctx.Done():
	}
}

// Unregister removes a client (called automatically by readPump).
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.ctx.Done():
	}
}

// ActiveRoomCount returns the number of active rooms for monitoring.
func (h *Hub) ActiveRoomCount() int {
	return h.Count()
}

// ActiveConnectionCount returns the total number of connected clients.
func (h *Hub) ActiveConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, room := range h.rooms {
		count += room.MemberCount()
	}
	return count
}

// ---- Periodic snapshot (called per room) ----

// startSnapshotLoop persists room state on an interval.
// This is started per-room, not globally.
func startSnapshotLoop(ctx context.Context, room *Room, snapshotRepo collaboration.SnapshotRepository) {
	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush before shutdown.
			data := room.Snapshot()
			if len(data) > 0 {
				id := room.DomainRoom().ID()
				if err := snapshotRepo.Save(context.Background(), id, data); err != nil {
					log.Printf("[hub] failed to persist snapshot for room %d: %v", id, err)
				}
			}
			return
		case <-ticker.C:
			data := room.Snapshot()
			if len(data) > 0 {
				id := room.DomainRoom().ID()
				if err := snapshotRepo.Save(ctx, id, data); err != nil {
					log.Printf("[hub] failed to persist snapshot for room %d: %v", id, err)
				}
			}
		}
	}
}
