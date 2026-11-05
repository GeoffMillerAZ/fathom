package graphmemo

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// TestGraphForTypedNeverReadsADeclineAsATypedFailure (RT-191): a typed-edge
// load declined for its caller's deadline is the call's error, never the
// typed-edge read failure that degrades it to the syntax graph (I38).
func TestGraphForTypedNeverReadsADeclineAsATypedFailure(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: "decline-typed", Service: "svc", Watermark: "w1"}
	snap, err := SnapshotFor(context.Background(), sc, func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(context.Background(), typedCost/10)
	defer cancel()
	g, _, typedErr, err := GraphForTyped(short, snap, GraphBoth, func(context.Context) ([]core.Edge, error) {
		t.Error("a declined typed load ran")
		return nil, nil
	})
	if !errors.Is(err, memo.ErrWouldNotFinish) || typedErr != nil || g != nil {
		t.Fatalf("GraphForTyped = graph %v, typedErr %v, err %v; want memo.ErrWouldNotFinish as err", g != nil, typedErr, err)
	}
}
