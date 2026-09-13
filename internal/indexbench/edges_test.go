package indexbench

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// smallTruth is a hand-built go/types truth small enough to reason about by
// eye: Foo is called by Bar and Baz; Recv.Method is called by Bar.
func smallTruth() publicloc.GoTruth {
	return publicloc.GoTruth{
		Module: "example.com/small",
		CallersByName: map[string][]string{
			"Foo":         {"Bar", "Baz"},
			"Recv.Method": {"Bar"},
		},
		DeclsByFile: map[string][]string{
			"a.go": {"Foo", "Bar", "Baz", "Qux"},
			"b.go": {"Recv.Method"},
		},
		UsesByName: map[string][]string{
			"Bar": {"a.go", "b.go"},
			"Baz": {"a.go"},
		},
	}
}

// ambiguousTruth declares a bare name ("Foo") shared by a top-level function
// and a method — the same collision refsubset.go excludes references
// queries on (BP-30), extended here to call-edge targets.
func ambiguousTruth() publicloc.GoTruth {
	return publicloc.GoTruth{
		CallersByName: map[string][]string{
			"Foo":      {"A"},
			"Recv.Foo": {"B"},
		},
		DeclsByFile: map[string][]string{
			"a.go": {"Foo", "A", "Recv.Foo", "B"},
		},
		UsesByName: map[string][]string{},
	}
}

func TestCompareEdges_CallEdgesScoreAgainstCallersByName(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "Bar", To: "Foo", Kind: EdgeKindCall},
		{From: "Baz", To: "Foo", Kind: EdgeKindCall},
	}
	got := CompareEdges(theirs, truth)
	if got.Queries != 1 {
		t.Fatalf("Queries = %d, want 1 (one distinct callee asked about)", got.Queries)
	}
	if got.Precision != 1 || got.Recall != 1 {
		t.Fatalf("precision=%v recall=%v, want 1/1 — theirs exactly matches CallersByName[Foo]", got.Precision, got.Recall)
	}
	if got.Correct != 2 || got.Predicted != 2 || got.Truth != 2 {
		t.Fatalf("Correct=%d Predicted=%d Truth=%d, want 2/2/2", got.Correct, got.Predicted, got.Truth)
	}
}

func TestCompareEdges_MissedCallerHurtsRecallNotPrecision(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "Bar", To: "Foo", Kind: EdgeKindCall}, // misses Baz->Foo
	}
	got := CompareEdges(theirs, truth)
	if got.Precision != 1 {
		t.Errorf("precision = %v, want 1 (everything predicted was correct)", got.Precision)
	}
	if got.Recall != 0.5 {
		t.Errorf("recall = %v, want 0.5 (found 1 of 2 real callers)", got.Recall)
	}
}

func TestCompareEdges_WrongCallerHurtsPrecisionNotRecall(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "Bar", To: "Foo", Kind: EdgeKindCall},
		{From: "Baz", To: "Foo", Kind: EdgeKindCall},
		{From: "Qux", To: "Foo", Kind: EdgeKindCall}, // Qux is declared but does not call Foo in the truth
	}
	got := CompareEdges(theirs, truth)
	if got.Recall != 1 {
		t.Errorf("recall = %v, want 1 (both real callers were found)", got.Recall)
	}
	if got.Precision >= 1 {
		t.Errorf("precision = %v, want < 1 (Qux is a wrong caller)", got.Precision)
	}
}

func TestCompareEdges_UnqualifiedCalleeResolvesWhenUnambiguous(t *testing.T) {
	truth := smallTruth()
	// "Method" bare (no receiver) uniquely means Recv.Method here.
	theirs := []Edge{
		{From: "Bar", To: "Method", Kind: EdgeKindCall},
	}
	got := CompareEdges(theirs, truth)
	if got.Queries != 1 || got.Precision != 1 || got.Recall != 1 {
		t.Fatalf("bare unambiguous method name should resolve cleanly: got %+v", got)
	}
}

func TestCompareEdges_AmbiguousCalleeIsUnadjudicatedNeverWrong(t *testing.T) {
	truth := ambiguousTruth()
	theirs := []Edge{
		{From: "A", To: "Foo", Kind: EdgeKindCall}, // "Foo" is exact — not ambiguous
	}
	got := CompareEdges(theirs, truth)
	if got.Queries != 1 || got.Unadjudicated != 0 {
		t.Fatalf("an exact qualified-name match must not be treated as ambiguous: %+v", got)
	}
}

func TestCompareEdges_UnknownCalleeIsUnadjudicatedNeverWrong(t *testing.T) {
	truth := ambiguousTruth()
	theirs := []Edge{
		{From: "Caller", To: "NeverDeclared", Kind: EdgeKindCall},
	}
	got := CompareEdges(theirs, truth)
	if got.Queries != 0 || got.UnadjudicatedQueries != 1 || got.Unadjudicated != 1 {
		t.Fatalf("unknown target must be Unadjudicated, never a wrong answer: %+v", got)
	}
	if got.Precision != 1 || got.Recall != 1 {
		t.Fatalf("an unadjudicated-only run reads 1.0 over nothing asked, not a failure: %+v", got)
	}
}

func TestCompareEdges_ReferenceEdgesScoreAgainstUsesByName(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "a.go:5", To: "Bar", Kind: EdgeKindReference},
		{From: "b.go:9", To: "Bar", Kind: EdgeKindReference},
	}
	got := CompareEdges(theirs, truth)
	if got.Precision != 1 || got.Recall != 1 {
		t.Fatalf("reference edges exactly matching UsesByName[Bar]'s files should score 1/1: %+v", got)
	}
}

func TestCompareEdges_ReferenceInUnwalkedFileIsUnadjudicated(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "a.go:5", To: "Bar", Kind: EdgeKindReference},
		{From: "README.md:1", To: "Bar", Kind: EdgeKindReference}, // not a Go file go/types walked
	}
	got := CompareEdges(theirs, truth)
	if got.Unadjudicated != 1 {
		t.Fatalf("Unadjudicated = %d, want 1 (the README hit)", got.Unadjudicated)
	}
	if got.Precision != 1 {
		t.Errorf("precision = %v, want 1 — the unadjudicated hit must never lower it", got.Precision)
	}
}

func TestCompareEdges_UnknownRelationIsDroppedNeverScored(t *testing.T) {
	truth := smallTruth()
	theirs := []Edge{
		{From: "a.go", To: "Bar", Kind: "contains"},
	}
	got := CompareEdges(theirs, truth)
	if got.Queries != 0 || got.Predicted != 0 || got.Unadjudicated != 1 {
		t.Fatalf("an edge of an unscored kind must be counted Unadjudicated and never enter a rate: %+v", got)
	}
}

func TestCompareEdges_EmptyInputScoresOneNotZero(t *testing.T) {
	got := CompareEdges(nil, smallTruth())
	if got.Precision != 1 || got.Recall != 1 {
		t.Fatalf("an empty edge set over anything asked should read 1.0 with zero denominators, not a spurious failure: %+v", got)
	}
	if got.Predicted != 0 || got.Truth != 0 || got.Queries != 0 {
		t.Fatalf("zero denominators must be visible: %+v", got)
	}
}
