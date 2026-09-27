package memo

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestGraphForTyped_ReadFailureIsNeverCached (review finding, I38): a
// typed-edge read failure (SQLITE_BUSY, a heap-limit NOMEM) must not be
// cached as "no typed edges" for the whole watermark — that turned every
// resolved-by-default nav answer heuristic until the next ingest. The
// failing call degrades to the syntax graph and names the failure; the
// next call reads the typed edges again.
func TestGraphForTyped_ReadFailureIsNeverCached(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	sc := Scope{Store: "typed-fail", Service: "svc", Watermark: "w1"}
	snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	busy := errors.New("database is locked")
	typed := func(context.Context) ([]core.Edge, error) {
		if calls.Add(1) == 1 {
			return nil, busy
		}
		return []core.Edge{{FromSymbol: "svc::a.Handler", ToSymbol: "svc::d.log", Kind: core.EdgeDirect}}, nil
	}
	g, edges, terr, err := GraphForTyped(ctx, snap, GraphBoth, typed)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(terr, busy) || edges != nil || g == nil {
		t.Fatalf("failed read: graph %v, edges %v, typedErr %v; want the syntax graph and the named failure", g != nil, edges, terr)
	}
	if _, ok := typedGroup.Peek(typedKey{sc}); ok {
		t.Fatal("a failed typed read was cached")
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
		t.Fatal("a both graph built without the typed edges was cached")
	}
	g2, edges2, terr2, err := GraphForTyped(ctx, snap, GraphBoth, typed)
	if err != nil || terr2 != nil || len(edges2) != 1 || g2 == g {
		t.Fatalf("second call: edges %d, typedErr %v, err %v, same graph %v; want the typed edges read again", len(edges2), terr2, err, g2 == g)
	}
	if _, _, terr3, err := GraphForTyped(ctx, snap, GraphResolved, typed); err != nil || terr3 != nil {
		t.Fatalf("resolved after recovery: typedErr %v, err %v", terr3, err)
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphResolved}); !ok {
		t.Fatal("the resolved graph built on read typed edges was not cached")
	}
}
