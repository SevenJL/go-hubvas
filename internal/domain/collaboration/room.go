package collaboration

import (
	"time"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// Room is the aggregate root for the Collaboration bounded context.
// Each canvas has exactly one Room, which is created lazily on first join
// and garbage-collected after an idle timeout.
//
// The Room's state is mutated EXCLUSIVELY through the ProcessOp method,
// which ensures serial access — no locks needed.
type Room struct {
	shared.AggregateRoot
	id          RoomID
	snapshot    []byte        // Latest CRDT document snapshot (binary)
	version     int64         // Monotonically increasing version
	members     []*RoomMember // Online members
	objectLocks map[string]LockInfo
	status      RoomStatus
	createdAt   time.Time
	lastActive  time.Time
}

// NewRoom creates a new Room for the given canvas.
func NewRoom(id RoomID, snapshot []byte) *Room {
	now := time.Now()
	r := &Room{
		id:          id,
		snapshot:    snapshot,
		version:     0,
		members:     make([]*RoomMember, 0),
		objectLocks: make(map[string]LockInfo),
		status:      RoomStatusActive,
		createdAt:   now,
		lastActive:  now,
	}
	return r
}

// ---- Accessors ----

func (r *Room) ID() RoomID             { return r.id }
func (r *Room) Snapshot() []byte       { return r.snapshot }
func (r *Room) Version() int64         { return r.version }
func (r *Room) Members() []*RoomMember { return r.members }
func (r *Room) Status() RoomStatus     { return r.status }
func (r *Room) CreatedAt() time.Time   { return r.createdAt }
func (r *Room) LastActive() time.Time  { return r.lastActive }
func (r *Room) MemberCount() int       { return len(r.members) }
func (r *Room) ObjectLocks() []LockInfo {
	locks := make([]LockInfo, 0, len(r.objectLocks))
	for _, lock := range r.objectLocks {
		locks = append(locks, lock)
	}
	return locks
}

// ---- Mutations (serialized via the hub goroutine) ----

// ProcessOp is the single entry point for all operations on a Room.
// It validates, applies, and returns a list of broadcast targets.
func (r *Room) ProcessOp(op Operation) (*BroadcastResult, error) {
	r.lastActive = time.Now()

	switch op.Type {
	case OpSync:
		return r.handleSync(op)
	case OpAwareness:
		return r.handleAwareness(op)
	case OpPresence:
		return r.handlePresence(op)
	case OpChat:
		return r.handleChat(op)
	default:
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "unknown op type: "+string(op.Type))
	}
}

// Join adds a user to the room. Returns the new member.
func (r *Room) Join(userID identity.UserID, username string) *RoomMember {
	// If already present, remove old session first.
	_ = r.Leave(userID)

	member := NewRoomMember(r.id, userID, username)
	r.members = append(r.members, member)
	r.lastActive = time.Now()

	r.AddEvent(UserJoinedRoomEvent{
		BaseEvent: shared.NewBaseEvent("UserJoinedRoom"),
		RoomID:    r.id,
		UserID:    userID,
	})

	return member
}

// Leave removes a user from the room and releases any locks they hold.
func (r *Room) Leave(userID identity.UserID) error {
	for i, m := range r.members {
		if m.UserID == userID {
			r.members = append(r.members[:i], r.members[i+1:]...)
			// Release all locks held by this user.
			for objID, lock := range r.objectLocks {
				if lock.UserID == userID {
					delete(r.objectLocks, objID)
				}
			}
			r.lastActive = time.Now()

			r.AddEvent(UserLeftRoomEvent{
				BaseEvent: shared.NewBaseEvent("UserLeftRoom"),
				RoomID:    r.id,
				UserID:    userID,
			})
			return nil
		}
	}
	return shared.NewDomainError(shared.ErrNotFound, "member not in room")
}

// LockObject attempts to lock an object for a user.
// Returns an error if the object is already locked by someone else.
func (r *Room) LockObject(objectID string, userID identity.UserID) error {
	if lock, exists := r.objectLocks[objectID]; exists && lock.UserID != userID {
		return shared.NewDomainError(shared.ErrConflict, "object is locked by another user")
	}
	r.objectLocks[objectID] = LockInfo{ObjectID: objectID, UserID: userID}

	r.AddEvent(ObjectLockedEvent{
		BaseEvent: shared.NewBaseEvent("ObjectLocked"),
		RoomID:    r.id,
		ObjectID:  objectID,
		UserID:    userID,
	})
	return nil
}

