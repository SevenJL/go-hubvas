package canvas

import (
	"context"
	"errors"
	"testing"

	canvasDomain "github.com/hubvas/internal/domain/canvas"
	"github.com/hubvas/internal/domain/identity"
	"github.com/hubvas/internal/domain/shared"
)

type canvasRepoStub struct {
	canvas *canvasDomain.Canvas
	saves  int
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
func (r *canvasRepoStub) Delete(context.Context, canvasDomain.CanvasID) error { return nil }

type snapshotRepoStub struct{}

func (snapshotRepoStub) Save(context.Context, canvasDomain.CanvasID, []byte, string) error {
	return nil
}
func (snapshotRepoStub) Load(context.Context, canvasDomain.CanvasID) ([]byte, string, error) {
	return nil, "", nil
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
	svc := NewCanvasApplicationService(repo, snapshotRepoStub{}, nil, fixedIDGenerator{id: 2})

	fork, err := svc.Fork(context.Background(), source.ID(), identity.UserID(99))
	if err != nil {
		t.Fatalf("published fork failed: %v", err)
	}
	if fork.OwnerID != 99 || repo.saves != 1 {
		t.Fatalf("unexpected fork result: %#v, saves=%d", fork, repo.saves)
	}
}
