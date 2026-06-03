package canvas

import (
	"context"

	"github.com/hubvas/internal/domain/identity"
)

// CanvasRepository defines the contract for Canvas aggregate persistence.
// Implementations manage both the canvases and canvas_members tables.
type CanvasRepository interface {
	Save(ctx context.Context, canvas *Canvas) error
	FindByID(ctx context.Context, id CanvasID) (*Canvas, error)
	FindByOwner(ctx context.Context, ownerID identity.UserID) ([]*Canvas, error)
	FindByMember(ctx context.Context, userID identity.UserID) ([]*Canvas, error)
	Delete(ctx context.Context, id CanvasID) error
}

// SnapshotRepository defines the contract for canvas visual snapshot persistence.
// Snapshot data is the tldraw store JSON — it is NOT part of the Canvas aggregate
// but a supporting read/write projection managed through its own repository.
type SnapshotRepository interface {
	Save(ctx context.Context, canvasID CanvasID, data []byte) error
	Load(ctx context.Context, canvasID CanvasID) ([]byte, error)
}
