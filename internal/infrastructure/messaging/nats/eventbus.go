package nats

import (
	"encoding/json"
	"log"
	"sync"

	natsgo "github.com/nats-io/nats.go"

	"github.com/hubvas/internal/domain/shared"
)

// EventBus implements shared.EventBus using NATS for distributed event delivery
// with a fallback to in-process subscribers for same-process event handling.
//
// Usage in api-server:
//
//	eb := nats.NewEventBus(nc)  // nc is an optional NATS connection
//	eb.Subscribe("CanvasPublished", func(e shared.DomainEvent) error {
//	    // Create PublishedCanvas projection in community context.
//	})
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]func(shared.DomainEvent) error

	// NATS connection for cross-service event distribution. Nil if NATS is unavailable.
	nc *natsgo.Conn
}

// NewEventBus creates a new EventBus. Pass an optional NATS connection for
// cross-service event distribution. If nc is nil, events are only delivered
// to in-process subscribers.
func NewEventBus(nc *natsgo.Conn) *EventBus {
	return &EventBus{
		subscribers: make(map[string][]func(shared.DomainEvent) error),
		nc:          nc,
	}
}

// Publish sends a domain event to all in-process subscribers and,
// if configured, publishes to NATS for cross-service consumption.
func (eb *EventBus) Publish(event shared.DomainEvent) error {
	name := event.EventName()

	// 1. In-process subscribers.
	eb.mu.RLock()
	handlers := eb.subscribers[name]
	eb.mu.RUnlock()

	for _, h := range handlers {
		if err := h(event); err != nil {
			log.Printf("[eventbus] handler error for %s: %v", name, err)
		}
	}

	// 2. NATS cross-service publish.
	if eb.nc != nil {
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("[eventbus] marshal error for %s: %v", name, err)
			return nil // Don't fail on marshal errors — in-process already delivered.
		}
		subject := "events." + name
		if err := eb.nc.Publish(subject, data); err != nil {
			log.Printf("[eventbus] nats publish error for %s: %v", name, err)
		}
	}

	return nil
}

// Subscribe registers an in-process handler for a specific event name.
func (eb *EventBus) Subscribe(eventName string, handler func(shared.DomainEvent) error) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventName] = append(eb.subscribers[eventName], handler)

	// Also subscribe on NATS for cross-service events (if connected).
	if eb.nc != nil {
		subject := "events." + eventName
		_, err := eb.nc.Subscribe(subject, func(msg *natsgo.Msg) {
			// We don't need to deserialize here — NATS subscriptions for
			// cross-service consumers should be set up separately.
			// This is a best-effort relay; the handler receives a raw message.
			_ = msg
		})
		if err != nil {
			log.Printf("[eventbus] nats subscribe error for %s: %v", subject, err)
		}
	}

	return nil
}

// SubscribeNATS subscribes to cross-service events on NATS.
// This is used when one service wants to react to events from another service.
func (eb *EventBus) SubscribeNATS(eventName string, handler func([]byte) error) error {
	if eb.nc == nil {
		return nil // NATS not available — silently skip.
	}

	subject := "events." + eventName
	_, err := eb.nc.Subscribe(subject, func(msg *natsgo.Msg) {
		if err := handler(msg.Data); err != nil {
			log.Printf("[eventbus] nats handler error for %s: %v", eventName, err)
		}
	})
	return err
}

// Close cleans up NATS subscriptions if any.
func (eb *EventBus) Close() {
	if eb.nc != nil {
		eb.nc.Drain()
	}
}

// Ensure it satisfies the interface.
var _ shared.EventBus = (*EventBus)(nil)
