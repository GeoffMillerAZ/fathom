package correctionledger_test

// correctionledger_fixes_test.go — TDD tests for audit-confirmed findings.
// Each test is RED until the corresponding fix is applied.

import (
	"testing"
	"time"

	cl "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger"
)

// ── Finding 1: Validation mirrors CUE schema ────────────────────────────────

func TestOpenFinding_RejectsEmptyID(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("", "some description")
	if err == nil {
		t.Fatal("OpenFinding with empty ID must return an error")
	}
}

func TestRecordCorrection_RejectsEmptyRequiredFields(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("f1", "desc")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	base := cl.Correction{
		FindingID:   "f1",
		Target:      cl.CorrectionTarget{StableTarget: "doc/s/1.v1"},
		Original:    "old",
		Corrected:   "new",
		CorrectedBy: "alice",
		Reason:      "wrong",
		Confidence:  3,
	}

	cases := []struct {
		name string
		mut  func(*cl.Correction)
	}{
		{"empty FindingID", func(c *cl.Correction) { c.FindingID = "" }},
		{"empty Original", func(c *cl.Correction) { c.Original = "" }},
		{"empty Corrected", func(c *cl.Correction) { c.Corrected = "" }},
		{"empty CorrectedBy", func(c *cl.Correction) { c.CorrectedBy = "" }},
		{"empty Reason", func(c *cl.Correction) { c.Reason = "" }},
		{"empty StableTarget", func(c *cl.Correction) { c.Target.StableTarget = "" }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mut(&c)
			if err := l.RecordCorrection(c); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestRecordDismissal_RejectsEmptyRequiredFields(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("f2", "desc")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	base := cl.Dismissal{
		FindingID:   "f2",
		DismissedBy: "bob",
		Reason:      "false positive",
	}

	cases := []struct {
		name string
		mut  func(*cl.Dismissal)
	}{
		{"empty FindingID", func(d *cl.Dismissal) { d.FindingID = "" }},
		{"empty DismissedBy", func(d *cl.Dismissal) { d.DismissedBy = "" }},
		{"empty Reason", func(d *cl.Dismissal) { d.Reason = "" }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.mut(&d)
			if err := l.RecordDismissal(d); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

// ── Finding 2: Injectable clock ─────────────────────────────────────────────

func TestNewWithClock_UsesInjectedClock(t *testing.T) {
	fixed := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	l := cl.NewWithClock(func() time.Time { return fixed })

	f, err := l.OpenFinding("clock-f1", "clock test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	if !f.CreatedAt.Equal(fixed) {
		t.Errorf("CreatedAt: want %v, got %v", fixed, f.CreatedAt)
	}
	if !f.UpdatedAt.Equal(fixed) {
		t.Errorf("UpdatedAt: want %v, got %v", fixed, f.UpdatedAt)
	}

	// The ledger event's OccurredAt should also use the injected clock.
	events := l.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].OccurredAt.Equal(fixed) {
		t.Errorf("event OccurredAt: want %v, got %v", fixed, events[0].OccurredAt)
	}
}

func TestNewWithClock_TransitionUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)
	l := cl.NewWithClock(func() time.Time { return fixed })

	f, err := l.OpenFinding("clock-f2", "transition clock test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	updated, err := l.ApplyTransition(f, cl.EventClaim)
	if err != nil {
		t.Fatalf("ApplyTransition: %v", err)
	}
	if !updated.UpdatedAt.Equal(fixed) {
		t.Errorf("UpdatedAt after transition: want %v, got %v", fixed, updated.UpdatedAt)
	}
}

// ── Finding 4: Events() deep-copies pointer fields ──────────────────────────

func TestLedger_EventsDeepCopiesPointerFields(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("deep-f1", "deep copy test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	events := l.Events()
	if events[0].FindingOpened == nil {
		t.Fatal("FindingOpened is nil")
	}

	// Mutate through the returned pointer — must NOT affect internal state.
	original := events[0].FindingOpened.Description
	events[0].FindingOpened.Description = "tampered"

	// Fetch again and verify internal event is unchanged.
	events2 := l.Events()
	if events2[0].FindingOpened.Description != original {
		t.Errorf("internal event was mutated via Events() pointer: want %q, got %q",
			original, events2[0].FindingOpened.Description)
	}
}

func TestLedger_EventsDeepCopiesTransitionEvent(t *testing.T) {
	l := cl.New()
	f, err := l.OpenFinding("deep-f2", "transition deep copy")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	_, err = l.ApplyTransition(f, cl.EventClaim)
	if err != nil {
		t.Fatalf("ApplyTransition: %v", err)
	}

	events := l.Events()
	var transEv *cl.LedgerEvent
	for i := range events {
		if events[i].Kind == cl.LedgerEventKindTransition {
			transEv = &events[i]
			break
		}
	}
	if transEv == nil || transEv.Transition == nil {
		t.Fatal("no transition event found")
	}

	// Mutate through returned pointer.
	transEv.Transition.FindingID = "hacked"

	events2 := l.Events()
	for _, ev := range events2 {
		if ev.Kind == cl.LedgerEventKindTransition && ev.Transition != nil {
			if ev.Transition.FindingID == "hacked" {
				t.Error("internal TransitionEvent was mutated via Events() pointer")
			}
		}
	}
}

// ── Finding 5: Duplicate OpenFinding IDs rejected ───────────────────────────

func TestOpenFinding_RejectsDuplicateID(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("dup-f1", "first open")
	if err != nil {
		t.Fatalf("first OpenFinding: %v", err)
	}
	_, err = l.OpenFinding("dup-f1", "second open — must fail")
	if err == nil {
		t.Fatal("expected error when opening a finding with a duplicate ID, got nil")
	}
}

// ── Finding 6: Optional provenance on Correction and Dismissal ──────────────

func TestCorrection_OptionalAuthorField(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("prov-f1", "provenance test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	// Correction with human author provenance should be accepted.
	c := cl.Correction{
		FindingID:   "prov-f1",
		Target:      cl.CorrectionTarget{StableTarget: "doc/s/1.v1"},
		Original:    "old",
		Corrected:   "new",
		CorrectedBy: "alice",
		Reason:      "wrong",
		Confidence:  4,
		Author: &cl.Author{
			Kind:       cl.AuthorKindHuman,
			IdentityID: "alice@example.com",
		},
		SourceHash: "abc123",
		Locator:    "doc://section/3#para2",
	}
	if err := l.RecordCorrection(c); err != nil {
		t.Fatalf("RecordCorrection with Author: %v", err)
	}

	// Correction without Author (nil) must still be accepted — field is optional.
	c2 := cl.Correction{
		FindingID:   "prov-f1",
		Target:      cl.CorrectionTarget{StableTarget: "doc/s/2.v1"},
		Original:    "old2",
		Corrected:   "new2",
		CorrectedBy: "bob",
		Reason:      "also wrong",
		Confidence:  3,
	}
	if err := l.RecordCorrection(c2); err != nil {
		t.Fatalf("RecordCorrection without Author: %v", err)
	}
}

func TestDismissal_OptionalAuthorField(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("prov-f2", "dismissal provenance test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	d := cl.Dismissal{
		FindingID:   "prov-f2",
		DismissedBy: "carol",
		Reason:      "false positive",
		Author: &cl.Author{
			Kind:       cl.AuthorKindAI,
			IdentityID: "claude-opus-4-8",
			AIModelID:  "claude-opus-4-8",
		},
	}
	if err := l.RecordDismissal(d); err != nil {
		t.Fatalf("RecordDismissal with Author: %v", err)
	}

	// Without Author — must still be accepted.
	d2 := cl.Dismissal{
		FindingID:   "prov-f2",
		DismissedBy: "dave",
		Reason:      "acceptable risk",
	}
	if err := l.RecordDismissal(d2); err != nil {
		t.Fatalf("RecordDismissal without Author: %v", err)
	}
}

// ── Finding 7: Reject corrections/dismissals for non-existent FindingIDs ────

func TestRecordCorrection_RejectsPhantomFindingID(t *testing.T) {
	l := cl.New()
	// No findings opened — any correction should be rejected.
	c := cl.Correction{
		FindingID:   "phantom-id",
		Target:      cl.CorrectionTarget{StableTarget: "doc/s/1.v1"},
		Original:    "old",
		Corrected:   "new",
		CorrectedBy: "alice",
		Reason:      "wrong",
		Confidence:  3,
	}
	if err := l.RecordCorrection(c); err == nil {
		t.Fatal("expected error for correction referencing non-existent FindingID, got nil")
	}
}

func TestRecordDismissal_RejectsPhantomFindingID(t *testing.T) {
	l := cl.New()
	d := cl.Dismissal{
		FindingID:   "phantom-id",
		DismissedBy: "bob",
		Reason:      "false positive",
	}
	if err := l.RecordDismissal(d); err == nil {
		t.Fatal("expected error for dismissal referencing non-existent FindingID, got nil")
	}
}

// ── Regression: existing Ledger zero-value still works (backward compat) ────

func TestLedger_ZeroValueOpenFinding_StillWorks(t *testing.T) {
	// The zero-value Ledger is still usable (uses time.Now internally).
	var l cl.Ledger
	f := l.OpenFindingOrPanic("zv-f1", "zero value compat")
	if f.State != cl.StateOpen {
		t.Errorf("expected open, got %q", f.State)
	}
}

// ── Regression: New() constructor works ─────────────────────────────────────

func TestNew_ReturnsUsableLedger(t *testing.T) {
	l := cl.New()
	f, err := l.OpenFinding("new-f1", "new constructor test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	if f.State != cl.StateOpen {
		t.Errorf("expected open, got %q", f.State)
	}
}

// ── Regression: RecordDismissal signature change doesn't break existing tests ─

func TestRecordDismissal_ValidInputSucceeds(t *testing.T) {
	l := cl.New()
	_, err := l.OpenFinding("dism-compat-f1", "compat test")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	d := cl.Dismissal{
		FindingID:   "dism-compat-f1",
		DismissedBy: "alice",
		Reason:      "false positive",
		DismissedAt: time.Now().UTC(),
	}
	if err := l.RecordDismissal(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
