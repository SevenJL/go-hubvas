package canvas

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/shared"
)

// CreateCanvasRequest is the input DTO for creating a canvas.
type CreateCanvasRequest struct {
	Title string `json:"title" binding:"required,min=1,max=200"`
}

// UpdateCanvasRequest is the input DTO for updating canvas metadata.
type UpdateCanvasRequest struct {
	Title string `json:"title" binding:"required,min=1,max=200"`
}

// CanvasDTO is the public representation of a canvas.
type CanvasDTO struct {
	ID          int64     `json:"id,string"`
	OwnerID     int64     `json:"owner_id,string"`
	Title       string    `json:"title"`
	Visibility  string    `json:"visibility"`
	ForkedFrom  *int64    `json:"forked_from,omitempty,string"`
	MemberCount int       `json:"member_count"`
	OnlineCount int       `json:"online_count"`
	CurrentRole string    `json:"current_role,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MemberDTO represents a member's permission assignment.
type MemberDTO struct {
	UserID   int64  `json:"user_id,string"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// AddMemberRequest adds a registered user to a canvas by username.
type AddMemberRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Role     string `json:"role" binding:"required,oneof=editor viewer commenter"`
}

// UpdateMemberRoleRequest changes an existing member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=editor viewer commenter"`
}

func parseAssignableRole(value string) (canvas.Role, error) {
	switch value {
	case "editor":
		return canvas.RoleEditor, nil
	case "viewer":
		return canvas.RoleViewer, nil
	case "commenter":
		return canvas.RoleCommenter, nil
	default:
		return -1, shared.NewDomainError(shared.ErrInvalidArgument, "role must be editor, viewer, or commenter")
	}
}
