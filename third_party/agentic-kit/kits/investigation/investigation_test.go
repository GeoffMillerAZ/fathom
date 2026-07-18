package investigation_test

import (
	"testing"
	"time"

	dadinvestigator "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator"
	investigation "github.com/geoffmilleraz/agentic-kit/kits/investigation"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newInv is a helper that creates an Investigation and fails the test on error.
func newInv(t *testing.T, question string) *investigation.Investigation {
	t.Helper()
	inv, err := investigation.New(question)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return inv
}

// addEvidence is a helper that adds evidence and fails the test on error.
func addEvidence(t *testing.T, inv *investigation.Investigation, e investigation.EvidenceInput) {
	t.Helper()
	if err := inv.AddEvidence(e); err != nil {
		t.Fatalf("AddEvidence(%s): %v", e.ID, err)
	}
}

// step is a helper that advances one DAD phase and fails the test on error.
func step(t *testing.T, inv *investigation.Investigation) {
	t.Helper()
	if err := inv.Step(); err != nil {
		t.Fatalf("Step: %v", err)
	}
}

// ── Test 1: New requires a non-empty question ────────────────────────────────

func TestNew_RequiresQuestion(t *testing.T) {
	t.Parallel()
	_, err := investigation.New("")
	if err == nil {
		t.Fatal("expected error for empty question; got nil")
	}
}

// ── Test 2: AddEvidence rejects empty ID ─────────────────────────────────────

func TestAddEvidence_RejectsEmptyID(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "ownership gap analysis")
	err := inv.AddEvidence(investigation.EvidenceInput{
		ID:      "",
		Content: "some content",
		Weight:  1,
	})
	if err == nil {
		t.Fatal("expected error for empty evidence ID; got nil")
	}
}

// ── Test 3: Journal captures opening + evidence entries ──────────────────────

func TestJournal_RecordsOpenAndEvidence(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "find the ownership gap")
	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "ev-001", Content: "service alpha has no owner", Weight: 2,
	})

	entries := inv.JournalEntries()
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 journal entries; got %d", len(entries))
	}
	if entries[0].Step != "investigation-opened" {
		t.Errorf("entries[0].Step = %q; want %q", entries[0].Step, "investigation-opened")
	}
	if entries[1].Step != "evidence-added" {
		t.Errorf("entries[1].Step = %q; want %q", entries[1].Step, "evidence-added")
	}
}

// ── Test 4: Step journals DAD phases ─────────────────────────────────────────

func TestStep_JournalsDadPhase(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "detect regressions")
	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "ev-a", Content: "regression in module A", Weight: 3,
	})
	step(t, inv) // decompose → analyse

	entries := inv.JournalEntries()
	var foundDadStep bool
	for _, e := range entries {
		if e.Step == "dad-step" {
			foundDadStep = true
			break
		}
	}
	if !foundDadStep {
		t.Error("no 'dad-step' journal entry found after Step()")
	}
}

// ── Test 5: Full converging investigation ────────────────────────────────────
// This is the primary end-to-end test. We feed enough evidence to push the
// DAD machine to PhaseFound (accumulated weight >= 30 → ConfidenceHigh),
// then verify Status reports "converged" and Findings are non-empty.

func TestConvergingInvestigation(t *testing.T) {
	t.Parallel()

	inv := newInv(t, "ownership gap analysis: find unowned services")

	// Add evidence in batches. Each item has weight=10, so 3 items → weight=30 → ConfidenceHigh.
	// DAD cycle: Step1=Decompose→Analyse, Step2=Analyse→Decide.
	// After Decide with weight>=30 the machine transitions to PhaseFound.

	// Batch 1: add 3 items.
	for i, id := range []string{"ev-001", "ev-002", "ev-003"} {
		addEvidence(t, inv, investigation.EvidenceInput{
			ID:         id,
			Content:    "service gap evidence " + id,
			Weight:     10,
			EntityID:   "service-alpha",
			Field:      "owner",
			Value:      "unknown",
			Confidence: 0.9,
			Source:     "code-review",
		})
		_ = i
	}

	// Advance through DAD: Decompose→Analyse→Decide→Found (3 step calls).
	for range 3 {
		step(t, inv)
		s := inv.Status()
		if s.Verdict == "converged" {
			break
		}
	}

	s := inv.Status()
	if s.Verdict != "converged" {
		t.Errorf("Status.Verdict = %q; want %q (steps=%d findings=%d)",
			s.Verdict, "converged", s.Steps, s.FindingCount)
	}

	findings := inv.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding; got none")
	}

	// The terminal finding must have ConfidenceHigh.
	var terminalFound bool
	for _, f := range findings {
		if f.Terminal {
			terminalFound = true
			if f.Confidence != dadinvestigator.ConfidenceHigh {
				t.Errorf("terminal finding confidence = %q; want %q", f.Confidence, dadinvestigator.ConfidenceHigh)
			}
		}
	}
	if !terminalFound {
		t.Error("no terminal finding found; investigation did not converge to PhaseFound")
	}
}

