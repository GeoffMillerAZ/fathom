package semantic

// rejectioncause_test.go — Q-1 (docs/plans/expert-suite-q.md, O19's fix):
// classifyRejectionCause must recover the exact cause each verifyDraft
// rejection site records, and default an unclassified error to
// causeMalformed rather than dropping it silently.

import (
	"errors"
	"testing"
)

func TestClassifyRejectionCause_EmptyRule_Malformed(t *testing.T) {
	err := rejectErr(causeMalformed, "empty rule")
	if got := classifyRejectionCause(err); got != causeMalformed {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeMalformed)
	}
}

func TestClassifyRejectionCause_WordBudget_Malformed(t *testing.T) {
	err := rejectErr(causeMalformed, "rule exceeds %d-word budget (%d)", 28, 40)
	if got := classifyRejectionCause(err); got != causeMalformed {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeMalformed)
	}
}

func TestClassifyRejectionCause_CitationOutsideRange_AnchorMissing(t *testing.T) {
	err := rejectErr(causeAnchorMissing, "citation outside section range")
	if got := classifyRejectionCause(err); got != causeAnchorMissing {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeAnchorMissing)
	}
}

func TestClassifyRejectionCause_CitationDoesNotResolve_AnchorMissing(t *testing.T) {
	// Both HashLines and LinesText failure sites use the identical message
	// and cause — one representative check covers both call sites' shape.
	err := rejectErr(causeAnchorMissing, "citation range does not resolve")
	if got := classifyRejectionCause(err); got != causeAnchorMissing {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeAnchorMissing)
	}
}

func TestClassifyRejectionCause_EmptyScope_Malformed(t *testing.T) {
	err := rejectErr(causeMalformed, "empty scope")
	if got := classifyRejectionCause(err); got != causeMalformed {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeMalformed)
	}
}

func TestClassifyRejectionCause_ScopeMatchesNoFile_ScopeUnverifiable(t *testing.T) {
	err := rejectErr(causeScopeUnverifiable, "scope %q matches no indexed file", "nonexistent/**")
	if got := classifyRejectionCause(err); got != causeScopeUnverifiable {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeScopeUnverifiable)
	}
}

func TestClassifyRejectionCause_ScopeOverBroad_OverBroad(t *testing.T) {
	err := rejectErr(causeOverBroad, "scope %q is over-broad: matches %d/%d indexed files", "**", 3, 3)
	if got := classifyRejectionCause(err); got != causeOverBroad {
		t.Errorf("classifyRejectionCause = %q, want %q", got, causeOverBroad)
	}
}

func TestClassifyRejectionCause_UnclassifiedError_DefaultsMalformed(t *testing.T) {
	err := errors.New("some other error never built via rejectErr")
	if got := classifyRejectionCause(err); got != causeMalformed {
		t.Errorf("classifyRejectionCause(unclassified) = %q, want default %q", got, causeMalformed)
	}
}

func TestBuildRejectionReasons_DeterministicOrder(t *testing.T) {
	counts := map[rejectionCause]int{
		causeOverBroad:         2,
		causeMalformed:         5,
		causeAnchorMissing:     1,
		causeScopeUnverifiable: 3,
	}
	samples := map[rejectionCause][]string{
		causeMalformed: {"a: empty rule"},
	}
	got := buildRejectionReasons(counts, samples)
	if len(got) != 4 {
		t.Fatalf("buildRejectionReasons returned %d rows, want 4", len(got))
	}
	wantOrder := []string{"anchor-missing", "malformed", "over-broad", "scope-unverifiable"}
	for i, w := range wantOrder {
		if got[i].Cause != w {
			t.Errorf("row %d cause = %q, want %q (alphabetical order)", i, got[i].Cause, w)
		}
	}
	for _, row := range got {
		if row.Cause == "malformed" && row.Count != 5 {
			t.Errorf("malformed count = %d, want 5", row.Count)
		}
	}
}

// TestBuildRejectionReasons_TrustsCallerForSampleCap documents where the
// maxRejectionSamplesPerCause cap actually lives: the Compile loop's own
// accumulation site (compile.go), not this pure reshaping function — a
// samples slice handed in is passed straight through unchanged.
// TestCompile_RejectionReasons_SamplesCapped (compile_test.go) is the
// integration-level proof that a run with many same-cause rejections ends
// up capped in the final CompileRun.
func TestBuildRejectionReasons_TrustsCallerForSampleCap(t *testing.T) {
	counts := map[rejectionCause]int{causeMalformed: 10}
	var many []string
	for i := 0; i < 10; i++ {
		many = append(many, "sample")
	}
	samples := map[rejectionCause][]string{causeMalformed: many}
	got := buildRejectionReasons(counts, samples)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if len(got[0].Samples) != len(many) {
		t.Errorf("buildRejectionReasons must not itself re-cap; got %d samples", len(got[0].Samples))
	}
}

func TestBuildRejectionReasons_EmptyInput_Nil(t *testing.T) {
	if got := buildRejectionReasons(nil, nil); got != nil {
		t.Errorf("buildRejectionReasons(nil, nil) = %+v, want nil", got)
	}
}
