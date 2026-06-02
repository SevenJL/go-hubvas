package collaboration

import (
	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// RoomID is a strongly-typed room identifier. It equals the CanvasID.
type RoomID = canvas.CanvasID

// OpType enumerates the kinds of operations flowing through a Room.
type OpType string

const (
	OpSync      OpType = "sync"      // CRDT document update
	OpAwareness OpType = "awareness" // Cursor, selection, editing-object
	OpPresence  OpType = "presence"  // Member join/leave, online list
	OpChat      OpType = "chat"      // Room chat message
	OpAck       OpType = "ack"       // Acknowledgement
	OpError     OpType = "error"     // Error notification
)

func (t OpType) IsValid() bool {
	switch t {
	case OpSync, OpAwareness, OpPresence, OpChat, OpAck, OpError:
		return true
	default:
		return false
	}
}

// RoomStatus represents the lifecycle state of a Room.
type RoomStatus int8

const (
	RoomStatusActive   RoomStatus = 0
	RoomStatusIdle     RoomStatus = 1
	RoomStatusDraining RoomStatus = 2
)

func (s RoomStatus) String() string {
	switch s {
	case RoomStatusActive:
		return "active"
	case RoomStatusIdle:
		return "idle"
	case RoomStatusDraining:
		return "draining"
	default:
		return "unknown"
	}
}

// CursorPosition is a value object representing a user's cursor coordinates.
type CursorPosition struct {
	X float64
	Y float64
}

// Selection represents a user's selection bounds on the canvas.
type Selection struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// PresenceInfo is the presence data broadcast for a single member.
type PresenceInfo struct {
	UserID      identity.UserID
	Username    string
	AvatarURL   string
	Role        canvas.Role
	Cursor      *CursorPosition
	Selection   *Selection
	EditingObj  string // ID of the object being edited
}

// Operation is a message flowing through the Room's inbound channel.
type Operation struct {
	Type      OpType
	UserID    identity.UserID
	Seq       int64               // Client-assigned sequence number for ack
	Payload   []byte              // Message body (JSON or binary for CRDT updates)
	Timestamp int64               // Unix millis
}

// LockInfo tracks which object is locked by whom.
type LockInfo struct {
	ObjectID string
	UserID   identity.UserID
}
