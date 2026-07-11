package canvas

import (
	"context"
	"errors"
	"testing"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	communityDomain "github.com/hubvas/internal/domain/community"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type canvasRepoStub struct {
	canvas  *canvasDomain.Canvas
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
	return nil, nil
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
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, fixedIDGenerator{id: 2})

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
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, fixedIDGenerator{id: 2})

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
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, communityRepo, fixedIDGenerator{id: 2})

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
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, communityRepo, fixedIDGenerator{id: 2})

	if _, err := svc.Fork(context.Background(), source.ID(), identity.UserID(99)); err == nil {
		t.Fatal("expected fork error")
	}
	if len(repo.deletes) != 1 || repo.deletes[0] != 2 {
		t.Fatalf("expected compensating delete for canvas 2, got %#v", repo.deletes)
	}
}
