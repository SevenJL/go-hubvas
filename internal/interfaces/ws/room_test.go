package ws

import (
	"context"
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
	room := NewRoom(7, nil, nil, pubsub, nil)
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
	room := NewRoom(7, nil, nil, nil, repo)
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
	room := NewRoom(7, nil, nil, nil, nil)
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
	room := NewRoom(7, nil, nil, nil, nil)
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
