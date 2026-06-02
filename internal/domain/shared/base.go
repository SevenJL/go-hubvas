// Package shared provides base types for the domain layer following DDD patterns.
// All domain aggregates, entities, and value objects build on these primitives.
package shared

import (
	"time"
)

// AggregateRoot is the base type for all aggregate roots.
// Each aggregate root owns a unique identifier and tracks domain events.
type AggregateRoot struct {
	events []DomainEvent
}

// AddEvent records a domain event that occurred within the aggregate.
func (a *AggregateRoot) AddEvent(event DomainEvent) {
	a.events = append(a.events, event)
}

// Events returns all recorded domain events and clears the internal buffer.
func (a *AggregateRoot) Events() []DomainEvent {
	events := a.events
	a.events = nil
	return events
}

// HasEvents returns true if there are uncommitted domain events.
func (a *AggregateRoot) HasEvents() bool {
	return len(a.events) > 0
}

// Entity is a base type for domain entities that have a distinct identity.
type Entity[ID any] struct {
	ID ID
}

// ValueObject is a marker interface for immutable value objects.
// Value objects have no identity and are compared by their properties.
type ValueObject interface {
	Equals(other ValueObject) bool
}

// DomainEvent is the base interface for all domain events.
// Every event carries a timestamp and an event name for routing.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// BaseEvent provides a common implementation of DomainEvent.
type BaseEvent struct {
	Name      string
	Timestamp time.Time
}

// EventName returns the name of the event.
func (e BaseEvent) EventName() string { return e.Name }

// OccurredAt returns the time the event occurred.
func (e BaseEvent) OccurredAt() time.Time { return e.Timestamp }

// NewBaseEvent creates a new BaseEvent with the given name and the current timestamp.
func NewBaseEvent(name string) BaseEvent {
	return BaseEvent{Name: name, Timestamp: time.Now()}
}

// EventBus defines the contract for publishing domain events.
// It is implemented by infrastructure (e.g., NATS).
type EventBus interface {
	Publish(event DomainEvent) error
	Subscribe(eventName string, handler func(DomainEvent) error) error
}
