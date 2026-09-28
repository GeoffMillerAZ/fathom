package fusion_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestCentralityAtIngest proves TQ-2 (docs/plans/test-intelligence.md;
// design doc docs/design/test-intelligence.md §1, §3 T1): centrality per
// symbol — its transitive caller count, normalized within the service — is
// computed at ingest and stored, so a later read is a lookup; the stored
// counts equal the condensation count over the same graph; and the result
// is memoized per watermark: an unchanged re-ingest keeps the stored rows,
// a moved watermark recomputes them.
func TestCentralityAtIngest(t *testing.T) {
	ctx := context.Background()
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	wm, rows, err := st.SymbolCentrality(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("SymbolCentrality: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no centrality rows stored at ingest")
	}
	if wm == "" {
		t.Error("centrality rows carry no watermark (the memo key)")
	}

	// The stored counts are exactly the condensation count over the
	// stored graph.
	obs, _ := st.QueryObservations(ctx, paymentSvc.ID, "")
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, _ := st.TypedEdges(ctx, paymentSvc.ID)
	want := callgraph.TransitiveCallerCounts(callgraph.BuildBoth(nodes, typed))
	max, sawCaller := 0.0, false
	for _, r := range rows {
		if r.Callers != want[r.Symbol] {
			t.Errorf("%s: stored callers %d, condensation count %d", r.Symbol, r.Callers, want[r.Symbol])
		}
		if r.Centrality < 0 || r.Centrality > 1 {
			t.Errorf("%s: centrality %v outside [0,1]", r.Symbol, r.Centrality)
		}
		if r.Centrality > max {
			max = r.Centrality
		}
		if r.Callers > 0 {
			sawCaller = true
			if r.File == "" {
				t.Errorf("%s: no file recorded", r.Symbol)
			}
		}
	}
	if !sawCaller {
		t.Fatal("fixture service has no symbol with a caller; the test proves nothing")
	}
	if max != 1 {
		t.Errorf("max normalized centrality = %v, want 1", max)
	}

	// Memo: plant a sentinel at the same watermark; an unchanged
	// re-ingest must leave it alone.
	sentinel := []store.SymbolCentrality{{Symbol: "sentinel", File: "x.go", Callers: 7, Centrality: 1}}
	if err := st.SetSymbolCentrality(ctx, paymentSvc.ID, wm, sentinel); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if _, got, _ := st.SymbolCentrality(ctx, paymentSvc.ID); len(got) != 1 || got[0].Symbol != "sentinel" {
		t.Fatalf("unchanged re-ingest at the same watermark recomputed centrality (memo miss): %d rows", len(got))
	}

	// A different stored watermark forces the recompute.
	if err := st.SetSymbolCentrality(ctx, paymentSvc.ID, "older-watermark", sentinel); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("third Ingest: %v", err)
	}
	if _, got, _ := st.SymbolCentrality(ctx, paymentSvc.ID); len(got) != len(rows) {
		t.Fatalf("moved watermark: got %d rows, want the recomputed %d", len(got), len(rows))
	}
}
