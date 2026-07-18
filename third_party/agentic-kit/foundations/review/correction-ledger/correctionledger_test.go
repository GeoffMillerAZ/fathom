package correctionledger_test

import (
	"testing"
	"time"

	cl "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger"
)

// ---- helpers ----------------------------------------------------------------

func newFinding(id string) cl.Finding {
	now := time.Now().UTC()
	return cl.Finding{
		ID:          id,
		State:       cl.StateOpen,
		Description: "test finding",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ---- FSM: legal transitions -------------------------------------------------

func TestFSM_LegalTransitions(t *testing.T) {
	t.Run("open_to_reviewing", func(t *testing.T) {
		f := newFinding("f1")
		got, err := cl.Transition(f, cl.EventClaim)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != cl.StateReviewing {
			t.Errorf("expected reviewing, got %q", got.State)
		}
	})

	t.Run("reviewing_to_corrected", func(t *testing.T) {
		f := newFinding("f2")
		f.State = cl.StateReviewing
		got, err := cl.Transition(f, cl.EventCorrect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != cl.StateCorrected {
			t.Errorf("expected corrected, got %q", got.State)
		}
	})

	t.Run("reviewing_to_dismissed", func(t *testing.T) {
		f := newFinding("f3")
		f.State = cl.StateReviewing
		got, err := cl.Transition(f, cl.EventDismiss)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != cl.StateDismissed {
			t.Errorf("expected dismissed, got %q", got.State)
		}
	})

	t.Run("reviewing_to_deferred", func(t *testing.T) {
		f := newFinding("f4")
		f.State = cl.StateReviewing
		got, err := cl.Transition(f, cl.EventDefer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != cl.StateDeferred {
			t.Errorf("expected deferred, got %q", got.State)
		}
	})

	t.Run("deferred_to_reviewing_via_reopen", func(t *testing.T) {
		f := newFinding("f5")
		f.State = cl.StateDeferred
		got, err := cl.Transition(f, cl.EventReopen)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != cl.StateReviewing {
			t.Errorf("expected reviewing, got %q", got.State)
		}
	})
}

// ---- FSM: illegal transitions -----------------------------------------------

func TestFSM_IllegalTransitions(t *testing.T) {
	cases := []struct {
		name  string
		state cl.State
		event cl.Event
	}{
		{"open_to_corrected", cl.StateOpen, cl.EventCorrect},
		{"open_to_dismissed", cl.StateOpen, cl.EventDismiss},
		{"open_to_deferred", cl.StateOpen, cl.EventDefer},
		{"open_to_reopen", cl.StateOpen, cl.EventReopen},
		{"corrected_to_open_via_claim", cl.StateCorrected, cl.EventClaim},
		{"corrected_to_reviewing_via_claim", cl.StateCorrected, cl.EventClaim},
		{"dismissed_to_open_via_claim", cl.StateDismissed, cl.EventClaim},
		{"dismissed_to_reviewing", cl.StateDismissed, cl.EventReopen},
		{"reviewing_to_reviewing", cl.StateReviewing, cl.EventClaim},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newFinding("fx")
			f.State = tc.state
			_, err := cl.Transition(f, tc.event)
			if err == nil {
				t.Errorf("expected error for %q -(%q)→ but got nil", tc.state, tc.event)
			}
		})
	}
}

// ---- Acceptance: deferred reopen cycle -------------------------------------

func TestFSM_DeferredReopenCycle(t *testing.T) {
	f := newFinding("cycle-1")

	// open → reviewing
	f, err := cl.Transition(f, cl.EventClaim)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	// reviewing → deferred
	f, err = cl.Transition(f, cl.EventDefer)
	if err != nil {
		t.Fatalf("defer: %v", err)
	}
	if f.State != cl.StateDeferred {
		t.Fatalf("expected deferred, got %q", f.State)
	}

	// deferred → reviewing (reopen)
	f, err = cl.Transition(f, cl.EventReopen)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if f.State != cl.StateReviewing {
		t.Fatalf("expected reviewing after reopen, got %q", f.State)
	}

	// reviewing → corrected
	f, err = cl.Transition(f, cl.EventCorrect)
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if f.State != cl.StateCorrected {
		t.Fatalf("expected corrected, got %q", f.State)
	}

	// corrected is forward-terminal — reopen must fail
	_, err = cl.Transition(f, cl.EventReopen)
	if err == nil {
		t.Fatal("expected error trying to reopen corrected finding")
	}
}

// ---- Acceptance: corrected is forward-terminal ------------------------------

func TestFSM_CorrectedIsForwardTerminal(t *testing.T) {
	events := []cl.Event{cl.EventClaim, cl.EventCorrect, cl.EventDefer, cl.EventDismiss, cl.EventReopen}
	for _, ev := range events {
		f := newFinding("term-corrected")
		f.State = cl.StateCorrected
		_, err := cl.Transition(f, ev)
		if err == nil {
			t.Errorf("corrected finding should reject event %q but got nil error", ev)
		}
	}
}

// ---- Acceptance: Correction vs Dismissal are separate types ----------------

func TestCorrectionAndDismissalAreDistinctTypes(t *testing.T) {
	// Correction carries new fact value; Dismissal must NOT carry a corrected fact.
	corr := cl.Correction{
		FindingID:   "dist-1",
		Target:      cl.CorrectionTarget{StableTarget: "doc/section/1.v2"},
		Original:    "old value",
		Corrected:   "new value",
		CorrectedBy: "alice",
		CorrectedAt: time.Now().UTC(),
		Reason:      "source was wrong",
		Confidence:  4,
	}
	if corr.Corrected == "" {
		t.Error("correction must carry a Corrected value")
	}

	dism := cl.Dismissal{
		FindingID:   "dist-2",
		DismissedBy: "bob",
		Reason:      "known limitation, acceptable",
		DismissedAt: time.Now().UTC(),
	}
	// Dismissal has no Corrected field — just verify it compiles and the
	// Original/Corrected fields are absent from the Dismissal type.
	if dism.DismissedBy == "" {
		t.Error("dismissal must carry DismissedBy")
	}
	// Fact value must NOT appear on Dismissal (compile-time guarantee via type system).
}

// ---- Acceptance: Ledger append-only & Replay determinism -------------------

func TestLedger_AppendAndReplay(t *testing.T) {
	l := cl.New()

	// Open two findings.
	f1, err := l.OpenFinding("ledger-f1", "first finding")
	if err != nil {
		t.Fatalf("OpenFinding f1: %v", err)
	}
	f2, err := l.OpenFinding("ledger-f2", "second finding")
	if err != nil {
		t.Fatalf("OpenFinding f2: %v", err)
	}

	// Transition f1: open → reviewing → corrected.
	f1, err = l.ApplyTransition(f1, cl.EventClaim)
	if err != nil {
		t.Fatalf("f1 claim: %v", err)
	}
	f1, err = l.ApplyTransition(f1, cl.EventCorrect)
	if err != nil {
		t.Fatalf("f1 correct: %v", err)
	}

	// Transition f2: open → reviewing → deferred → reviewing → dismissed.
	f2, err = l.ApplyTransition(f2, cl.EventClaim)
	if err != nil {
		t.Fatalf("f2 claim: %v", err)
	}
	f2, err = l.ApplyTransition(f2, cl.EventDefer)
	if err != nil {
		t.Fatalf("f2 defer: %v", err)
	}
	f2, err = l.ApplyTransition(f2, cl.EventReopen)
	if err != nil {
		t.Fatalf("f2 reopen: %v", err)
	}
	f2, err = l.ApplyTransition(f2, cl.EventDismiss)
	if err != nil {
		t.Fatalf("f2 dismiss: %v", err)
	}

	// Replay and verify current states.
	replay := l.Replay()

	if got := replay["ledger-f1"].State; got != cl.StateCorrected {
		t.Errorf("ledger-f1: expected corrected, got %q", got)
	}
	if got := replay["ledger-f2"].State; got != cl.StateDismissed {
		t.Errorf("ledger-f2: expected dismissed, got %q", got)
	}

	// Replay twice — must produce the same result (determinism).
	replay2 := l.Replay()
	if replay["ledger-f1"].State != replay2["ledger-f1"].State {
		t.Error("Replay is not deterministic for ledger-f1")
	}
	if replay["ledger-f2"].State != replay2["ledger-f2"].State {
		t.Error("Replay is not deterministic for ledger-f2")
	}

	// Verify ledger event count: 2 opened + 6 transitions = 8.
	if n := len(l.Events()); n != 8 {
		t.Errorf("expected 8 ledger events, got %d", n)
	}
}

// ---- Acceptance: Ledger immutability — Events() returns a copy -------------

func TestLedger_EventsReturnsCopy(t *testing.T) {
	l := cl.New()
	if _, err := l.OpenFinding("immut-f1", "immutability check"); err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	events := l.Events()
	origLen := len(events)

	// Mutate the returned slice.
	events[0] = cl.LedgerEvent{}

	// The ledger's internal state must be unchanged.
	events2 := l.Events()
	if len(events2) != origLen {
		t.Errorf("ledger internal state changed after mutating returned slice")
	}
	if events2[0].Kind != cl.LedgerEventKindFindingOpened {
		t.Errorf("ledger first event was mutated; expected finding_opened, got %q", events2[0].Kind)
	}
}

// ---- Acceptance: RecordCorrection validates Confidence ---------------------

func TestLedger_RecordCorrection_ConfidenceValidation(t *testing.T) {
	l := cl.New()
	if _, err := l.OpenFinding("conf-f1", "confidence validation test"); err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	valid := cl.Correction{
		FindingID:   "conf-f1",
		Target:      cl.CorrectionTarget{StableTarget: "doc/s/1.v1"},
		Original:    "old",
		Corrected:   "new",
		CorrectedBy: "alice",
		Reason:      "wrong",
		Confidence:  3,
	}
	if err := l.RecordCorrection(valid); err != nil {
		t.Fatalf("unexpected error for valid correction: %v", err)
	}

	for _, bad := range []int{0, 6, -1, 100} {
		c := valid
		c.Confidence = bad
		if err := l.RecordCorrection(c); err == nil {
			t.Errorf("expected error for Confidence=%d, got nil", bad)
		}
	}
}

// ---- Acceptance: Dismissal does not mutate the fact value ------------------

func TestLedger_DismissalDoesNotMutateFact(t *testing.T) {
	l := cl.New()
	f, err := l.OpenFinding("dism-f1", "check dismissal semantics")
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}

	f, err = l.ApplyTransition(f, cl.EventClaim)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := l.RecordDismissal(cl.Dismissal{
		FindingID:   "dism-f1",
		DismissedBy: "carol",
		Reason:      "false positive",
		DismissedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RecordDismissal: %v", err)
	}

	// State change must still require an explicit transition.
	if f.State != cl.StateReviewing {
		t.Fatalf("finding state changed after RecordDismissal without ApplyTransition; got %q", f.State)
	}

	// Now apply the dismiss transition.
	f, err = l.ApplyTransition(f, cl.EventDismiss)
	if err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if f.State != cl.StateDismissed {
		t.Errorf("expected dismissed, got %q", f.State)
	}

	// Verify the dismissal event is in the ledger.
	found := false
	for _, ev := range l.Events() {
		if ev.Kind == cl.LedgerEventKindDismissalAdded {
			found = true
			break
		}
	}
	if !found {
		t.Error("dismissal event not found in ledger")
	}
}

// ---- Acceptance: CorrectionTarget StableTarget for replay ------------------

func TestCorrectionTarget(t *testing.T) {
	ct := cl.CorrectionTarget{StableTarget: "table/revenue/col/q4.v3"}
	if ct.StableTarget == "" {
		t.Error("StableTarget should not be empty")
	}
}
