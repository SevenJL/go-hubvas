package canvas

import "context"

// CanvasRepository defines the contract for Canvas aggregate persistence.
type CanvasRepository interface {
	Save(ctx context.Context, canvas *Canvas) error
	FindByID(ctx context.Context, id CanvasID) (*Canvas, error)
	FindByOwner(ctx context.Context, ownerID uint64) ([]*Canvas, error)
	FindByMember(ctx context.Context, userID uint64) ([]*Canvas, error)
	Delete(ctx context.Context, id CanvasID) error
}

// MemberRepository defines the contract for canvas member persistence.
// In some designs this is folded into CanvasRepository; separated here for clarity.
type MemberRepository interface {
	Save(ctx context.Context, member *CanvasMember) error
	FindByCanvas(ctx context.Context, canvasID CanvasID) ([]*CanvasMember, error)
	Remove(ctx context.Context, canvasID CanvasID, userID uint64) error
}
