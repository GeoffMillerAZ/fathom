package contextbench

import (
	"reflect"
	"sort"
	"testing"
)

func TestScore_ExactMatch(t *testing.T) {
	task := Task{ID: "T1", GroundTruth: []string{"cmd/fathom/servehttp.go"}, ScoringRule: ExactMatch}

	r := Score(task, "The call logger is constructed in cmd/fathom/servehttp.go, line 105.")
	if !r.Pass {
		t.Fatalf("expected pass, got fail: %+v", r)
	}
	if got := r.Matched; !reflect.DeepEqual(got, []string{"cmd/fathom/servehttp.go"}) {
		t.Errorf("matched = %v", got)
	}
	if len(r.Missing) != 0 {
		t.Errorf("missing = %v, want none", r.Missing)
	}

	r2 := Score(task, "It's constructed in cmd/fathom/main.go.")
	if r2.Pass {
		t.Fatalf("expected fail, got pass: %+v", r2)
	}
	if got := r2.Missing; !reflect.DeepEqual(got, []string{"cmd/fathom/servehttp.go"}) {
		t.Errorf("missing = %v", got)
	}
}

func TestScore_AllOf(t *testing.T) {
	task := Task{
		ID:          "T2",
		GroundTruth: []string{"cmd/fathom/verify.go", "cmd/fathom/main.go"},
		ScoringRule: AllOf,
	}

	pass := Score(task, "cmdVerify lives in cmd/fathom/verify.go; main.go's switch in cmd/fathom/main.go dispatches to it.")
	if !pass.Pass {
		t.Fatalf("expected pass, got fail: %+v", pass)
	}

	partial := Score(task, "cmdVerify lives in cmd/fathom/verify.go.")
	if partial.Pass {
		t.Fatalf("expected fail (missing one ground-truth item), got pass: %+v", partial)
	}
	if got, want := partial.Matched, []string{"cmd/fathom/verify.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("matched = %v, want %v", got, want)
	}
	if got, want := partial.Missing, []string{"cmd/fathom/main.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
	}
}

func TestScore_AnyOf(t *testing.T) {
	task := Task{
		ID:          "T3",
		GroundTruth: []string{"cmd/fathom/verify.go", "internal/mcp/verify_plan_tool.go"},
		ScoringRule: AnyOf,
	}

	pass := Score(task, "One caller is internal/mcp/verify_plan_tool.go.")
	if !pass.Pass {
		t.Fatalf("expected pass, got fail: %+v", pass)
	}

	fail := Score(task, "One caller is cmd/fathom/main.go.")
	if fail.Pass {
		t.Fatalf("expected fail, got pass: %+v", fail)
	}
}

func TestScore_CaseSensitiveSubstring(t *testing.T) {
	// File paths and symbol names are case-sensitive tokens; a scorer that
	// lowercased everything could produce false positives across
	// differently-cased symbols in the same codebase.
	task := Task{ID: "T4", GroundTruth: []string{"ParseMarkdownPlan"}, ScoringRule: ExactMatch}
	r := Score(task, "the function is parsemarkdownplan, in lowercase")
	if r.Pass {
		t.Fatalf("expected case-sensitive match to fail, got pass: %+v", r)
	}
}

func TestScore_EmptyOutputAlwaysFails(t *testing.T) {
	task := Task{ID: "T5", GroundTruth: []string{"a"}, ScoringRule: AnyOf}
	r := Score(task, "")
	if r.Pass {
		t.Fatal("empty output must never pass")
	}
}

func TestScore_MatchedMissingAreSortedAndDeterministic(t *testing.T) {
	task := Task{
		ID:          "T6",
		GroundTruth: []string{"zeta", "alpha", "mu"},
		ScoringRule: AllOf,
	}
	r := Score(task, "zeta and alpha and mu all appear here")
	wantSorted := append([]string(nil), r.Matched...)
	sort.Strings(wantSorted)
	if !reflect.DeepEqual(r.Matched, wantSorted) {
		t.Errorf("Matched not sorted: %v", r.Matched)
	}
}
