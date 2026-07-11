package community

import "time"

// PublishedCanvasDTO is the public representation of a published canvas.
type PublishedCanvasDTO struct {
	CanvasID     int64    `json:"canvas_id,string"`
	AuthorID     int64    `json:"author_id,string"`
	AuthorName   string   `json:"author_name"`
	Title        string   `json:"title"`
	SnapshotURL  string   `json:"snapshot_url"`
	Tags         []string `json:"tags"`
	LikeCount    int64    `json:"like_count"`
	IsLiked      bool     `json:"is_liked"`
	CommentCount int64    `json:"comment_count"`
	ForkCount    int64    `json:"fork_count"`
	PublishedAt  int64    `json:"published_at"`
}

// CommentDTO is the public representation of a comment.
type CommentDTO struct {
	ID         int64  `json:"id,string"`
	CanvasID   int64  `json:"canvas_id,string"`
	AuthorID   int64  `json:"author_id,string"`
	AuthorName string `json:"author_name"`
	Content    string `json:"content"`
	CreatedAt  int64  `json:"created_at"`
}

// FeedResponse is the paginated response for the community feed.
type FeedResponse struct {
	Items      []PublishedCanvasDTO `json:"items"`
	TotalCount int64                `json:"total_count"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
}

// NewCommentRequest is the input DTO for posting a comment.
type NewCommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=5000"`
}

// SearchRequest is the input DTO for searching the community.
type SearchRequest struct {
	Keyword  string   `form:"q"`
	Tags     []string `form:"tags"`
	SortBy   string   `form:"sort_by"` // "latest", "popular", "trending"
	Page     int      `form:"page"`
	PageSize int      `form:"page_size"`
}

// ForkDTO is a simple fork lineage record.
type ForkDTO struct {
	OriginalID int64     `json:"original_id"`
	NewID      int64     `json:"new_id"`
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// LikeStatusDTO reports the requesting user's like state and the current total.
type LikeStatusDTO struct {
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"like_count"`
}
