package collaboration

import (
	"testing"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

func TestOpType_IsValid(t *testing.T) {
	tests := []struct {
		opType OpType
		valid  bool
	}{
		{OpSync, true},
		{OpAwareness, true},
		{OpPresence, true},
		{OpChat, true},
		{OpAck, true},
		{OpError, true},
		{OpType("unknown"), false},
		{OpType(""), false},
		{OpType("draw"), false},
	}

	for _, tt := range tests {
		if got := tt.opType.IsValid(); got != tt.valid {
			t.Errorf("OpType(%q).IsValid() = %v, want %v", tt.opType, got, tt.valid)
		}
	}
}

func TestRoomStatus_String(t *testing.T) {
	tests := []struct {
		status RoomStatus
		str    string
	}{
		{RoomStatusActive, "active"},
		{RoomStatusIdle, "idle"},
		{RoomStatusDraining, "draining"},
		{RoomStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.str {
			t.Errorf("RoomStatus(%d).String() = %q, want %q", tt.status, got, tt.str)
		}
	}
}

func TestNewRoomMember(t *testing.T) {
	m := NewRoomMember(RoomID(1), identity.UserID(10), "alice")
	if m.UserID != 10 {
		t.Fatalf("expected user id 10, got %d", m.UserID)
	}
	if m.Username != "alice" {
		t.Fatalf("expected username 'alice', got '%s'", m.Username)
	}
	if m.JoinedAt.IsZero() {
		t.Fatal("expected non-zero joined time")
	}
	if m.Cursor != nil {
		t.Fatal("expected nil initial cursor")
	}
	if m.Selection != nil {
		t.Fatal("expected nil initial selection")
	}
}

func TestRoomMember_UpdateCursor(t *testing.T) {
	m := NewRoomMember(RoomID(1), 10, "alice")
	m.UpdateCursor(100.5, 200.3)

	if m.Cursor == nil {
		t.Fatal("expected non-nil cursor after update")
	}
	if m.Cursor.X != 100.5 || m.Cursor.Y != 200.3 {
		t.Fatalf("expected cursor (100.5, 200.3), got (%f, %f)", m.Cursor.X, m.Cursor.Y)
	}
}

func TestRoomMember_UpdateSelection(t *testing.T) {
	m := NewRoomMember(RoomID(1), 10, "alice")
	m.UpdateSelection(10, 20, 100, 50)

	if m.Selection == nil {
		t.Fatal("expected non-nil selection after update")
	}
	if m.Selection.X != 10 || m.Selection.Y != 20 ||
		m.Selection.Width != 100 || m.Selection.Height != 50 {
		t.Fatalf("selection mismatch: %+v", m.Selection)
	}
}

func TestRoomMember_ToPresenceInfo(t *testing.T) {
	m := NewRoomMember(RoomID(1), 10, "alice")
	m.UpdateCursor(10, 20)

	info := m.ToPresenceInfo()
	if info.UserID != 10 {
		t.Fatalf("expected user id 10, got %d", info.UserID)
	}
	if info.Username != "alice" {
		t.Fatalf("expected username 'alice', got '%s'", info.Username)
	}
	if info.Cursor == nil || info.Cursor.X != 10 || info.Cursor.Y != 20 {
		t.Fatal("cursor info mismatch")
	}
}

func TestPresenceInfo_ZeroValues(t *testing.T) {
	info := PresenceInfo{
		UserID:   identity.UserID(1),
		Username: "bob",
	}
	if info.Cursor != nil {
		t.Fatal("expected nil cursor for zero-value PresenceInfo")
	}
	if info.Role != canvas.Role(0) {
		t.Fatal("expected zero-value role")
	}
}
