package community

import (
	"context"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

// UserLookup resolves public author information without coupling this service
// to a concrete identity repository implementation.
type UserLookup interface {
	FindByID(ctx context.Context, id identity.UserID) (*identity.User, error)
}

// CommunityApplicationService orchestrates community-related use cases.
type CommunityApplicationService struct {
	communityRepo communityDomain.CommunityRepository
	canvasRepo    canvasDomain.CanvasRepository
	userLookup    UserLookup
	idGen         shared.IDGenerator
}

// NewCommunityApplicationService creates the application service.
func NewCommunityApplicationService(
	communityRepo communityDomain.CommunityRepository,
	canvasRepo canvasDomain.CanvasRepository,
	userLookup UserLookup,
	idGen shared.IDGenerator,
) *CommunityApplicationService {
	return &CommunityApplicationService{
		communityRepo: communityRepo,
		canvasRepo:    canvasRepo,
		userLookup:    userLookup,
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
	authorNames := make(map[identity.UserID]string)
	for i, p := range published {
		authorName, err := s.resolveAuthorName(ctx, p.AuthorID(), authorNames)
		if err != nil {
			return nil, err
		}
		items[i] = PublishedCanvasDTO{
			CanvasID:     int64(p.CanvasID()),
			AuthorID:     int64(p.AuthorID()),
			AuthorName:   authorName,
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

// GetPublished returns one published canvas with author and requester-specific like state.
func (s *CommunityApplicationService) GetPublished(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (*PublishedCanvasDTO, error) {
	p, err := s.communityRepo.FindPublishedByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	authorName, err := s.resolveAuthorName(ctx, p.AuthorID(), nil)
	if err != nil {
		return nil, err
	}
	liked := false
	if userID != 0 {
		liked, err = s.communityRepo.HasLiked(ctx, canvasID, userID)
		if err != nil {
			return nil, err
		}
	}
	return &PublishedCanvasDTO{
		CanvasID: int64(p.CanvasID()), AuthorID: int64(p.AuthorID()), AuthorName: authorName,
		Title: p.Title(), SnapshotURL: p.SnapshotURL(), Tags: p.Tags(), LikeCount: p.LikeCount(),
		IsLiked: liked, CommentCount: p.CommentCount(), ForkCount: p.ForkCount(), PublishedAt: p.PublishedAt().Unix(),
	}, nil
}

// LikeCanvas atomically adds a like and increments the published counter.
func (s *CommunityApplicationService) LikeCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (*LikeStatusDTO, error) {
	canvas, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	if !canvas.Visibility().IsPublished() {
		return nil, shared.NewDomainError(shared.ErrForbidden, "canvas is not published")
	}

	count, err := s.communityRepo.LikeCanvas(ctx, communityDomain.NewLike(canvasID, userID))
	if err != nil {
		return nil, err
	}
	return &LikeStatusDTO{Liked: true, LikeCount: count}, nil
}

// UnlikeCanvas atomically removes a like and decrements the published counter.
func (s *CommunityApplicationService) UnlikeCanvas(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (*LikeStatusDTO, error) {
	count, err := s.communityRepo.UnlikeCanvas(ctx, canvasID, userID)
	if err != nil {
		return nil, err
	}
	return &LikeStatusDTO{Liked: false, LikeCount: count}, nil
}

// GetLikeStatus returns a public count and, when authenticated, the user's state.
func (s *CommunityApplicationService) GetLikeStatus(ctx context.Context, canvasID canvasDomain.CanvasID, userID identity.UserID) (*LikeStatusDTO, error) {
	count, err := s.communityRepo.CountLikes(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	liked := false
	if userID != 0 {
		liked, err = s.communityRepo.HasLiked(ctx, canvasID, userID)
		if err != nil {
			return nil, err
		}
	}
	return &LikeStatusDTO{Liked: liked, LikeCount: count}, nil
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

	authorName, err := s.resolveAuthorName(ctx, authorID, nil)
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
	if err := s.communityRepo.SavePublished(ctx, published); err != nil {
		return nil, err
	}

	return &CommentDTO{
		ID:         int64(comment.ID()),
		CanvasID:   int64(comment.CanvasID()),
		AuthorID:   int64(comment.AuthorID()),
		AuthorName: authorName,
		Content:    comment.Content(),
		CreatedAt:  comment.CreatedAt().Unix(),
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
	authorNames := make(map[identity.UserID]string)
	for i, c := range comments {
		authorName, err := s.resolveAuthorName(ctx, c.AuthorID(), authorNames)
		if err != nil {
			return nil, 0, err
		}
		dtos[i] = CommentDTO{
			ID:         int64(c.ID()),
			CanvasID:   int64(c.CanvasID()),
			AuthorID:   int64(c.AuthorID()),
			AuthorName: authorName,
			Content:    c.Content(),
			CreatedAt:  c.CreatedAt().Unix(),
		}
	}
	return dtos, total, nil
}

func (s *CommunityApplicationService) resolveAuthorName(ctx context.Context, userID identity.UserID, cache map[identity.UserID]string) (string, error) {
	if name, ok := cache[userID]; ok {
		return name, nil
	}
	user, err := s.userLookup.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", shared.NewDomainError(shared.ErrNotFound, "author not found")
	}
	name := user.Username()
	if cache != nil {
		cache[userID] = name
	}
	return name, nil
}
