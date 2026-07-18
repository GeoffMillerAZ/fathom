package intaketriage_test

import (
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/review/intake-triage"
)

// fixedClock returns a clock pinned to a single instant.
func fixedClock() func() time.Time {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time { return t }
}

var (
	humanActor = intaketriage.Actor{Kind: intaketriage.ActorKindHuman, ID: "human:web"}
	aiActor    = intaketriage.Actor{Kind: intaketriage.ActorKindAI, ID: "ai:planner"}
)

func newFSM(t *testing.T) *intaketriage.FSM {
	t.Helper()
	fsm, err := intaketriage.NewFSM("prop-001", fixedClock())
	if err != nil {
		t.Fatalf("NewFSM: %v", err)
	}
	return fsm
}

// ─── constructor ─────────────────────────────────────────────────────────────

func TestNewFSMStartsProposed(t *testing.T) {
	fsm := newFSM(t)
	if got := fsm.State(); got != intaketriage.StateProposed {
		t.Fatalf("initial state = %q, want %q", got, intaketriage.StateProposed)
	}
}

func TestNewFSMRejectsEmptyID(t *testing.T) {
	if _, err := intaketriage.NewFSM("", fixedClock()); err == nil {
		t.Fatal("NewFSM with empty ID should return error")
	}
}

func TestNewFSMRejectsNilClock(t *testing.T) {
	if _, err := intaketriage.NewFSM("p", nil); err == nil {
		t.Fatal("NewFSM with nil clock should return error")
	}
}

// ─── AI actor guard ──────────────────────────────────────────────────────────

func TestDecideRejectsAIActor(t *testing.T) {
	fsm := newFSM(t)
	err := fsm.Decide(intaketriage.StateImported, "", aiActor)
	if !errors.Is(err, intaketriage.ErrAIActorForbidden) {
		t.Fatalf("Decide with AI actor: got %v, want ErrAIActorForbidden", err)
	}
	// State must be unchanged.
	if fsm.State() != intaketriage.StateProposed {
		t.Fatalf("state changed after rejected AI Decide: %q", fsm.State())
	}
}

func TestRestoreRejectsAIActor(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateExcluded, "not relevant", humanActor)
	err := fsm.Restore(aiActor)
	if !errors.Is(err, intaketriage.ErrAIActorForbidden) {
		t.Fatalf("Restore with AI actor: got %v, want ErrAIActorForbidden", err)
	}
}

// ─── valid transitions ────────────────────────────────────────────────────────

func TestDecideProposedToImported(t *testing.T) {
	fsm := newFSM(t)
	if err := fsm.Decide(intaketriage.StateImported, "", humanActor); err != nil {
		t.Fatalf("Decide → imported: %v", err)
	}
	if fsm.State() != intaketriage.StateImported {
		t.Fatalf("state = %q, want imported", fsm.State())
	}
}

func TestDecideProposedToDeferred(t *testing.T) {
	fsm := newFSM(t)
	if err := fsm.Decide(intaketriage.StateDeferred, "", humanActor); err != nil {
		t.Fatalf("Decide → deferred: %v", err)
	}
	if fsm.State() != intaketriage.StateDeferred {
		t.Fatalf("state = %q, want deferred", fsm.State())
	}
}

func TestDecideProposedToExcluded(t *testing.T) {
	fsm := newFSM(t)
	const reason = "privileged communication"
	if err := fsm.Decide(intaketriage.StateExcluded, reason, humanActor); err != nil {
		t.Fatalf("Decide → excluded: %v", err)
	}
	if fsm.State() != intaketriage.StateExcluded {
		t.Fatalf("state = %q, want excluded", fsm.State())
	}
	if fsm.ExclusionReason() != reason {
		t.Fatalf("ExclusionReason = %q, want %q", fsm.ExclusionReason(), reason)
	}
}

func TestDecideDeferredToImported(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateDeferred, "", humanActor)
	if err := fsm.Decide(intaketriage.StateImported, "", humanActor); err != nil {
		t.Fatalf("deferred → imported: %v", err)
	}
	if fsm.State() != intaketriage.StateImported {
		t.Fatalf("state = %q, want imported", fsm.State())
	}
}

// ─── exclusion reason guard ──────────────────────────────────────────────────

func TestDecideExcludedRequiresReason(t *testing.T) {
	fsm := newFSM(t)
	err := fsm.Decide(intaketriage.StateExcluded, "", humanActor)
	if !errors.Is(err, intaketriage.ErrExclusionReasonRequired) {
		t.Fatalf("Decide excluded without reason: got %v, want ErrExclusionReasonRequired", err)
	}
}

// ─── reversible exclusion ─────────────────────────────────────────────────────

func TestRestoreExcludedToProposed(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateExcluded, "reason", humanActor)
	if err := fsm.Restore(humanActor); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if fsm.State() != intaketriage.StateProposed {
		t.Fatalf("state after Restore = %q, want proposed", fsm.State())
	}
	if fsm.ExclusionReason() != "" {
		t.Fatalf("ExclusionReason should be cleared after Restore, got %q", fsm.ExclusionReason())
	}
}

