package auth

import (
	"context"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CanvasPermissionService implements collaboration.PermissionService by
// reading Canvas aggregate membership from the CanvasRepository.
type CanvasPermissionService struct {
	canvasRepo canvas.CanvasRepository
}

// NewCanvasPermissionService creates a permission service backed by the canvas repo.
func NewCanvasPermissionService(canvasRepo canvas.CanvasRepository) *CanvasPermissionService {
	return &CanvasPermissionService{canvasRepo: canvasRepo}
}

// CanEdit returns true if the user is the owner or an editor.
func (s *CanvasPermissionService) CanEdit(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error) {
	role, err := s.GetRole(ctx, canvasID, userID)
	if err != nil {
		return false, err
	}
	return role.CanEdit(), nil
}

// CanView returns true if the user is a member or the canvas is published.
func (s *CanvasPermissionService) CanView(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error) {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		// If not found, check if it's a shared link scenario.
		return false, err
	}

	// Published canvases are viewable by anyone.
	if c.Visibility().IsPublished() {
		return true, nil
	}

	// Otherwise, must be a member.
	return c.IsMember(userID), nil
}

// CanComment returns true unless the user is a viewer.
func (s *CanvasPermissionService) CanComment(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error) {
	role, err := s.GetRole(ctx, canvasID, userID)
	if err != nil {
		// Non-members can comment on published canvases.
		c, findErr := s.canvasRepo.FindByID(ctx, canvasID)
		if findErr == nil && c.Visibility().IsPublished() {
			return true, nil
		}
		return false, err
	}
	return role.CanComment(), nil
}

// GetRole returns the user's role, or an error if they are not a member.
func (s *CanvasPermissionService) GetRole(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (canvas.Role, error) {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return -1, err
	}

	role := c.GetRole(userID)
	if role < 0 {
		return -1, shared.NewDomainError(shared.ErrForbidden, "not a member of this canvas")
	}
	return role, nil
}

// Ensure it satisfies the interface.
var _ collaboration.PermissionService = (*CanvasPermissionService)(nil)
