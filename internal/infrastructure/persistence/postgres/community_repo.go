package postgres

import (
	"context"
	"database/sql"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
)

// CommunityRepo implements community.CommunityRepository using PostgreSQL.
type CommunityRepo struct {
	db *sql.DB
}

// NewCommunityRepo creates a new CommunityRepo.
func NewCommunityRepo(db *sql.DB) *CommunityRepo {
	return &CommunityRepo{db: db}
}

func (r *CommunityRepo) SavePublished(ctx context.Context, pc *communityDomain.PublishedCanvas) error { return nil }
func (r *CommunityRepo) FindPublishedByID(ctx context.Context, id canvasDomain.CanvasID) (*communityDomain.PublishedCanvas, error) {
	return nil, nil
}
func (r *CommunityRepo) FindPublished(ctx context.Context, query communityDomain.SearchQuery) ([]*communityDomain.PublishedCanvas, int64, error) {
	return nil, 0, nil
}
func (r *CommunityRepo) RemovePublished(ctx context.Context, id canvasDomain.CanvasID) error { return nil }
func (r *CommunityRepo) SaveLike(ctx context.Context, like *communityDomain.Like) error       { return nil }
func (r *CommunityRepo) RemoveLike(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) error {
	return nil
}
func (r *CommunityRepo) HasLiked(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (bool, error) {
	return false, nil
}
func (r *CommunityRepo) CountLikes(ctx context.Context, canvasID canvasDomain.CanvasID) (int64, error) { return 0, nil }
func (r *CommunityRepo) SaveComment(ctx context.Context, comment *communityDomain.Comment) error       { return nil }
func (r *CommunityRepo) FindComments(ctx context.Context, canvasID canvasDomain.CanvasID, page communityDomain.Pagination) ([]*communityDomain.Comment, int64, error) {
	return nil, 0, nil
}
func (r *CommunityRepo) DeleteComment(ctx context.Context, id communityDomain.CommentID) error { return nil }
func (r *CommunityRepo) SaveFork(ctx context.Context, fork *communityDomain.Fork) error         { return nil }
func (r *CommunityRepo) FindForks(ctx context.Context, canvasID canvasDomain.CanvasID, page communityDomain.Pagination) ([]*communityDomain.Fork, int64, error) {
	return nil, 0, nil
}
func (r *CommunityRepo) CountForks(ctx context.Context, canvasID canvasDomain.CanvasID) (int64, error) { return 0, nil }
func (r *CommunityRepo) SearchByTags(ctx context.Context, tags []string, page communityDomain.Pagination) ([]*communityDomain.PublishedCanvas, int64, error) {
	return nil, 0, nil
}