// ── Test 6: Status before any steps reports NeedsMoreSources ─────────────────

func TestStatus_BeforeAnySteps(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "baseline status check")
	s := inv.Status()
	// No findings yet → oracle returns "needs_sources".
	if s.Verdict != "needs_sources" {
		t.Errorf("Status.Verdict = %q; want %q", s.Verdict, "needs_sources")
	}
}

// ── Test 7: Contradiction detection raises a concern flag ─────────────────────

func TestAddEvidence_ContradictionRaisesConcernFlag(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "contradiction detection test")

	// Two spans with the same EntityID+Field but different Values → contradiction.
	addEvidence(t, inv, investigation.EvidenceInput{
		ID:         "ev-c-1",
		Content:    "owner is team-A",
		Weight:     1,
		EntityID:   "service-beta",
		Field:      "owner",
		Value:      "team-A",
		Confidence: 0.8,
		Source:     "docs",
	})
	addEvidence(t, inv, investigation.EvidenceInput{
		ID:         "ev-c-2",
		Content:    "owner is team-B",
		Weight:     1,
		EntityID:   "service-beta",
		Field:      "owner",
		Value:      "team-B",
		Confidence: 0.8,
		Source:     "code",
	})

	step(t, inv)

	findings := inv.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding after step")
	}

	// A concern flag for the contradiction must be present.
	var hasConcern bool
	for _, f := range findings {
		for _, cf := range f.ConcernFlags {
			if cf.Type == "contradiction" {
				hasConcern = true
			}
		}
	}
	if !hasConcern {
		t.Error("expected a contradiction concern flag; found none")
	}
}

// ── Test 8a: Findings carry SupportingEvidenceIDs ─────────────────────────────
//
// Evidence added before a Step call must appear in the finding's
// SupportingEvidenceIDs. EvidenceDegraded must be false when evidence is present.

func TestFindings_SupportingEvidenceIDs(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "evidence provenance test")

	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "ev-prov-1", Content: "ownership gap in service alpha", Weight: 10,
		EntityID: "service-alpha", Field: "owner", Value: "unknown",
		Confidence: 0.9, Source: "scanner",
	})
	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "ev-prov-2", Content: "no owner in catalog", Weight: 10,
		Confidence: 0.9, Source: "catalog",
	})

	step(t, inv)

	findings := inv.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding after step")
	}
	f := findings[0]

	// The finding must carry both evidence IDs.
	if f.EvidenceDegraded {
		t.Error("EvidenceDegraded = true; want false (evidence was provided)")
	}
	if len(f.SupportingEvidenceIDs) == 0 {
		t.Fatal("SupportingEvidenceIDs is empty; want [ev-prov-1, ev-prov-2]")
	}

	seen := make(map[string]bool)
	for _, id := range f.SupportingEvidenceIDs {
		seen[id] = true
	}
	for _, want := range []string{"ev-prov-1", "ev-prov-2"} {
		if !seen[want] {
			t.Errorf("SupportingEvidenceIDs missing %q; got %v", want, f.SupportingEvidenceIDs)
		}
	}
}

// ── Test 8b: EvidenceDegraded when Step is called without prior AddEvidence ────
//
// Calling Step without adding evidence first is a degraded state. The finding
// must have EvidenceDegraded=true and a "finding_without_evidence" concern flag.

