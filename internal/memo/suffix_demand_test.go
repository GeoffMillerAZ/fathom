package memo

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Same-watermark package refresh must admit new raw-call demand; a held
// graph stays immutable. Resolved-only graphs are evicted rather than
// acquiring syntax edges. A new watermark must replace the old snapshot.
func TestSuffixDemandGraphStorePatchAndFreshness(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	sc := Scope{Store: "suffix-demand-test", Service: "fixture", Watermark: "w1"}
	caller := core.SymbolNode{Service: "fixture", Package: "api", Name: "Entry", Kind: core.KindGoFunc, FilePath: "api/entry.go"}
	target := core.SymbolNode{Service: "fixture", Package: "Handlers", Name: "Handle.Execute", Kind: core.KindCSharpMethod, FilePath: "lib/handle.cs"}
	nodes := []core.SymbolNode{caller, target}
	snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return nodes, nil })
	if err != nil {
		t.Fatal(err)
	}
	old := map[GraphKind]*callgraph.Graph{}
	for _, kind := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		old[kind], err = GraphFor(ctx, snap, kind, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	caller.Calls = []string{"receiver.Execute"}
	PatchPackage(sc, "api", []core.SymbolNode{caller}, nil)
	patched, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) {
		t.Fatal("patch lost cached snapshot")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		got, err := GraphFor(ctx, patched, kind, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := kind != GraphResolved
		if callgraph.Reachable(got, caller.Key(), target.Key()) != want {
			t.Fatalf("kind %v: newly demanded call changed", kind)
		}
		if old[kind].EdgeCount() != 0 {
			t.Fatal("held prepatch graph mutated")
		}
	}
	sc.Watermark = "w2"
	fresh, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return nodes, nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []GraphKind{GraphSyntax, GraphBoth, GraphResolved} {
		got, err := GraphFor(ctx, fresh, kind, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.EdgeCount() != 0 {
			t.Fatalf("kind %v reused stale query at new watermark", kind)
		}
	}
}

// A typed-read failure is an error, never a cached syntax-only success.
// Retrying the same scope after recovery must build the typed graph.
func TestSuffixDemandGraphStoreTypedFailure(t *testing.T) {
	for _, kind := range []GraphKind{GraphBoth, GraphResolved} {
		t.Run(string(kind), func(t *testing.T) {
			isolateGraphStore(t)
			ctx := context.Background()
			sc := Scope{Store: "suffix-demand-failure", Service: "svc", Watermark: "w1"}
			nodes := fixtureNodes()
			snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return nodes, nil })
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("typed read failed")
			got, err := GraphFor(ctx, snap, kind, func(context.Context) ([]core.Edge, error) { return nil, failure })
			if got != nil || !errors.Is(err, failure) {
				t.Fatalf("failure returned graph=%v error=%v", got, err)
			}
			if _, ok := graphGroup.Peek(graphKey{sc, kind}); ok {
				t.Fatal("failed graph cached")
			}
			edge := core.Edge{FromSymbol: nodes[0].Key(), ToSymbol: nodes[3].Key(), FromFile: nodes[0].FilePath, Kind: core.EdgeDirect}
			got, err = GraphFor(ctx, snap, kind, func(context.Context) ([]core.Edge, error) { return []core.Edge{edge}, nil })
			if err != nil || got == nil || !callgraph.Reachable(got, edge.FromSymbol, edge.ToSymbol) {
				t.Fatalf("typed recovery graph=%v error=%v", got, err)
			}
		})
	}
}
