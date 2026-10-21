package memo

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestNavBuildCostForBothDoesNotRequireOtherKinds(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	sc := Scope{Store: "scoped-cost", Service: "svc", Watermark: "w1"}
	if _, resident := NavBuildCostFor(sc, GraphBoth); resident {
		t.Fatal("unbuilt Both is resident")
	}
	if NavBuildMeasuredFor(sc, GraphBoth) {
		t.Fatal("unbuilt Both is measured")
	}
	snap, err := SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GraphFor(ctx, snap, GraphBoth, func(context.Context) ([]core.Edge, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if _, resident := NavBuildCostFor(sc, GraphBoth); !resident {
		t.Fatal("snapshot + typed + Both should be resident")
	}
	if !NavBuildMeasuredFor(sc, GraphBoth) {
		t.Fatal("Both should be measured")
	}
	typedGroup.Remove(typedKey{sc}, ReasonBudget)
	if _, resident := NavBuildCostFor(sc, GraphBoth); resident {
		t.Fatal("Both graph alone cannot serve typed-edge details")
	}
	if !NavBuildMeasuredFor(sc, GraphBoth) {
		t.Fatal("evicted typed edges lost their service cost estimate")
	}
	if _, resident := NavBuildCost(sc); resident {
		t.Fatal("full navigation incorrectly resident without Syntax and Resolved")
	}
	// Measurement is per service and graph group, so the completed Both
	// build supplies an estimate for the other two kinds without making
	// them resident. Keep the full accessor's existing meaning.
	if !NavBuildMeasured(sc) {
		t.Fatal("full navigation lost its per-service graph estimate")
	}
	sc.Watermark = "w2"
	if _, resident := NavBuildCostFor(sc, GraphBoth); resident {
		t.Fatal("new watermark reused old Both")
	}
	if !NavBuildMeasuredFor(sc, GraphBoth) {
		t.Fatal("new watermark lost the service's completed cost estimate")
	}
}
