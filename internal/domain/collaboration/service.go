package collaboration

import (
	"context"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// PermissionService defines the contract for authorizing actions within a Room.
// It bridges the Collaboration and Canvas contexts.
type PermissionService interface {
	// CanEdit returns true if the user has edit permission on the canvas.
	CanEdit(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error)

	// CanView returns true if the user has view permission on the canvas.
	CanView(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error)

	// CanComment returns true if the user has comment permission on the canvas.
	CanComment(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error)

	// GetRole returns the user's role on the canvas.
	GetRole(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (canvas.Role, error)
}

// ConflictResolutionService defines the contract for resolving edit conflicts.
// In the CRDT relay mode this is mostly a no-op — the client handles merging.
// If the project later adopts server-authoritative mode, this service would
// implement last-write-wins with version vectors or operational transforms.
type ConflictResolutionService interface {
	// ValidateOp checks whether an operation is valid given the current version.
	ValidateOp(op Operation, currentVersion int64) error

	// MergeOp applies an operation to the current state and returns the result.
	// In relay mode, this simply records the op for persistence.
	MergeOp(state []byte, op Operation) ([]byte, error)
}

// ThrottleService defines the contract for rate-limiting operations.
type ThrottleService interface {
	// AllowConnection checks if a new connection is allowed (connection-level throttling).
	AllowConnection(ctx context.Context, userID identity.UserID, roomID RoomID) (bool, error)

	// AllowOperation checks if an operation is within the user's rate limit.
	AllowOperation(ctx context.Context, userID identity.UserID, roomID RoomID, opType OpType) (bool, error)
}
