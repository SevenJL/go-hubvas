package ws

import (
	"encoding/json"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// Message is the JSON envelope for all WebSocket communication.
// CRDT binary payloads are base64-encoded inside the Payload field
// or sent as separate binary frames.
type Message struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Message types matching the protocol spec.
const (
	MsgTypeSync      = "sync"
	MsgTypeAwareness = "awareness"
	MsgTypePresence  = "presence"
	MsgTypeChat      = "chat"
	MsgTypeAck       = "ack"
	MsgTypeError     = "error"
)

// SyncPayload is the inner payload for sync messages (CRDT updates).
type SyncPayload struct {
	Update string `json:"update"` // base64-encoded Yjs binary update
}

// AwarenessPayload is the inner payload for awareness messages.
type AwarenessPayload struct {
	Cursor     *CursorData `json:"cursor,omitempty"`
	Selection  *SelectionData `json:"selection,omitempty"`
	EditingObj string      `json:"editing_obj,omitempty"`
}

// CursorData represents cursor coordinates.
type CursorData struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SelectionData represents selection bounds.
type SelectionData struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PresencePayload is the inner payload for presence messages (downstream only).
type PresencePayload struct {
	Online []PresenceMember `json:"online"`
}

// PresenceMember is a single member in the online list.
type PresenceMember struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role"`
}

// ChatPayload is the inner payload for chat messages.
type ChatPayload struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Content  string `json:"content"`
}

// AckPayload is the inner payload for acknowledgement messages.
type AckPayload struct {
	Seq int64 `json:"seq"`
}

// ErrorPayload is the inner payload for error messages.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---- Helpers ----

// NewErrorMessage creates an error message envelope.
func NewErrorMessage(code, message string) Message {
	payload, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
	return Message{
		Type:    MsgTypeError,
		Payload: payload,
	}
}

// NewAckMessage creates an ack message envelope.
func NewAckMessage(seq int64) Message {
	payload, _ := json.Marshal(AckPayload{Seq: seq})
	return Message{
		Type:    MsgTypeAck,
		Payload: payload,
	}
}

// ToOperation converts a Message to a domain Operation.
func ToOperation(msg Message, userID identity.UserID) collaboration.Operation {
	return collaboration.Operation{
		Type:    collaboration.OpType(msg.Type),
		UserID:  userID,
		Seq:     msg.Seq,
		Payload: msg.Payload,
	}
}

// FromOperation converts a domain Operation to a Message for broadcasting.
func FromOperation(op collaboration.Operation) Message {
	return Message{
		Type:    string(op.Type),
		Seq:     op.Seq,
		Payload: op.Payload,
	}
}