func TestStep_WithoutEvidence_EvidenceDegraded(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "degraded finding test")

	// Step without any prior AddEvidence — no evidence in the pending queue.
	step(t, inv)

	findings := inv.Findings()
	if len(findings) == 0 {
		t.Fatal("expected at least one finding after step")
	}
	f := findings[0]

	if !f.EvidenceDegraded {
		t.Error("EvidenceDegraded = false; want true (no evidence was provided before Step)")
	}
	if len(f.SupportingEvidenceIDs) != 0 {
		t.Errorf("SupportingEvidenceIDs = %v; want empty", f.SupportingEvidenceIDs)
	}

	// A "finding_without_evidence" concern flag must be raised.
	var hasDegradedFlag bool
	for _, cf := range f.ConcernFlags {
		if cf.Type == "finding_without_evidence" {
			hasDegradedFlag = true
		}
	}
	if !hasDegradedFlag {
		t.Error("expected a 'finding_without_evidence' concern flag; found none")
	}
}

// ── Test 8c: Evidence added in separate batches is scoped to the right finding ─
//
// Evidence added before Step 1 should appear in finding[0].
// Evidence added before Step 2 should appear in finding[1] (not finding[0]).

func TestFindings_EvidenceScopedToStep(t *testing.T) {
	t.Parallel()
	inv := newInv(t, "evidence scoping test")

	// Batch 1 — will go into finding[0].
	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "batch1-a", Content: "evidence batch 1", Weight: 10, Confidence: 0.9, Source: "s",
	})
	step(t, inv)

	// Batch 2 — will go into finding[1].
	addEvidence(t, inv, investigation.EvidenceInput{
		ID: "batch2-a", Content: "evidence batch 2", Weight: 10, Confidence: 0.9, Source: "s",
	})
	step(t, inv)

	findings := inv.Findings()
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings; got %d", len(findings))
	}

	// finding[0] must contain batch1-a but not batch2-a.
	ids0 := make(map[string]bool)
	for _, id := range findings[0].SupportingEvidenceIDs {
		ids0[id] = true
	}
	if !ids0["batch1-a"] {
		t.Errorf("finding[0] missing batch1-a; got %v", findings[0].SupportingEvidenceIDs)
	}
	if ids0["batch2-a"] {
		t.Errorf("finding[0] unexpectedly contains batch2-a")
	}

	// finding[1] must contain batch2-a but not batch1-a.
	ids1 := make(map[string]bool)
	for _, id := range findings[1].SupportingEvidenceIDs {
		ids1[id] = true
	}
	if !ids1["batch2-a"] {
		t.Errorf("finding[1] missing batch2-a; got %v", findings[1].SupportingEvidenceIDs)
	}
	if ids1["batch1-a"] {
		t.Errorf("finding[1] unexpectedly contains batch1-a")
	}
}

// ── Test 8: Determinism — same inputs, same outputs ──────────────────────────

func TestDeterminism(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	run := func() investigation.Status {
		inv, err := investigation.New("determinism test")
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// Inject fixed time (not exported, but evidence/step is deterministic
		// regardless of wall clock in the oracle/ranker).
		_ = fixedNow
		for _, id := range []string{"d-001", "d-002", "d-003"} {
			if err := inv.AddEvidence(investigation.EvidenceInput{
				ID: id, Content: "evidence " + id, Weight: 10,
				EntityID: "svc", Field: "status", Value: "degraded",
				Confidence: 0.9, Source: "monitor",
			}); err != nil {
				t.Fatalf("AddEvidence: %v", err)
			}
		}
		for range 3 {
			if err := inv.Step(); err != nil {
				t.Fatalf("Step: %v", err)
			}
		}
		return inv.Status()
	}

	s1 := run()
	s2 := run()

	if s1.Verdict != s2.Verdict {
		t.Errorf("non-deterministic Verdict: %q vs %q", s1.Verdict, s2.Verdict)
	}
	if s1.FindingCount != s2.FindingCount {
		t.Errorf("non-deterministic FindingCount: %d vs %d", s1.FindingCount, s2.FindingCount)
	}
}
