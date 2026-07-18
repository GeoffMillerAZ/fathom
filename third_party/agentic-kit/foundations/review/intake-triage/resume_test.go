package intaketriage_test

import (
	"errors"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/review/intake-triage"
)

// ─── ResumeFSM: hydration at every state ─────────────────────────────────────

func TestResumeFSMEveryState(t *testing.T) {
	tests := []struct {
		name       string
		state      intaketriage.ProposalState
		reason     string
		wantReason string
	}{
		{"proposed", intaketriage.StateProposed, "", ""},
		{"imported (terminal)", intaketriage.StateImported, "", ""},
		{"excluded (terminal, reason required)", intaketriage.StateExcluded, "privileged communication", "privileged communication"},
		{"deferred", intaketriage.StateDeferred, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm, err := intaketriage.ResumeFSM("prop-001", tt.state, tt.reason, fixedClock())
			if err != nil {
				t.Fatalf("ResumeFSM(%q): %v", tt.state, err)
			}
			if got := fsm.State(); got != tt.state {
				t.Errorf("State() = %q, want %q", got, tt.state)
			}
			if got := fsm.ExclusionReason(); got != tt.wantReason {
				t.Errorf("ExclusionReason() = %q, want %q", got, tt.wantReason)
			}
			// History-since-resume: no replayed transitions.
			if h := fsm.History(); h != nil {
				t.Errorf("History() = %v, want empty (history-since-resume)", h)
			}
		})
	}
}

// ─── ResumeFSM: argument validation ──────────────────────────────────────────

func TestResumeFSMValidation(t *testing.T) {
	tests := []struct {
		name       string
		proposalID string
		state      intaketriage.ProposalState
		reason     string
		nilClock   bool
		wantErr    error // nil = any non-nil error accepted
	}{
		{name: "invalid state", proposalID: "p", state: intaketriage.ProposalState("bogus")},
		{name: "empty state", proposalID: "p", state: intaketriage.ProposalState("")},
		{name: "excluded without reason", proposalID: "p", state: intaketriage.StateExcluded, wantErr: intaketriage.ErrExclusionReasonRequired},
		{name: "reason with non-excluded state", proposalID: "p", state: intaketriage.StateProposed, reason: "stray"},
		{name: "empty proposalID", proposalID: "", state: intaketriage.StateProposed},
		{name: "nil clock", proposalID: "p", state: intaketriage.StateProposed, nilClock: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := fixedClock()
			if tt.nilClock {
				clock = nil
			}
			fsm, err := intaketriage.ResumeFSM(tt.proposalID, tt.state, tt.reason, clock)
			if err == nil {
				t.Fatalf("ResumeFSM = %v, want error", fsm)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, tt.wantErr)
			}
		})
	}
}

// ─── ResumeFSM: terminal hydration is for read/guard use ────────────────────

func TestResumeFSMDecideAfterTerminalHydration(t *testing.T) {
	tests := []struct {
		name   string
		state  intaketriage.ProposalState
		reason string
	}{
		{"imported", intaketriage.StateImported, ""},
		{"excluded", intaketriage.StateExcluded, "not relevant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm, err := intaketriage.ResumeFSM("prop-001", tt.state, tt.reason, fixedClock())
			if err != nil {
				t.Fatalf("ResumeFSM: %v", err)
			}
			// Decide on a hydrated terminal returns the existing transition
			// error, exactly as on a live FSM.
			if err := fsm.Decide(intaketriage.StateDeferred, "", humanActor); err == nil {
				t.Fatal("Decide on hydrated terminal state should return the transition error")
			}
			if got := fsm.State(); got != tt.state {
				t.Errorf("state changed after rejected Decide: %q, want %q", got, tt.state)
			}
		})
	}
}

func TestResumeFSMDeferredCanDecide(t *testing.T) {
	fsm, err := intaketriage.ResumeFSM("prop-001", intaketriage.StateDeferred, "", fixedClock())
	if err != nil {
		t.Fatalf("ResumeFSM: %v", err)
	}
	if err := fsm.Decide(intaketriage.StateImported, "", humanActor); err != nil {
		t.Fatalf("hydrated deferred → imported: %v", err)
	}
	if fsm.State() != intaketriage.StateImported {
		t.Fatalf("state = %q, want imported", fsm.State())
	}
	// History-since-resume: exactly the one post-resume transition.
	h := fsm.History()
	if len(h) != 1 {
		t.Fatalf("History() has %d events, want 1 (history-since-resume)", len(h))
	}
	if h[0].From != intaketriage.StateDeferred || h[0].To != intaketriage.StateImported {
		t.Errorf("event = %+v, want deferred→imported", h[0])
	}
}

func TestResumeFSMExcludedCanRestore(t *testing.T) {
	fsm, err := intaketriage.ResumeFSM("prop-001", intaketriage.StateExcluded, "duplicate", fixedClock())
	if err != nil {
		t.Fatalf("ResumeFSM: %v", err)
	}
	if err := fsm.Restore(humanActor); err != nil {
		t.Fatalf("Restore on hydrated excluded: %v", err)
	}
	if fsm.State() != intaketriage.StateProposed {
		t.Fatalf("state = %q, want proposed", fsm.State())
	}
	if fsm.ExclusionReason() != "" {
		t.Fatalf("ExclusionReason = %q, want cleared", fsm.ExclusionReason())
	}
}
