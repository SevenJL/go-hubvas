package ws

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

type operationPubSubStub struct {
	mu        sync.Mutex
	published []collaboration.Operation
}

func (s *operationPubSubStub) Publish(_ collaboration.RoomID, op collaboration.Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.published = append(s.published, op)
	return nil
}
func (s *operationPubSubStub) Subscribe(collaboration.RoomID, func(collaboration.Operation)) error {
	return nil
}
func (s *operationPubSubStub) Unsubscribe(collaboration.RoomID) error { return nil }
func (s *operationPubSubStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.published)
}

type presenceRepositoryStub struct {
	mu      sync.Mutex
	set     []collaboration.PresenceInfo
	removed []identity.UserID
}

func (s *presenceRepositoryStub) SetPresence(_ context.Context, _ collaboration.RoomID, info collaboration.PresenceInfo, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set = append(s.set, info)
	return nil
}
func (s *presenceRepositoryStub) GetPresence(context.Context, collaboration.RoomID) ([]collaboration.PresenceInfo, error) {
	return nil, nil
}
func (s *presenceRepositoryStub) RemovePresence(_ context.Context, _ collaboration.RoomID, userID identity.UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, userID)
	return nil
}
func (s *presenceRepositoryStub) RefreshPresence(context.Context, collaboration.RoomID, identity.UserID, time.Duration) error {
	return nil
}
func (s *presenceRepositoryStub) GetOnlineCount(context.Context, collaboration.RoomID) (int, error) {
	return 0, nil
}

func TestRoomPublishesOnlyLocalOperations(t *testing.T) {
	pubsub := &operationPubSubStub{}
	room := NewRoom(7, nil, nil, pubsub, nil, nil)
	defer room.Shutdown()

	op := collaboration.Operation{Type: collaboration.OpChat, UserID: 42, Payload: []byte(`{"message":"hello"}`)}
	room.processOp(op, true)
	if got := pubsub.count(); got != 1 {
		t.Fatalf("expected one publish for local operation, got %d", got)
	}

	room.processOp(op, false)
	if got := pubsub.count(); got != 1 {
		t.Fatalf("remote operation must not be republished, got %d publishes", got)
	}
}

func TestRoomPersistsAuthoritativePresenceAndCursor(t *testing.T) {
	repo := &presenceRepositoryStub{}
	room := NewRoom(7, nil, nil, nil, repo, nil)
	defer room.Shutdown()

	client := &Client{
		UserID:     42,
		Username:   "alice",
		AvatarURL:  "https://example.com/alice.png",
		DomainRole: canvas.RoleEditor,
	}
	room.clients[client] = true
	room.setClientPresence(client, nil)
	room.updatePresenceFromAwareness(collaboration.Operation{
		Type:    collaboration.OpAwareness,
		UserID:  client.UserID,
		Payload: []byte(`{"cursor":{"x":12.5,"y":8}}`),
	})

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.set) != 2 {
		t.Fatalf("expected join and awareness presence writes, got %d", len(repo.set))
	}
	latest := repo.set[1]
	if latest.Username != "alice" || latest.Role != canvas.RoleEditor || latest.Cursor == nil {
		t.Fatalf("unexpected persisted presence: %#v", latest)
	}
	if latest.Cursor.X != 12.5 || latest.Cursor.Y != 8 {
		t.Fatalf("unexpected cursor: %#v", latest.Cursor)
	}
}

func TestRoomKeepsDomainMemberUntilLastLocalConnectionLeaves(t *testing.T) {
	room := NewRoom(7, nil, nil, nil, nil, nil)
	defer room.Shutdown()

	first := &Client{UserID: 42, Username: "alice", send: make(chan []byte, 1)}
	second := &Client{UserID: 42, Username: "alice", send: make(chan []byte, 1)}
	room.clients[first] = true
	room.clients[second] = true
	room.domainRoom.Join(42, "alice")

	room.Unregister(first)
	if room.domainRoom.FindMember(42) == nil {
		t.Fatal("domain member was removed while another local connection remained")
	}

	room.Unregister(second)
	if room.domainRoom.FindMember(42) != nil {
		t.Fatal("domain member should be removed after the final local connection leaves")
	}
}

