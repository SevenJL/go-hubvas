package community

import (
	"context"
	"errors"
	"testing"
	"time"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type fixedIDGenerator struct{ id int64 }

func (g fixedIDGenerator) NextID() int64 { return g.id }

type communityRepositoryStub struct {
	published      []*communityDomain.PublishedCanvas
	comments       []*communityDomain.Comment
	findCommentErr error
	saveCommentErr error
}

func (r *communityRepositoryStub) SavePublished(context.Context, *communityDomain.PublishedCanvas) error {
	return nil
}
func (r *communityRepositoryStub) FindPublishedByID(context.Context, canvasDomain.CanvasID) (*communityDomain.PublishedCanvas, error) {
	return r.published[0], nil
}
func (r *communityRepositoryStub) FindPublished(context.Context, communityDomain.SearchQuery) ([]*communityDomain.PublishedCanvas, int64, error) {
	return r.published, int64(len(r.published)), nil
}
func (r *communityRepositoryStub) RemovePublished(context.Context, canvasDomain.CanvasID) error {
	return nil
}
func (r *communityRepositoryStub) LikeCanvas(context.Context, *communityDomain.Like) (int64, error) {
	return 1, nil
}
func (r *communityRepositoryStub) UnlikeCanvas(context.Context, canvasDomain.CanvasID, identity.UserID) (int64, error) {
	return 0, nil
}
func (r *communityRepositoryStub) HasLiked(context.Context, canvasDomain.CanvasID, identity.UserID) (bool, error) {
	return false, nil
}
func (r *communityRepositoryStub) CountLikes(context.Context, canvasDomain.CanvasID) (int64, error) {
	return 0, nil
}
func (r *communityRepositoryStub) SaveComment(context.Context, *communityDomain.Comment) error {
	return r.saveCommentErr
}
func (r *communityRepositoryStub) FindComment(_ context.Context, id communityDomain.CommentID) (*communityDomain.Comment, error) {
	if r.findCommentErr != nil {
		return nil, r.findCommentErr
	}
	for _, c := range r.comments {
		if c.ID() == id {
			return c, nil
		}
	}
	return nil, nil
}
func (r *communityRepositoryStub) SoftDeleteComment(context.Context, communityDomain.CommentID, identity.UserID) error {
	return nil
}
func (r *communityRepositoryStub) FindComments(context.Context, canvasDomain.CanvasID, identity.UserID, communityDomain.Pagination) ([]*communityDomain.Comment, int64, error) {
	return r.comments, int64(len(r.comments)), nil
}
func (r *communityRepositoryStub) DeleteComment(context.Context, communityDomain.CommentID) error {
	return nil
}
func (r *communityRepositoryStub) SaveFork(context.Context, *communityDomain.Fork) error { return nil }
func (r *communityRepositoryStub) FindForks(context.Context, canvasDomain.CanvasID, communityDomain.Pagination) ([]*communityDomain.Fork, int64, error) {
	return nil, 0, nil
}
func (r *communityRepositoryStub) CountForks(context.Context, canvasDomain.CanvasID) (int64, error) {
	return 0, nil
}
func (r *communityRepositoryStub) SearchByTags(context.Context, []string, communityDomain.Pagination) ([]*communityDomain.PublishedCanvas, int64, error) {
	return nil, 0, nil
}

type canvasRepositoryStub struct {
	canvas *canvasDomain.Canvas
	err    error
}

func (r *canvasRepositoryStub) Save(context.Context, *canvasDomain.Canvas) error { return nil }
func (r *canvasRepositoryStub) FindByID(context.Context, canvasDomain.CanvasID) (*canvasDomain.Canvas, error) {
	return r.canvas, r.err
}
func (r *canvasRepositoryStub) FindByOwner(context.Context, identity.UserID) ([]*canvasDomain.Canvas, error) {
	return nil, nil
}
func (r *canvasRepositoryStub) FindByMember(context.Context, identity.UserID) ([]*canvasDomain.Canvas, error) {
	return nil, nil
}
func (r *canvasRepositoryStub) Delete(context.Context, canvasDomain.CanvasID) error { return nil }

type userLookupStub struct {
	users map[identity.UserID]*identity.User
	calls map[identity.UserID]int
}

func (s *userLookupStub) FindByID(_ context.Context, id identity.UserID) (*identity.User, error) {
	s.calls[id]++
	return s.users[id], nil
}

func TestBrowsePopulatesAuthorNamesAndCachesLookups(t *testing.T) {
	authorID := identity.UserID(9)
	repo := &communityRepositoryStub{published: []*communityDomain.PublishedCanvas{
		communityDomain.ReconstitutePublishedCanvas(1, authorID, "one", "", nil, 0, 0, 0, time.Now()),
		communityDomain.ReconstitutePublishedCanvas(2, authorID, "two", "", nil, 0, 0, 0, time.Now()),
	}}
	users := &userLookupStub{
		users: map[identity.UserID]*identity.User{
			authorID: identity.ReconstituteUser(authorID, "alice", "alice@example.com", "hash", "", time.Now()),
		},
		calls: make(map[identity.UserID]int),
	}
	svc := NewCommunityApplicationService(repo, nil, users, fixedIDGenerator{id: 1})

	feed, err := svc.Browse(context.Background(), SearchRequest{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 || feed.Items[0].AuthorName != "alice" || feed.Items[1].AuthorName != "alice" {
		t.Fatalf("author names were not populated: %#v", feed.Items)
	}
	if users.calls[authorID] != 1 {
		t.Fatalf("expected one lookup for repeated author, got %d", users.calls[authorID])
	}
}

func TestGetCommentsPopulatesAuthorNames(t *testing.T) {
	authorID := identity.UserID(9)
	repo := &communityRepositoryStub{comments: []*communityDomain.Comment{
		communityDomain.ReconstituteComment(1, 7, authorID, "hello", time.Now()),
	}}
	users := &userLookupStub{
		users: map[identity.UserID]*identity.User{
			authorID: identity.ReconstituteUser(authorID, "alice", "alice@example.com", "hash", "", time.Now()),
		},
		calls: make(map[identity.UserID]int),
	}
	svc := NewCommunityApplicationService(repo, nil, users, fixedIDGenerator{id: 1})

	comments, total, err := svc.GetComments(context.Background(), 7, 0, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(comments) != 1 || comments[0].AuthorName != "alice" {
		t.Fatalf("unexpected comments response: total=%d comments=%#v", total, comments)
	}
}

func publishedCanvasAggregate(id canvasDomain.CanvasID, owner identity.UserID) *canvasDomain.Canvas {
	return canvasDomain.ReconstituteCanvas(id, owner, "published", "", canvasDomain.VisibilityPublished, nil, nil, time.Now(), time.Now())
}

func TestPostCommentReturnsContextualErrorForMissingParent(t *testing.T) {
	canvasID := canvasDomain.CanvasID(7)
	parentID := int64(999)
	repo := &communityRepositoryStub{findCommentErr: shared.NewDomainError(shared.ErrNotFound, "entity not found")}
	canvases := &canvasRepositoryStub{canvas: publishedCanvasAggregate(canvasID, 1)}
	users := &userLookupStub{users: map[identity.UserID]*identity.User{}, calls: map[identity.UserID]int{}}
	svc := NewCommunityApplicationService(repo, canvases, users, fixedIDGenerator{id: 10})

	_, err := svc.PostComment(context.Background(), canvasID, 2, NewCommentRequest{Content: "reply", ParentCommentID: &parentID})
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	var domainErr *shared.DomainError
	if !errors.As(err, &domainErr) || domainErr.Message != "parent comment not found" {
		t.Fatalf("expected contextual parent error, got %v", err)
	}
}

func TestPostCommentRejectsNilParentFromRepository(t *testing.T) {
	canvasID := canvasDomain.CanvasID(7)
	parentID := int64(999)
	repo := &communityRepositoryStub{}
	canvases := &canvasRepositoryStub{canvas: publishedCanvasAggregate(canvasID, 1)}
	users := &userLookupStub{users: map[identity.UserID]*identity.User{}, calls: map[identity.UserID]int{}}
	svc := NewCommunityApplicationService(repo, canvases, users, fixedIDGenerator{id: 10})

	_, err := svc.PostComment(context.Background(), canvasID, 2, NewCommentRequest{Content: "reply", ParentCommentID: &parentID})
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected not found for nil parent, got %v", err)
	}
}

func TestPostCommentReturnsUnauthorizedWhenTokenUserNoLongerExists(t *testing.T) {
	canvasID := canvasDomain.CanvasID(7)
	repo := &communityRepositoryStub{}
	canvases := &canvasRepositoryStub{canvas: publishedCanvasAggregate(canvasID, 1)}
	users := &userLookupStub{users: map[identity.UserID]*identity.User{}, calls: map[identity.UserID]int{}}
	svc := NewCommunityApplicationService(repo, canvases, users, fixedIDGenerator{id: 10})

	_, err := svc.PostComment(context.Background(), canvasID, 2, NewCommentRequest{Content: "comment"})
	if !errors.Is(err, shared.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
