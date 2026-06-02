package community

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// CommentID is the strongly-typed identifier for a comment.
type CommentID int64

// SearchQuery is a value object encapsulating community feed search parameters.
type SearchQuery struct {
	Keyword  string
	Tags     []string
	SortBy   SortBy
	Page     int
	PageSize int
}

// SortBy defines the sort order for the community feed.
type SortBy int8

const (
	SortByLatest  SortBy = 0
	SortByPopular SortBy = 1
	SortByTrending SortBy = 2
)

// Pagination is a reusable value object for paginated queries.
type Pagination struct {
	Page     int
	PageSize int
}

// Offset returns the zero-based offset for the pagination.
func (p Pagination) Offset() int {
	if p.Page <= 0 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Limit returns the page size, clamped to a reasonable default.
func (p Pagination) Limit(defaultSize int) int {
	if p.PageSize <= 0 || p.PageSize > 100 {
		return defaultSize
	}
	return p.PageSize
}

// ---- Events ----

// CanvasLikedEvent is fired when a user likes a published canvas.
type CanvasLikedEvent struct {
	CanvasID canvas.CanvasID
	UserID   identity.UserID
	Time     time.Time
}

// CanvasUnlikedEvent is fired when a user removes their like.
type CanvasUnlikedEvent struct {
	CanvasID canvas.CanvasID
	UserID   identity.UserID
}

// CommentPostedEvent is fired when a comment is created.
type CommentPostedEvent struct {
	CommentID CommentID
	CanvasID  canvas.CanvasID
	AuthorID  identity.UserID
	Content   string
	Time      time.Time
}

// CanvasForkedInCommunityEvent is fired when a canvas is forked from the community.
type CanvasForkedInCommunityEvent struct {
	OriginalID canvas.CanvasID
	NewID      canvas.CanvasID
	UserID     identity.UserID
}
