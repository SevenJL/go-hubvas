package community

import (
	"time"
	"unicode/utf8"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// Comment is an aggregate root representing a comment on a published canvas.
type Comment struct {
	id        CommentID
	canvasID  canvas.CanvasID
	authorID  identity.UserID
	content   string
	createdAt time.Time
}

// NewComment creates a new Comment after validating content length.
func NewComment(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, content string) (*Comment, error) {
	if content == "" || utf8.RuneCountInString(content) > 5000 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "comment content must be 1-5000 characters")
	}
	return &Comment{
		id:        id,
		canvasID:  canvasID,
		authorID:  authorID,
		content:   content,
		createdAt: time.Now(),
	}, nil
}

// ReconstituteComment rebuilds a Comment from persistence.
func ReconstituteComment(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, content string, createdAt time.Time) *Comment {
	return &Comment{
		id:        id,
		canvasID:  canvasID,
		authorID:  authorID,
		content:   content,
		createdAt: createdAt,
	}
}

// ---- Accessors ----

func (c *Comment) ID() CommentID           { return c.id }
func (c *Comment) CanvasID() canvas.CanvasID { return c.canvasID }
func (c *Comment) AuthorID() identity.UserID { return c.authorID }
func (c *Comment) Content() string         { return c.content }
func (c *Comment) CreatedAt() time.Time    { return c.createdAt }

// EditContent updates the comment content after validation.
func (c *Comment) EditContent(newContent string) error {
	if newContent == "" || utf8.RuneCountInString(newContent) > 5000 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "comment content must be 1-5000 characters")
	}
	c.content = newContent
	return nil
}