// UnlockObject releases a lock on an object.
func (r *Room) UnlockObject(objectID string, userID identity.UserID) {
	if lock, exists := r.objectLocks[objectID]; exists && lock.UserID == userID {
		delete(r.objectLocks, objectID)

		r.AddEvent(ObjectUnlockedEvent{
			BaseEvent: shared.NewBaseEvent("ObjectUnlocked"),
			RoomID:    r.id,
			ObjectID:  objectID,
			UserID:    userID,
		})
	}
}

// IsLockedBy checks whether the given object is locked by a specific user.
func (r *Room) IsLockedBy(objectID string, userID identity.UserID) bool {
	lock, exists := r.objectLocks[objectID]
	return exists && lock.UserID == userID
}

// IsLocked checks whether the given object is locked by anyone.
func (r *Room) IsLocked(objectID string) bool {
	_, exists := r.objectLocks[objectID]
	return exists
}

// ApplyLockState replaces an object lock from a trusted distributed source.
func (r *Room) ApplyLockState(objectID string, ownerID *identity.UserID) {
	if ownerID == nil {
		delete(r.objectLocks, objectID)
		return
	}
	r.objectLocks[objectID] = LockInfo{ObjectID: objectID, UserID: *ownerID}
}

// UpdateSnapshot replaces the in-memory snapshot and bumps the version.
func (r *Room) UpdateSnapshot(data []byte) {
	r.snapshot = data
	r.version++
}

// SetStatus transitions the room's lifecycle state.
func (r *Room) SetStatus(status RoomStatus) {
	r.status = status
}

// IsIdle returns true if the room has been inactive for the given duration.
func (r *Room) IsIdle(timeout time.Duration) bool {
	return time.Since(r.lastActive) > timeout
}

// FindMember returns the room member by user ID, or nil.
func (r *Room) FindMember(userID identity.UserID) *RoomMember {
	for _, m := range r.members {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}

// ---- Internal handlers ----

func (r *Room) handleSync(op Operation) (*BroadcastResult, error) {
	// In CRDT relay mode, we just append the update to the snapshot and broadcast.
	// The actual merge is done client-side by Yjs.
	r.UpdateSnapshot(op.Payload)

	r.AddEvent(SyncReceivedEvent{
		BaseEvent: shared.NewBaseEvent("SyncReceived"),
		RoomID:    r.id,
		UserID:    op.UserID,
		Version:   r.version,
	})

	return &BroadcastResult{
		Target:        BroadcastAll,
		ExcludeUserID: &op.UserID, // Don't echo back to sender
		Operation:     op,
	}, nil
}

func (r *Room) handleAwareness(op Operation) (*BroadcastResult, error) {
	// Awareness messages are relayed to all other members.
	return &BroadcastResult{
		Target:        BroadcastAll,
		ExcludeUserID: &op.UserID,
		Operation:     op,
	}, nil
}

func (r *Room) handlePresence(op Operation) (*BroadcastResult, error) {
	// Presence updates go to everyone.
	return &BroadcastResult{
		Target:    BroadcastAll,
		Operation: op,
	}, nil
}

func (r *Room) handleChat(op Operation) (*BroadcastResult, error) {
	// Chat messages are broadcast to all members including sender.
	return &BroadcastResult{
		Target:    BroadcastAll,
		Operation: op,
	}, nil
}

// ---- Broadcast ----

// BroadcastTarget specifies which members receive a message.
type BroadcastTarget int8

const (
	BroadcastAll    BroadcastTarget = 0
	BroadcastOthers BroadcastTarget = 1
	BroadcastSingle BroadcastTarget = 2
)

// BroadcastResult describes the result of processing an operation.
type BroadcastResult struct {
	Target        BroadcastTarget
	ExcludeUserID *identity.UserID // nil means no exclusion
	TargetUserID  *identity.UserID // used with BroadcastSingle
	Operation     Operation
}
