package social

import "time"

type ActorDTO struct {
	ID          int64  `json:"id,string"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}
type PublicProfileDTO struct {
	ActorDTO
	Bio            string    `json:"bio"`
	Website        string    `json:"website"`
	PublishedCount int64     `json:"published_count"`
	FollowersCount int64     `json:"followers_count"`
	FollowingCount int64     `json:"following_count"`
	IsFollowing    bool      `json:"is_following"`
	IsBlocked      bool      `json:"is_blocked"`
	IsBlockedBy    bool      `json:"is_blocked_by"`
	JoinedAt       time.Time `json:"joined_at"`
}
type RelationshipPage struct {
	Items    []ActorDTO `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}
type NotificationDTO struct {
	ID         int64          `json:"id,string"`
	EventType  string         `json:"event_type"`
	Actor      *ActorDTO      `json:"actor,omitempty"`
	TargetType string         `json:"target_type"`
	TargetID   int64          `json:"target_id,string"`
	Data       map[string]any `json:"data"`
	ReadAt     *time.Time     `json:"read_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}
type NotificationPage struct {
	Items    []NotificationDTO `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}
type ReportRequest struct {
	TargetType string `json:"target_type" binding:"required,oneof=user canvas comment"`
	TargetID   int64  `json:"target_id,string" binding:"required"`
	Reason     string `json:"reason" binding:"required,oneof=spam harassment inappropriate copyright other"`
	Details    string `json:"details" binding:"max=1000"`
}
type ReportDTO struct {
	ID         int64      `json:"id,string"`
	ReporterID int64      `json:"reporter_id,string"`
	TargetType string     `json:"target_type"`
	TargetID   int64      `json:"target_id,string"`
	Reason     string     `json:"reason"`
	Details    string     `json:"details"`
	Status     string     `json:"status"`
	ReviewerID *int64     `json:"reviewer_id,omitempty,string"`
	ReviewNote string     `json:"review_note"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}
type ReviewReportRequest struct {
	Status string `json:"status" binding:"required,oneof=reviewing resolved dismissed"`
	Note   string `json:"note" binding:"max=1000"`
}
type UserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended"`
}
type ModerationRequest struct {
	Status string `json:"status" binding:"required,oneof=visible hidden"`
}

type PublishedItemDTO struct {
	CanvasID        int64  `json:"canvas_id,string"`
	AuthorID        int64  `json:"author_id,string"`
	AuthorName      string `json:"author_name"`
	AuthorUsername  string `json:"author_username"`
	AuthorAvatarURL string `json:"author_avatar_url,omitempty"`
	Title           string `json:"title"`
	SnapshotURL     string `json:"snapshot_url"`
	LikeCount       int64  `json:"like_count"`
	CommentCount    int64  `json:"comment_count"`
	ForkCount       int64  `json:"fork_count"`
	PublishedAt     int64  `json:"published_at"`
}
type PublishedPage struct {
	Items      []PublishedItemDTO `json:"items"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}

type AdminAuditLogDTO struct {
	ID               int64          `json:"id,string"`
	AdminID          *int64         `json:"admin_id,omitempty,string"`
	AdminUsername    string         `json:"admin_username"`
	AdminDisplayName string         `json:"admin_display_name"`
	AdminAvatarURL   string         `json:"admin_avatar_url,omitempty"`
	Action           string         `json:"action"`
	TargetType       string         `json:"target_type"`
	TargetID         int64          `json:"target_id,string"`
	Metadata         map[string]any `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
}
