package community

import (
	"time"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// PublishedCanvas is an aggregate root representing a canvas that has been
// published to the community. It is a read-side projection of the Canvas
// aggregate, enriched with community-specific counters.
type PublishedCanvas struct {
	shared.AggregateRoot
	canvasID     canvas.CanvasID
	authorID     identity.UserID
	title        string
	snapshotURL  string
	tags         []string
	likeCount    int64
	commentCount int64
	forkCount    int64
	publishedAt  time.Time
}

// NewPublishedCanvas creates a new PublishedCanvas projection.
func NewPublishedCanvas(
	canvasID canvas.CanvasID,
	authorID identity.UserID,
	title, snapshotURL string,
	tags []string,
) *PublishedCanvas {
	if tags == nil {
		tags = []string{}
	}
	return &PublishedCanvas{
		canvasID:    canvasID,
		authorID:    authorID,
		title:       title,
		snapshotURL: snapshotURL,
		tags:        tags,
		publishedAt: time.Now(),
	}
}

// ReconstitutePublishedCanvas rebuilds from persistence.
func ReconstitutePublishedCanvas(
	canvasID canvas.CanvasID,
	authorID identity.UserID,
	title, snapshotURL string,
	tags []string,
	likeCount, commentCount, forkCount int64,
	publishedAt time.Time,
) *PublishedCanvas {
	return &PublishedCanvas{
		canvasID:     canvasID,
		authorID:     authorID,
		title:        title,
		snapshotURL:  snapshotURL,
		tags:         tags,
		likeCount:    likeCount,
		commentCount: commentCount,
		forkCount:    forkCount,
		publishedAt:  publishedAt,
	}
}

// ---- Accessors ----

func (p *PublishedCanvas) CanvasID() canvas.CanvasID   { return p.canvasID }
func (p *PublishedCanvas) AuthorID() identity.UserID     { return p.authorID }
func (p *PublishedCanvas) Title() string                { return p.title }
func (p *PublishedCanvas) SnapshotURL() string          { return p.snapshotURL }
func (p *PublishedCanvas) Tags() []string               { return p.tags }
func (p *PublishedCanvas) LikeCount() int64             { return p.likeCount }
func (p *PublishedCanvas) CommentCount() int64          { return p.commentCount }
func (p *PublishedCanvas) ForkCount() int64             { return p.forkCount }
func (p *PublishedCanvas) PublishedAt() time.Time       { return p.publishedAt }

// ---- Mutations ----

// IncrementLike increments the like count.
func (p *PublishedCanvas) IncrementLike()  { p.likeCount++ }

// DecrementLike decrements the like count. Does not go below zero.
func (p *PublishedCanvas) DecrementLike() {
	if p.likeCount > 0 {
		p.likeCount--
	}
}

// IncrementComment increments the comment count.
func (p *PublishedCanvas) IncrementComment() { p.commentCount++ }

// IncrementFork increments the fork count.
func (p *PublishedCanvas) IncrementFork() { p.forkCount++ }

// UpdateTags replaces the tag set.
func (p *PublishedCanvas) UpdateTags(tags []string) {
	if tags == nil {
		tags = []string{}
	}
	p.tags = tags
}
