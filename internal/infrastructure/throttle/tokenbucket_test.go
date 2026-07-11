package throttle

import (
	"context"
	"testing"
	"time"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

func TestTokenBucket_Allow(t *testing.T) {
	b := NewTokenBucket(10, 5)

	// First 5 should all pass (full burst).
	for i := 0; i < 5; i++ {
		if !b.Allow() {
			t.Fatalf("expected allow on call %d", i+1)
		}
	}

	// 6th should fail — burst exhausted.
	if b.Allow() {
		t.Fatal("expected deny after burst exhausted")
	}

	// After refilling (simulate time passing).
	time.Sleep(110 * time.Millisecond) // ~1 token at 10/s
	if !b.Allow() {
		t.Fatal("expected allow after refill")
	}
}

func TestThrottleService_AllowConnection(t *testing.T) {
	svc := NewThrottleService()
	ctx := context.Background()
	uid := identity.UserID(1)
	rid := collaboration.RoomID(1001)

	// First 10 connections should be allowed (burst).
	for i := 0; i < 10; i++ {
		allowed, err := svc.AllowConnection(ctx, uid, rid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("expected allow on connection %d", i+1)
		}
	}

	// 11th should fail.
	allowed, _ := svc.AllowConnection(ctx, uid, rid)
	if allowed {
		t.Fatal("expected deny after connection burst exhausted")
	}
}

func TestThrottleService_AllowOperation(t *testing.T) {
	svc := NewThrottleService()
	ctx := context.Background()
	uid := identity.UserID(1)
	rid := collaboration.RoomID(1001)

	// Chat ops have burst=5.
	for i := 0; i < 5; i++ {
		allowed, err := svc.AllowOperation(ctx, uid, rid, collaboration.OpChat)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Fatalf("expected allow on chat %d", i+1)
		}
	}

	// 6th chat should fail.
	allowed, _ := svc.AllowOperation(ctx, uid, rid, collaboration.OpChat)
	if allowed {
		t.Fatal("expected deny after chat burst exhausted")
	}
}

func TestThrottleService_DifferentUsersIndependent(t *testing.T) {
	svc := NewThrottleService()
	ctx := context.Background()
	rid := collaboration.RoomID(1001)

	// User 1 exhausts burst.
	for i := 0; i < 10; i++ {
		svc.AllowConnection(ctx, identity.UserID(1), rid)
	}

	// User 2 should still be allowed.
	allowed, _ := svc.AllowConnection(ctx, identity.UserID(2), rid)
	if !allowed {
		t.Fatal("expected user 2 to be independent of user 1 limit")
	}
}

func TestThrottleService_CleanupExpired(t *testing.T) {
	svc := NewThrottleService()
	ctx := context.Background()
	uid := identity.UserID(1)
	rid := collaboration.RoomID(1001)

	// Use one token (not full burst, so it stays).
	svc.AllowConnection(ctx, uid, rid)

	// Create a full limiter separately — insert directly.
	svc.mu.Lock()
	fullLim := NewTokenBucket(5, 10)
	fullLim.tokens = fullLim.burst // mark as full
	svc.connLimiters[identity.UserID(999)] = fullLim
	svc.mu.Unlock()

	svc.CleanupExpired()

	// The partially-used limiter should remain.
	svc.mu.Lock()
	_, partialExists := svc.connLimiters[uid]
	_, fullExists := svc.connLimiters[identity.UserID(999)]
	svc.mu.Unlock()

	if !partialExists {
		t.Fatal("partial limiter should not be cleaned")
	}
	if fullExists {
		t.Fatal("full limiter should be cleaned")
	}
}

func TestThrottleService_ContextCancellation(t *testing.T) {
	svc := NewThrottleService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.AllowConnection(ctx, identity.UserID(1), collaboration.RoomID(1001))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestThrottleService_ChatLimiterRefills(t *testing.T) {
	svc := NewThrottleService()
	lim := svc.newOpLimiter(collaboration.OpChat)
	if lim.rate <= 0 {
		t.Fatalf("chat limiter must refill, got rate %f", lim.rate)
	}
	if lim.rate != 10.0/60.0 {
		t.Fatalf("expected 10 messages per minute, got %f tokens/second", lim.rate)
	}
}

func TestThrottleService_RoomConnectionLimit(t *testing.T) {
	svc := NewThrottleService()
	ctx := context.Background()
	roomID := collaboration.RoomID(1001)

	for i := 0; i < 400; i++ {
		allowed, err := svc.AllowConnection(ctx, identity.UserID(i+1), roomID)
		if err != nil || !allowed {
			t.Fatalf("expected room burst connection %d to pass: allowed=%v err=%v", i+1, allowed, err)
		}
	}
	allowed, err := svc.AllowConnection(ctx, identity.UserID(401), roomID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected room connection burst to be enforced")
	}
}

func TestThrottleService_CleanupRefillsIdleLimiter(t *testing.T) {
	svc := NewThrottleService()
	uid := identity.UserID(1)
	lim := NewTokenBucket(10, 1)
	if !lim.Allow() {
		t.Fatal("expected initial token")
	}
	lim.mu.Lock()
	lim.lastRefill = time.Now().Add(-time.Second)
	lim.mu.Unlock()

	svc.mu.Lock()
	svc.connLimiters[uid] = lim
	svc.mu.Unlock()
	svc.CleanupExpired()

	svc.mu.Lock()
	_, exists := svc.connLimiters[uid]
	svc.mu.Unlock()
	if exists {
		t.Fatal("idle limiter should be removed after refilling")
	}
}

func TestThrottleService_RealtimeLimiterCapacity(t *testing.T) {
	svc := NewThrottleService()
	syncLimiter := svc.newOpLimiter(collaboration.OpSync)
	if syncLimiter.rate < 120 || syncLimiter.burst < 240 {
		t.Fatalf("sync limiter is too restrictive for frame batches: rate=%v burst=%v", syncLimiter.rate, syncLimiter.burst)
	}
	awarenessLimiter := svc.newOpLimiter(collaboration.OpAwareness)
	if awarenessLimiter.rate < 60 || awarenessLimiter.burst < 120 {
		t.Fatalf("awareness limiter is too restrictive for smooth cursors: rate=%v burst=%v", awarenessLimiter.rate, awarenessLimiter.burst)
	}
}
