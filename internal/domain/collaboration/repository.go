package collaboration

import (
	"context"
	"time"

	"github.com/hubvas/internal/domain/identity"
)

// SnapshotRepository defines the contract for CRDT snapshot persistence.
// Implementations store snapshots in object storage (MinIO/S3).
type SnapshotRepository interface {
	// Save persists a CRDT document snapshot for the given canvas.
	Save(ctx context.Context, canvasID RoomID, data []byte) error

	// Load retrieves the latest CRDT document snapshot.
	Load(ctx context.Context, canvasID RoomID) ([]byte, error)

	// Delete removes all snapshots for a canvas.
	Delete(ctx context.Context, canvasID RoomID) error
}

// PresenceRepository defines the contract for real-time presence storage.
// Implementations use Redis with TTL-based liveness.
type PresenceRepository interface {
	// SetPresence upserts presence info for a user in a room with a TTL.
	SetPresence(ctx context.Context, roomID RoomID, info PresenceInfo, ttl time.Duration) error

	// GetPresence retrieves all online members for a room.
	GetPresence(ctx context.Context, roomID RoomID) ([]PresenceInfo, error)

	// RemovePresence deletes a user's presence record.
	RemovePresence(ctx context.Context, roomID RoomID, userID identity.UserID) error

	// RefreshPresence extends the TTL for a user's presence (heartbeat).
	RefreshPresence(ctx context.Context, roomID RoomID, userID identity.UserID, ttl time.Duration) error

	// GetOnlineCount returns the number of online members in a room.
	GetOnlineCount(ctx context.Context, roomID RoomID) (int, error)
}

// LockRepository defines the contract for distributed object locking.
type LockRepository interface {
	// TryLock attempts to acquire a lock on an object. Returns false if already locked.
	TryLock(ctx context.Context, roomID RoomID, objectID string, userID identity.UserID, ttl time.Duration) (bool, error)

	// Unlock releases a lock on an object.
	Unlock(ctx context.Context, roomID RoomID, objectID string, userID identity.UserID) error

	// GetLockOwner returns the user ID that holds the lock, or nil.
	GetLockOwner(ctx context.Context, roomID RoomID, objectID string) (*identity.UserID, error)
}
