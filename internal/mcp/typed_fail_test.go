package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// typedFailFake fails its first typed-edge read (SQLITE_BUSY, a heap-limit
// NOMEM), then serves one resolved edge.
type typedFailFake struct {
	patchFake
	typedCalls int
}

var errTypedBusy = errors.New("database is locked")

func (f *typedFailFake) TypedEdges(context.Context, string) ([]core.Edge, error) {
	f.typedCalls++
	if f.typedCalls == 1 {
		return nil, errTypedBusy
	}
	return []core.Edge{{FromSymbol: f.nodes[0].Key(), ToSymbol: f.nodes[1].Key(), Kind: core.EdgeDirect}}, nil
}

// TestNavGraphs_TypedReadFailureIsNotCachedForTheWatermark (review
// finding, I38): one failed typed-edge read made TypedAvailable false —
// every resolved-by-default nav answer heuristic — for the whole
// watermark, because the loader read the failure as "no typed edges" and
// the graph store cached that. The failing call degrades and names the
// failure; the next call reads the typed edges and answers resolved.
func TestNavGraphs_TypedReadFailureIsNotCachedForTheWatermark(t *testing.T) {
	f := &typedFailFake{patchFake: patchFake{nodes: []core.SymbolNode{
		{Service: "svc", Package: "main", Name: "A", FilePath: "a.go", Line: 1, Kind: core.KindGoFunc},
		{Service: "svc", Package: "main", Name: "B", FilePath: "b.go", Line: 1, Kind: core.KindGoFunc},
	}}}
	ctx := context.Background()
	g, err := loadNavGraphsErr(ctx, f, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if g.TypedAvailable || !errors.Is(g.TypedErr, errTypedBusy) {
		t.Fatalf("failed read: TypedAvailable %v, TypedErr %v; want degraded and named", g.TypedAvailable, g.TypedErr)
	}
	g, err = loadNavGraphsErr(ctx, f, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if g.TypedErr != nil || !g.TypedAvailable {
		t.Fatalf("second call: TypedAvailable %v, TypedErr %v; the failure was cached for the watermark", g.TypedAvailable, g.TypedErr)
	}
	if f.loads != 1 {
		t.Fatalf("symbol loads = %d, want 1 (the snapshot is cached; only the typed read is retried)", f.loads)
	}
}
