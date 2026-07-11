package canvas

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

type canvasRepoStub struct {
	canvas  *canvasDomain.Canvas
	members []*canvasDomain.Canvas
	saves   int
	deletes []canvasDomain.CanvasID
}

func (r *canvasRepoStub) Save(_ context.Context, c *canvasDomain.Canvas) error {
	r.canvas = c
	r.saves++
	return nil
}
func (r *canvasRepoStub) FindByID(context.Context, canvasDomain.CanvasID) (*canvasDomain.Canvas, error) {
	return r.canvas, nil
}
func (r *canvasRepoStub) FindByOwner(context.Context, identity.UserID) ([]*canvasDomain.Canvas, error) {
	return nil, nil
}
func (r *canvasRepoStub) FindByMember(context.Context, identity.UserID) ([]*canvasDomain.Canvas, error) {
	return r.members, nil
}
func (r *canvasRepoStub) Delete(_ context.Context, id canvasDomain.CanvasID) error {
	r.deletes = append(r.deletes, id)
	return nil
}

type snapshotRepoStub struct{}

func (snapshotRepoStub) Save(context.Context, canvasDomain.CanvasID, []byte, string) error {
	return nil
}
func (snapshotRepoStub) Load(context.Context, canvasDomain.CanvasID) ([]byte, string, error) {
	return nil, "", nil
}

type communityRepoStub struct {
	forks   []*communityDomain.Fork
	forkErr error
}

func (r *communityRepoStub) SavePublished(context.Context, *communityDomain.PublishedCanvas) error {
	return nil
}
func (r *communityRepoStub) SaveFork(_ context.Context, fork *communityDomain.Fork) error {
	r.forks = append(r.forks, fork)
	return r.forkErr
}

type fixedIDGenerator struct{ id int64 }

func (g fixedIDGenerator) NextID() int64 { return g.id }

func TestGetPrivateCanvasRejectsNonMember(t *testing.T) {
	source, err := canvasDomain.NewCanvas(1, identity.UserID(10), "private")
	if err != nil {
		t.Fatal(err)
	}
	repo := &canvasRepoStub{canvas: source}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, nil, fixedIDGenerator{id: 2})

	_, err = svc.Get(context.Background(), source.ID(), identity.UserID(99))
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestForkPrivateCanvasRejectsNonMember(t *testing.T) {
	source, err := canvasDomain.NewCanvas(1, identity.UserID(10), "private")
	if err != nil {
		t.Fatal(err)
	}
	repo := &canvasRepoStub{canvas: source}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, nil, fixedIDGenerator{id: 2})

	_, err = svc.Fork(context.Background(), source.ID(), identity.UserID(99))
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if repo.saves != 0 {
		t.Fatalf("unauthorized fork must not be saved; got %d saves", repo.saves)
	}
}

func TestForkPublishedCanvasAllowsAuthenticatedUser(t *testing.T) {
	source, err := canvasDomain.NewCanvas(1, identity.UserID(10), "published")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Publish(); err != nil {
		t.Fatal(err)
	}
	repo := &canvasRepoStub{canvas: source}
	communityRepo := &communityRepoStub{}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, communityRepo, nil, fixedIDGenerator{id: 2})

	fork, err := svc.Fork(context.Background(), source.ID(), identity.UserID(99))
	if err != nil {
		t.Fatalf("published fork failed: %v", err)
	}
	if fork.OwnerID != 99 || repo.saves != 1 {
		t.Fatalf("unexpected fork result: %#v, saves=%d", fork, repo.saves)
	}
	if len(communityRepo.forks) != 1 {
		t.Fatalf("expected one community fork record, got %d", len(communityRepo.forks))
	}
	recorded := communityRepo.forks[0]
	if recorded.OriginalCanvasID() != source.ID() || recorded.NewCanvasID() != 2 || recorded.UserID() != 99 {
		t.Fatalf("unexpected fork lineage: %#v", recorded)
	}
}

