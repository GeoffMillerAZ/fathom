package graphmemo

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
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
	// A three-edge adjacency has spare capacity in a fresh Build. Cloning
	// trims it, giving a route fingerprint without mismatching snapshot data.
	nodes := fixtureNodes()
	nodes[0].Calls = []string{"b.Store", "c.Log", "d.log"}
	snap, err := SnapshotFor(context.Background(), sc, func(context.Context) ([]core.SymbolNode, error) { return nodes, nil })
	if err != nil {
		t.Fatal(err)
	}
	syntax := callgraph.Build(nodes)
	before := callgraph.Build(nodes)
	_, err = graphGroup.Get(context.Background(), graphKey{sc, GraphSyntax}, func(context.Context) (*callgraph.Graph, int64, error) { return syntax, syntax.EstimatedBytes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	edges := []core.Edge{{FromSymbol: "svc::a.Handler", ToSymbol: "svc::d.log", FromFile: "a/h.go", Kind: core.EdgeIfaceDispatch}}
	var loads atomic.Int32
	typed := func(context.Context) ([]core.Edge, error) { loads.Add(1); return edges, nil }
	want := callgraph.BuildBoth(nodes, edges)
	cloneBytes := syntax.WithTypedEdges(edges).EstimatedBytes()
	if cloneBytes == want.EstimatedBytes() {
		t.Fatal("fixture lacks clone capacity fingerprint")
	}
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
			t.Error("production GraphBoth differs from fresh full-field oracle")
		}
		if g.EstimatedBytes() != cloneBytes {
			t.Error("production GraphBoth re-resolved matching syntax instead of cloning it")
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

// F1: a package patch may leave other packages' syntax calls approximate.
// The first BOTH load must retain its pre-P2 fresh-build answer in that case.
func TestCloneBothFirstLoadAfterProductionRefresh(t *testing.T) {
	isolateGraphStore(t)
	extract := func(file, src string) []core.SymbolNode {
		ns, err := extractor.NewGoExtractor().Extract("svc", file, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		return ns
	}
	an := extract("a/handler.go", "package a\ntype A struct{}\nfunc (A) Run() {}\n")
	bn := extract("b/handler.go", "package b\ntype B struct{}\n")
	changed := extract("b/handler.go", "package b\ntype B struct{}\nfunc (B) Run() {}\n")
	cn := extract("c/call.go", "package c\nimport \"example/a\"\nfunc Call(h a.A) { h.Run() }\n")
	named := func(ns []core.SymbolNode, name string) core.SymbolNode {
		for _, n := range ns {
			if n.Name == name {
				return n
			}
		}
		t.Fatalf("missing extracted %s", name)
		return core.SymbolNode{}
	}
	ar, br, caller := named(an, "A.Run"), named(changed, "B.Run"), named(cn, "Call")
	if !reflect.DeepEqual(caller.Calls, []string{"h.Run"}) {
		t.Fatalf("not the receiver-call witness: %v", caller.Calls)
	}
	nodes := append(append(append([]core.SymbolNode{}, an...), bn...), cn...)
	orders := [][]core.SymbolNode{slices.Clone(nodes), slices.Clone(nodes), slices.Clone(nodes), slices.Clone(nodes)}
	slices.Reverse(orders[1])
	slices.SortStableFunc(orders[2], func(a, b core.SymbolNode) int { return strings.Compare(a.Key(), b.Key()) })
	rand.New(rand.NewSource(218)).Shuffle(len(orders[3]), func(i, j int) { orders[3][i], orders[3][j] = orders[3][j], orders[3][i] })
	for oi, order := range orders {
		for ti, edges := range [][]core.Edge{nil, {}, {{FromSymbol: caller.Key(), ToSymbol: br.Key(), Kind: core.EdgeDirect}}} {
			t.Run(fmt.Sprintf("order%d/typed%d", oi, ti), func(t *testing.T) {
				ctx := context.Background()
				sc := Scope{Store: fmt.Sprintf("refresh-clone-%d-%d", oi, ti), Service: "svc", Watermark: "same"}
				snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return order, nil })
				if err != nil {
					t.Fatal(err)
				}
				syntax, err := GraphFor(ctx, snap, GraphSyntax, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !callgraph.Reachable(syntax, caller.Key(), ar.Key()) {
					t.Fatal("fixture lacks initially unique A.Run edge")
				}
				before := callgraph.Build(order)
				if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
					t.Fatal("BOTH was not cold before refresh")
				}
				PatchPackage(sc, br.Package, []core.SymbolNode{br}, nil)
				refreshed, ok := PeekSnapshot(sc)
				if !ok {
					t.Fatal("refresh dropped snapshot")
				}
				seen := map[string]bool{}
				for _, n := range refreshed.Nodes {
					if seen[n.Key()] {
						t.Fatalf("duplicate key %q", n.Key())
					}
					seen[n.Key()] = true
				}
				if !seen[br.Key()] {
					t.Fatal("refresh did not add B.Run")
				}
				savedNodes := slices.Clone(refreshed.Nodes)
				patched, err := GraphFor(ctx, refreshed, GraphSyntax, nil)
				if err != nil {
					t.Fatal(err)
				}
				patchOracle := before.WithPackagePatch(br.Package, []core.SymbolNode{br}, nil)
				if patched.CanReuseSyntax(refreshed.Nodes) {
					t.Fatal("production refresh left syntax eligible for fresh BOTH reuse")
				}
				if !reflect.DeepEqual(patched, patchOracle) || !callgraph.Reachable(patched, caller.Key(), ar.Key()) {
					t.Fatal("existing approximate patch contract changed")
				}
				want := callgraph.BuildBoth(refreshed.Nodes, edges)
				if callgraph.Reachable(want, caller.Key(), ar.Key()) {
					t.Fatal("fresh oracle should refuse ambiguous Run")
				}
				got, err := GraphFor(ctx, refreshed, GraphBoth, func(context.Context) ([]core.Edge, error) { return edges, nil })
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("first BOTH after production refresh differs from fresh BuildBoth (all Graph fields); inherited A.Run=%v", callgraph.Reachable(got, caller.Key(), ar.Key()))
				}
				if !reflect.DeepEqual(callgraph.CalleesDetail(got, caller.Key()), callgraph.CalleesDetail(want, caller.Key())) {
					t.Error("served receiver-call answer differs from fresh oracle")
				}
				if !reflect.DeepEqual(syntax, before) || !reflect.DeepEqual(patched, patchOracle) || !reflect.DeepEqual(refreshed.Nodes, savedNodes) {
					t.Fatal("first BOTH mutated a held graph or refreshed snapshot")
				}
			})
		}
	}
}

