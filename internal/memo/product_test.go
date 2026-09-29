package memo

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestProductReachFor_OnePerWatermarkDroppedWithItsGraph: the product-entry
// reach set verify's `wired` criterion reads (ADR-018 amendment
// 2026-09-28) is walked once per (store, service, watermark, kind) and
// shared; a package patch or a dropped scope drops it with the graph it
// was walked over, so it is never read against a graph it did not come
// from.
func TestProductReachFor_OnePerWatermarkDroppedWithItsGraph(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "main", Name: "main", Kind: core.KindGoFunc, FilePath: "cmd/app/main.go", Calls: []string{"lib.Run"}},
		{Service: "svc", Package: "lib", Name: "Run", Kind: core.KindGoFunc, FilePath: "lib/lib.go"},
		{Service: "svc", Package: "lib", Name: "Spare", Kind: core.KindGoFunc, FilePath: "lib/lib.go"},
	}
	var loads atomic.Int32
	load := func(context.Context) ([]core.SymbolNode, error) { loads.Add(1); return nodes, nil }
	sc := Scope{Store: "product-test", Service: "svc", Watermark: "w1"}
	snap, err := SnapshotFor(ctx, sc, load)
	if err != nil {
		t.Fatal(err)
	}
	a, err := ProductReachFor(ctx, snap, GraphSyntax, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ProductReachFor(ctx, snap, GraphSyntax, nil)
	if a != b {
		t.Fatal("two callers at one (service, watermark, kind) walked twice")
	}
	if _, ok := a.ReachedBy("Run"); !ok {
		t.Fatal("main reaches lib.Run")
	}
	if _, ok := a.ReachedBy("Spare"); ok {
		t.Fatal("nothing reaches lib.Spare")
	}

	PatchPackage(sc, "lib", nodes[1:], nil)
	if _, ok := productGroup.Peek(graphKey{sc, GraphSyntax}); ok {
		t.Fatal("a package patch left the reach set of the pre-patch graph cached")
	}
	c, _ := ProductReachFor(ctx, snap, GraphSyntax, nil)
	if c == a {
		t.Fatal("the reach set was not re-walked after the patch")
	}
	DropScope(sc)
	if _, ok := productGroup.Peek(graphKey{sc, GraphSyntax}); ok {
		t.Fatal("DropScope left the reach set cached")
	}
}
