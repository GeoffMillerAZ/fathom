package review_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// githubFixtureNodes: pkg.A (changed, pkg/a.go) is called by
// pkg.ResolvedCaller (unchanged, via a typed EdgeDirect edge),
// pkg.HeuristicCaller (unchanged, syntax-only — no typed edge), and
// pkg.SameFileCaller (changed, same file as A — exercises call-site
// completeness's "updated in this diff" count). pkg.TestA (unchanged)
// reaches A so the affected-tests branch has a real row too.
// FilePath is deliberately root-relative (no directory component): a
// directory named the same as the package (e.g. "pkg/a.go" with Package
// "pkg") would make core.SymbolNode.Key qualify the key as
// "svc::pkg/pkg.A" (core.QualifiedPackage) — a real, deliberate PC-32
// behavior this fixture must not fight. Root-relative paths keep the keys
// exactly "svc::pkg.<Name>", matching the typed-edge literals below.
func githubFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "a.go", Line: 10},
		{Service: "svc", Package: "pkg", Name: "ResolvedCaller", Kind: core.KindGoFunc, FilePath: "resolved.go", Line: 3, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "HeuristicCaller", Kind: core.KindGoFunc, FilePath: "heuristic.go", Line: 4, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "SameFileCaller", Kind: core.KindGoFunc, FilePath: "a.go", Line: 20, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "TestA", Kind: core.KindGoFunc, FilePath: "a_test.go", Line: 1, Calls: []string{"svc::pkg.A"}},
	}
}

func githubFixtureTypedEdges() []core.Edge {
	return []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.ResolvedCaller", ToSymbol: "svc::pkg.A", Kind: core.EdgeDirect},
	}
}

// TestChangedSymbolReviews_CallerTiersAndCompleteness pins the caller-tier
// classification (core.EdgeConfidenceOf is the ONE accessor — resolved for
// a typed edge, heuristic for a syntax-only one or no typed edge at all)
// and the call-site-completeness counts (N total, M updated in this diff).
func TestChangedSymbolReviews_CallerTiersAndCompleteness(t *testing.T) {
	nodes := githubFixtureNodes()
	typed := githubFixtureTypedEdges()
	got := review.ChangedSymbolReviews(nodes, typed, []string{"a.go"})

	var a, sameFileCaller *review.ChangedSymbolReview
	for i := range got {
		switch got[i].Symbol {
		case "svc::pkg.A":
			a = &got[i]
		case "svc::pkg.SameFileCaller":
			sameFileCaller = &got[i]
		}
	}
	if a == nil {
		t.Fatalf("svc::pkg.A missing from %+v", got)
	}
	if a.CallSitesTotal != 4 {
		t.Errorf("A.CallSitesTotal = %d, want 4 (ResolvedCaller, HeuristicCaller, SameFileCaller, TestA)", a.CallSitesTotal)
	}
	if a.CallSitesUpdated != 1 {
		t.Errorf("A.CallSitesUpdated = %d, want 1 (only SameFileCaller shares A's changed file)", a.CallSitesUpdated)
	}
	tierOf := make(map[string]core.EdgeConfidence, len(a.Callers))
	for _, c := range a.Callers {
		tierOf[c.Symbol] = c.Tier
	}
	if tierOf["svc::pkg.ResolvedCaller"] != core.EdgeConfidenceResolved {
		t.Errorf("ResolvedCaller tier = %q, want resolved", tierOf["svc::pkg.ResolvedCaller"])
	}
	if tierOf["svc::pkg.HeuristicCaller"] != core.EdgeConfidenceHeuristic {
		t.Errorf("HeuristicCaller tier = %q, want heuristic", tierOf["svc::pkg.HeuristicCaller"])
	}
	if tierOf["svc::pkg.SameFileCaller"] != core.EdgeConfidenceHeuristic {
		t.Errorf("SameFileCaller tier = %q, want heuristic (syntax-only, no typed edge)", tierOf["svc::pkg.SameFileCaller"])
	}

	if sameFileCaller == nil {
		t.Fatalf("svc::pkg.SameFileCaller missing from %+v", got)
	}
	if sameFileCaller.CallSitesTotal != 0 {
		t.Errorf("SameFileCaller.CallSitesTotal = %d, want 0 (nothing calls it)", sameFileCaller.CallSitesTotal)
	}
}