func TestRestoreOnNonExcludedErrors(t *testing.T) {
	fsm := newFSM(t)
	// proposed → Restore should fail
	if err := fsm.Restore(humanActor); !errors.Is(err, intaketriage.ErrNotExcluded) {
		t.Fatalf("Restore on proposed: got %v, want ErrNotExcluded", err)
	}
	// imported → Restore should fail
	_ = fsm.Decide(intaketriage.StateImported, "", humanActor)
	if err := fsm.Restore(humanActor); !errors.Is(err, intaketriage.ErrNotExcluded) {
		t.Fatalf("Restore on imported: got %v, want ErrNotExcluded", err)
	}
}

func TestRestoreThenReExclude(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateExcluded, "first", humanActor)
	_ = fsm.Restore(humanActor)
	if err := fsm.Decide(intaketriage.StateExcluded, "second", humanActor); err != nil {
		t.Fatalf("re-exclude after restore: %v", err)
	}
	if fsm.ExclusionReason() != "second" {
		t.Fatalf("ExclusionReason = %q, want %q", fsm.ExclusionReason(), "second")
	}
}

// ─── terminal-state guards ───────────────────────────────────────────────────

func TestDecideFromImportedIsTerminal(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateImported, "", humanActor)
	if err := fsm.Decide(intaketriage.StateDeferred, "", humanActor); err == nil {
		t.Fatal("Decide from imported should return error (terminal)")
	}
}

func TestDecideFromExcludedRequiresRestoreFirst(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateExcluded, "reason", humanActor)
	if err := fsm.Decide(intaketriage.StateImported, "", humanActor); err == nil {
		t.Fatal("Decide from excluded should require Restore first")
	}
}

// ─── AI suggestions ──────────────────────────────────────────────────────────

func TestSuggestRecordedButDoesNotChangeState(t *testing.T) {
	fsm := newFSM(t)
	err := fsm.Suggest(intaketriage.StateImported, "looks relevant", 0.8, aiActor)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	// State unchanged.
	if fsm.State() != intaketriage.StateProposed {
		t.Fatalf("state changed after Suggest: %q", fsm.State())
	}
	sgs := fsm.Suggestions()
	if len(sgs) != 1 {
		t.Fatalf("Suggestions() = %d, want 1", len(sgs))
	}
	if sgs[0].ProposedState != intaketriage.StateImported {
		t.Fatalf("suggestion ProposedState = %q, want imported", sgs[0].ProposedState)
	}
	if sgs[0].Confidence != 0.8 {
		t.Fatalf("suggestion Confidence = %v, want 0.8", sgs[0].Confidence)
	}
}

func TestSuggestRejectsHumanActor(t *testing.T) {
	fsm := newFSM(t)
	if err := fsm.Suggest(intaketriage.StateImported, "reason", 0.5, humanActor); err == nil {
		t.Fatal("Suggest with human actor should return error")
	}
}

// ─── append-only history ─────────────────────────────────────────────────────

func TestHistoryAppendsOnlyGrowsCorrectly(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateExcluded, "reason", humanActor)
	_ = fsm.Restore(humanActor)
	_ = fsm.Decide(intaketriage.StateImported, "", humanActor)

	hist := fsm.History()
	if len(hist) != 3 {
		t.Fatalf("History() = %d events, want 3", len(hist))
	}
	if hist[0].To != intaketriage.StateExcluded {
		t.Errorf("history[0].To = %q, want excluded", hist[0].To)
	}
	if hist[1].To != intaketriage.StateProposed {
		t.Errorf("history[1].To = %q, want proposed (restore)", hist[1].To)
	}
	if hist[2].To != intaketriage.StateImported {
		t.Errorf("history[2].To = %q, want imported", hist[2].To)
	}
}

func TestHistoryDefensiveCopy(t *testing.T) {
	fsm := newFSM(t)
	_ = fsm.Decide(intaketriage.StateImported, "", humanActor)
	h1 := fsm.History()
	h1[0].To = intaketriage.StateExcluded // mutate returned copy
	h2 := fsm.History()
	if h2[0].To != intaketriage.StateImported {
		t.Fatal("History() is not a defensive copy: mutation of returned slice affected FSM")
	}
}

// ─── actor validation ────────────────────────────────────────────────────────

func TestDecideRejectsInvalidActor(t *testing.T) {
	fsm := newFSM(t)
	bad := intaketriage.Actor{Kind: "unknown", ID: "x"}
	if err := fsm.Decide(intaketriage.StateImported, "", bad); err == nil {
		t.Fatal("Decide with invalid actor kind should return error")
	}
	bad2 := intaketriage.Actor{Kind: intaketriage.ActorKindHuman, ID: ""}
	if err := fsm.Decide(intaketriage.StateImported, "", bad2); err == nil {
		t.Fatal("Decide with empty actor ID should return error")
	}
}