func TestRoomExcludesOnlySendingConnection(t *testing.T) {
	room := NewRoom(7, nil, nil, nil, nil, nil)
	defer room.Shutdown()

	sender := &Client{UserID: 42, send: make(chan []byte, 1)}
	sameUserOtherTab := &Client{UserID: 42, send: make(chan []byte, 1)}
	room.clients[sender] = true
	room.clients[sameUserOtherTab] = true

	op := collaboration.Operation{
		Type: collaboration.OpSync, UserID: 42, Seq: 1,
		Payload: []byte(`{"kind":"tldraw-diff-v1","diffs":[]}`),
	}
	room.processOpFrom(op, true, sender)

	select {
	case <-sender.send:
		t.Fatal("sending connection must not receive its own diff")
	default:
	}
	select {
	case <-sameUserOtherTab.send:
	default:
		t.Fatal("another connection for the same user must receive the diff")
	}
}

type lockRepositoryStub struct {
	mu     sync.Mutex
	owners map[string]identity.UserID
}

func newLockRepositoryStub() *lockRepositoryStub {
	return &lockRepositoryStub{owners: make(map[string]identity.UserID)}
}

func (s *lockRepositoryStub) TryLock(_ context.Context, _ collaboration.RoomID, objectID string, userID identity.UserID, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, exists := s.owners[objectID]
	if exists && owner != userID {
		return false, nil
	}
	s.owners[objectID] = userID
	return true, nil
}

func (s *lockRepositoryStub) Unlock(_ context.Context, _ collaboration.RoomID, objectID string, userID identity.UserID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owners[objectID] == userID {
		delete(s.owners, objectID)
	}
	return nil
}

func (s *lockRepositoryStub) GetLockOwner(_ context.Context, _ collaboration.RoomID, objectID string) (*identity.UserID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, exists := s.owners[objectID]
	if !exists {
		return nil, nil
	}
	result := owner
	return &result, nil
}

func TestRoomLockRequiresEditPermissionAndTrustedIdentity(t *testing.T) {
	locks := newLockRepositoryStub()
	room := NewRoom(7, nil, nil, nil, nil, locks)
	defer room.Shutdown()

	viewer := &Client{UserID: 42, CanEdit: false, send: make(chan []byte, 4)}
	room.processOpFrom(collaboration.Operation{
		Type: collaboration.OpLock, UserID: 42, Payload: []byte(`{"object_id":"shape:1"}`),
	}, true, viewer)
	if owner, _ := locks.GetLockOwner(context.Background(), 7, "shape:1"); owner != nil {
		t.Fatal("viewer acquired an object lock")
	}

	editor := &Client{UserID: 42, CanEdit: true, send: make(chan []byte, 4)}
	room.processOpFrom(collaboration.Operation{
		Type: collaboration.OpLock, UserID: 999, Payload: []byte(`{"object_id":"shape:1"}`),
	}, true, editor)
	if owner, _ := locks.GetLockOwner(context.Background(), 7, "shape:1"); owner != nil {
		t.Fatal("operation with a non-authoritative identity acquired an object lock")
	}
}

