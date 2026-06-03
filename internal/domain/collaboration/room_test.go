package collaboration

import (
	"testing"
	"time"

	"github.com/hubvas/internal/domain/identity"
)

func TestNewRoom(t *testing.T) {
	id := RoomID(1)
	r := NewRoom(id, nil)

	if r.ID() != id {
		t.Fatalf("expected id %d, got %d", id, r.ID())
	}
	if r.Version() != 0 {
		t.Fatalf("expected version 0, got %d", r.Version())
	}
	if r.Status() != RoomStatusActive {
		t.Fatalf("expected status active, got %s", r.Status())
	}
	if r.MemberCount() != 0 {
		t.Fatalf("expected 0 members, got %d", r.MemberCount())
	}
}

func TestRoom_Join(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)
	username := "alice"

	member := r.Join(uid, username)

	if member == nil {
		t.Fatal("expected non-nil member")
	}
	if member.UserID != uid {
		t.Fatalf("expected userID %d, got %d", uid, member.UserID)
	}
	if member.Username != username {
		t.Fatalf("expected username %s, got %s", username, member.Username)
	}
	if r.MemberCount() != 1 {
		t.Fatalf("expected 1 member, got %d", r.MemberCount())
	}
	if !r.HasEvents() {
		t.Fatal("expected domain events after join")
	}
}

func TestRoom_JoinDuplicate(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)

	r.Join(uid, "alice")
	r.Join(uid, "alice2") // Same user, new connection

	if r.MemberCount() != 1 {
		t.Fatalf("expected 1 member after duplicate join, got %d", r.MemberCount())
	}
	if m := r.FindMember(uid); m == nil || m.Username != "alice2" {
		t.Fatal("expected username to be updated to 'alice2'")
	}
}

func TestRoom_Leave(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)

	r.Join(uid, "alice")
	err := r.Leave(uid)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.MemberCount() != 0 {
		t.Fatalf("expected 0 members, got %d", r.MemberCount())
	}
	// Locks held by the user should be released.
	r.LockObject("obj1", uid)
	r.Leave(uid) // Should not error even if already left (but leave was called, so adding lock, then leaving again)
}

func TestRoom_LeaveUnknownMember(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	err := r.Leave(identity.UserID(999))
	if err == nil {
		t.Fatal("expected error for unknown member")
	}
}

func TestRoom_LockObject(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid1 := identity.UserID(1)
	uid2 := identity.UserID(2)

	// First lock succeeds.
	err := r.LockObject("obj1", uid1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsLocked("obj1") {
		t.Fatal("expected obj1 to be locked")
	}
	if !r.IsLockedBy("obj1", uid1) {
		t.Fatal("expected obj1 locked by uid1")
	}
	if r.IsLockedBy("obj1", uid2) {
		t.Fatal("expected obj1 NOT locked by uid2")
	}

	// Another user cannot lock the same object.
	err = r.LockObject("obj1", uid2)
	if err == nil {
		t.Fatal("expected conflict error for already-locked object")
	}

	// But the same user can re-lock (idempotent).
	err = r.LockObject("obj1", uid1)
	if err != nil {
		t.Fatalf("unexpected error on re-lock: %v", err)
	}
}

func TestRoom_UnlockObject(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid1 := identity.UserID(1)
	uid2 := identity.UserID(2)

	r.LockObject("obj1", uid1)
	r.UnlockObject("obj1", uid2) // Wrong user — no-op.
	if !r.IsLocked("obj1") {
		t.Fatal("expected obj1 still locked after wrong-user unlock")
	}

	r.UnlockObject("obj1", uid1) // Correct user.
	if r.IsLocked("obj1") {
		t.Fatal("expected obj1 unlocked")
	}
}

func TestRoom_LeaveReleasesLocks(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(1)

	r.Join(uid, "alice")
	r.LockObject("obj1", uid)
	r.LockObject("obj2", uid)

	r.Leave(uid)

	if r.IsLocked("obj1") || r.IsLocked("obj2") {
		t.Fatal("expected all locks released after member leaves")
	}
}

func TestRoom_ProcessOp_Sync(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)

	r.Join(uid, "alice")

	op := Operation{
		Type:   OpSync,
		UserID: uid,
		Seq:    1,
		Payload: []byte(`{"update":"test"}`),
	}

	result, err := r.ProcessOp(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected broadcast result for sync op")
	}
	if result.Target != BroadcastAll {
		t.Fatalf("expected BroadcastAll, got %d", result.Target)
	}
	if result.ExcludeUserID == nil || *result.ExcludeUserID != uid {
		t.Fatal("expected sender to be excluded from broadcast")
	}
	// Version should be bumped.
	if r.Version() != 1 {
		t.Fatalf("expected version 1 after sync, got %d", r.Version())
	}
}

func TestRoom_ProcessOp_Awareness(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)

	op := Operation{Type: OpAwareness, UserID: uid, Payload: []byte(`{"cursor":{"x":10,"y":20}}`)}

	result, err := r.ProcessOp(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Target != BroadcastAll {
		t.Fatal("expected broadcast to all for awareness")
	}
	if result.ExcludeUserID == nil || *result.ExcludeUserID != uid {
		t.Fatal("expected sender excluded for awareness")
	}
}

func TestRoom_ProcessOp_Chat(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	uid := identity.UserID(100)

	op := Operation{Type: OpChat, UserID: uid, Payload: []byte(`{"content":"hello"}`)}

	result, err := r.ProcessOp(op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Target != BroadcastAll {
		t.Fatal("expected broadcast to all for chat")
	}
	// Chat should NOT exclude the sender so they see their own message.
	if result.ExcludeUserID != nil {
		t.Fatal("expected chat NOT to exclude sender")
	}
}

func TestRoom_ProcessOp_UnknownType(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	op := Operation{Type: OpType("unknown")}
	_, err := r.ProcessOp(op)
	if err == nil {
		t.Fatal("expected error for unknown op type")
	}
}

func TestRoom_IsIdle(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	// A freshly created room with no members should go idle after the timeout.
	if !r.IsIdle(time.Nanosecond) {
		t.Fatal("expected room to be idle immediately with nanosecond timeout")
	}

	// Join a member — room becomes active.
	r.Join(identity.UserID(1), "alice")
	if r.IsIdle(10 * time.Millisecond) {
		t.Fatal("expected room NOT to be idle with active member")
	}
}

func TestRoom_UpdateSnapshot(t *testing.T) {
	r := NewRoom(RoomID(1), nil)
	data := []byte("snapshot data")

	r.UpdateSnapshot(data)
	if string(r.Snapshot()) != string(data) {
		t.Fatal("expected snapshot to be updated")
	}
	if r.Version() != 1 {
		t.Fatalf("expected version 1 after snapshot update, got %d", r.Version())
	}
}
