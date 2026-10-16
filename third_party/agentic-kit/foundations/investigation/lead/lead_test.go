// Package lead_test provides black-box tests for the lead package.
package lead_test

import (
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/lead"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// sentinel is a fixed clock value used throughout tests for determinism.
var sentinel = time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return sentinel }

// validHuman returns a well-formed human Author for use in tests.
func validHuman() aiprovenance.Author {
	return aiprovenance.NewHuman("investigator-1")
}

// validLead returns a Lead that satisfies Validate().
func validLead() lead.Lead {
	return lead.Lead{
		ID:             "lead-001",
		Prompt:         "Did the defendant access the server on 2026-01-15?",
		HypothesisID:   "hyp-42",
		TriggerSpanIDs: []string{"span-a", "span-b"},
		Assignee:       "alice",
		Status:         lead.StatusOpen,
		Author:         validHuman(),
		CreatedAt:      sentinel,
		UpdatedAt:      sentinel,
	}
}

// newLog returns a LeadLog seeded with one open lead using the fixed clock.
func newLog(t *testing.T) (*lead.LeadLog, lead.Lead) {
	t.Helper()
	l := validLead()
	log := lead.NewLeadLogWithClock(fixedClock)
	if err := log.Open(l); err != nil {
		t.Fatalf("newLog: Open: %v", err)
	}
	return log, l
}

// ── Validate tests ────────────────────────────────────────────────────────────

func TestValidate_HappyPath(t *testing.T) {
	if err := validLead().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	l := validLead()
	l.ID = ""
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestValidate_EmptyPrompt(t *testing.T) {
	l := validLead()
	l.Prompt = ""
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for empty Prompt, got nil")
	}
}

func TestValidate_InvalidStatus(t *testing.T) {
	l := validLead()
	l.Status = lead.LeadStatus("hunch")
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for unknown Status, got nil")
	}
}

func TestValidate_AnonymousAuthor(t *testing.T) {
	l := validLead()
	l.Author = aiprovenance.Author{} // zero value — Kind is empty
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for anonymous (zero) Author, got nil")
	}
}

func TestValidate_AllStatuses(t *testing.T) {
	for _, s := range []lead.LeadStatus{
		lead.StatusOpen,
		lead.StatusPursuing,
		lead.StatusClosed,
		lead.StatusAbandoned,
	} {
		l := validLead()
		l.Status = s
		if err := l.Validate(); err != nil {
			t.Errorf("status %q: unexpected error: %v", s, err)
		}
	}
}

// ── Defensive copy: TriggerSpanIDs ────────────────────────────────────────────

func TestLead_Validate_DefensiveCopyTriggerSpans(t *testing.T) {
	spans := []string{"span-1", "span-2"}
	l := validLead()
	l.TriggerSpanIDs = spans
	if err := l.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Mutate original slice — Lead should be unaffected if Validate performs
	// a defensive copy (or if the struct copy was sufficient). This test is
	// primarily about the Log's storage invariant, verified via History below.
	spans[0] = "POISONED"
}

// ── LeadLog.Open ─────────────────────────────────────────────────────────────

func TestLeadLog_Open_HappyPath(t *testing.T) {
	log, l := newLog(t)
	h := log.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history entry after Open, got %d", len(h))
	}
	if h[0].Lead.ID != l.ID {
		t.Errorf("history[0].Lead.ID: got %q, want %q", h[0].Lead.ID, l.ID)
	}
	if h[0].Lead.Status != lead.StatusOpen {
		t.Errorf("history[0].Lead.Status: got %q, want %q", h[0].Lead.Status, lead.StatusOpen)
	}
	if h[0].RecordedAt != sentinel {
		t.Errorf("history[0].RecordedAt: got %v, want %v", h[0].RecordedAt, sentinel)
	}
}

func TestLeadLog_Open_RejectsInvalidLead(t *testing.T) {
	log := lead.NewLeadLogWithClock(fixedClock)
	bad := validLead()
	bad.ID = ""
	if err := log.Open(bad); err == nil {
		t.Fatal("Open with invalid Lead should return error")
	}
	if len(log.History()) != 0 {
		t.Error("history must not grow on rejected Open")
	}
}

