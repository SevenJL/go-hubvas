package identity

import "github.com/hubvas/internal/domain/shared"

// UserRegisteredEvent is fired when a new user completes registration.
type UserRegisteredEvent struct {
	shared.BaseEvent
	UserID   UserID
	Username string
	Email    string
}

// UserLoggedInEvent is fired when a user successfully authenticates.
type UserLoggedInEvent struct {
	shared.BaseEvent
	UserID UserID
}
