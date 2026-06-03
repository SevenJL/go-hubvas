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
