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
	for _, kind := range []GraphKind{GraphSyntax, GraphResolved} {
		if _, resident := NavBuildCostFor(sc, kind); resident {
			t.Fatalf("unbuilt graph kind %v is resident", kind)
		}
		// Measurements are per service and graph group, so Both supplies
		// an estimate for other kinds without making them resident.
		if !NavBuildMeasuredFor(sc, kind) {
			t.Fatalf("graph kind %v lost its per-service estimate", kind)
		}
	}
	sc.Watermark = "w2"
	if _, resident := NavBuildCostFor(sc, GraphBoth); resident {
		t.Fatal("new watermark reused old Both")
	}
	if !NavBuildMeasuredFor(sc, GraphBoth) {
		t.Fatal("new watermark lost the service's completed cost estimate")
	}
}
