package nats

import (
	"encoding/json"
	"fmt"
	"sync"

	natsgo "github.com/nats-io/nats.go"

	"github.com/hubvas/internal/domain/collaboration"
	"github.com/hubvas/internal/domain/identity"
)

// PubSub implements cross-node fan-out using NATS.
//
// When a local Room processes an operation, it calls Publish to fan it out
// to other nodes hosting the same Room. Subscribe listens for ops from
// other nodes and routes them into the local Room's inbound channel.
//
// Subject: canvas.{canvasID}
type PubSub struct {
	conn *natsgo.Conn

	mu   sync.Mutex
	subs map[collaboration.RoomID]*natsgo.Subscription
}

// NewPubSub creates a PubSub backed by a NATS connection.
func NewPubSub(conn *natsgo.Conn) *PubSub {
	return &PubSub{
		conn: conn,
		subs: make(map[collaboration.RoomID]*natsgo.Subscription),
	}
}

// pubSubEnvelope wraps an Operation for JSON serialization over NATS.
type pubSubEnvelope struct {
	Type      string `json:"type"`
	UserID    int64  `json:"user_id"`
	Seq       int64  `json:"seq"`
	Payload   []byte `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

// Publish sends an operation to all other nodes hosting the same room.
func (ps *PubSub) Publish(canvasID collaboration.RoomID, op collaboration.Operation) error {
	env := pubSubEnvelope{
		Type:      string(op.Type),
		UserID:    int64(op.UserID),
		Seq:       op.Seq,
		Payload:   op.Payload,
		Timestamp: op.Timestamp,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("nats: marshal op: %w", err)
	}
	return ps.conn.Publish(subject(canvasID), data)
}

// Subscribe listens for operations from other nodes. When an op arrives,
// onOp is called — the caller should route it to the local Room's inbound.
func (ps *PubSub) Subscribe(canvasID collaboration.RoomID, onOp func(collaboration.Operation)) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Already subscribed.
	if _, ok := ps.subs[canvasID]; ok {
		return nil
	}

	sub, err := ps.conn.Subscribe(subject(canvasID), func(msg *natsgo.Msg) {
		var env pubSubEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			return // Skip malformed messages.
		}
		onOp(collaboration.Operation{
			Type:      collaboration.OpType(env.Type),
			UserID:    identity.UserID(env.UserID),
			Seq:       env.Seq,
			Payload:   env.Payload,
			Timestamp: env.Timestamp,
		})
	})
	if err != nil {
		return fmt.Errorf("nats: subscribe canvas %d: %w", canvasID, err)
	}

	ps.subs[canvasID] = sub
	return nil
}

// Unsubscribe stops listening for the given room.
func (ps *PubSub) Unsubscribe(canvasID collaboration.RoomID) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	sub, ok := ps.subs[canvasID]
	if !ok {
		return nil
	}
	delete(ps.subs, canvasID)
	return sub.Unsubscribe()
}

// Close drains all subscriptions.
func (ps *PubSub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for id, sub := range ps.subs {
		sub.Unsubscribe()
		delete(ps.subs, id)
	}
}

func subject(canvasID collaboration.RoomID) string {
	return fmt.Sprintf("canvas.%d", canvasID)
}
