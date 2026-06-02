package collaboration

import (
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// UserJoinedRoomEvent is fired when a user enters a collaboration room.
type UserJoinedRoomEvent struct {
	shared.BaseEvent
	RoomID RoomID
	UserID identity.UserID
}

// UserLeftRoomEvent is fired when a user leaves a collaboration room.
type UserLeftRoomEvent struct {
	shared.BaseEvent
	RoomID RoomID
	UserID identity.UserID
}

// SyncReceivedEvent is fired when a CRDT sync update is received and applied.
type SyncReceivedEvent struct {
	shared.BaseEvent
	RoomID  RoomID
	UserID  identity.UserID
	Version int64
}

// ObjectLockedEvent is fired when an object is locked for editing.
type ObjectLockedEvent struct {
	shared.BaseEvent
	RoomID   RoomID
	ObjectID string
	UserID   identity.UserID
}

// ObjectUnlockedEvent is fired when an object lock is released.
type ObjectUnlockedEvent struct {
	shared.BaseEvent
	RoomID   RoomID
	ObjectID string
	UserID   identity.UserID
}

// SnapshotCreatedEvent is fired when a new snapshot is persisted to storage.
type SnapshotCreatedEvent struct {
	shared.BaseEvent
	CanvasID    RoomID
	SnapshotKey string
	Version     int64
}
