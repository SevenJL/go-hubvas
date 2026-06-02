package shared

import "errors"

// Domain-level error sentinels. Each bounded context may extend these.
var (
	ErrNotFound       = errors.New("domain: entity not found")
	ErrAlreadyExists  = errors.New("domain: entity already exists")
	ErrInvalidArgument = errors.New("domain: invalid argument")
	ErrUnauthorized   = errors.New("domain: unauthorized")
	ErrForbidden      = errors.New("domain: forbidden")
	ErrConflict       = errors.New("domain: conflict")
	ErrLimitExceeded  = errors.New("domain: limit exceeded")
)

// DomainError wraps a sentinel error with a contextual message.
type DomainError struct {
	Err     error
	Message string
}

func (e *DomainError) Error() string {
	return e.Message + ": " + e.Err.Error()
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new DomainError wrapping the given sentinel.
func NewDomainError(err error, message string) *DomainError {
	return &DomainError{Err: err, Message: message}
}