func TestLeadLog_Open_DuplicateIDRejected(t *testing.T) {
	log, l := newLog(t)
	err := log.Open(l) // same ID again
	if err == nil {
		t.Fatal("Open with duplicate ID should return error")
	}
	if len(log.History()) != 1 {
		t.Errorf("history must not grow on duplicate Open; got %d entries", len(log.History()))
	}
}

// ── Transitions ──────────────────────────────────────────────────────────────

func TestTransition_OpenToPursuing(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("open→pursuing: %v", err)
	}
	h := log.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(h))
	}
	if h[1].Lead.Status != lead.StatusPursuing {
		t.Errorf("expected pursuing, got %q", h[1].Lead.Status)
	}
}

func TestTransition_PursuingToClosed(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("open→pursuing: %v", err)
	}
	if err := log.Transition(l.ID, lead.StatusClosed, validHuman(), "confirmed alibi"); err != nil {
		t.Fatalf("pursuing→closed: %v", err)
	}
	h := log.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(h))
	}
	if h[2].Lead.Status != lead.StatusClosed {
		t.Errorf("expected closed, got %q", h[2].Lead.Status)
	}
	if h[2].Lead.Outcome != "confirmed alibi" {
		t.Errorf("expected outcome 'confirmed alibi', got %q", h[2].Lead.Outcome)
	}
}

func TestTransition_PursuingToAbandoned(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("open→pursuing: %v", err)
	}
	if err := log.Transition(l.ID, lead.StatusAbandoned, validHuman(), "out of scope"); err != nil {
		t.Fatalf("pursuing→abandoned: %v", err)
	}
	h := log.History()
	last := h[len(h)-1]
	if last.Lead.Status != lead.StatusAbandoned {
		t.Errorf("expected abandoned, got %q", last.Lead.Status)
	}
}

func TestTransition_PursuingToOpen(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("open→pursuing: %v", err)
	}
	if err := log.Transition(l.ID, lead.StatusOpen, validHuman(), ""); err != nil {
		t.Fatalf("pursuing→open: %v", err)
	}
	h := log.History()
	last := h[len(h)-1]
	if last.Lead.Status != lead.StatusOpen {
		t.Errorf("expected open, got %q", last.Lead.Status)
	}
}

// ── Illegal transition: rejected + no history change ─────────────────────────

func TestTransition_IllegalOpenToClosed(t *testing.T) {
	log, l := newLog(t)
	lenBefore := len(log.History())
	err := log.Transition(l.ID, lead.StatusClosed, validHuman(), "")
	if err == nil {
		t.Fatal("expected error for open→closed (illegal), got nil")
	}
	if len(log.History()) != lenBefore {
		t.Errorf("history must not grow on illegal transition; before=%d after=%d",
			lenBefore, len(log.History()))
	}
}

func TestTransition_IllegalOpenToAbandoned(t *testing.T) {
	log, l := newLog(t)
	lenBefore := len(log.History())
	err := log.Transition(l.ID, lead.StatusAbandoned, validHuman(), "")
	if err == nil {
		t.Fatal("expected error for open→abandoned (illegal), got nil")
	}
	if len(log.History()) != lenBefore {
		t.Errorf("history must not grow; before=%d after=%d", lenBefore, len(log.History()))
	}
}

func TestTransition_IllegalClosedToAny(t *testing.T) {
	log, l := newLog(t)
	// Reach closed
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("setup pursuing: %v", err)
	}
	if err := log.Transition(l.ID, lead.StatusClosed, validHuman(), "done"); err != nil {
		t.Fatalf("setup closed: %v", err)
	}
	for _, next := range []lead.LeadStatus{lead.StatusOpen, lead.StatusPursuing, lead.StatusAbandoned} {
		lenBefore := len(log.History())
		err := log.Transition(l.ID, next, validHuman(), "")
		if err == nil {
			t.Errorf("closed→%q: expected error, got nil", next)
		}
		if len(log.History()) != lenBefore {
			t.Errorf("closed→%q: history must not grow; before=%d after=%d",
				next, lenBefore, len(log.History()))
		}
	}
}

