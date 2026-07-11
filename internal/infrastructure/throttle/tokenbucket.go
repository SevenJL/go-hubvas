// Package throttle implements rate-limiting services for WebSocket connections and operations.
package throttle

import (
	"context"
	"sync"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// TokenBucket is a simple token bucket rate limiter.
type TokenBucket struct {
	rate       float64 // tokens per second
	burst      float64 // max tokens
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a token bucket with the given rate and burst.
func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:       ratePerSec,
		burst:      float64(burst),
		tokens:     float64(burst), // start full
		lastRefill: time.Now(),
	}
}

// Allow consumes one token from the bucket. Returns true if allowed.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (b *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
	b.lastRefill = now
}

func (b *TokenBucket) isFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return b.tokens >= b.burst
}

// ThrottleService implements collaboration.ThrottleService using per-connection
// and per-user token buckets stored in memory.
//
// Connection limits:
//   - Per user: 5 connections per second (global)
//   - Per room:  200 connections per second
//
// Operation limits:
//   - sync:      120 ops/sec per user per room (animation-frame batches)
//   - awareness: 60 ops/sec per user per room (cursors are throttled client-side too)
//   - chat:      10 ops/min per user per room
type ThrottleService struct {
	mu sync.Mutex

	// Connection limiters per user and per room.
	connLimiters     map[identity.UserID]*TokenBucket
	roomConnLimiters map[collaboration.RoomID]*TokenBucket

	// Operation limiters: per user per room per op type.
	opLimiters map[opLimiterKey]*TokenBucket
}

type opLimiterKey struct {
	userID identity.UserID
	roomID collaboration.RoomID
	opType collaboration.OpType
}

// NewThrottleService creates a ThrottleService with sensible defaults.
func NewThrottleService() *ThrottleService {
	return &ThrottleService{
		connLimiters:     make(map[identity.UserID]*TokenBucket),
		roomConnLimiters: make(map[collaboration.RoomID]*TokenBucket),
		opLimiters:       make(map[opLimiterKey]*TokenBucket),
	}
}

// AllowConnection checks whether a new connection is allowed.
func (s *ThrottleService) AllowConnection(ctx context.Context, userID identity.UserID, roomID collaboration.RoomID) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	s.mu.Lock()
	userLimiter, ok := s.connLimiters[userID]
	if !ok {
		// 5 connections per second, burst up to 10.
		userLimiter = NewTokenBucket(5, 10)
		s.connLimiters[userID] = userLimiter
	}
	roomLimiter, ok := s.roomConnLimiters[roomID]
	if !ok {
		// 200 connections per second, burst up to 400.
		roomLimiter = NewTokenBucket(200, 400)
		s.roomConnLimiters[roomID] = roomLimiter
	}
	s.mu.Unlock()

	if !roomLimiter.Allow() {
		return false, nil
	}
	return userLimiter.Allow(), nil
}

// AllowOperation checks whether an operation is within the user's rate limit.
func (s *ThrottleService) AllowOperation(ctx context.Context, userID identity.UserID, roomID collaboration.RoomID, opType collaboration.OpType) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	key := opLimiterKey{userID: userID, roomID: roomID, opType: opType}

	s.mu.Lock()
	lim, ok := s.opLimiters[key]
	if !ok {
		lim = s.newOpLimiter(opType)
		s.opLimiters[key] = lim
	}
	s.mu.Unlock()

	return lim.Allow(), nil
}

func (s *ThrottleService) newOpLimiter(opType collaboration.OpType) *TokenBucket {
	switch opType {
	case collaboration.OpChat:
		// 10 chat messages per minute
		return NewTokenBucket(10.0/60.0, 5)
	case collaboration.OpAwareness:
		// Smooth cursors at up to 60 updates/second, with room for short bursts.
		return NewTokenBucket(60, 120)
	default:
		// Realtime drawing diffs are batched once per animation frame. Leave
		// headroom for high-refresh-rate devices and reconnect catch-up.
		return NewTokenBucket(120, 240)
	}
}

// CleanupExpired removes stale limiters to prevent memory leaks.
// Should be called periodically (e.g., every 5 minutes).
func (s *ThrottleService) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean limiters that have refilled to their full burst capacity.
	for uid, lim := range s.connLimiters {
		if lim.isFull() {
			delete(s.connLimiters, uid)
		}
	}
	for roomID, lim := range s.roomConnLimiters {
		if lim.isFull() {
			delete(s.roomConnLimiters, roomID)
		}
	}
	for key, lim := range s.opLimiters {
		if lim.isFull() {
			delete(s.opLimiters, key)
		}
	}
}

// Ensure it satisfies the interface.
var _ collaboration.ThrottleService = (*ThrottleService)(nil)