func TestForkCompensatesCanvasWhenCommunityRecordFails(t *testing.T) {
	source, err := canvasDomain.NewCanvas(1, identity.UserID(10), "published")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Publish(); err != nil {
		t.Fatal(err)
	}
	repo := &canvasRepoStub{canvas: source}
	communityRepo := &communityRepoStub{forkErr: errors.New("community unavailable")}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, communityRepo, nil, fixedIDGenerator{id: 2})

	if _, err := svc.Fork(context.Background(), source.ID(), identity.UserID(99)); err == nil {
		t.Fatal("expected fork error")
	}
	if len(repo.deletes) != 1 || repo.deletes[0] != 2 {
		t.Fatalf("expected compensating delete for canvas 2, got %#v", repo.deletes)
	}
}

type canvasUserLookupStub struct {
	byID       map[identity.UserID]*identity.User
	byUsername map[string]*identity.User
}

func (r *canvasUserLookupStub) FindByID(_ context.Context, id identity.UserID) (*identity.User, error) {
	user, ok := r.byID[id]
	if !ok {
		return nil, shared.NewDomainError(shared.ErrNotFound, "user not found")
	}
	return user, nil
}

func (r *canvasUserLookupStub) FindByUsername(_ context.Context, username string) (*identity.User, error) {
	user, ok := r.byUsername[username]
	if !ok {
		return nil, shared.NewDomainError(shared.ErrNotFound, "user not found")
	}
	return user, nil
}

func testUser(id identity.UserID, username string) *identity.User {
	return identity.ReconstituteUser(id, username, username+"@example.com", "hash", "", time.Now())
}

func TestListSharedExcludesOwnedCanvasesAndSetsRole(t *testing.T) {
	owned, _ := canvasDomain.NewCanvas(1, 10, "owned")
	sharedCanvas, _ := canvasDomain.NewCanvas(2, 20, "shared")
	sharedCanvas.AddMember(10, canvasDomain.RoleViewer)
	repo := &canvasRepoStub{members: []*canvasDomain.Canvas{owned, sharedCanvas}}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, nil, fixedIDGenerator{id: 3})

	items, err := svc.ListShared(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 2 || items[0].CurrentRole != "viewer" {
		t.Fatalf("unexpected shared canvases: %#v", items)
	}
}

func TestOwnerCanAddUpdateAndRemoveMember(t *testing.T) {
	c, _ := canvasDomain.NewCanvas(1, 10, "canvas")
	repo := &canvasRepoStub{canvas: c}
	alice := testUser(20, "alice")
	users := &canvasUserLookupStub{
		byID:       map[identity.UserID]*identity.User{20: alice},
		byUsername: map[string]*identity.User{"alice": alice},
	}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, users, fixedIDGenerator{id: 2})

	member, err := svc.AddMember(context.Background(), 1, 10, AddMemberRequest{Username: "alice", Role: "editor"})
	if err != nil || member.Role != "editor" || c.GetRole(20) != canvasDomain.RoleEditor {
		t.Fatalf("add failed: member=%#v err=%v", member, err)
	}
	member, err = svc.UpdateMemberRole(context.Background(), 1, 10, 20, UpdateMemberRoleRequest{Role: "viewer"})
	if err != nil || member.Role != "viewer" || c.GetRole(20) != canvasDomain.RoleViewer {
		t.Fatalf("update failed: member=%#v err=%v", member, err)
	}
	if err := svc.RemoveMember(context.Background(), 1, 10, 20); err != nil || c.IsMember(20) {
		t.Fatalf("remove failed: err=%v member=%v", err, c.IsMember(20))
	}
}

func TestNonOwnerCannotManageMembersOrChangeOwnerRole(t *testing.T) {
	c, _ := canvasDomain.NewCanvas(1, 10, "canvas")
	c.AddMember(20, canvasDomain.RoleEditor)
	repo := &canvasRepoStub{canvas: c}
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, &canvasUserLookupStub{}, fixedIDGenerator{id: 2})

	_, err := svc.UpdateMemberRole(context.Background(), 1, 20, 10, UpdateMemberRoleRequest{Role: "viewer"})
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("expected non-owner forbidden, got %v", err)
	}
	_, err = svc.UpdateMemberRole(context.Background(), 1, 10, 10, UpdateMemberRoleRequest{Role: "viewer"})
	if !errors.Is(err, shared.ErrInvalidArgument) {
		t.Fatalf("expected owner role protection, got %v", err)
	}
}
