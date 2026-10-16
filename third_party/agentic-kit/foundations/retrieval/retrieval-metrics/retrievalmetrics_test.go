package retrievalmetrics_test

import (
	"testing"

	evidencesignature "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-signature"
	retrievalmetrics "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/retrieval/retrieval-metrics"
)

// ---- Precision tests -------------------------------------------------------

func TestPrecision_NotApplicableForAbsence(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeAbsence, []string{"a"}, []string{"a"})
	if r.Applicable {
		t.Errorf("Precision should not be applicable for absence archetype")
	}
	if r.Reason == "" {
		t.Errorf("Reason must be non-empty when Applicable == false")
	}
}

func TestPrecision_NotApplicableForEmptyFound(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeExistence, []string{"a"}, nil)
	if r.Applicable {
		t.Errorf("Precision should not be applicable when found is empty")
	}
}

func TestPrecision_PerfectPrecision(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeExistence, []string{"a", "b"}, []string{"a", "b"})
	if !r.Applicable {
		t.Fatalf("expected applicable, reason=%q", r.Reason)
	}
	if r.Value != 1.0 {
		t.Errorf("Precision = %g; want 1.0", r.Value)
	}
}

func TestPrecision_ZeroPrecision(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeExistence, []string{"x"}, []string{"a", "b"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.0 {
		t.Errorf("Precision = %g; want 0.0", r.Value)
	}
}

func TestPrecision_PartialPrecision(t *testing.T) {
	// 1 of 2 found is in expected
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeExistence, []string{"a"}, []string{"a", "b"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.5 {
		t.Errorf("Precision = %g; want 0.5", r.Value)
	}
}

func TestPrecision_ThresholdArchetype(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeThreshold, []string{"a"}, []string{"a"})
	if !r.Applicable {
		t.Errorf("Precision should be applicable for threshold archetype")
	}
}

func TestPrecision_ContraddictsArchetype(t *testing.T) {
	r := retrievalmetrics.Precision(evidencesignature.ArchetypeContradicts, []string{"a"}, []string{"a"})
	if !r.Applicable {
		t.Errorf("Precision should be applicable for contradicts archetype")
	}
}

// ---- Recall tests ----------------------------------------------------------

func TestRecall_NotApplicableForAbsence(t *testing.T) {
	r := retrievalmetrics.Recall(evidencesignature.ArchetypeAbsence, []string{"a"}, []string{"a"})
	if r.Applicable {
		t.Errorf("Recall should not be applicable for absence archetype")
	}
}

func TestRecall_NotApplicableForEmptyExpected(t *testing.T) {
	r := retrievalmetrics.Recall(evidencesignature.ArchetypeExistence, nil, []string{"a"})
	if r.Applicable {
		t.Errorf("Recall should not be applicable when expected is empty")
	}
}

func TestRecall_PerfectRecall(t *testing.T) {
	r := retrievalmetrics.Recall(evidencesignature.ArchetypeExistence, []string{"a", "b"}, []string{"a", "b", "c"})
	if !r.Applicable {
		t.Fatalf("expected applicable, reason=%q", r.Reason)
	}
	if r.Value != 1.0 {
		t.Errorf("Recall = %g; want 1.0", r.Value)
	}
}

func TestRecall_ZeroRecall(t *testing.T) {
	r := retrievalmetrics.Recall(evidencesignature.ArchetypeExistence, []string{"a", "b"}, []string{"x", "y"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.0 {
		t.Errorf("Recall = %g; want 0.0", r.Value)
	}
}

func TestRecall_PartialRecall(t *testing.T) {
	// found 1 of 2 expected
	r := retrievalmetrics.Recall(evidencesignature.ArchetypeExistence, []string{"a", "b"}, []string{"a"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.5 {
		t.Errorf("Recall = %g; want 0.5", r.Value)
	}
}

// ---- MRR tests -------------------------------------------------------------

func TestMRR_NotApplicableForAbsence(t *testing.T) {
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeAbsence, []string{}, []string{"a"})
	if r.Applicable {
		t.Errorf("MRR should not be applicable for absence archetype")
	}
	if r.Reason == "" {
		t.Errorf("Reason must be non-empty when Applicable == false")
	}
}

func TestMRR_NotApplicableForEmptyGoldSet(t *testing.T) {
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, nil, []string{"a"})
	if r.Applicable {
		t.Errorf("MRR should not be applicable on empty gold set")
	}
}

func TestMRR_NeverNaN(t *testing.T) {
	// All paths that could produce NaN must return not-applicable instead.
	for _, archetype := range []evidencesignature.Archetype{
		evidencesignature.ArchetypeAbsence,
	} {
		r := retrievalmetrics.MRR(archetype, nil, nil)
		if r.Applicable && isNaN(r.Value) {
			t.Errorf("MRR returned NaN for archetype=%q with empty inputs", archetype)
		}
	}
	// Empty gold set with existence archetype.
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, nil, []string{"a"})
	if r.Applicable && isNaN(r.Value) {
		t.Errorf("MRR returned NaN for empty gold set")
	}
}

func TestMRR_FirstRankRelevant(t *testing.T) {
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, []string{"a"}, []string{"a", "b"})
	if !r.Applicable {
		t.Fatalf("expected applicable, reason=%q", r.Reason)
	}
	if r.Value != 1.0 {
		t.Errorf("MRR = %g; want 1.0 (first result is relevant)", r.Value)
	}
}

func TestMRR_SecondRankRelevant(t *testing.T) {
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, []string{"b"}, []string{"a", "b"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	want := 1.0 / 2.0
	if r.Value != want {
		t.Errorf("MRR = %g; want %g", r.Value, want)
	}
}

func TestMRR_NoneRelevant(t *testing.T) {
	r := retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, []string{"z"}, []string{"a", "b", "c"})
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.0 {
		t.Errorf("MRR = %g; want 0.0 (no relevant result found)", r.Value)
	}
}

// ---- FalseAbsentRate tests -------------------------------------------------

func TestFalseAbsentRate_NotApplicableForExistence(t *testing.T) {
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeExistence, []retrievalmetrics.CanaryRecord{
		{SignatureID: "s1", CanaryFound: true},
	})
	if r.Applicable {
		t.Errorf("FalseAbsentRate should not be applicable for existence archetype")
	}
}

