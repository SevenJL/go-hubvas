package community

import (
	"context"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// CommunityApplicationService orchestrates community-related use cases.
type CommunityApplicationService struct {
	communityRepo communityDomain.CommunityRepository
	canvasRepo    canvasDomain.CanvasRepository
	idGen         IDGenerator
}

// IDGenerator generates unique IDs for community entities.
type IDGenerator interface {
	NextID() int64
}

// NewCommunityApplicationService creates the application service.
func NewCommunityApplicationService(
	communityRepo communityDomain.CommunityRepository,
	canvasRepo canvasDomain.CanvasRepository,
	idGen IDGenerator,
) *CommunityApplicationService {
	return &CommunityApplicationService{
		communityRepo: communityRepo,
		canvasRepo:    canvasRepo,
		idGen:         idGen,
	}
}

// Browse returns a paginated feed of published canvases.
func (s *CommunityApplicationService) Browse(ctx context.Context, req SearchRequest) (*FeedResponse, error) {
	sortBy := communityDomain.SortByLatest
	switch req.SortBy {
	case "popular":
		sortBy = communityDomain.SortByPopular
	case "trending":
		sortBy = communityDomain.SortByTrending
	}

	query := communityDomain.SearchQuery{
		Keyword:  req.Keyword,
		Tags:     req.Tags,
		SortBy:   sortBy,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 50 {
		query.PageSize = 20
	}

	published, total, err := s.communityRepo.FindPublished(ctx, query)
	if err != nil {
		return nil, err
	}

	items := make([]PublishedCanvasDTO, len(published))
	for i, p := range published {
		items[i] = PublishedCanvasDTO{
			CanvasID:     int64(p.CanvasID()),
			AuthorID:     int64(p.AuthorID()),
			Title:        p.Title(),
			SnapshotURL:  p.SnapshotURL(),
			Tags:         p.Tags(),
			LikeCount:    p.LikeCount(),
			CommentCount: p.CommentCount(),
			ForkCount:    p.ForkCount(),
			PublishedAt:  p.PublishedAt().Unix(),
		}
	}

	return &FeedResponse{
		Items:      items,
		TotalCount: total,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

// LikeCanvas adds a like to a published canvas.
func (s *CommunityApplicationService) LikeCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) error {
	// Verify the canvas is published.
	canvas, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return err
	}
	if !canvas.Visibility().IsPublished() {
		return shared.NewDomainError(shared.ErrForbidden, "canvas is not published")
	}

	// Check for duplicate like.
	hasLiked, err := s.communityRepo.HasLiked(ctx, canvasID, userID)
	if err != nil {
		return err
	}
	if hasLiked {
		return shared.NewDomainError(shared.ErrAlreadyExists, "already liked")
	}

	like := communityDomain.NewLike(canvasID, userID)
	if err := s.communityRepo.SaveLike(ctx, like); err != nil {
		return err
	}

	// Increment the counter on the published canvas.
	published, err := s.communityRepo.FindPublishedByID(ctx, canvasID)
	if err != nil {
		return err
	}
	published.IncrementLike()
	return s.communityRepo.SavePublished(ctx, published)
}

// UnlikeCanvas removes a like from a published canvas.
func (s *CommunityApplicationService) UnlikeCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) error {
	if err := s.communityRepo.RemoveLike(ctx, canvasID, userID); err != nil {
		return err
	}

	published, err := s.communityRepo.FindPublishedByID(ctx, canvasID)
	if err != nil {
		return err
	}
	published.DecrementLike()
	return s.communityRepo.SavePublished(ctx, published)
}

// PostComment creates a new comment on a published canvas.
func (s *CommunityApplicationService) PostComment(
	ctx context.Context,
	canvasID canvasDomain.CanvasID,
	authorID identity.UserID,
	req NewCommentRequest,
) (*CommentDTO, error) {
	canvas, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	if !canvas.Visibility().IsPublished() {
		return nil, shared.NewDomainError(shared.ErrForbidden, "canvas is not published")
	}

	comment, err := communityDomain.NewComment(
		communityDomain.CommentID(s.idGen.NextID()),
		canvasID,
		authorID,
		req.Content,
	)
	if err != nil {
		return nil, err
	}

	if err := s.communityRepo.SaveComment(ctx, comment); err != nil {
		return nil, err
	}

	// Update comment count.
	published, err := s.communityRepo.FindPublishedByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	published.IncrementComment()
	s.communityRepo.SavePublished(ctx, published)

	return &CommentDTO{
		ID:        int64(comment.ID()),
		CanvasID:  int64(comment.CanvasID()),
		AuthorID:  int64(comment.AuthorID()),
		Content:   comment.Content(),
		CreatedAt: comment.CreatedAt().Unix(),
	}, nil
}

// GetComments retrieves comments for a published canvas.
func (s *CommunityApplicationService) GetComments(
	ctx context.Context,
	canvasID canvasDomain.CanvasID,
	page, pageSize int,
) ([]CommentDTO, int64, error) {
	pagination := communityDomain.Pagination{Page: page, PageSize: pageSize}
	comments, total, err := s.communityRepo.FindComments(ctx, canvasID, pagination)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]CommentDTO, len(comments))
	for i, c := range comments {
		dtos[i] = CommentDTO{
			ID:        int64(c.ID()),
			CanvasID:  int64(c.CanvasID()),
			AuthorID:  int64(c.AuthorID()),
			Content:   c.Content(),
			CreatedAt: c.CreatedAt().Unix(),
		}
	}
	return dtos, total, nil
}
