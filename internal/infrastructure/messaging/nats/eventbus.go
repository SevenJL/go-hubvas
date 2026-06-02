package nats

import (
	"sync"

	"github.com/hubvas/internal/domain/shared"
)

// EventBus implements shared.EventBus using NATS for distributed event delivery.
// It also supports in-process subscribers for same-process event handling.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]func(shared.DomainEvent) error
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]func(shared.DomainEvent) error),
	}
}

// Publish sends a domain event to all subscribers.
// In production, this also publishes to NATS for cross-service consumption.
func (eb *EventBus) Publish(event shared.DomainEvent) error {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	name := event.EventName()
	if handlers, ok := eb.subscribers[name]; ok {
		for _, h := range handlers {
			// Fire-and-forget per handler; production code would handle errors.
			_ = h(event)
		}
	}

	// TODO: Also publish to NATS subject "events.{EventName}" for
	// cross-context subscribers (e.g., CanvasPublished → Community).

	return nil
}

// Subscribe registers a handler for a specific event name.
func (eb *EventBus) Subscribe(eventName string, handler func(shared.DomainEvent) error) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventName] = append(eb.subscribers[eventName], handler)
	return nil
}
