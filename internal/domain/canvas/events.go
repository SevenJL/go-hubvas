package canvas

import (
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CanvasCreatedEvent is fired when a new canvas is created.
type CanvasCreatedEvent struct {
	shared.BaseEvent
	CanvasID CanvasID
	OwnerID  identity.UserID
}

// CanvasMemberAddedEvent is fired when a user is added to a canvas.
type CanvasMemberAddedEvent struct {
	shared.BaseEvent
	CanvasID CanvasID
	UserID   identity.UserID
	Role     Role
}

// CanvasMemberRemovedEvent is fired when a user is removed from a canvas.
type CanvasMemberRemovedEvent struct {
	shared.BaseEvent
	CanvasID CanvasID
	UserID   identity.UserID
}

// CanvasPublishedEvent is fired when a canvas is published to the community.
// This event triggers the Community context to create a PublishedCanvas projection.
type CanvasPublishedEvent struct {
	shared.BaseEvent
	CanvasID CanvasID
}

// CanvasForkedEvent is fired when a canvas is forked.
type CanvasForkedEvent struct {
	shared.BaseEvent
	OriginalID CanvasID
	NewID      CanvasID
	UserID     identity.UserID
}
