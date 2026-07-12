package canvas

import (
	"context"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CanvasCommunityRepository is the community projection surface used by canvas use cases.
type CanvasCommunityRepository interface {
	SavePublished(ctx context.Context, pc *communityDomain.PublishedCanvas) error
	SaveFork(ctx context.Context, fork *communityDomain.Fork) error
}

// CanvasUserLookup resolves users needed by membership use cases.
type CanvasUserLookup interface {
	FindByID(ctx context.Context, id identity.UserID) (*identity.User, error)
	FindByUsername(ctx context.Context, username string) (*identity.User, error)
}

// CanvasApplicationService orchestrates canvas-related use cases.
type CanvasApplicationService struct {
	canvasRepo    canvasDomain.CanvasRepository
	snapshotRepo  canvasDomain.SnapshotRepository
	communityRepo CanvasCommunityRepository
	userRepo      CanvasUserLookup
	idGen         shared.IDGenerator
}

// NewCanvasApplicationService creates the application service.
func NewCanvasApplicationService(
	canvasRepo canvasDomain.CanvasRepository,
	snapshotRepo canvasDomain.SnapshotRepository,
	communityRepo CanvasCommunityRepository,
	userRepo CanvasUserLookup,
	idGen shared.IDGenerator,
) *CanvasApplicationService {
	return &CanvasApplicationService{
		canvasRepo:    canvasRepo,
		snapshotRepo:  snapshotRepo,
		communityRepo: communityRepo,
		userRepo:      userRepo,
		idGen:         idGen,
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

	dto := toCanvasDTO(c, 0)
	dto.CurrentRole = canvasDomain.RoleOwner.String()
	return dto, nil
}

// Get retrieves a canvas by ID if it is published or the requester is a member.
// requesterID may be zero for anonymous requests.
func (s *CanvasApplicationService) Get(ctx context.Context, id canvasDomain.CanvasID, requesterID identity.UserID) (*CanvasDTO, error) {
	c, err := s.canvasRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !c.Visibility().IsPublished() && !c.IsMember(requesterID) {
		return nil, shared.NewDomainError(shared.ErrForbidden, "you do not have access to this canvas")
	}
	dto := toCanvasDTO(c, 0)
	if role := c.GetRole(requesterID); role >= 0 {
		dto.CurrentRole = role.String()
	}
	return dto, nil
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
		dtos[i].CurrentRole = canvasDomain.RoleOwner.String()
	}
	return dtos, nil
}

// ListShared returns canvases shared with a user, excluding canvases they own.
func (s *CanvasApplicationService) ListShared(ctx context.Context, userID identity.UserID) ([]*CanvasDTO, error) {
	canvases, err := s.canvasRepo.FindByMember(ctx, userID)
	if err != nil {
		return nil, err
	}
	dtos := make([]*CanvasDTO, 0, len(canvases))
	for _, c := range canvases {
		if c.OwnerID() == userID {
			continue
		}
		dto := toCanvasDTO(c, 0)
		if role := c.GetRole(userID); role >= 0 {
			dto.CurrentRole = role.String()
		}
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

// ListMembers returns membership details to users who can access the canvas.
func (s *CanvasApplicationService) ListMembers(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID) ([]MemberDTO, error) {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	if !c.IsMember(operatorID) {
		return nil, shared.NewDomainError(shared.ErrForbidden, "only canvas members can list members")
	}
	members := make([]MemberDTO, 0, len(c.Members()))
	for _, member := range c.Members() {
		dto := MemberDTO{UserID: int64(member.ID), Role: member.Role.String()}
		if s.userRepo != nil {
			user, lookupErr := s.userRepo.FindByID(ctx, member.ID)
			if lookupErr != nil {
				return nil, lookupErr
			}
			dto.Username = user.Username()
			dto.DisplayName = user.DisplayName()
			dto.AvatarURL = user.AvatarURL()
		}
		members = append(members, dto)
	}
	return members, nil
}

// AddMember grants a registered user access to a canvas. Only the owner may do this.
func (s *CanvasApplicationService) AddMember(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID, req AddMemberRequest) (*MemberDTO, error) {
	c, err := s.ownerCanvas(ctx, canvasID, operatorID)
	if err != nil {
		return nil, err
	}
	role, err := parseAssignableRole(req.Role)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if user.ID() == c.OwnerID() {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "owner role cannot be changed")
	}
	c.AddMember(user.ID(), role)
	if err := s.canvasRepo.Save(ctx, c); err != nil {
		return nil, err
	}
	return &MemberDTO{UserID: int64(user.ID()), Username: user.Username(), DisplayName: user.DisplayName(), AvatarURL: user.AvatarURL(), Role: role.String()}, nil
}

// UpdateMemberRole changes an existing non-owner member's role.
func (s *CanvasApplicationService) UpdateMemberRole(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID, memberID identity.UserID, req UpdateMemberRoleRequest) (*MemberDTO, error) {
	c, err := s.ownerCanvas(ctx, canvasID, operatorID)
	if err != nil {
		return nil, err
	}
	if memberID == c.OwnerID() {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "owner role cannot be changed")
	}
	if !c.IsMember(memberID) {
		return nil, shared.NewDomainError(shared.ErrNotFound, "member not found")
	}
	role, err := parseAssignableRole(req.Role)
	if err != nil {
		return nil, err
	}
	c.AddMember(memberID, role)
	if err := s.canvasRepo.Save(ctx, c); err != nil {
		return nil, err
	}
	dto := &MemberDTO{UserID: int64(memberID), Role: role.String()}
	if s.userRepo != nil {
		user, lookupErr := s.userRepo.FindByID(ctx, memberID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		dto.Username = user.Username()
		dto.DisplayName = user.DisplayName()
		dto.AvatarURL = user.AvatarURL()
	}
	return dto, nil
}

// RemoveMember revokes a non-owner member's canvas access.
func (s *CanvasApplicationService) RemoveMember(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID, memberID identity.UserID) error {
	c, err := s.ownerCanvas(ctx, canvasID, operatorID)
	if err != nil {
		return err
	}
	if err := c.RemoveMember(memberID); err != nil {
		return err
	}
	return s.canvasRepo.Save(ctx, c)
}

func (s *CanvasApplicationService) ownerCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, operatorID identity.UserID) (*canvasDomain.Canvas, error) {
	c, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	if c.OwnerID() != operatorID {
		return nil, shared.NewDomainError(shared.ErrForbidden, "only the owner can manage members")
	}
	return c, nil
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
	if err := s.canvasRepo.Save(ctx, c); err != nil {
		return err
	}

	// Also create/update the community read-side projection.
	published := communityDomain.NewPublishedCanvas(
		c.ID(),
		c.OwnerID(),
		c.Title(),
		"", // snapshotURL — populated later when snapshot is available
		nil,
	)
	return s.communityRepo.SavePublished(ctx, published)
}

// Fork creates a fork of the source canvas, including its visual snapshot.
func (s *CanvasApplicationService) Fork(ctx context.Context, sourceID canvasDomain.CanvasID, userID identity.UserID) (*CanvasDTO, error) {
	source, err := s.canvasRepo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if !source.Visibility().IsPublished() && !source.IsMember(userID) {
		return nil, shared.NewDomainError(shared.ErrForbidden, "you do not have access to fork this canvas")
	}

	newID := canvasDomain.CanvasID(s.idGen.NextID())
	fork, err := source.ForkCanvas(newID, userID, "")
	if err != nil {
		return nil, err
	}

	if err := s.canvasRepo.Save(ctx, fork); err != nil {
		return nil, err
	}

	// Copy the source canvas's visual snapshot and thumbnail to the fork.
	snapshot, thumbnail, err := s.snapshotRepo.Load(ctx, sourceID)
	if err == nil && len(snapshot) > 0 {
		_ = s.snapshotRepo.Save(ctx, newID, snapshot, thumbnail)
		// Best-effort: fork succeeds even if snapshot copy fails.
	}

	// Published forks are part of the community graph. SaveFork also updates
	// the source projection's fork counter atomically in the repository.
	if source.Visibility().IsPublished() {
		if err := s.communityRepo.SaveFork(ctx, communityDomain.NewFork(sourceID, newID, userID)); err != nil {
			// Compensate the already-created canvas so callers do not receive an
			// error while a hidden partial fork remains persisted.
			_ = s.canvasRepo.Delete(ctx, newID)
			return nil, err
		}
	}

	dto := toCanvasDTO(fork, 0)
	dto.CurrentRole = canvasDomain.RoleOwner.String()
	return dto, nil
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
