package nats

import (
	"fmt"

	"github.com/hubvas/internal/domain/collaboration"
)

// PubSub implements cross-node fan-out for Room operations using NATS.
// Each Room subscribes to subject "canvas.{canvasID}" and publishes ops
// so that instances on different nodes can relay to their local members.
type PubSub struct {
	// conn *nats.Conn — to be wired in during implementation.
}

// NewPubSub creates a new PubSub.
func NewPubSub( /* conn *nats.Conn */ ) *PubSub {
	return &PubSub{}
}

// Publish sends an operation to all other nodes hosting the same room.
func (ps *PubSub) Publish(canvasID collaboration.RoomID, op collaboration.Operation) error {
	subject := subject(canvasID)
	_ = subject // TODO: publish to NATS subject
	return nil
}

// Subscribe listens for operations from other nodes and forwards them
// to the local Room's inbound channel.
func (ps *PubSub) Subscribe(canvasID collaboration.RoomID, onOp func(collaboration.Operation)) error {
	subject := subject(canvasID)
	_ = subject // TODO: subscribe to NATS subject
	return nil
}

// Unsubscribe stops listening for the given room.
func (ps *PubSub) Unsubscribe(canvasID collaboration.RoomID) error {
	return nil
}

func subject(canvasID collaboration.RoomID) string {
	return fmt.Sprintf("canvas.%d", canvasID)
}
