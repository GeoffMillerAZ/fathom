package verify_test

// reachguard_test.go — RF-5 (ADR-067 D3, I57): Evidence.Reachable and
// ProductEvidence.ProductReachedBy answer over typed edges only; an
// Evidence that can also name the heuristic edge a refused reach rested on
// (verify.HeuristicReachEvidence) gets that edge and its tier into the
// criterion's detail, so the refusal says why.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

type heuristicEvidence struct {
	productEvidence
	edge  map[string]verify.HeuristicEdge // "from->to" -> the heuristic edge
	pedge map[string]verify.HeuristicEdge // symbol -> the heuristic edge a product entry reaches it through
	entry string
}

func (h heuristicEvidence) HeuristicReach(from, to string) (verify.HeuristicEdge, bool) {
	e, ok := h.edge[from+"->"+to]
	return e, ok
}

func (h heuristicEvidence) ProductHeuristicReach(symbol string) (string, verify.HeuristicEdge, bool) {
	e, ok := h.pedge[symbol]
	return h.entry, e, ok
}

func TestReachGuard_RefusalNamesTheHeuristicEdgeAndTier(t *testing.T) {
	base := wiredFixture()
	ev := heuristicEvidence{
		productEvidence: base,
		edge: map[string]verify.HeuristicEdge{
			"TestLib->TestOnly": {From: "svc::lib/lib.TestLib", To: "svc::lib/lib.TestOnly", Tier: "heuristic"},
		},
		pedge: map[string]verify.HeuristicEdge{
			"TestOnly": {From: "svc::cmd/app/main.main", To: "svc::lib/lib.TestOnly", Kind: "file-resolved", Tier: "heuristic"},
		},
		entry: "svc::cmd/app/main.main",
	}
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "G-1", Claimed: true, Expect: verify.Expect{Symbol: "TestOnly", ReachableFrom: "TestLib"}},
	}}
	v := verdict(t, verify.VerifyPlan(plan, ev), "G-1")
	if v.Status == verify.Pass {
		t.Fatalf("G-1 passed on heuristic reach: %+v", v)
	}
	ew, _ := findCrit(v, "edge_wired")
	if ew.Status != verify.Fail || !strings.Contains(ew.Detail, "svc::lib/lib.TestLib -> svc::lib/lib.TestOnly") || !strings.Contains(ew.Detail, "tier heuristic") {
		t.Errorf("edge_wired = %+v, want a fail naming the edge TestLib -> TestOnly and its tier", ew)
	}
	w, _ := findCrit(v, "wired")
	if w.Status != verify.Unknown || !strings.Contains(w.Detail, "svc::cmd/app/main.main -> svc::lib/lib.TestOnly") ||
		!strings.Contains(w.Detail, "tier heuristic") || !strings.Contains(w.Detail, "file-resolved") {
		t.Errorf("wired = %+v, want unknown naming the edge main -> TestOnly, its kind and its tier", w)
	}

	// An evidence with no heuristic capability keeps the plain refusal.
	v = verdict(t, verify.VerifyPlan(plan, base), "G-1")
	if ew, _ := findCrit(v, "edge_wired"); ew.Status != verify.Fail || strings.Contains(ew.Detail, "heuristic") {
		t.Errorf("plain evidence edge_wired = %+v, want the plain does-not-reach fail", ew)
	}
}

// TestReachGuard_WiredJudgesTheFunctionNotASameNamedType: a row whose
// symbol names a type and one method (`symbol: Viz` finding the type Viz
// and the method Corpus.Viz) is wired when the method is: the type's own
// "reach" was only ever a reference to it, an edge of tier heuristic.
func TestReachGuard_WiredJudgesTheFunctionNotASameNamedType(t *testing.T) {
	typ := core.SymbolNode{Service: "svc", Package: "corpus", Name: "Viz", FilePath: "corpus/viz.go", Line: 3, Tier: core.TierVerified, Kind: core.KindGoType}
	method := core.SymbolNode{Service: "svc", Package: "Corpus", Name: "Corpus.Viz", FilePath: "corpus/viz.go", Line: 9, Tier: core.TierVerified, Kind: core.KindGoMethod}
	ev := productEvidence{
		fakeEvidence: fakeEvidence{
			symbols: map[string][]core.SymbolNode{"Viz": {typ, method}},
			tests:   map[string][]string{"Viz": {"svc::corpus/corpus.TestViz"}},
		},
		entries: 1,
		reached: map[string]string{method.Key(): "svc::cmd/app/main.main"},
	}
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "V-1", Claimed: true, Expect: verify.Expect{Symbol: "Viz", Package: "corpus"}},
	}}
	v := verdict(t, verify.VerifyPlan(plan, ev), "V-1")
	w, _ := findCrit(v, "wired")
	if w.Status != verify.Pass || !strings.Contains(w.Detail, method.Key()) {
		t.Errorf("wired = %+v, want pass naming the method %s", w, method.Key())
	}
}
