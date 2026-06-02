package collaboration

import (
	"time"

	"github.com/hubvas/internal/domain/identity"
)

// RoomMember represents an online connection within a Room.
// It is NOT the same as a CanvasMember (which stores persistent permissions);
// RoomMember represents the live WebSocket session.
type RoomMember struct {
	UserID    identity.UserID
	Username  string
	AvatarURL string
	Cursor    *CursorPosition
	Selection *Selection
	JoinedAt  time.Time
}

// NewRoomMember creates a new room member.
func NewRoomMember(roomID RoomID, userID identity.UserID, username string) *RoomMember {
	return &RoomMember{
		UserID:   userID,
		Username: username,
		JoinedAt: time.Now(),
	}
}

// UpdateCursor updates the member's cursor position.
func (m *RoomMember) UpdateCursor(x, y float64) {
	m.Cursor = &CursorPosition{X: x, Y: y}
}

// UpdateSelection updates the member's current selection.
func (m *RoomMember) UpdateSelection(x, y, w, h float64) {
	m.Selection = &Selection{X: x, Y: y, Width: w, Height: h}
}

// ToPresenceInfo converts the room member to a presence payload.
func (m *RoomMember) ToPresenceInfo() PresenceInfo {
	return PresenceInfo{
		UserID:    m.UserID,
		Username:  m.Username,
		AvatarURL: m.AvatarURL,
		Cursor:    m.Cursor,
		Selection: m.Selection,
	}
}
