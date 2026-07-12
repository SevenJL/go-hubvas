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
func (s *CommunityApplicationService) Browse(ctx context.Context, req SearchRequest, viewerID identity.UserID) (*FeedResponse, error) {
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
		ViewerID: viewerID,
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
	authors := make(map[identity.UserID]*identity.User)
	for i, p := range published {
		author, err := s.resolveAuthor(ctx, p.AuthorID(), authors)
		if err != nil {
			return nil, err
		}
		items[i] = PublishedCanvasDTO{
			CanvasID:        int64(p.CanvasID()),
			AuthorID:        int64(p.AuthorID()),
			AuthorName:      author.DisplayName(),
			AuthorUsername:  author.Username(),
			AuthorAvatarURL: author.AvatarURL(),
			Title:           p.Title(),
			SnapshotURL:     p.SnapshotURL(),
			Tags:            p.Tags(),
			LikeCount:       p.LikeCount(),
			CommentCount:    p.CommentCount(),
			ForkCount:       p.ForkCount(),
			PublishedAt:     p.PublishedAt().Unix(),
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
	author, err := s.resolveAuthor(ctx, p.AuthorID(), nil)
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
		CanvasID: int64(p.CanvasID()), AuthorID: int64(p.AuthorID()), AuthorName: author.DisplayName(), AuthorUsername: author.Username(), AuthorAvatarURL: author.AvatarURL(),
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

// PostComment creates a top-level comment or one-level reply.
func (s *CommunityApplicationService) PostComment(ctx context.Context, canvasID canvasDomain.CanvasID, authorID identity.UserID, req NewCommentRequest) (*CommentDTO, error) {
	canvas, err := s.canvasRepo.FindByID(ctx, canvasID)
	if err != nil {
		return nil, err
	}
	if !canvas.Visibility().IsPublished() {
		return nil, shared.NewDomainError(shared.ErrForbidden, "canvas is not published")
	}
	var parent *communityDomain.CommentID
	if req.ParentCommentID != nil {
		p, err := s.communityRepo.FindComment(ctx, communityDomain.CommentID(*req.ParentCommentID))
		if err != nil {
			return nil, err
		}
		if p.CanvasID() != canvasID {
			return nil, shared.NewDomainError(shared.ErrInvalidArgument, "parent comment belongs to another canvas")
		}
		if p.DeletedAt() != nil || p.ModerationStatus() != "visible" {
			return nil, shared.NewDomainError(shared.ErrConflict, "cannot reply to a deleted or hidden comment")
		}
		root := p.ID()
		if p.ParentID() != nil {
			root = *p.ParentID()
		}
		parent = &root
	}
	comment, err := communityDomain.NewReply(communityDomain.CommentID(s.idGen.NextID()), canvasID, authorID, parent, req.Content)
	if err != nil {
		return nil, err
	}
	user, err := s.userLookup.FindByID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	if err = s.communityRepo.SaveComment(ctx, comment); err != nil {
		return nil, err
	}
	return commentDTO(comment, user), nil
}
func (s *CommunityApplicationService) DeleteOwnComment(ctx context.Context, id communityDomain.CommentID, author identity.UserID) error {
	return s.communityRepo.SoftDeleteComment(ctx, id, author)
}
func (s *CommunityApplicationService) GetComments(ctx context.Context, canvasID canvasDomain.CanvasID, viewerID identity.UserID, page, pageSize int) ([]CommentDTO, int64, error) {
	comments, total, err := s.communityRepo.FindComments(ctx, canvasID, viewerID, communityDomain.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]CommentDTO, 0, len(comments))
	cache := map[identity.UserID]*identity.User{}
	for _, c := range comments {
		u := cache[c.AuthorID()]
		if u == nil {
			u, err = s.userLookup.FindByID(ctx, c.AuthorID())
			if err != nil {
				return nil, 0, err
			}
			cache[c.AuthorID()] = u
		}
		dtos = append(dtos, *commentDTO(c, u))
	}
	return dtos, total, nil
}
func commentDTO(c *communityDomain.Comment, u *identity.User) *CommentDTO {
	var parent *int64
	if c.ParentID() != nil {
		v := int64(*c.ParentID())
		parent = &v
	}
	content := c.VisibleContent()
	return &CommentDTO{ID: int64(c.ID()), CanvasID: int64(c.CanvasID()), AuthorID: int64(c.AuthorID()), AuthorName: u.DisplayName(), AuthorUsername: u.Username(), AuthorAvatarURL: u.AvatarURL(), ParentCommentID: parent, Content: content, Deleted: c.DeletedAt() != nil, ModerationStatus: c.ModerationStatus(), CreatedAt: c.CreatedAt().Unix()}
}

func (s *CommunityApplicationService) resolveAuthor(ctx context.Context, userID identity.UserID, cache map[identity.UserID]*identity.User) (*identity.User, error) {
	if cache != nil {
		if user, ok := cache[userID]; ok {
			return user, nil
		}
	}
	user, err := s.userLookup.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, shared.NewDomainError(shared.ErrNotFound, "author not found")
	}
	if cache != nil {
		cache[userID] = user
	}
	return user, nil
}