func TestFalseAbsentRate_NotApplicableForThreshold(t *testing.T) {
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeThreshold, nil)
	if r.Applicable {
		t.Errorf("FalseAbsentRate should not be applicable for threshold archetype")
	}
}

func TestFalseAbsentRate_NotApplicableForEmptyRecords(t *testing.T) {
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeAbsence, nil)
	if r.Applicable {
		t.Errorf("FalseAbsentRate should not be applicable when records is empty")
	}
}

func TestFalseAbsentRate_AllFound(t *testing.T) {
	records := []retrievalmetrics.CanaryRecord{
		{SignatureID: "s1", CanaryFound: true},
		{SignatureID: "s2", CanaryFound: true},
	}
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeAbsence, records)
	if !r.Applicable {
		t.Fatalf("expected applicable, reason=%q", r.Reason)
	}
	if r.Value != 0.0 {
		t.Errorf("FalseAbsentRate = %g; want 0.0 (all found)", r.Value)
	}
}

func TestFalseAbsentRate_AllMissed(t *testing.T) {
	records := []retrievalmetrics.CanaryRecord{
		{SignatureID: "s1", CanaryFound: false},
		{SignatureID: "s2", CanaryFound: false},
	}
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeAbsence, records)
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 1.0 {
		t.Errorf("FalseAbsentRate = %g; want 1.0 (all missed)", r.Value)
	}
}

func TestFalseAbsentRate_Mixed(t *testing.T) {
	records := []retrievalmetrics.CanaryRecord{
		{SignatureID: "s1", CanaryFound: true},
		{SignatureID: "s2", CanaryFound: false},
		{SignatureID: "s3", CanaryFound: false},
		{SignatureID: "s4", CanaryFound: true},
	}
	r := retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeAbsence, records)
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	want := 0.5
	if r.Value != want {
		t.Errorf("FalseAbsentRate = %g; want %g", r.Value, want)
	}
}

// ---- CoverageFraction tests ------------------------------------------------

