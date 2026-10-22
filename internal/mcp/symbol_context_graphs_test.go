package mcp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// Empty watermarks prevent memo hits from hiding cold allocations.
type contextGraphBackend struct {
	*slowSnapshotBackend
	typed        []core.Edge
	typedFailure error
	typedReads   int
}

func (b *contextGraphBackend) StoreWatermark(context.Context, string) (string, error) { return "", nil }
func (b *contextGraphBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	b.typedReads++
	return b.typed, b.typedFailure
}

func TestSymbolContextNavigationParity(t *testing.T) {
	base := newSlowSnapshotBackend(t)
	close(base.gate)
	b := &contextGraphBackend{slowSnapshotBackend: base}
	b.typed = []core.Edge{{FromService: "svc", FromSymbol: b.nodes[0].Key(), ToSymbol: b.nodes[1].Key(), Kind: core.EdgeDirect}}
	for _, failure := range []error{nil, errors.New("typed unavailable")} {
		b.typedFailure = failure
		sc := navScope(context.Background(), b, "svc")
		want, err := loadNavGraphsAt(context.Background(), b, "svc", sc)
		if err != nil {
			t.Fatal(err)
		}
		got, cold, err := navGraphsWithin(context.Background(), b, "svc", sc)
		if err != nil || cold != nil {
			t.Fatalf("cold=%v err=%v", cold, err)
		}
		if !reflect.DeepEqual(got.Nodes, want.Nodes) || !reflect.DeepEqual(got.Typed, want.Typed) || !reflect.DeepEqual(got.TypedErr, want.TypedErr) {
			t.Fatal("navigation inputs changed")
		}
		for _, n := range b.nodes {
			if !reflect.DeepEqual(symbolnav.SymbolContext(got.Nodes, got.Both, n.Key()), symbolnav.SymbolContext(want.Nodes, want.Both, n.Key())) {
				t.Fatalf("context changed for %s", n.Key())
			}
		}
		actual := callToolWithin(t, context.Background(), NewServer(b), "symbol_context", map[string]any{"service": "svc", "symbol": "Run", "snippet": true})
		if actual["def"] == nil || actual["snippet"] == nil {
			t.Fatalf("definition/snippet lost: %v", actual)
		}
	}
	b.typedFailure = nil
	got := callToolWithin(t, context.Background(), NewServer(b), "symbol_context", map[string]any{"service": "svc", "symbol": "Run", "snippet": false})
	if got["typed_edges"] == nil {
		t.Fatal("failed typed read was cached")
	}
}

func TestSymbolContextNavigationAllocationBound(t *testing.T) {
	base := newSlowSnapshotBackend(t)
	close(base.gate)
	b := &contextGraphBackend{slowSnapshotBackend: base}
	for i := 0; i < 1024; i++ {
		b.nodes = append(b.nodes, core.SymbolNode{Service: "svc", Package: "work", FilePath: "work/run.go", Name: fmt.Sprintf("Extra%d", i), Kind: core.KindGoFunc})
	}
	sc := memo.Scope{}
	var full *navGraphs
	var sink *symbolContextGraphs
	baseline := testing.AllocsPerRun(3, func() {
		var err error
		full, err = loadNavGraphsAt(context.Background(), b, "svc", sc)
		if err != nil {
			t.Fatal(err)
		}
	})
	allocs := testing.AllocsPerRun(3, func() {
		var err error
		sink, _, err = navGraphsWithin(context.Background(), b, "svc", sc)
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(full.Nodes) != len(sink.Nodes) {
		t.Fatal("snapshot size changed")
	}
	t.Logf("full=%.0f context=%.0f nodes=%d", baseline, allocs, len(sink.Nodes))
	if allocs > baseline*0.65 {
		t.Fatalf("context allocations %.0f exceed 65%% of full navigation %.0f", allocs, baseline)
	}
}
