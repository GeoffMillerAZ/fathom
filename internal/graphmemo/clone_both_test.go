package graphmemo

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func cloneBothSnapshot(t *testing.T, sc Scope) *Snapshot {
	t.Helper()
	s, err := SnapshotFor(context.Background(), sc, func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCloneBothProductionReusesCompletedSyntax(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: "clone-route", Service: "svc", Watermark: "w1"}
	snap := cloneBothSnapshot(t, sc)
	// Seed the existing syntax key with a completed graph carrying a marker
	// edge absent from the snapshot. This is a route probe: re-resolving Nodes
	// loses that edge. No production build hook or additional cache is needed.
	marked := fixtureNodes()
	marked[2].Calls = []string{"d.log"}
	syntax := callgraph.Build(marked)
	before := callgraph.Build(marked)
	_, err := graphGroup.Get(context.Background(), graphKey{sc, GraphSyntax}, func(context.Context) (*callgraph.Graph, int64, error) { return syntax, syntax.EstimatedBytes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	edges := []core.Edge{{FromSymbol: "svc::a.Handler", ToSymbol: "svc::d.log", FromFile: "a/h.go", Kind: core.EdgeIfaceDispatch}}
	var loads atomic.Int32
	typed := func(context.Context) ([]core.Edge, error) { loads.Add(1); return edges, nil }
	want := callgraph.BuildBoth(marked, edges)
	var wg sync.WaitGroup
	answers := make(chan *callgraph.Graph, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := GraphFor(context.Background(), snap, GraphBoth, typed)
			if err != nil {
				t.Error(err)
				return
			}
			answers <- g
		}()
	}
	wg.Wait()
	close(answers)
	var first *callgraph.Graph
	for g := range answers {
		if !reflect.DeepEqual(g, want) {
			t.Error("production GraphBoth re-resolved snapshot instead of cloning completed syntax")
		}
		if first == nil {
			first = g
		} else if first != g {
			t.Error("combined graph was not singleflighted")
		}
		if g == syntax {
			t.Error("combined graph shares mutable syntax instance")
		}
	}
	if first == nil || loads.Load() != 1 {
		t.Fatalf("combined missing or typed loads=%d", loads.Load())
	}
	if !reflect.DeepEqual(syntax, before) {
		t.Fatal("typed augmentation mutated published syntax")
	}
	if b := groupBytes(t, "graphstore"); b != syntax.EstimatedBytes()+first.EstimatedBytes() {
		t.Fatalf("graph accounting=%d want %d", b, syntax.EstimatedBytes()+first.EstimatedBytes())
	}
	if hit, err := GraphFor(context.Background(), snap, GraphBoth, typed); err != nil || hit != first {
		t.Fatal("memo hit changed")
	}
	sc.Watermark = "w2"
	next := cloneBothSnapshot(t, sc)
	fresh, err := GraphFor(context.Background(), next, GraphBoth, typed)
	if err != nil || !reflect.DeepEqual(fresh, callgraph.BuildBoth(next.Nodes, edges)) {
		t.Fatal("new watermark reused completed old syntax")
	}
}

func TestCloneBothNoAdditionalRetainedSyntax(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: "clone-cold", Service: "svc", Watermark: "w1"}
	snap := cloneBothSnapshot(t, sc)
	g, err := GraphFor(context.Background(), snap, GraphBoth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, callgraph.BuildBoth(snap.Nodes, nil)) {
		t.Fatal("cold both changed")
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphSyntax}); ok {
		t.Fatal("cold both created an additional retained syntax graph")
	}
	// Empty watermark must neither reuse a graph nor publish either key.
	unscoped := NewSnapshot(fixtureNodes())
	a, err := GraphFor(context.Background(), unscoped, GraphBoth, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GraphFor(context.Background(), unscoped, GraphBoth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("empty watermark memoized")
	}
}

func TestCloneBothFailureCancellationAndAdmission(t *testing.T) {
	reg := isolateGraphStore(t)
	sc := Scope{Store: "clone-laws", Service: "svc", Watermark: "w1"}
	snap := cloneBothSnapshot(t, sc)
	syntax, err := GraphFor(context.Background(), snap, GraphSyntax, nil)
	if err != nil {
		t.Fatal(err)
	}
	busy := errors.New("typed source unavailable")
	g, _, terr, err := GraphForTyped(context.Background(), snap, GraphBoth, func(context.Context) ([]core.Edge, error) { return nil, busy })
	if err != nil || !errors.Is(terr, busy) || g != syntax {
		t.Fatal("typed failure failed to disclose/degrade to original syntax")
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
		t.Fatal("typed failure cached combined graph")
	}
	started := make(chan struct{})
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := GraphFor(ctx, snap, GraphBoth, func(ctx context.Context) ([]core.Edge, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(waitSafety):
		cancel()
		t.Fatal("typed loader did not start")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	select {
	case <-stopped:
	case <-time.After(waitSafety):
		t.Fatal("cancelled typed loader did not stop")
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
		t.Fatal("cancelled build published combined graph")
	}
	// Graph kinds share the service's completed-build cost, so the syntax
	// build above already measured this scope. Use a distinct cold scope to
	// assert cold admission rather than demanding the default estimate after
	// it has legitimately been replaced by a measured one.
	cold := cloneBothSnapshot(t, Scope{Store: "clone-admission", Service: "cold", Watermark: "w1"})
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelShed })
	short, stop := context.WithTimeout(context.Background(), graphCost/10)
	defer stop()
	_, err = GraphFor(short, cold, GraphBoth, nil)
	if !errors.Is(err, memo.ErrWouldNotFinish) {
		t.Fatalf("combined deadline admission=%v", err)
	}
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelNormal })
	good, err := GraphFor(context.Background(), snap, GraphBoth, nil)
	if err != nil || !reflect.DeepEqual(good, callgraph.BuildBoth(snap.Nodes, nil)) {
		t.Fatalf("failed/cancelled flight did not recover: %v", err)
	}
}
