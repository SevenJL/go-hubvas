package community

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// Like is an aggregate root representing a user's like on a published canvas.
// It uses a composite key (CanvasID, UserID).
type Like struct {
	CanvasID  canvas.CanvasID
	UserID    identity.UserID
	CreatedAt time.Time
}

// NewLike creates a new Like with the current timestamp.
func NewLike(canvasID canvas.CanvasID, userID identity.UserID) *Like {
	return &Like{
		CanvasID:  canvasID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
}

// ReconstituteLike rebuilds a Like from persistence.
func ReconstituteLike(canvasID canvas.CanvasID, userID identity.UserID, createdAt time.Time) *Like {
	return &Like{
		CanvasID:  canvasID,
		UserID:    userID,
		CreatedAt: createdAt,
	}
}