// F2 pins the controlled publication state without adding a production test hook.
func TestCloneBothRejectsMixedSnapshotPublication(t *testing.T) {
	isolateGraphStore(t)
	extract := func(file, src string) []core.SymbolNode {
		ns, err := extractor.NewGoExtractor().Extract("svc", file, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		return ns
	}
	an := extract("a/handler.go", "package a\ntype A struct{}\nfunc (A) Run() {}\n")
	bn := extract("b/handler.go", "package b\ntype B struct{}\n")
	changed := extract("b/handler.go", "package b\ntype B struct{}\nfunc (B) Run() {}\n")
	cn := extract("c/call.go", "package c\nimport \"example/a\"\nfunc Call(h a.A) { h.Run() }\n")
	named := func(ns []core.SymbolNode, name string) core.SymbolNode {
		for _, n := range ns {
			if n.Name == name {
				return n
			}
		}
		t.Fatalf("missing extracted %s", name)
		return core.SymbolNode{}
	}
	ar, br, caller := named(an, "A.Run"), named(changed, "B.Run"), named(cn, "Call")
	if !reflect.DeepEqual(caller.Calls, []string{"h.Run"}) {
		t.Fatalf("not the receiver-call witness: %v", caller.Calls)
	}
	nodes := append(append(append([]core.SymbolNode{}, an...), bn...), cn...)
	orders := [][]core.SymbolNode{slices.Clone(nodes), slices.Clone(nodes), slices.Clone(nodes), slices.Clone(nodes)}
	slices.Reverse(orders[1])
	slices.SortStableFunc(orders[2], func(a, b core.SymbolNode) int { return strings.Compare(a.Key(), b.Key()) })
	rand.New(rand.NewSource(218)).Shuffle(len(orders[3]), func(i, j int) { orders[3][i], orders[3][j] = orders[3][j], orders[3][i] })
	for oi, order := range orders {
		for ti, edges := range [][]core.Edge{nil, {}, {{FromSymbol: caller.Key(), ToSymbol: br.Key(), Kind: core.EdgeDirect}}} {
			t.Run(fmt.Sprintf("order%d/typed%d", oi, ti), func(t *testing.T) {
				ctx := context.Background()
				sc := Scope{Store: fmt.Sprintf("publication-clone-%d-%d", oi, ti), Service: "svc", Watermark: "same"}
				snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return order, nil })
				if err != nil {
					t.Fatal(err)
				}
				syntax, err := GraphFor(ctx, snap, GraphSyntax, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !callgraph.Reachable(syntax, caller.Key(), ar.Key()) {
					t.Fatal("fixture lacks initially unique A.Run edge")
				}
				before := callgraph.Build(order)
				if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
					t.Fatal("BOTH was not cold before refresh")
				}
				// Construct the exact snapshot-published/syntax-not-yet-updated
				// boundary pinned by the actual PatchPackage barrier replay.
				snapshotGroup.Update(snapKey{sc}, func(old *Snapshot) (*Snapshot, int64) {
					kept := make([]core.SymbolNode, 0, len(old.Nodes)+1)
					for _, n := range old.Nodes {
						if n.Package != br.Package {
							kept = append(kept, n)
						}
					}
					kept = append(kept, br)
					next := newSnapshot(sc, kept)
					next.Extra = old.Extra
					return next, snapshotBytes(next)
				})
				refreshed, ok := PeekSnapshot(sc)
				if !ok {
					t.Fatal("refresh dropped snapshot")
				}
				seen := map[string]bool{}
				for _, n := range refreshed.Nodes {
					if seen[n.Key()] {
						t.Fatalf("duplicate key %q", n.Key())
					}
					seen[n.Key()] = true
				}
				if !seen[br.Key()] {
					t.Fatal("refresh did not add B.Run")
				}
				savedNodes := slices.Clone(refreshed.Nodes)
				patched, err := GraphFor(ctx, refreshed, GraphSyntax, nil)
				if err != nil {
					t.Fatal(err)
				}
				patchOracle := before
				if !patched.CanReuseSyntax(order) {
					t.Fatal("window must contain the previous unmarked syntax graph")
				}
				if !reflect.DeepEqual(patched, patchOracle) || !callgraph.Reachable(patched, caller.Key(), ar.Key()) {
					t.Fatal("previous-generation graph changed")
				}
				want := callgraph.BuildBoth(refreshed.Nodes, edges)
				if callgraph.Reachable(want, caller.Key(), ar.Key()) {
					t.Fatal("fresh oracle should refuse ambiguous Run")
				}
				beforeBytes := groupBytes(t, "graphstore")
				got, err := GraphFor(ctx, refreshed, GraphBoth, func(context.Context) ([]core.Edge, error) { return edges, nil })
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("first BOTH in publication window differs from fresh BuildBoth (all Graph fields); inherited A.Run=%v", callgraph.Reachable(got, caller.Key(), ar.Key()))
				}
				if !reflect.DeepEqual(callgraph.CalleesDetail(got, caller.Key()), callgraph.CalleesDetail(want, caller.Key())) {
					t.Error("served receiver-call answer differs from fresh oracle")
				}
				if bytes := groupBytes(t, "graphstore"); bytes != beforeBytes+got.EstimatedBytes() {
					t.Fatalf("publication graph bytes=%d", bytes)
				}
				if hit, err := GraphFor(ctx, refreshed, GraphBoth, nil); err != nil || hit != got {
					t.Fatal("publication BOTH memo identity changed")
				}
				if !reflect.DeepEqual(syntax, before) || !reflect.DeepEqual(patched, patchOracle) || !reflect.DeepEqual(refreshed.Nodes, savedNodes) {
					t.Fatal("first BOTH mutated a held graph or refreshed snapshot")
				}
			})
		}
	}
}
