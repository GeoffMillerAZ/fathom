package governeddocextracthitl_test

import (
	"strings"
	"testing"

	investigationjournal "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	scenario "github.com/geoffmilleraz/agentic-kit/scenarios/governed-docextract-hitl"
)

// ── docextract: CSV was extracted into non-empty text ─────────────────────────

func TestRun_DocextractProducedText(t *testing.T) {
	r := scenario.Run()

	if r.ExtractedText == "" {
		t.Error("kits/docextract: ExtractedText is empty; docextract did not run")
	}
	// The CSV contains field names; verify the extracted text carries them.
	if !strings.Contains(r.ExtractedText, "department") {
		t.Errorf("kits/docextract: expected 'department' in ExtractedText, got %q", r.ExtractedText)
	}
	if !strings.Contains(r.ExtractedText, "engineering") {
		t.Errorf("kits/docextract: expected 'engineering' in ExtractedText, got %q", r.ExtractedText)
	}
}

// ── agent-harness: journal has expected governed-loop entries ─────────────────

func TestRun_HarnessJournalHasExpectedSteps(t *testing.T) {
	r := scenario.Run()

	if len(r.Journal) == 0 {
		t.Fatal("kits/agent-harness: Journal is empty; harness did not journal")
	}

	stepSet := make(map[string]bool, len(r.Journal))
	for _, e := range r.Journal {
		stepSet[e.Step] = true
	}

	wantSteps := []string{"run-started", "turn-completed", "run-ended"}
	for _, want := range wantSteps {
		if !stepSet[want] {
			t.Errorf("kits/agent-harness: expected journal step %q to be present; got steps: %v",
				want, journalSteps(r.Journal))
		}
	}
}

// ── agent-harness: harness made the expected number of turns ──────────────────

func TestRun_HarnessUsedTwoTurns(t *testing.T) {
	r := scenario.Run()

	// Script: one tool-use turn + one final-text turn = 2 model calls.
	if r.Turns != 2 {
		t.Errorf("kits/agent-harness: expected 2 turns, got %d", r.Turns)
	}
}

// ── agent-harness: budget guard was charged ───────────────────────────────────

func TestRun_BudgetWasCharged(t *testing.T) {
	r := scenario.Run()

	if r.BudgetUsed == 0 {
		t.Error("kits/agent-harness: BudgetUsed is 0; budget guard did not run or was never charged")
	}
	// One charge per turn (pre-call), so charges == turns.
	if r.BudgetUsed != r.Turns {
		t.Errorf("kits/agent-harness: expected BudgetUsed=%d (one per turn), got %d", r.Turns, r.BudgetUsed)
	}
}

// ── hitl: preview ran and produced a non-empty summary ───────────────────────

func TestRun_HitlPreviewRan(t *testing.T) {
	r := scenario.Run()

	if r.PreviewSummary == "" {
		t.Error("kits/hitl: PreviewSummary is empty; hitl Preview did not run")
	}
	// The applier produces a summary mentioning the field name.
	if !strings.Contains(r.PreviewSummary, "status") {
		t.Errorf("kits/hitl: expected 'status' in PreviewSummary, got %q", r.PreviewSummary)
	}
}

// ── hitl: Apply committed the expected mutation ───────────────────────────────

func TestRun_HitlApplyMutatedState(t *testing.T) {
	r := scenario.Run()

	got := r.AppliedState.Fields["status"]
	if got != "approved" {
		t.Errorf("kits/hitl: expected AppliedState.Fields[\"status\"]=%q, got %q", "approved", got)
	}
	// Fields the applier did NOT touch must be unchanged.
	if r.AppliedState.Fields["department"] != "engineering" {
		t.Errorf("kits/hitl: unexpected mutation of department field: %q", r.AppliedState.Fields["department"])
	}
}

// ── hitl: preview is consistent with apply (same applier invariant) ───────────

func TestRun_HitlPreviewMatchesApply(t *testing.T) {
	r := scenario.Run()

	// Both preview and apply use the same Applier, so the summary must reference
	// the same mutation.
	if !strings.Contains(r.PreviewSummary, "approved") {
		t.Errorf("kits/hitl: PreviewSummary should reference the new value 'approved', got %q", r.PreviewSummary)
	}
}

// ── harness run-report: agent emitted a non-empty report ─────────────────────

func TestRun_RunReportNonEmpty(t *testing.T) {
	r := scenario.Run()

	if r.RunReport == "" {
		t.Error("kits/agent-harness: RunReport is empty; harness FinalText was not set")
	}
	// The scripted final response mentions "run".
	if !strings.Contains(strings.ToLower(r.RunReport), "run") {
		t.Errorf("harness RunReport does not mention 'run': %q", r.RunReport)
	}
}

// ── determinism: two Run calls produce identical Reports ─────────────────────

func TestRun_IsDeterministic(t *testing.T) {
	r1 := scenario.Run()
	r2 := scenario.Run()

	if r1.ExtractedText != r2.ExtractedText {
		t.Errorf("ExtractedText non-deterministic:\n  r1: %q\n  r2: %q", r1.ExtractedText, r2.ExtractedText)
	}
	if r1.PreviewSummary != r2.PreviewSummary {
		t.Errorf("PreviewSummary non-deterministic: %q vs %q", r1.PreviewSummary, r2.PreviewSummary)
	}
	if r1.RunReport != r2.RunReport {
		t.Errorf("RunReport non-deterministic: %q vs %q", r1.RunReport, r2.RunReport)
	}
	if len(r1.Journal) != len(r2.Journal) {
		t.Errorf("Journal length non-deterministic: %d vs %d", len(r1.Journal), len(r2.Journal))
	}
	if r1.Turns != r2.Turns {
		t.Errorf("Turns non-deterministic: %d vs %d", r1.Turns, r2.Turns)
	}
	if r1.AppliedState.Fields["status"] != r2.AppliedState.Fields["status"] {
		t.Errorf("AppliedState non-deterministic: %q vs %q",
			r1.AppliedState.Fields["status"], r2.AppliedState.Fields["status"])
	}
}

// ── race safety ───────────────────────────────────────────────────────────────

func TestRun_DeterministicUnderRace(t *testing.T) {
	t.Parallel()

	r1 := scenario.Run()
	r2 := scenario.Run()

	if r1.PreviewSummary != r2.PreviewSummary {
		t.Errorf("race: PreviewSummary non-deterministic: %q vs %q", r1.PreviewSummary, r2.PreviewSummary)
	}
	if r1.RunReport != r2.RunReport {
		t.Errorf("race: RunReport non-deterministic: %q vs %q", r1.RunReport, r2.RunReport)
	}
}

// journalSteps extracts Step fields for test error messages.
func journalSteps(entries []investigationjournal.Entry) []string {
	steps := make([]string, len(entries))
	for i, e := range entries {
		steps[i] = e.Step
	}
	return steps
}
