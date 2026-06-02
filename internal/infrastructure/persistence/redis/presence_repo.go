package redis

import (
	"context"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// PresenceRepo implements collaboration.PresenceRepository using Redis.
// Keys: presence:{roomID}:{userID} → JSON(PresenceInfo)
// TTL is refreshed on each heartbeat; expired keys indicate offline users.
type PresenceRepo struct {
	// client *redis.Client — to be wired in during implementation.
}

// NewPresenceRepo creates a new PresenceRepo.
func NewPresenceRepo( /* client *redis.Client */ ) *PresenceRepo {
	return &PresenceRepo{}
}

func (r *PresenceRepo) SetPresence(ctx context.Context, roomID collaboration.RoomID, info collaboration.PresenceInfo, ttl time.Duration) error {
	return nil
}
func (r *PresenceRepo) GetPresence(ctx context.Context, roomID collaboration.RoomID) ([]collaboration.PresenceInfo, error) {
	return nil, nil
}
func (r *PresenceRepo) RemovePresence(ctx context.Context, roomID collaboration.RoomID, userID identity.UserID) error {
	return nil
}
func (r *PresenceRepo) RefreshPresence(ctx context.Context, roomID collaboration.RoomID, userID identity.UserID, ttl time.Duration) error {
	return nil
}
func (r *PresenceRepo) GetOnlineCount(ctx context.Context, roomID collaboration.RoomID) (int, error) {
	return 0, nil
}
