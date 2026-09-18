package reviewbench

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// TestScorePair_FourOutcomes pins the strict re-score rule
// (docs/bench/readings/vb24-review-historical-prs.md "Strict re-score"): a
// gold (file, func) inside A's own changed files is HitOwnSymbol (trivial,
// no call-graph inference); a gold entry OUTSIDE A's changed files that
// appears as a caller (any depth) is HitCaller; one that appears as an
// affected test is HitAffectedTest; anything else is HitMiss.
func TestScorePair_FourOutcomes(t *testing.T) {
	pair := Pair{Repo: "example/repo"}
	pair.PRB.FilesChanged = []PRFile{
		{Filename: "a.go", Patch: strp("@@ -1,1 +1,1 @@ func Changed() {\n")},              // own-symbol
		{Filename: "caller.go", Patch: strp("@@ -1,1 +1,1 @@ func OutsideCaller() {\n")},   // caller (depth 1)
		{Filename: "deep.go", Patch: strp("@@ -1,1 +1,1 @@ func DeepCaller() {\n")},        // caller (depth 2)
		{Filename: "a_test.go", Patch: strp("@@ -1,1 +1,1 @@ func TestChanged() {\n")},     // affected test
		{Filename: "unrelated.go", Patch: strp("@@ -1,1 +1,1 @@ func NeverAppears() {\n")}, // miss
	}
	changedFiles := []string{"a.go"}

	changedSymbols := []review.ChangedSymbolReview{
		{
			Symbol: "svc::pkg.Changed",
			File:   "a.go",
			Callers: []review.CallerRef{
				{Symbol: "svc::pkg.OutsideCaller", File: "caller.go", Depth: 1, Tier: "heuristic"},
				{Symbol: "svc::pkg.DeepCaller", File: "deep.go", Depth: 2, Tier: "resolved"},
			},
		},
	}
	testGap := impact.TestGapResult{
		Checked: true,
		AffectedTests: []impact.TestReach{
			{Test: "svc::pkg.TestChanged", File: "a_test.go", Reaches: []string{"svc::pkg.Changed"}},
		},
	}

	got := ScorePair(pair, changedFiles, changedSymbols, testGap)
	kindOf := make(map[string]HitKind, len(got.Matches))
	depthOf := make(map[string]int, len(got.Matches))
	for _, m := range got.Matches {
		kindOf[m.Gold.File+":"+m.Gold.Func] = m.Kind
		depthOf[m.Gold.File+":"+m.Gold.Func] = m.Depth
	}

	cases := []struct {
		key  string
		kind HitKind
	}{
		{"a.go:Changed", HitOwnSymbol},
		{"caller.go:OutsideCaller", HitCaller},
		{"deep.go:DeepCaller", HitCaller},
		{"a_test.go:TestChanged", HitAffectedTest},
		{"unrelated.go:NeverAppears", HitMiss},
	}
	for _, c := range cases {
		if kindOf[c.key] != c.kind {
			t.Errorf("%s: kind = %q, want %q", c.key, kindOf[c.key], c.kind)
		}
	}
	if depthOf["deep.go:DeepCaller"] != 2 {
		t.Errorf("deep.go:DeepCaller depth = %d, want 2", depthOf["deep.go:DeepCaller"])
	}
	if !got.StrictNewHit() {
		t.Errorf("StrictNewHit() = false, want true (OutsideCaller/DeepCaller/TestChanged all qualify)")
	}
}

// TestScorePair_OwnSymbolAloneIsNotAStrictNewHit pins the row's actual bar:
// a pair whose ONLY match is an own-symbol (file-level) recitation must NOT
// count as a strict-new hit — this is exactly the trivial-match class the
// 2026-09-17 re-score subtracted out (32/37 of the original pass's "hits").
func TestScorePair_OwnSymbolAloneIsNotAStrictNewHit(t *testing.T) {
	pair := Pair{Repo: "example/repo"}
	pair.PRB.FilesChanged = []PRFile{
		{Filename: "a.go", Patch: strp("@@ -1,1 +1,1 @@ func Changed() {\n")},
	}
	changedSymbols := []review.ChangedSymbolReview{{Symbol: "svc::pkg.Changed", File: "a.go"}}

	got := ScorePair(pair, []string{"a.go"}, changedSymbols, impact.TestGapResult{Checked: true})
	if got.StrictNewHit() {
		t.Errorf("StrictNewHit() = true, want false (own-symbol match alone is trivial, not index inference)")
	}
	if !got.AnyHit() {
		t.Errorf("AnyHit() = false, want true (the original, superseded scoring rule still counts this)")
	}
}

// TestScorePair_CallerInsideChangedFilesDoesNotCountAsCaller pins the
// "outside A's own diff" boundary: a caller-shaped match whose FILE is
// itself one of A's changed files must classify as HitOwnSymbol context
// (never HitCaller) — HitCaller requires evidence the index inferred
// something beyond what A's own diff already told a reviewer.
func TestScorePair_CallerInsideChangedFilesDoesNotCountAsCaller(t *testing.T) {
	pair := Pair{Repo: "example/repo"}
	pair.PRB.FilesChanged = []PRFile{
		{Filename: "a.go", Patch: strp("@@ -1,1 +1,1 @@ func SameFileCaller() {\n")},
	}
	changedSymbols := []review.ChangedSymbolReview{
		{
			Symbol: "svc::pkg.Changed",
			File:   "a.go",
			Callers: []review.CallerRef{
				{Symbol: "svc::pkg.SameFileCaller", File: "a.go", Depth: 1, Tier: "heuristic"},
			},
		},
		{Symbol: "svc::pkg.SameFileCaller", File: "a.go"},
	}
	got := ScorePair(pair, []string{"a.go"}, changedSymbols, impact.TestGapResult{Checked: true})
	if got.StrictNewHit() {
		t.Errorf("StrictNewHit() = true, want false: SameFileCaller lives in a.go, one of A's own changed files")
	}
	if len(got.Matches) != 1 || got.Matches[0].Kind != HitOwnSymbol {
		t.Errorf("Matches = %+v, want exactly one HitOwnSymbol", got.Matches)
	}
}
