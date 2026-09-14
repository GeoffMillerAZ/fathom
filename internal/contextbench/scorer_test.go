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

// TestScore_AnswerAnyOf_GroupSatisfiedByEitherSpelling pins VB-36/O47: an
// answer_any_of group is satisfied by ANY of its members, so the specific
// spelling an arm chose does not matter as long as it names one of them.
func TestScore_AnswerAnyOf_GroupSatisfiedByEitherSpelling(t *testing.T) {
	task := Task{
		ID:          "T7",
		GroundTruth: []string{"ADR-021"},
		AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}},
		ScoringRule: AllOf,
	}

	canonical := Score(task, "Per ADR-021, typed edges must be carried on core.Edge.")
	if !canonical.Pass {
		t.Fatalf("expected pass on the canonical spelling, got fail: %+v", canonical)
	}
	if got, want := canonical.Matched, []string{"ADR-021", "core.Edge"}; !reflect.DeepEqual(got, want) {
		t.Errorf("matched = %v, want %v", got, want)
	}

	variant := Score(task, "Per ADR-021, typed edges must be carried on the Edge struct.")
	if !variant.Pass {
		t.Fatalf("expected pass on the alternate spelling, got fail: %+v", variant)
	}
	if got, want := variant.Matched, []string{"ADR-021", "Edge struct"}; !reflect.DeepEqual(got, want) {
		t.Errorf("matched = %v, want %v", got, want)
	}
}

// TestScore_AnswerAnyOf_UnsatisfiedByNeitherSpelling pins the failure side:
// a group with no member present is missing, named by its full alternatives
// so a reader can see what was demanded.
func TestScore_AnswerAnyOf_UnsatisfiedByNeitherSpelling(t *testing.T) {
	task := Task{
		ID:          "T8",
		GroundTruth: []string{"ADR-021"},
		AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}},
		ScoringRule: AllOf,
	}
	r := Score(task, "Per ADR-021, typed edges must be carried on a Node.")
	if r.Pass {
		t.Fatalf("expected fail (neither spelling present), got pass: %+v", r)
	}
	if got, want := r.Missing, []string{"core.Edge | Edge struct"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
	}
}

// TestScore_AnswerAnyOf_MixedWithRequiredTokens pins the combination O47
// actually needs: a plain GroundTruth token stays required exactly as
// before, alongside a spelling-tolerant group.
func TestScore_AnswerAnyOf_MixedWithRequiredTokens(t *testing.T) {
	task := Task{
		ID:          "T9",
		GroundTruth: []string{"ADR-021", "internal/core"},
		AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}},
		ScoringRule: AllOf,
	}

	pass := Score(task, "ADR-021 governs internal/core; typed edges live on the Edge struct.")
	if !pass.Pass {
		t.Fatalf("expected pass (required tokens present, group satisfied by alt spelling), got fail: %+v", pass)
	}

	missingRequired := Score(task, "ADR-021 says typed edges live on the Edge struct.")
	if missingRequired.Pass {
		t.Fatalf("expected fail (required token internal/core missing even though the group is satisfied), got pass: %+v", missingRequired)
	}
	if got, want := missingRequired.Missing, []string{"internal/core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
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