func TestRoomLockConflictAndOwnerOnlyUnlock(t *testing.T) {
	locks := newLockRepositoryStub()
	pubsub := &operationPubSubStub{}
	room := NewRoom(7, nil, nil, pubsub, nil, locks)
	defer room.Shutdown()

	ownerClient := &Client{UserID: 42, CanEdit: true, send: make(chan []byte, 16)}
	otherClient := &Client{UserID: 77, CanEdit: true, send: make(chan []byte, 16)}
	room.clients[ownerClient] = true
	room.clients[otherClient] = true
	payload := []byte(`{"object_id":"shape:1"}`)

	room.processOpFrom(collaboration.Operation{Type: collaboration.OpLock, UserID: 42, Payload: payload}, true, ownerClient)
	owner, _ := locks.GetLockOwner(context.Background(), 7, "shape:1")
	if owner == nil || *owner != 42 {
		t.Fatalf("expected user 42 to own lock, got %v", owner)
	}
	if got := pubsub.count(); got != 1 {
		t.Fatalf("expected acquired lock state to publish once, got %d", got)
	}

	room.processOpFrom(collaboration.Operation{Type: collaboration.OpUnlock, UserID: 77, Payload: payload}, true, otherClient)
	owner, _ = locks.GetLockOwner(context.Background(), 7, "shape:1")
	if owner == nil || *owner != 42 {
		t.Fatal("non-owner released another user's lock")
	}
	if got := pubsub.count(); got != 1 {
		t.Fatalf("rejected unlock must not publish, got %d", got)
	}

	room.processOpFrom(collaboration.Operation{Type: collaboration.OpUnlock, UserID: 42, Payload: payload}, true, ownerClient)
	owner, _ = locks.GetLockOwner(context.Background(), 7, "shape:1")
	if owner != nil {
		t.Fatal("owner failed to release object lock")
	}
	if got := pubsub.count(); got != 2 {
		t.Fatalf("expected unlock state to publish, got %d", got)
	}
}

type linkedPubSub struct {
	mu           sync.Mutex
	subscription func(collaboration.Operation)
	peer         *linkedPubSub
	published    int
}

func (s *linkedPubSub) Publish(_ collaboration.RoomID, op collaboration.Operation) error {
	s.mu.Lock()
	s.published++
	peer := s.peer
	s.mu.Unlock()
	if peer != nil {
		peer.mu.Lock()
		subscriber := peer.subscription
		peer.mu.Unlock()
		if subscriber != nil {
			subscriber(op)
		}
	}
	return nil
}
func (s *linkedPubSub) Subscribe(_ collaboration.RoomID, subscriber func(collaboration.Operation)) error {
	s.mu.Lock()
	s.subscription = subscriber
	s.mu.Unlock()
	return nil
}
func (s *linkedPubSub) Unsubscribe(collaboration.RoomID) error { return nil }
func (s *linkedPubSub) publishCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.published
}

func TestTwoRoomsExchangeOperationsWithoutRepublishLoop(t *testing.T) {
	leftBus, rightBus := &linkedPubSub{}, &linkedPubSub{}
	leftBus.peer, rightBus.peer = rightBus, leftBus
	left := NewRoom(7, nil, nil, leftBus, nil, nil)
	right := NewRoom(7, nil, nil, rightBus, nil, nil)
	defer left.Shutdown()
	defer right.Shutdown()
	if err := leftBus.Subscribe(7, left.EnqueueRemote); err != nil {
		t.Fatal(err)
	}
	if err := rightBus.Subscribe(7, right.EnqueueRemote); err != nil {
		t.Fatal(err)
	}

	receiver := &Client{UserID: 77, send: make(chan []byte, 4)}
	right.clients[receiver] = true
	left.processOp(collaboration.Operation{
		Type: collaboration.OpChat, UserID: 42, Payload: []byte(`{"content":"hello"}`),
	}, true)

	select {
	case data := <-receiver.send:
		var message Message
		if err := json.Unmarshal(data, &message); err != nil || message.Type != MsgTypeChat {
			t.Fatalf("unexpected remote message: %s (%v)", data, err)
		}
	case <-time.After(time.Second):
		t.Fatal("remote room did not receive published operation")
	}
	if leftBus.publishCount() != 1 || rightBus.publishCount() != 0 {
		t.Fatalf("remote operation was republished: left=%d right=%d", leftBus.publishCount(), rightBus.publishCount())
	}
}
