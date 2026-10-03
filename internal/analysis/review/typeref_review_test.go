package review_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestChangedSymbolReviewsKeepTypeConsumers pins wave lg's D (LB-F200) on
// the review engine: a syntax-only edge to a Go type is a reference that
// callgraph.CallersDetail leaves out, and the review still lists the
// type's consumers (DependentsDetail), each tiered heuristic, so a changed
// type never reads as having no callers.
func TestChangedSymbolReviewsKeepTypeConsumers(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "svc", Name: "Config", Kind: core.KindGoType, FilePath: "svc/store.go", Line: 3},
		{Service: "svc", Package: "svc", Name: "NewConfig", Kind: core.KindGoFunc, FilePath: "svc/store.go", Line: 9},
		{Service: "svc", Package: "app", Name: "Build", Kind: core.KindGoFunc, FilePath: "app/app.go", Line: 5, Calls: []string{"svc.Config", "svc.NewConfig"}},
	}
	cfg, build, newCfg := nodes[0].Key(), nodes[2].Key(), nodes[1].Key()
	typed := []core.Edge{{FromService: "svc", FromFile: "app/app.go", FromSymbol: build, ToSymbol: newCfg, Kind: core.EdgeDirect}}

	var got *review.ChangedSymbolReview
	revs := review.ChangedSymbolReviews(nodes, typed, []string{"svc/store.go"})
	for i := range revs {
		if revs[i].Symbol == cfg {
			got = &revs[i]
		}
	}
	if got == nil {
		t.Fatalf("no review for %s in %+v", cfg, revs)
	}
	if got.CallSitesTotal != 1 || len(got.Callers) != 1 || got.Callers[0].Symbol != build {
		t.Fatalf("Config's review = %+v, want Build as its one dependent (a reference to the type)", got)
	}
	if got.Callers[0].Tier != core.EdgeConfidenceHeuristic {
		t.Errorf("Build's tier = %q, want heuristic: a type reference is syntax-only evidence", got.Callers[0].Tier)
	}
}
