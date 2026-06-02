package canvas

import (
	"context"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// IDGenerator is a simple contract for generating unique IDs.
// The real implementation uses PostgreSQL sequences or Snowflake.
type IDGenerator interface {
	NextID() int64
}

// CanvasApplicationService orchestrates canvas-related use cases.
type CanvasApplicationService struct {
	canvasRepo canvasDomain.CanvasRepository
	idGen      IDGenerator
}

// NewCanvasApplicationService creates the application service.
func NewCanvasApplicationService(
	canvasRepo canvasDomain.CanvasRepository,
	idGen IDGenerator,
) *CanvasApplicationService {
	return &CanvasApplicationService{
		canvasRepo: canvasRepo,
		idGen:      idGen,
	}
}

// Create creates a new canvas for the given owner.
func (s *CanvasApplicationService) Create(ctx context.Context, ownerID identity.UserID, req CreateCanvasRequest) (*CanvasDTO, error) {
	c, err := canvasDomain.NewCanvas(canvasDomain.CanvasID(s.idGen.NextID()), ownerID, req.Title)
	if err != nil {
		return nil, err
	}

	if err := s.canvasRepo.Save(ctx, c); err != nil {
		return nil, err
	}

	return toCanvasDTO(c, 0), nil
}

// Get retrieves a canvas by ID.
func (s *CanvasApplicationService) Get(ctx context.Context, id canvasDomain.CanvasID) (*CanvasDTO, error) {
	c, err := s.canvasRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toCanvasDTO(c, 0), nil
}

// ListByOwner returns all canvases owned by a user.
func (s *CanvasApplicationService) ListByOwner(ctx context.Context, ownerID identity.UserID) ([]*CanvasDTO, error) {
	canvases, err := s.canvasRepo.FindByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	dtos := make([]*CanvasDTO, len(canvases))
	for i, c := range canvases {
		dtos[i] = toCanvasDTO(c, 0)
	}
	return dtos, nil
}

// Rename updates the title of a canvas.
func (s *CanvasApplicationService) Rename(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID, newTitle string) error {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return err
	}

	// Only owner or editor can rename.
	role := c.GetRole(operatorID)
	if !role.CanEdit() {
		return shared.NewDomainError(shared.ErrForbidden, "only editors can rename the canvas")
	}

	if err := c.Rename(newTitle); err != nil {
		return err
	}
	return s.canvasRepo.Save(ctx, c)
}

// Publish publishes a canvas to the community.
func (s *CanvasApplicationService) Publish(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID) error {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return err
	}

	// Only the owner can publish.
	if c.OwnerID() != operatorID {
		return shared.NewDomainError(shared.ErrForbidden, "only the owner can publish")
	}

	if err := c.Publish(); err != nil {
		return err
	}
	return s.canvasRepo.Save(ctx, c)
}

// Fork creates a fork of the source canvas.
func (s *CanvasApplicationService) Fork(ctx context.Context, sourceID canvasDomain.CanvasID, userID identity.UserID) (*CanvasDTO, error) {
	source, err := s.canvasRepo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	newID := canvasDomain.CanvasID(s.idGen.NextID())
	fork, err := source.ForkCanvas(newID, userID, "")
	if err != nil {
		return nil, err
	}

	if err := s.canvasRepo.Save(ctx, fork); err != nil {
		return nil, err
	}

	return toCanvasDTO(fork, 0), nil
}

// Delete removes a canvas. Only the owner can delete.
func (s *CanvasApplicationService) Delete(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID) error {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return err
	}
	if c.OwnerID() != operatorID {
		return shared.NewDomainError(shared.ErrForbidden, "only the owner can delete")
	}
	return s.canvasRepo.Delete(ctx, canvasID)
}

// toCanvasDTO converts a domain Canvas to a DTO.
func toCanvasDTO(c *canvasDomain.Canvas, onlineCount int) *CanvasDTO {
	var forkedFrom *int64
	if f := c.ForkedFrom(); f != nil {
		v := int64(*f)
		forkedFrom = &v
	}

	return &CanvasDTO{
		ID:          int64(c.ID()),
		OwnerID:     int64(c.OwnerID()),
		Title:       c.Title(),
		Visibility:  c.Visibility().String(),
		ForkedFrom:  forkedFrom,
		MemberCount: len(c.Members()),
		OnlineCount: onlineCount,
		CreatedAt:   c.CreatedAt(),
		UpdatedAt:   c.UpdatedAt(),
	}
}