// TestChangedSymbolReviews_TransitiveCallerDepth2 pins VB-24 recall-loop
// mechanism (a) (docs/bench/readings/review-recall-loop-2026-09-18.md): a
// caller of a caller (GrandCaller -> ResolvedCaller -> A, GrandCaller has
// NO direct edge to A) now appears in A's Callers at Depth 2, labeled with
// its own tier, WITHOUT inflating CallSitesTotal/CallSitesUpdated (those
// stay direct-callers-only — "how many places call this AT ALL"). A
// SelfLoop-shaped diamond (both GrandCaller and DeepCaller reach A only
// through ResolvedCaller) also checks dedup: ResolvedCaller must not
// appear twice.
func TestChangedSymbolReviews_TransitiveCallerDepth2(t *testing.T) {
	nodes := append(
		githubFixtureNodes(),
		core.SymbolNode{Service: "svc", Package: "pkg", Name: "GrandCaller", Kind: core.KindGoFunc, FilePath: "grand.go", Line: 5, Calls: []string{"svc::pkg.ResolvedCaller"}},
		core.SymbolNode{Service: "svc", Package: "pkg", Name: "GreatGrandCaller", Kind: core.KindGoFunc, FilePath: "greatgrand.go", Line: 6, Calls: []string{"svc::pkg.GrandCaller"}},
	)
	typed := githubFixtureTypedEdges()
	got := review.ChangedSymbolReviews(nodes, typed, []string{"a.go"})

	var a *review.ChangedSymbolReview
	for i := range got {
		if got[i].Symbol == "svc::pkg.A" {
			a = &got[i]
		}
	}
	if a == nil {
		t.Fatalf("svc::pkg.A missing from %+v", got)
	}
	if a.CallSitesTotal != 4 {
		t.Errorf("A.CallSitesTotal = %d, want 4 (depth-2/3 callers must not inflate direct call-site counts)", a.CallSitesTotal)
	}

	depthOf := map[string]int{}
	for _, c := range a.Callers {
		if _, dup := depthOf[c.Symbol]; dup {
			t.Errorf("caller %s listed more than once: %+v", c.Symbol, a.Callers)
		}
		depthOf[c.Symbol] = c.Depth
	}
	if depthOf["svc::pkg.ResolvedCaller"] != 1 {
		t.Errorf("ResolvedCaller depth = %d, want 1", depthOf["svc::pkg.ResolvedCaller"])
	}
	if depthOf["svc::pkg.GrandCaller"] != 2 {
		t.Errorf("GrandCaller depth = %d, want 2 (caller of a direct caller)", depthOf["svc::pkg.GrandCaller"])
	}
	if _, present := depthOf["svc::pkg.GreatGrandCaller"]; present {
		t.Errorf("GreatGrandCaller (depth 3) must NOT appear — changedSymbolCallerDepth caps the walk at 2: %+v", a.Callers)
	}

	// Depth-ascending order: every depth-1 caller sorts before every
	// depth-2 caller.
	sawDepth2 := false
	for _, c := range a.Callers {
		if c.Depth == 2 {
			sawDepth2 = true
		} else if sawDepth2 {
			t.Errorf("caller %+v at depth %d sorted after a depth-2 caller", c, c.Depth)
		}
	}
}

// githubReviewFixtureInput builds a review.Input over githubFixtureNodes
// with one rules-layer obligation and one opt-in claims-layer binding on
// A's file, so the render's obligations filter (rules only) has something
// real to exclude.
func githubReviewFixtureInput() review.Input {
	nodes := githubFixtureNodes()
	typed := githubFixtureTypedEdges()
	return review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"a.go"},
		Nodes:                nodes,
		TypedEdges:           typed,
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"a.go": {
				{File: "a.go", Rule: "exported functions need a doc comment", Src: "CLAUDE.md#L1-L1", Kind: ""},
				{File: "a.go", Rule: "plan-row VB-1 binds here", Src: "docs/plans/verbs.md#L11-L11", Kind: "finding"},
			},
		},
	}
}

