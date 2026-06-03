package canvas

import (
	"context"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// SnapshotApplicationService orchestrates canvas snapshot save/load use cases.
type SnapshotApplicationService struct {
	canvasRepo   canvasDomain.CanvasRepository
	snapshotRepo canvasDomain.SnapshotRepository
}

// NewSnapshotApplicationService creates the snapshot application service.
func NewSnapshotApplicationService(
	canvasRepo canvasDomain.CanvasRepository,
	snapshotRepo canvasDomain.SnapshotRepository,
) *SnapshotApplicationService {
	return &SnapshotApplicationService{canvasRepo: canvasRepo, snapshotRepo: snapshotRepo}
}

// Save persists a canvas snapshot. Verifies the user has edit access.
func (s *SnapshotApplicationService) Save(
	ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID, data []byte, thumbnail string,
) error {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return err
	}
	if !c.GetRole(userID).CanEdit() && c.OwnerID() != userID {
		return shared.NewDomainError(shared.ErrForbidden, "you do not have permission to edit this canvas")
	}
	return s.snapshotRepo.Save(ctx, canvasID, data, thumbnail)
}

// Load retrieves the latest snapshot and thumbnail. Returns nil if none.
func (s *SnapshotApplicationService) Load(
	ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID,
) ([]byte, string, error) {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, "", err
	}
	if !c.Visibility().IsPublished() && !c.IsMember(userID) {
		return nil, "", shared.NewDomainError(shared.ErrForbidden, "you do not have access to this canvas")
	}
	return s.snapshotRepo.Load(ctx, canvasID)
}
