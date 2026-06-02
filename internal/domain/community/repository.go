package community

import (
	"context"

	"github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
)

// CommunityRepository defines the contract for community data access.
type CommunityRepository interface {
	// PublishedCanvas operations
	SavePublished(ctx context.Context, pc *PublishedCanvas) error
	FindPublishedByID(ctx context.Context, id canvas.CanvasID) (*PublishedCanvas, error)
	FindPublished(ctx context.Context, query SearchQuery) ([]*PublishedCanvas, int64, error)
	RemovePublished(ctx context.Context, id canvas.CanvasID) error

	// Like operations
	SaveLike(ctx context.Context, like *Like) error
	RemoveLike(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) error
	HasLiked(ctx context.Context, canvasID canvas.CanvasID, userID identity.UserID) (bool, error)
	CountLikes(ctx context.Context, canvasID canvas.CanvasID) (int64, error)

	// Comment operations
	SaveComment(ctx context.Context, comment *Comment) error
	FindComments(ctx context.Context, canvasID canvas.CanvasID, page Pagination) ([]*Comment, int64, error)
	DeleteComment(ctx context.Context, id CommentID) error

	// Fork operations
	SaveFork(ctx context.Context, fork *Fork) error
	FindForks(ctx context.Context, canvasID canvas.CanvasID, page Pagination) ([]*Fork, int64, error)
	CountForks(ctx context.Context, canvasID canvas.CanvasID) (int64, error)

	// Tag operations
	SearchByTags(ctx context.Context, tags []string, page Pagination) ([]*PublishedCanvas, int64, error)
}