func TestRenderGitHubReview_ComposesTheFixedSections(t *testing.T) {
	in := githubReviewFixtureInput()
	v := review.Run(in)
	gap := impact.TestGap(in.Nodes, in.ChangedFiles, in.TypedEdges)

	ghIn := review.GitHubReviewInput{
		Verdict:        v,
		TestGap:        gap,
		IndexCommit:    "deadbeef",
		DiffRange:      "origin/main..HEAD",
		ChangedSymbols: review.ChangedSymbolReviews(in.Nodes, in.TypedEdges, in.ChangedFiles),
		TouchedPlanRows: []review.TouchedPlanRow{
			{Plan: "docs/plans/verbs.md", RowID: "VB-22", Symbol: "A"},
		},
	}
	out := review.RenderGitHubReview(ghIn)

	if !strings.HasPrefix(out, review.GitHubReviewMarker) {
		t.Fatalf("output does not open with the marker line:\n%s", out)
	}
	if !strings.Contains(out, "Index commit `deadbeef`") || !strings.Contains(out, "diff range `origin/main..HEAD`") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "`svc::pkg.A`") {
		t.Errorf("missing changed symbol A: %s", out)
	}
	if !strings.Contains(out, "called from 4 place(s), 1 updated in this diff") {
		t.Errorf("missing call-site completeness line: %s", out)
	}
	if !strings.Contains(out, "[resolved, depth 1]") || !strings.Contains(out, "[heuristic, depth 1]") {
		t.Errorf("missing tier+depth labels: %s", out)
	}
	if !strings.Contains(out, "exported functions need a doc comment") {
		t.Errorf("missing rules-layer governing rule: %s", out)
	}
	if strings.Contains(out, "plan-row VB-1 binds here") {
		t.Errorf("claims-layer (opt-in) binding leaked into governing rules: %s", out)
	}
	if !strings.Contains(out, "plan row `VB-22` in `docs/plans/verbs.md` anchors `A`") {
		t.Errorf("missing verify line: %s", out)
	}
	if !strings.Contains(out, "run `fathom verify --service svc --plan docs/plans/verbs.md`") {
		t.Errorf("missing verify command: %s", out)
	}
	if !strings.Contains(out, "`svc::pkg.TestA`") || !strings.Contains(out, "reaches") {
		t.Errorf("missing affected test: %s", out)
	}
	if strings.Contains(out, "### Coverage") {
		t.Errorf("coverage section rendered with no profile supplied: %s", out)
	}
	if strings.Contains(out, "unknown") {
		t.Errorf("a resolved index commit/diff range fell back to 'unknown': %s", out)
	}

	// Determinism (internal/AGENTS.md): identical input renders byte-identical
	// Markdown, and every finding line's claim id is stable across renders.
	again := review.RenderGitHubReview(ghIn)
	if out != again {
		t.Fatalf("renderGitHubReview is not deterministic:\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

func TestRenderGitHubReview_CoverageBlockOnlyWhenSupplied(t *testing.T) {
	in := githubReviewFixtureInput()
	in.CoverageChecked = true
	in.CoverageLayered = []coverage.SymbolLayerCoverage{} // checked, but no profile data joined for any changed symbol

	v := review.Run(in)
	out := review.RenderGitHubReview(review.GitHubReviewInput{Verdict: v})
	if !strings.Contains(out, "### Coverage") {
		t.Errorf("coverage section missing when CoverageChecked:true: %s", out)
	}
	if !strings.Contains(out, "no changed symbol has profile data") {
		t.Errorf("expected the honest empty-coverage line: %s", out)
	}

	in2 := githubReviewFixtureInput()
	v2 := review.Run(in2)
	out2 := review.RenderGitHubReview(review.GitHubReviewInput{Verdict: v2})
	if strings.Contains(out2, "### Coverage") {
		t.Errorf("coverage section rendered with CoverageChecked:false: %s", out2)
	}
}

func TestRenderGitHubReview_EmptyInputStaysHonest(t *testing.T) {
	out := review.RenderGitHubReview(review.GitHubReviewInput{})
	if !strings.HasPrefix(out, review.GitHubReviewMarker) {
		t.Fatalf("empty input must still open with the marker: %s", out)
	}
	for _, want := range []string{
		"Index commit `unknown`",
		"diff range `unknown`",
		"_no indexed symbols in the changed files._",
		"_none._",
		"_no plan row anchors a symbol in this diff._",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty-input render missing %q:\n%s", want, out)
		}
	}
}
