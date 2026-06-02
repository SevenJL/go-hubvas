package collaboration

import (
	"context"
	"time"

	collabDomain "github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// RoomManager is the interface for the in-memory Room registry (the Hub).
// It is defined here so the application service can depend on it without
// importing the WebSocket implementation directly.
type RoomManager interface {
	// GetOrCreate returns the Room for the given ID, creating it if needed.
	GetOrCreate(roomID collabDomain.RoomID) *collabDomain.Room

	// Get returns the Room if it exists, or nil.
	Get(roomID collabDomain.RoomID) *collabDomain.Room

	// Remove unloads a Room after persisting its state.
	Remove(roomID collabDomain.RoomID)

	// Count returns the total number of active Rooms.
	Count() int
}

// CollaborationApplicationService orchestrates real-time collaboration use cases.
type CollaborationApplicationService struct {
	roomManager    RoomManager
	snapshotRepo   collabDomain.SnapshotRepository
	presenceRepo   collabDomain.PresenceRepository
	lockRepo       collabDomain.LockRepository
	permissionSvc  collabDomain.PermissionService
	throttleSvc    collabDomain.ThrottleService
}

// NewCollaborationApplicationService creates the application service.
func NewCollaborationApplicationService(
	roomManager RoomManager,
	snapshotRepo collabDomain.SnapshotRepository,
	presenceRepo collabDomain.PresenceRepository,
	lockRepo collabDomain.LockRepository,
	permissionSvc collabDomain.PermissionService,
	throttleSvc collabDomain.ThrottleService,
) *CollaborationApplicationService {
	return &CollaborationApplicationService{
		roomManager:   roomManager,
		snapshotRepo:  snapshotRepo,
		presenceRepo:  presenceRepo,
		lockRepo:      lockRepo,
		permissionSvc: permissionSvc,
		throttleSvc:   throttleSvc,
	}
}

// JoinRoom adds a user to a collaboration room.
// It loads the latest snapshot into the room if it was cold-started.
func (s *CollaborationApplicationService) JoinRoom(
	ctx context.Context,
	roomID collabDomain.RoomID,
	userID identity.UserID,
	username string,
) error {
	// Check permission
	canView, err := s.permissionSvc.CanView(ctx, roomID, userID)
	if err != nil {
		return err
	}
	if !canView {
		return shared.NewDomainError(shared.ErrForbidden, "you do not have access to this canvas")
	}

	// Check connection limit
	allowed, err := s.throttleSvc.AllowConnection(ctx, userID, roomID)
	if err != nil {
		return err
	}
	if !allowed {
		return shared.NewDomainError(shared.ErrLimitExceeded, "connection limit reached")
	}

	// Get or create the room
	room := s.roomManager.GetOrCreate(roomID)

	// If the room is newly created, load snapshot.
	if room.Snapshot() == nil || len(room.Snapshot()) == 0 {
		snapshot, err := s.snapshotRepo.Load(ctx, roomID)
		if err == nil && len(snapshot) > 0 {
			room.UpdateSnapshot(snapshot)
		}
		// If snapshot doesn't exist, start with empty document — that's fine.
	}

	room.Join(userID, username)

	// Set initial presence
	role, _ := s.permissionSvc.GetRole(ctx, roomID, userID)
	info := collabDomain.PresenceInfo{
		UserID: userID,
		Username: username,
		Role:    role,
	}
	s.presenceRepo.SetPresence(ctx, roomID, info, 30*time.Second)

	return nil
}

// LeaveRoom removes a user from a collaboration room.
func (s *CollaborationApplicationService) LeaveRoom(
	ctx context.Context,
	roomID collabDomain.RoomID,
	userID identity.UserID,
) error {
	room := s.roomManager.Get(roomID)
	if room == nil {
		return nil // Room already gone
	}

	if err := room.Leave(userID); err != nil {
		return err
	}

	s.presenceRepo.RemovePresence(ctx, roomID, userID)
	return nil
}

// SaveSnapshot persists the current room state to object storage.
func (s *CollaborationApplicationService) SaveSnapshot(
	ctx context.Context,
	roomID collabDomain.RoomID,
) error {
	room := s.roomManager.Get(roomID)
	if room == nil {
		return shared.NewDomainError(shared.ErrNotFound, "room not found")
	}
	return s.snapshotRepo.Save(ctx, roomID, room.Snapshot())
}

// GetPresence returns the online presence info for a room.
func (s *CollaborationApplicationService) GetPresence(
	ctx context.Context,
	roomID collabDomain.RoomID,
) ([]PresenceDTO, error) {
	infos, err := s.presenceRepo.GetPresence(ctx, roomID)
	if err != nil {
		return nil, err
	}
	dtos := make([]PresenceDTO, len(infos))
	for i, info := range infos {
		dtos[i] = ToPresenceDTO(info)
	}
	return dtos, nil
}

// GetRoomInfo returns lightweight info about an active room.
func (s *CollaborationApplicationService) GetRoomInfo(roomID collabDomain.RoomID) (*RoomInfoDTO, error) {
	room := s.roomManager.Get(roomID)
	if room == nil {
		return nil, shared.NewDomainError(shared.ErrNotFound, "room not found or unloaded")
	}
	return &RoomInfoDTO{
		RoomID:      int64(room.ID()),
		MemberCount: room.MemberCount(),
		Status:      room.Status().String(),
	}, nil
}
