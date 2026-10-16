package investigationjournal_test

import (
	"testing"
	"time"

	ij "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
)

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func validEntry() ij.Entry {
	return ij.Entry{
		Step: "hypothesis-formed",
		Note: "Formed initial hypothesis about ownership gap.",
		At:   epoch,
	}
}

// TestAppendHappyPath ensures a well-formed entry is accepted.
func TestAppendHappyPath(t *testing.T) {
	var j ij.Journal
	if err := j.Append(validEntry()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestAppendEmptyStep ensures an entry with an empty Step is rejected.
func TestAppendEmptyStep(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.Step = ""
	if err := j.Append(e); err == nil {
		t.Fatal("expected error for empty Step, got nil")
	}
}

// TestAppendEmptyNote ensures an entry with an empty Note is rejected.
func TestAppendEmptyNote(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.Note = ""
	if err := j.Append(e); err == nil {
		t.Fatal("expected error for empty Note, got nil")
	}
}

// TestEntriesOrder ensures entries come back in append order.
func TestEntriesOrder(t *testing.T) {
	var j ij.Journal
	steps := []string{"step-one", "step-two", "step-three"}
	for _, s := range steps {
		e := validEntry()
		e.Step = s
		if err := j.Append(e); err != nil {
			t.Fatalf("unexpected error appending %s: %v", s, err)
		}
	}
	got := j.Entries()
	if len(got) != len(steps) {
		t.Fatalf("expected %d entries, got %d", len(steps), len(got))
	}
	for i, e := range got {
		if e.Step != steps[i] {
			t.Errorf("entry[%d]: want step %q, got %q", i, steps[i], e.Step)
		}
	}
}

// TestEntriesReturnsCopy ensures the returned slice is independent of the
// journal's internal state.
func TestEntriesReturnsCopy(t *testing.T) {
	var j ij.Journal
	if err := j.Append(validEntry()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := j.Entries()
	// Mutate the returned slice.
	got[0].Step = "mutated"
	// Re-fetch — mutation must not have propagated.
	fresh := j.Entries()
	if fresh[0].Step == "mutated" {
		t.Error("Entries returned a live reference, not a copy")
	}
}

// TestEntriesEmptyJournal ensures nil is returned when no entries exist.
func TestEntriesEmptyJournal(t *testing.T) {
	var j ij.Journal
	if got := j.Entries(); got != nil {
		t.Errorf("expected nil for empty journal, got %v", got)
	}
}
