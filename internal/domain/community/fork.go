package community

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// Fork is an aggregate root recording a fork relationship between canvases.
// It provides the "Forked from" lineage for community browsing.
type Fork struct {
	originalCanvasID canvas.CanvasID
	newCanvasID      canvas.CanvasID
	userID           identity.UserID
	createdAt        time.Time
}

// NewFork creates a new Fork record.
func NewFork(originalID, newID canvas.CanvasID, userID identity.UserID) *Fork {
	return &Fork{
		originalCanvasID: originalID,
		newCanvasID:      newID,
		userID:           userID,
		createdAt:        time.Now(),
	}
}

// ReconstituteFork rebuilds a Fork from persistence.
func ReconstituteFork(originalID, newID canvas.CanvasID, userID identity.UserID, createdAt time.Time) *Fork {
	return &Fork{
		originalCanvasID: originalID,
		newCanvasID:      newID,
		userID:           userID,
		createdAt:        createdAt,
	}
}

// ---- Accessors ----

func (f *Fork) OriginalCanvasID() canvas.CanvasID { return f.originalCanvasID }
func (f *Fork) NewCanvasID() canvas.CanvasID      { return f.newCanvasID }
func (f *Fork) UserID() identity.UserID             { return f.userID }
func (f *Fork) CreatedAt() time.Time               { return f.createdAt }