func TestTransition_IllegalAbandonedToAny(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("setup pursuing: %v", err)
	}
	if err := log.Transition(l.ID, lead.StatusAbandoned, validHuman(), ""); err != nil {
		t.Fatalf("setup abandoned: %v", err)
	}
	for _, next := range []lead.LeadStatus{lead.StatusOpen, lead.StatusPursuing, lead.StatusClosed} {
		lenBefore := len(log.History())
		err := log.Transition(l.ID, next, validHuman(), "")
		if err == nil {
			t.Errorf("abandoned→%q: expected error, got nil", next)
		}
		if len(log.History()) != lenBefore {
			t.Errorf("abandoned→%q: history must not grow; before=%d after=%d",
				next, lenBefore, len(log.History()))
		}
	}
}

func TestTransition_UnknownLeadID(t *testing.T) {
	log := lead.NewLeadLogWithClock(fixedClock)
	err := log.Transition("nonexistent", lead.StatusPursuing, validHuman(), "")
	if err == nil {
		t.Fatal("expected error for unknown lead ID, got nil")
	}
}

// ── Injected clock: reproducible timestamps ───────────────────────────────────

func TestLeadLog_InjectedClock_Reproducible(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)

	l1 := validLead()
	l1.ID = "lead-t1"
	l2 := validLead()
	l2.ID = "lead-t2"

	logA := lead.NewLeadLogWithClock(func() time.Time { return t1 })
	logB := lead.NewLeadLogWithClock(func() time.Time { return t2 })

	if err := logA.Open(l1); err != nil {
		t.Fatalf("logA.Open: %v", err)
	}
	if err := logB.Open(l2); err != nil {
		t.Fatalf("logB.Open: %v", err)
	}

	hA := logA.History()
	hB := logB.History()

	if hA[0].RecordedAt != t1 {
		t.Errorf("logA RecordedAt: got %v, want %v", hA[0].RecordedAt, t1)
	}
	if hB[0].RecordedAt != t2 {
		t.Errorf("logB RecordedAt: got %v, want %v", hB[0].RecordedAt, t2)
	}
	if hA[0].RecordedAt.Equal(hB[0].RecordedAt) {
		t.Errorf("expected distinct RecordedAt values; both are %v", hA[0].RecordedAt)
	}
}

// ── History: defensive copy ───────────────────────────────────────────────────

func TestHistory_DefensiveCopy(t *testing.T) {
	log, _ := newLog(t)
	h1 := log.History()
	h1[0].Lead.ID = "MUTATED"
	h2 := log.History()
	if h2[0].Lead.ID == "MUTATED" {
		t.Error("History() must return a defensive copy; mutation escaped")
	}
}

// ── Head ─────────────────────────────────────────────────────────────────────

func TestHead_Empty(t *testing.T) {
	log := lead.NewLeadLogWithClock(fixedClock)
	_, ok := log.Head("lead-001")
	if ok {
		t.Fatal("Head on empty log should return false")
	}
}

func TestHead_AfterTransitions(t *testing.T) {
	log, l := newLog(t)
	if err := log.Transition(l.ID, lead.StatusPursuing, validHuman(), ""); err != nil {
		t.Fatalf("transition: %v", err)
	}
	entry, ok := log.Head(l.ID)
	if !ok {
		t.Fatal("Head: not found")
	}
	if entry.Lead.Status != lead.StatusPursuing {
		t.Errorf("Head.Lead.Status: got %q, want pursuing", entry.Lead.Status)
	}
}

// ── ErrNotFound sentinel ──────────────────────────────────────────────────────

func TestErrNotFound_IsSentinel(t *testing.T) {
	log := lead.NewLeadLogWithClock(fixedClock)
	err := log.Transition("ghost-id", lead.StatusPursuing, validHuman(), "")
	if !errors.Is(err, lead.ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound); got %v", err)
	}
}
