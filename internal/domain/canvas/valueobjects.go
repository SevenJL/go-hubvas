package canvas

import (
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CanvasID is the strongly-typed identifier for a canvas.
type CanvasID int64

// Visibility encodes whether a canvas is private or published to the community.
type Visibility int8

const (
	VisibilityPrivate   Visibility = 0
	VisibilityPublished Visibility = 1
)

func (v Visibility) String() string {
	switch v {
	case VisibilityPrivate:
		return "private"
	case VisibilityPublished:
		return "published"
	default:
		return "unknown"
	}
}

func (v Visibility) IsPublished() bool { return v == VisibilityPublished }

// Role defines the permission level of a member on a canvas.
type Role int8

const (
	RoleOwner     Role = 0
	RoleEditor    Role = 1
	RoleViewer    Role = 2
	RoleCommenter Role = 3
)

func (r Role) String() string {
	switch r {
	case RoleOwner:
		return "owner"
	case RoleEditor:
		return "editor"
	case RoleViewer:
		return "viewer"
	case RoleCommenter:
		return "commenter"
	default:
		return "unknown"
	}
}

func (r Role) CanEdit() bool    { return r == RoleOwner || r == RoleEditor }
func (r Role) CanComment() bool { return r != RoleViewer }
func (r Role) CanView() bool    { return true }

// SnapshotKey is a value object representing the object storage key
// for a canvas CRDT snapshot.
type SnapshotKey string

func (k SnapshotKey) String() string { return string(k) }
func (k SnapshotKey) IsEmpty() bool  { return k == "" }

// Equals implements the ValueObject interface.
func (k SnapshotKey) Equals(other shared.ValueObject) bool {
	o, ok := other.(SnapshotKey)
	return ok && k == o
}

// CanvasMember is an entity within the Canvas aggregate representing one
// user's membership and permission role on the canvas.
type CanvasMember struct {
	shared.Entity[identity.UserID]
	CanvasID CanvasID
	Role     Role
}

// NewCanvasMember creates a validated CanvasMember entity.
func NewCanvasMember(canvasID CanvasID, userID identity.UserID, role Role) *CanvasMember {
	return &CanvasMember{
		Entity:   shared.Entity[identity.UserID]{ID: userID},
		CanvasID: canvasID,
		Role:     role,
	}
}

// ChangeRole updates the member's role. Only the owner may perform this action;
// the caller is responsible for authorization.
func (m *CanvasMember) ChangeRole(newRole Role) error {
	if newRole < RoleOwner || newRole > RoleCommenter {
		return shared.NewDomainError(shared.ErrInvalidArgument, "invalid role")
	}
	m.Role = newRole
	return nil
}