func TestCoverageFraction_NotApplicableForExistence(t *testing.T) {
	m := evidencesignature.CoverageManifest{Tool: "test-tool", Files: []string{"a.go"}}
	r := retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeExistence, m)
	if r.Applicable {
		t.Errorf("CoverageFraction should not be applicable for existence archetype")
	}
}

func TestCoverageFraction_NotApplicableForEmptyManifest(t *testing.T) {
	m := evidencesignature.CoverageManifest{Tool: "test-tool"}
	r := retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeAbsence, m)
	if r.Applicable {
		t.Errorf("CoverageFraction should not be applicable when manifest has no files")
	}
}

func TestCoverageFraction_FullCoverage(t *testing.T) {
	m := evidencesignature.CoverageManifest{
		Tool:  "test-tool",
		Files: []string{"a.go", "b.go", "c.go"},
		// No skipped files.
	}
	r := retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeAbsence, m)
	if !r.Applicable {
		t.Fatalf("expected applicable, reason=%q", r.Reason)
	}
	if r.Value != 1.0 {
		t.Errorf("CoverageFraction = %g; want 1.0", r.Value)
	}
}

func TestCoverageFraction_PartialCoverage(t *testing.T) {
	m := evidencesignature.CoverageManifest{
		Tool:  "test-tool",
		Files: []string{"a.go", "b.go"},
		Skipped: evidencesignature.SkippedFiles{
			Binary:   []string{"large.bin"},
			Oversize: []string{"huge.log"},
		},
	}
	r := retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeAbsence, m)
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	// 2 searched, 2 skipped → 2/4 = 0.5
	want := 0.5
	if r.Value != want {
		t.Errorf("CoverageFraction = %g; want %g", r.Value, want)
	}
}

func TestCoverageFraction_AllSkipped(t *testing.T) {
	m := evidencesignature.CoverageManifest{
		Tool: "test-tool",
		Skipped: evidencesignature.SkippedFiles{
			Binary: []string{"a.bin"},
		},
	}
	r := retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeAbsence, m)
	if !r.Applicable {
		t.Fatalf("expected applicable")
	}
	if r.Value != 0.0 {
		t.Errorf("CoverageFraction = %g; want 0.0 (no files searched)", r.Value)
	}
}

// ---- Applicability invariant tests ----------------------------------------

// TestApplicabilityInvariant verifies that when Applicable == false, Reason
// is always non-empty, and when Applicable == true, Reason is always empty.
func TestApplicabilityInvariant(t *testing.T) {
	results := []retrievalmetrics.MetricResult{
		retrievalmetrics.Precision(evidencesignature.ArchetypeAbsence, nil, nil),
		retrievalmetrics.Precision(evidencesignature.ArchetypeExistence, []string{"a"}, []string{"a"}),
		retrievalmetrics.Recall(evidencesignature.ArchetypeAbsence, nil, nil),
		retrievalmetrics.Recall(evidencesignature.ArchetypeExistence, []string{"a"}, []string{"a"}),
		retrievalmetrics.MRR(evidencesignature.ArchetypeAbsence, nil, nil),
		retrievalmetrics.MRR(evidencesignature.ArchetypeExistence, []string{"a"}, []string{"a"}),
		retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeExistence, nil),
		retrievalmetrics.FalseAbsentRate(evidencesignature.ArchetypeAbsence, []retrievalmetrics.CanaryRecord{
			{SignatureID: "x", CanaryFound: true},
		}),
		retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeExistence,
			evidencesignature.CoverageManifest{Tool: "t", Files: []string{"f"}}),
		retrievalmetrics.CoverageFraction(evidencesignature.ArchetypeAbsence,
			evidencesignature.CoverageManifest{Tool: "t", Files: []string{"f"}}),
	}

	for i, r := range results {
		if !r.Applicable && r.Reason == "" {
			t.Errorf("result[%d]: Applicable=false but Reason is empty", i)
		}
		if r.Applicable && r.Reason != "" {
			t.Errorf("result[%d]: Applicable=true but Reason=%q (should be empty)", i, r.Reason)
		}
		if r.Value < 0 || r.Value > 1 {
			t.Errorf("result[%d]: Value=%g out of [0,1]", i, r.Value)
		}
	}
}

// isNaN reports whether f is IEEE 754 NaN.
func isNaN(f float64) bool {
	return f != f
}
