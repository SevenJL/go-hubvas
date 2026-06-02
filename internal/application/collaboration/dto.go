package collaboration

import "github.com/hubvas/internal/domain/collaboration"

// RoomInfoDTO is lightweight info about an active room.
type RoomInfoDTO struct {
	RoomID      int64  `json:"room_id"`
	MemberCount int    `json:"member_count"`
	Status      string `json:"status"`
}

// PresenceDTO represents a single member's presence in a room.
type PresenceDTO struct {
	UserID     int64   `json:"user_id"`
	Username   string  `json:"username"`
	AvatarURL  string  `json:"avatar_url,omitempty"`
	Role       string  `json:"role"`
	CursorX    float64 `json:"cursor_x,omitempty"`
	CursorY    float64 `json:"cursor_y,omitempty"`
	EditingObj string  `json:"editing_obj,omitempty"`
}

// ToPresenceDTO converts a domain PresenceInfo to a DTO.
func ToPresenceDTO(info collaboration.PresenceInfo) PresenceDTO {
	dto := PresenceDTO{
		UserID:    int64(info.UserID),
		Username:  info.Username,
		AvatarURL: info.AvatarURL,
		Role:      info.Role.String(),
	}
	if info.Cursor != nil {
		dto.CursorX = info.Cursor.X
		dto.CursorY = info.Cursor.Y
	}
	dto.EditingObj = info.EditingObj
	return dto
}
