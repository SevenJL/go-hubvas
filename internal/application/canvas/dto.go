package canvas

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
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

// InviteRequest is the input DTO for generating an invitation link.
type InviteRequest struct {
	Role canvas.Role `json:"role" binding:"required"`
}
