package fusion

// RT-178 (the wave-B live reading's F1): the in-process typed pass
// (go/packages inside ingest) is an ingest-class producer. At refuse it
// stops between packages and files with the producer governor's named
// ErrOverBudget, cancels its `go list` child, writes no further package, and
// the surrounding Ingest fails so the watermark stays put (RT-109).

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func TestTypedPassHaltsAtRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	st := store.NewMemoryStore()
	ctx := tenancy.WithProducerClass(context.Background(), tenancy.ClassIngest)

	tenancy.PublishLevel(tenancy.LevelRefuse)
	rep, err := EnrichTypedEdges(ctx, st, "typed-refuse", typedPassFixtureDir(t), nil)
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("typed pass at refuse returned err=%v (report %+v), want an error wrapping ErrOverBudget", err, rep)
	}
	if ids, _ := st.ListTypedEdgePackages(context.Background(), "typed-refuse"); len(ids) != 0 {
		t.Fatalf("typed pass at refuse wrote %d package(s), want none", len(ids))
	}

	// At shed an ingest producer is still allowed: the pass completes.
	tenancy.PublishLevel(tenancy.LevelShed)
	if _, err := EnrichTypedEdges(ctx, st, "typed-shed", typedPassFixtureDir(t), nil); err != nil {
		t.Fatalf("typed pass at shed: %v, want it to run", err)
	}
}

// The ladder reaches refuse while go/packages is parsing: the pass stops
// inside Load (the governed ParseFile cancels the load), never type-checks
// the rest, and writes nothing.
func TestTypedPassHaltsWhenRefuseLandsDuringLoad(t *testing.T) {
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		onTypedPassParse = nil
	})
	var parses atomic.Int64
	onTypedPassParse = func(string) {
		if parses.Add(1) == 1 {
			tenancy.PublishLevel(tenancy.LevelRefuse)
		}
	}
	st := store.NewMemoryStore()
	_, err := EnrichTypedEdges(context.Background(), st, "typed-midload", typedPassFixtureDir(t), nil)
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("typed pass with refuse landing during Load returned %v, want ErrOverBudget", err)
	}
	if ids, _ := st.ListTypedEdgePackages(context.Background(), "typed-midload"); len(ids) != 0 {
		t.Fatalf("typed pass halted during Load wrote %d package(s), want none", len(ids))
	}
}

// Refuse lands between two packages of the store loop: the pass stops
// before the next package; the Ingest around it fails with the named error.
func TestIngestFailsWhenTypedPassHaltsBetweenPackages(t *testing.T) {
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		onTypedPassPackage = nil
	})
	var seen atomic.Int64
	onTypedPassPackage = func(string) {
		if seen.Add(1) == 1 {
			tenancy.PublishLevel(tenancy.LevelRefuse)
		}
	}
	dir := typedPassFixtureDir(t)
	st := store.NewMemoryStore()
	p := &Pipeline{st: st}
	svc := core.ServiceRef{ID: "typed-between", Path: dir}
	refs := []core.SourceRef{{Service: svc.ID, Path: "direct.go"}}
	_, err := p.enrichTypedEdges(context.Background(), svc, refs)
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("enrichTypedEdges halted between packages returned %v, want ErrOverBudget", err)
	}
	if ids, _ := st.ListTypedEdgePackages(context.Background(), svc.ID); len(ids) > 1 {
		t.Fatalf("halted pass wrote %d packages, want at most the one before the halt", len(ids))
	}
	if s, ok := p.TypedEdgeStatus(svc.ID); !ok || !s.Failed {
		t.Fatalf("typed status after a halt = %+v (known %v), want Failed with the named reason", s, ok)
	}
}

// The extraction phase asks the governor before each file.
func TestExtractAndStoreHaltsAtRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	p := New(nil, store.NewMemoryStore(), Config{})
	tenancy.PublishLevel(tenancy.LevelRefuse)
	_, err := p.extractAndStore(context.Background(), "x", []pendingFile{{ref: core.SourceRef{Service: "x", Path: "a.go"}, content: []byte("package a\n"), hash: "h"}})
	if !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("extractAndStore at refuse returned %v, want ErrOverBudget", err)
	}
}
