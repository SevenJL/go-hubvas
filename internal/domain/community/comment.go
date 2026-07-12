package community

import (
	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
	"strings"
	"time"
	"unicode/utf8"
)

type Comment struct {
	id               CommentID
	canvasID         canvas.CanvasID
	authorID         identity.UserID
	parentID         *CommentID
	content          string
	deletedAt        *time.Time
	moderationStatus string
	createdAt        time.Time
}

func NewComment(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, content string) (*Comment, error) {
	return NewReply(id, canvasID, authorID, nil, content)
}
func NewReply(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, parent *CommentID, content string) (*Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > 5000 {
		return nil, shared.NewDomainError(shared.ErrInvalidArgument, "comment content must be 1-5000 characters")
	}
	return &Comment{id: id, canvasID: canvasID, authorID: authorID, parentID: parent, content: content, moderationStatus: "visible", createdAt: time.Now()}, nil
}
func ReconstituteComment(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, content string, createdAt time.Time) *Comment {
	return ReconstituteCommentFull(id, canvasID, authorID, nil, content, nil, "visible", createdAt)
}
func ReconstituteCommentFull(id CommentID, canvasID canvas.CanvasID, authorID identity.UserID, parent *CommentID, content string, deleted *time.Time, status string, createdAt time.Time) *Comment {
	return &Comment{id: id, canvasID: canvasID, authorID: authorID, parentID: parent, content: content, deletedAt: deleted, moderationStatus: status, createdAt: createdAt}
}
func (c *Comment) SetID(id CommentID) {
	if c.id == 0 {
		c.id = id
	}
}
func (c *Comment) ID() CommentID             { return c.id }
func (c *Comment) CanvasID() canvas.CanvasID { return c.canvasID }
func (c *Comment) AuthorID() identity.UserID { return c.authorID }
func (c *Comment) ParentID() *CommentID      { return c.parentID }
func (c *Comment) Content() string           { return c.content }
func (c *Comment) DeletedAt() *time.Time     { return c.deletedAt }
func (c *Comment) ModerationStatus() string  { return c.moderationStatus }
func (c *Comment) CreatedAt() time.Time      { return c.createdAt }
func (c *Comment) VisibleContent() string {
	if c.deletedAt != nil || c.moderationStatus == "hidden" {
		return ""
	}
	return c.content
}
func (c *Comment) EditContent(v string) error {
	v = strings.TrimSpace(v)
	if v == "" || utf8.RuneCountInString(v) > 5000 {
		return shared.NewDomainError(shared.ErrInvalidArgument, "comment content must be 1-5000 characters")
	}
	c.content = v
	return nil
}
