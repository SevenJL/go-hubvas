package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// PresenceRepo implements collaboration.PresenceRepository using Redis.
//
// Key design:
//
//	presence:{canvasID}:{userID}  → Hash   (username, avatar_url, role, cursor_x, cursor_y, editing_obj) with TTL
//	presence:{canvasID}:members   → Set    (user IDs, for enumeration without KEYS/SCAN)
//
// The hash TTL is the liveness heartbeat. Expired hashes are lazily cleaned
// from the member set when GetPresence is called.
type PresenceRepo struct {
	client *redis.Client
}

// NewPresenceRepo creates a PresenceRepo backed by a Redis client.
func NewPresenceRepo(client *redis.Client) *PresenceRepo {
	return &PresenceRepo{client: client}
}

// ---- key helpers ----

func presenceKey(roomID collaboration.RoomID, userID identity.UserID) string {
	return fmt.Sprintf("presence:%d:%d", roomID, userID)
}

func membersKey(roomID collaboration.RoomID) string {
	return fmt.Sprintf("presence:%d:members", roomID)
}

func lockKey(roomID collaboration.RoomID, objectID string) string {
	return fmt.Sprintf("lock:%d:%s", roomID, objectID)
}

// ---- SetPresence ----

// SetPresence stores presence info with a TTL. The hash is set, then the
// user is added to the room's member set, and the TTL is applied.
func (r *PresenceRepo) SetPresence(ctx context.Context, roomID collaboration.RoomID, info collaboration.PresenceInfo, ttl time.Duration) error {
	key := presenceKey(roomID, info.UserID)

	fields := map[string]any{
		"username":    info.Username,
		"avatar_url":  info.AvatarURL,
		"role":        int8(info.Role),
		"editing_obj": info.EditingObj,
	}
	if info.Cursor != nil {
		fields["cursor_x"] = info.Cursor.X
		fields["cursor_y"] = info.Cursor.Y
	}

	pipe := r.client.Pipeline()

	// Store presence data.
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)

	// Track in the room's member set.
	pipe.SAdd(ctx, membersKey(roomID), int64(info.UserID))
	pipe.Expire(ctx, membersKey(roomID), ttl+time.Minute) // slightly longer TTL

	_, err := pipe.Exec(ctx)
	return err
}

// ---- GetPresence ----

// GetPresence retrieves all online members for a room.
// It reads the member set, then pipelines HGETALL for each member hash.
// Expired hashes (nil response) are cleaned from the member set.
func (r *PresenceRepo) GetPresence(ctx context.Context, roomID collaboration.RoomID) ([]collaboration.PresenceInfo, error) {
	members, err := r.client.SMembers(ctx, membersKey(roomID)).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}

	// Pipeline all HGETALL calls.
	pipe := r.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(members))
	userIDs := make([]identity.UserID, len(members))
	staleMembers := make([]any, 0)

	for i, uidStr := range members {
		uid, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil || uid <= 0 {
			staleMembers = append(staleMembers, uidStr)
			continue
		}
		userIDs[i] = identity.UserID(uid)
		cmds[i] = pipe.HGetAll(ctx, presenceKey(roomID, identity.UserID(uid)))
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("load presence details: %w", err)
	}

	var results []collaboration.PresenceInfo

	for i, cmd := range cmds {
		if cmd == nil {
			continue
		}
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			// Expired or missing — queue for cleanup.
			if userIDs[i] != 0 {
				staleMembers = append(staleMembers, int64(userIDs[i]))
			}
			continue
		}

		info := parsePresenceInfo(userIDs[i], fields)
		results = append(results, info)
	}

	// Clean stale entries from the member set.
	if len(staleMembers) > 0 {
		if err := r.cleanStaleMembers(ctx, roomID, staleMembers); err != nil {
			slog.WarnContext(ctx, "failed to clean stale presence members", "room_id", roomID, "count", len(staleMembers), "error", err)
		}
	}

	return results, nil
}

// ---- RemovePresence ----

// RemovePresence deletes a user's presence hash and removes them from the member set.
func (r *PresenceRepo) RemovePresence(ctx context.Context, roomID collaboration.RoomID, userID identity.UserID) error {
	pipe := r.client.Pipeline()
	pipe.Del(ctx, presenceKey(roomID, userID))
	pipe.SRem(ctx, membersKey(roomID), int64(userID))
	_, err := pipe.Exec(ctx)
	return err
}

// ---- RefreshPresence ----

// RefreshPresence extends the TTL on a user's presence (heartbeat).
func (r *PresenceRepo) RefreshPresence(ctx context.Context, roomID collaboration.RoomID, userID identity.UserID, ttl time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Expire(ctx, presenceKey(roomID, userID), ttl)
	pipe.Expire(ctx, membersKey(roomID), ttl+time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

// ---- GetOnlineCount ----

// GetOnlineCount returns the number of online members in a room.
// Uses SCARD for O(1) set cardinality.
func (r *PresenceRepo) GetOnlineCount(ctx context.Context, roomID collaboration.RoomID) (int, error) {
	count, err := r.client.SCard(ctx, membersKey(roomID)).Result()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// ---- cleanStaleMembers ----

func (r *PresenceRepo) cleanStaleMembers(ctx context.Context, roomID collaboration.RoomID, members []any) error {
	return r.client.SRem(ctx, membersKey(roomID), members...).Err()
}

// ---- parsePresenceInfo ----

func parsePresenceInfo(userID identity.UserID, fields map[string]string) collaboration.PresenceInfo {
	info := collaboration.PresenceInfo{
		UserID:     userID,
		Username:   fields["username"],
		AvatarURL:  fields["avatar_url"],
		EditingObj: fields["editing_obj"],
	}

	if roleStr, ok := fields["role"]; ok {
		r, _ := strconv.Atoi(roleStr)
		info.Role = canvas.Role(r)
	}

	// Parse cursor if present.
	if cx, ok := fields["cursor_x"]; ok {
		if cy, ok := fields["cursor_y"]; ok {
			x, _ := strconv.ParseFloat(cx, 64)
			y, _ := strconv.ParseFloat(cy, 64)
			info.Cursor = &collaboration.CursorPosition{X: x, Y: y}
		}
	}

	return info
}

// ---- LockRepository implementation ----

// TryLock atomically acquires a lock or renews it when the same user already owns it.
func (r *PresenceRepo) TryLock(ctx context.Context, roomID collaboration.RoomID, objectID string, userID identity.UserID, ttl time.Duration) (bool, error) {
	key := lockKey(roomID, objectID)
	const script = `
		local current = redis.call("GET", KEYS[1])
		if not current or current == ARGV[1] then
			redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
			return 1
		end
		return 0`
	result, err := r.client.Eval(ctx, script, []string{key}, int64(userID), ttl.Milliseconds()).Int()
	return result == 1, err
}

// Unlock releases a distributed lock. Only the lock holder should call this,
// though the Lua script below would enforce it in production.
func (r *PresenceRepo) Unlock(ctx context.Context, roomID collaboration.RoomID, objectID string, userID identity.UserID) error {
	key := lockKey(roomID, objectID)
	// Safe unlock: only delete if the value matches (via Lua).
	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end`
	return r.client.Eval(ctx, script, []string{key}, int64(userID)).Err()
}

// GetLockOwner returns the user ID that holds the lock, or nil.
func (r *PresenceRepo) GetLockOwner(ctx context.Context, roomID collaboration.RoomID, objectID string) (*identity.UserID, error) {
	key := lockKey(roomID, objectID)
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return nil, nil
	}
	id := identity.UserID(uid)
	return &id, nil
}
