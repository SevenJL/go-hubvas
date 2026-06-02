package canvas

import (
	"time"
	"unicode/utf8"

	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// Canvas is the aggregate root for the Canvas bounded context.
// It owns the metadata, member list, and publishing state for a drawing canvas.
// The actual graphical data lives in CRDT snapshots stored in object storage.
type Canvas struct {
	shared.AggregateRoot
	id         CanvasID
	ownerID    identity.UserID
	title      string
	snapshotKey SnapshotKey
	visibility Visibility
	forkedFrom *CanvasID // nil if original
	members    []*CanvasMember
	createdAt  time.Time
	updatedAt  time.Time
}

// NewCanvas is the factory for creating a new Canvas.
func NewCanvas(id CanvasID, ownerID identity.UserID, title string) (*Canvas, error) {
	title = trimTitle(title)
	if title == "" || utf8.RuneCountInString(title) > 200 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "title must be 1-200 characters")
	}

	now := time.Now()
	c := &Canvas{
		id:         id,
		ownerID:    ownerID,
		title:      title,
		visibility: VisibilityPrivate,
		members:    make([]*CanvasMember, 0),
		createdAt:  now,
		updatedAt:  now,
	}

	// The owner is automatically a member with RoleOwner.
	c.AddMember(ownerID, RoleOwner)

	c.AddEvent(CanvasCreatedEvent{
		BaseEvent: shared.NewBaseEvent("CanvasCreated"),
		CanvasID:  id,
		OwnerID:   ownerID,
	})

	return c, nil
}

// ReconstituteCanvas rebuilds a Canvas from persistence.
func ReconstituteCanvas(
	id CanvasID, ownerID identity.UserID, title string,
	snapshotKey SnapshotKey, visibility Visibility,
	forkedFrom *CanvasID, members []*CanvasMember,
	createdAt, updatedAt time.Time,
) *Canvas {
	return &Canvas{
		id:         id,
		ownerID:    ownerID,
		title:      title,
		snapshotKey: snapshotKey,
		visibility: visibility,
		forkedFrom: forkedFrom,
		members:    members,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// ForkCanvas creates a new Canvas as a fork of the current one.
// The new canvas gets a distinct ID, the requesting user as owner,
// and a title that may be derived from the original.
func (c *Canvas) ForkCanvas(newID CanvasID, userID identity.UserID, newTitle string) (*Canvas, error) {
	if newTitle == "" {
		newTitle = "Fork of " + c.title
	}
	fork, err := NewCanvas(newID, userID, newTitle)
	if err != nil {
		return nil, err
	}
	fork.forkedFrom = &c.id
	fork.snapshotKey = c.snapshotKey // Copy the snapshot reference

	fork.AddEvent(CanvasForkedEvent{
		BaseEvent:  shared.NewBaseEvent("CanvasForked"),
		OriginalID: c.id,
		NewID:      newID,
		UserID:     userID,
	})
	return fork, nil
}

// SetID assigns the database-generated ID after INSERT. Idempotent.
// Only the repository layer should call this.
func (c *Canvas) SetID(id CanvasID) {
	if c.id == 0 {
		c.id = id
	}
}

// ---- Accessors ----

func (c *Canvas) ID() CanvasID             { return c.id }
func (c *Canvas) OwnerID() identity.UserID { return c.ownerID }
func (c *Canvas) Title() string            { return c.title }
func (c *Canvas) SnapshotKey() SnapshotKey { return c.snapshotKey }
func (c *Canvas) Visibility() Visibility    { return c.visibility }
func (c *Canvas) ForkedFrom() *CanvasID    { return c.forkedFrom }
func (c *Canvas) Members() []*CanvasMember  { return c.members }
func (c *Canvas) CreatedAt() time.Time     { return c.createdAt }
func (c *Canvas) UpdatedAt() time.Time     { return c.updatedAt }

// ---- Mutations ----

// Rename changes the canvas title.
func (c *Canvas) Rename(newTitle string) error {
	newTitle = trimTitle(newTitle)
	if newTitle == "" || utf8.RuneCountInString(newTitle) > 200 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "title must be 1-200 characters")
	}
	c.title = newTitle
	c.updatedAt = time.Now()
	return nil
}

// UpdateSnapshotKey records the latest snapshot key after persistence.
func (c *Canvas) UpdateSnapshotKey(key SnapshotKey) {
	c.snapshotKey = key
	c.updatedAt = time.Now()
}

// AddMember adds a user to the canvas with the given role.
// The caller must ensure the operator is authorized (owner).
func (c *Canvas) AddMember(userID identity.UserID, role Role) {
	// Check if already a member — if so, update role.
	for _, m := range c.members {
		if m.Entity.ID == userID {
			m.Role = role
			c.updatedAt = time.Now()
			return
		}
	}
	c.members = append(c.members, NewCanvasMember(c.id, userID, role))
	c.updatedAt = time.Now()
}

// RemoveMember removes a user from the canvas.
// The owner cannot be removed.
func (c *Canvas) RemoveMember(userID identity.UserID) error {
	if userID == c.ownerID {
		return shared.NewDomainError(shared.ErrInvalidArgument, "cannot remove the owner")
	}
	for i, m := range c.members {
		if m.Entity.ID == userID {
			c.members = append(c.members[:i], c.members[i+1:]...)
			c.updatedAt = time.Now()
			return nil
		}
	}
	return shared.NewDomainError(shared.ErrNotFound, "member not found")
}

// GetRole returns the role of a user on this canvas, or -1 if not a member.
func (c *Canvas) GetRole(userID identity.UserID) Role {
	for _, m := range c.members {
		if m.Entity.ID == userID {
			return m.Role
		}
	}
	return -1 // sentinel: not a member
}

// IsMember returns true if the user is a member of this canvas.
func (c *Canvas) IsMember(userID identity.UserID) bool {
	return c.GetRole(userID) >= 0
}

// Publish transitions the canvas from private to published.
// Once published, this is irreversible.
func (c *Canvas) Publish() error {
	if c.visibility == VisibilityPublished {
		return shared.NewDomainError(shared.ErrConflict, "canvas is already published")
	}
	c.visibility = VisibilityPublished
	c.updatedAt = time.Now()

	c.AddEvent(CanvasPublishedEvent{
		BaseEvent: shared.NewBaseEvent("CanvasPublished"),
		CanvasID:  c.id,
	})
	return nil
}

// trimTitle normalizes the title input.
func trimTitle(s string) string {
	// Trim spaces; in real code we'd also remove control characters.
	trimmed := ""
	for _, r := range s {
		if r != ' ' || (len(trimmed) > 0 && trimmed[len(trimmed)-1] != ' ') {
			trimmed += string(r)
		}
	}
	// Trim leading/trailing whitespace
	start, end := 0, len(trimmed)
	for start < end && trimmed[start] == ' ' {
		start++
	}
	for end > start && trimmed[end-1] == ' ' {
		end--
	}
	return trimmed[start:end]
}
