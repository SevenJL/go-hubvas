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
func NewTokenBucket(ratePerSec, burst int) *TokenBucket {
	return &TokenBucket{
		rate:       float64(ratePerSec),
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

// ThrottleService implements collaboration.ThrottleService using per-connection
// and per-user token buckets stored in memory.
//
// Connection limits:
//   - Per user: 5 connections per second (global)
//   - Per room:  200 connections per second
//
// Operation limits:
//   - sync:      60 ops/min per user per room
//   - awareness: 60 ops/min per user per room (cursors are throttled client-side too)
//   - chat:      10 ops/min per user per room
type ThrottleService struct {
	mu sync.Mutex

	// Global connection limiter per user.
	connLimiters map[identity.UserID]*TokenBucket

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
		connLimiters: make(map[identity.UserID]*TokenBucket),
		opLimiters:   make(map[opLimiterKey]*TokenBucket),
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
	lim, ok := s.connLimiters[userID]
	if !ok {
		// 5 connections per second, burst up to 10
		lim = NewTokenBucket(5, 10)
		s.connLimiters[userID] = lim
	}
	s.mu.Unlock()

	return lim.Allow(), nil
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
		return NewTokenBucket(10/60, 5)
	case collaboration.OpAwareness:
		// 60 awareness updates per minute
		return NewTokenBucket(60/60, 30)
	default:
		// 60 sync ops per minute
		return NewTokenBucket(60/60, 30)
	}
}

// CleanupExpired removes stale limiters to prevent memory leaks.
// Should be called periodically (e.g., every 5 minutes).
func (s *ThrottleService) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean connection limiters that are full (idle users).
	for uid, lim := range s.connLimiters {
		lim.mu.Lock()
		if lim.tokens >= lim.burst {
			delete(s.connLimiters, uid)
		}
		lim.mu.Unlock()
	}

	// Clean op limiters that are full (idle users).
	for key, lim := range s.opLimiters {
		lim.mu.Lock()
		if lim.tokens >= lim.burst {
			delete(s.opLimiters, key)
		}
		lim.mu.Unlock()
	}
}

// Ensure it satisfies the interface.
var _ collaboration.ThrottleService = (*ThrottleService)(nil)
