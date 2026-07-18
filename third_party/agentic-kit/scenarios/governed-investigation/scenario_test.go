package governedinvestigation_test

import (
	"strings"
	"testing"

	scenario "github.com/geoffmilleraz/agentic-kit/scenarios/governed-investigation"
)

// ── Run produces a non-empty Report ──────────────────────────────────────────

func TestRun_ProducesReport(t *testing.T) {
	r := scenario.Run()

	if r.EvidenceCount == 0 {
		t.Error("expected EvidenceCount > 0")
	}
	if len(r.Journal) == 0 {
		t.Error("expected at least one journal entry")
	}
	if len(r.RankedHypotheses) == 0 {
		t.Error("expected at least one ranked hypothesis")
	}
}

// ── Run produces a terminal finding ──────────────────────────────────────────

func TestRun_ProducesAFinding(t *testing.T) {
	r := scenario.Run()

	if r.Finding.Summary == "" {
		t.Error("expected non-empty Finding.Summary")
	}
	if r.Finding.Confidence == "" {
		t.Error("expected non-empty Finding.Confidence")
	}
	if r.Finding.TopHypothesis.ID == "" {
		t.Error("expected non-empty Finding.TopHypothesis.ID")
	}
}

// ── Run terminates with a high-confidence finding ────────────────────────────

func TestRun_FindingIsHighConfidence(t *testing.T) {
	r := scenario.Run()

	// The seeded evidence corpus is weighted so that the accumulated weight
	// exceeds the ConfidenceHigh threshold. Verify we got a high-confidence
	// terminal finding.
	if r.Finding.Confidence != "high" {
		t.Errorf("expected Confidence=high, got %q", r.Finding.Confidence)
	}
}

// ── Run is deterministic ──────────────────────────────────────────────────────

func TestRun_IsDeterministic(t *testing.T) {
	r1 := scenario.Run()
	r2 := scenario.Run()

	if r1.Finding.Summary != r2.Finding.Summary {
		t.Errorf("Finding.Summary is non-deterministic:\n  run1: %q\n  run2: %q",
			r1.Finding.Summary, r2.Finding.Summary)
	}
	if r1.Finding.Confidence != r2.Finding.Confidence {
		t.Errorf("Finding.Confidence is non-deterministic: %q vs %q",
			r1.Finding.Confidence, r2.Finding.Confidence)
	}
	if r1.EvidenceCount != r2.EvidenceCount {
		t.Errorf("EvidenceCount is non-deterministic: %d vs %d",
			r1.EvidenceCount, r2.EvidenceCount)
	}
	if len(r1.Journal) != len(r2.Journal) {
		t.Errorf("Journal length is non-deterministic: %d vs %d",
			len(r1.Journal), len(r2.Journal))
	}
	for i := range r1.Journal {
		if r1.Journal[i].Step != r2.Journal[i].Step {
			t.Errorf("Journal[%d].Step mismatch: %q vs %q", i, r1.Journal[i].Step, r2.Journal[i].Step)
		}
	}
	if len(r1.RankedHypotheses) != len(r2.RankedHypotheses) {
		t.Errorf("RankedHypotheses length non-deterministic: %d vs %d",
			len(r1.RankedHypotheses), len(r2.RankedHypotheses))
	}
	for i := range r1.RankedHypotheses {
		if r1.RankedHypotheses[i].ID != r2.RankedHypotheses[i].ID {
			t.Errorf("RankedHypotheses[%d] ID mismatch: %q vs %q",
				i, r1.RankedHypotheses[i].ID, r2.RankedHypotheses[i].ID)
		}
	}
}

// ── Run is deterministic under -race ─────────────────────────────────────────

func TestRun_DeterministicUnderRace(t *testing.T) {
	// Run in parallel to exercise the race detector.
	t.Parallel()

	r1 := scenario.Run()
	r2 := scenario.Run()

	if r1.Finding.Summary != r2.Finding.Summary {
		t.Errorf("race: Finding.Summary non-deterministic: %q vs %q",
			r1.Finding.Summary, r2.Finding.Summary)
	}
}

// ── Run journal has expected steps ───────────────────────────────────────────

func TestRun_JournalHasExpectedSteps(t *testing.T) {
	r := scenario.Run()

	wantSteps := []string{
		"investigation-started",
		"evidence-collected",
		"hypotheses-ranked",
		"investigation-complete",
	}

	if len(r.Journal) < len(wantSteps) {
		t.Fatalf("expected at least %d journal entries, got %d", len(wantSteps), len(r.Journal))
	}

	stepSet := make(map[string]bool, len(r.Journal))
	for _, e := range r.Journal {
		stepSet[e.Step] = true
	}

	for _, want := range wantSteps {
		if !stepSet[want] {
			t.Errorf("expected journal step %q to be present", want)
		}
	}
}

// ── Run hypotheses are ranked by support descending ──────────────────────────

func TestRun_HypothesesRankedDescending(t *testing.T) {
	r := scenario.Run()

	for i := 1; i < len(r.RankedHypotheses); i++ {
		prev := r.RankedHypotheses[i-1].Support
		curr := r.RankedHypotheses[i].Support
		if curr > prev {
			t.Errorf("hypotheses not sorted by support descending at [%d,%d]: %f > %f",
				i-1, i, curr, prev)
		}
	}
}

// ── Run top hypothesis matches the highest-support candidate ─────────────────

func TestRun_TopHypothesisIsHighestSupport(t *testing.T) {
	r := scenario.Run()

	top := r.Finding.TopHypothesis
	if top.ID != r.RankedHypotheses[0].ID {
		t.Errorf("expected TopHypothesis.ID=%q (ranked[0]), got %q",
			r.RankedHypotheses[0].ID, top.ID)
	}
}

// ── Run finding summary references evidence IDs ───────────────────────────────

func TestRun_FindingSummaryReferencesEvidence(t *testing.T) {
	r := scenario.Run()

	// The DAD investigator includes evidence IDs in the terminal finding.
	// Our seeded IDs all have the prefix "span-".
	if !strings.Contains(r.Finding.Summary, "span-") {
		t.Errorf("expected Finding.Summary to reference evidence IDs (span-*), got %q",
			r.Finding.Summary)
	}
}

// ── Run evidence count matches seed ──────────────────────────────────────────

func TestRun_EvidenceCountMatchesSeed(t *testing.T) {
	r := scenario.Run()
	// The scenario seeds exactly 4 evidence spans.
	if r.EvidenceCount != 4 {
		t.Errorf("expected EvidenceCount=4, got %d", r.EvidenceCount)
	}
}
