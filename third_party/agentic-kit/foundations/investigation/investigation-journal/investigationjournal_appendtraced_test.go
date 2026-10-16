package investigationjournal_test

import (
	"testing"
	"time"

	ij "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// ── author fixtures ───────────────────────────────────────────────────────────

func validHumanAuthor() aiprovenance.Author {
	return aiprovenance.NewHuman("alice@example.com")
}

func validAIAuthor() aiprovenance.Author {
	return aiprovenance.NewAIWithOpts(
		"claude-sonnet-4-6",
		"20260608",
		"sha256-promptABC",
		"sha256-inputXYZ",
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
}

// ── Append validation: zero At ────────────────────────────────────────────────

// TestAppendRejectsZeroAt ensures Append rejects an entry with a zero timestamp.
func TestAppendRejectsZeroAt(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.At = time.Time{} // zero value
	if err := j.Append(e); err == nil {
		t.Fatal("expected error for zero At, got nil")
	}
}

// TestAppendAcceptsNonZeroAt ensures a valid non-zero At is still accepted.
func TestAppendAcceptsNonZeroAt(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.At = epoch
	if err := j.Append(e); err != nil {
		t.Fatalf("expected no error for valid At, got %v", err)
	}
}

// ── Append validation: Author when set ───────────────────────────────────────

// TestAppendRejectsInvalidAuthorWhenSet ensures that when Author is set on an
// Entry, Append validates it and rejects a malformed Author.
func TestAppendRejectsInvalidAuthorWhenSet(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	// Author.Kind is empty — invalid discriminant.
	e.Author = &aiprovenance.Author{Kind: ""}
	if err := j.Append(e); err == nil {
		t.Fatal("expected error for invalid Author, got nil")
	}
}

// TestAppendAcceptsValidHumanAuthorWhenSet ensures a valid human Author on a
// base Append call is accepted.
func TestAppendAcceptsValidHumanAuthorWhenSet(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	a := validHumanAuthor()
	e.Author = &a
	if err := j.Append(e); err != nil {
		t.Fatalf("expected no error for valid human Author, got %v", err)
	}
}

// TestAppendNilAuthorAccepted ensures omitting Author (nil pointer) is still
// accepted — backward-compat for system/internal callers that record
// deterministic state without a provenance actor.
func TestAppendNilAuthorAccepted(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.Author = nil
	if err := j.Append(e); err != nil {
		t.Fatalf("expected no error for nil Author, got %v", err)
	}
}

// ── AppendTraced ──────────────────────────────────────────────────────────────

// TestAppendTracedHumanAuthorHappyPath verifies a well-formed human-authored
// traced entry is accepted and stored correctly.
func TestAppendTracedHumanAuthorHappyPath(t *testing.T) {
	var j ij.Journal
	author := validHumanAuthor()
	err := j.AppendTraced(ij.TracedEntry{
		Step:   "evidence-collected",
		Note:   "Collected 3 spans from audit log.",
		Author: author,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Step != "evidence-collected" {
		t.Errorf("step: want %q, got %q", "evidence-collected", entries[0].Step)
	}
	if entries[0].Author == nil {
		t.Error("Author must not be nil on traced entry")
	}
	if entries[0].Author.Kind != aiprovenance.KindHuman {
		t.Errorf("Author.Kind: want %q, got %q", aiprovenance.KindHuman, entries[0].Author.Kind)
	}
}

// TestAppendTracedAIAuthorHappyPath verifies a well-formed AI-authored traced
// entry is accepted and stored correctly.
func TestAppendTracedAIAuthorHappyPath(t *testing.T) {
	var j ij.Journal
	author := validAIAuthor()
	err := j.AppendTraced(ij.TracedEntry{
		Step:   "hypothesis-ranked",
		Note:   "Oracle ranked 3 hypotheses.",
		Author: author,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Author == nil || entries[0].Author.Kind != aiprovenance.KindAI {
		t.Errorf("Author.Kind: want %q, got %v", aiprovenance.KindAI, entries[0].Author)
	}
}

// TestAppendTracedInvalidAuthorRejected verifies that a zero-value / invalid
// Author is rejected by AppendTraced.
func TestAppendTracedInvalidAuthorRejected(t *testing.T) {
	var j ij.Journal
	err := j.AppendTraced(ij.TracedEntry{
		Step:   "evidence-collected",
		Note:   "Some note.",
		Author: aiprovenance.Author{Kind: ""},
	})
	if err == nil {
		t.Fatal("expected error for invalid Author, got nil")
	}
}

// TestAppendTracedEmptyStepRejected verifies that an empty Step is rejected.
func TestAppendTracedEmptyStepRejected(t *testing.T) {
	var j ij.Journal
	err := j.AppendTraced(ij.TracedEntry{
		Step:   "",
		Note:   "Some note.",
		Author: validHumanAuthor(),
	})
	if err == nil {
		t.Fatal("expected error for empty Step, got nil")
	}
}

// TestAppendTracedEmptyNoteRejected verifies that an empty Note is rejected.
func TestAppendTracedEmptyNoteRejected(t *testing.T) {
	var j ij.Journal
	err := j.AppendTraced(ij.TracedEntry{
		Step:   "some-step",
		Note:   "",
		Author: validHumanAuthor(),
	})
	if err == nil {
		t.Fatal("expected error for empty Note, got nil")
	}
}

// TestAppendTracedClockInjection verifies that Journal.Now is called when set,
// making timestamps deterministic in tests.
func TestAppendTracedClockInjection(t *testing.T) {
	fixed := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	j := ij.NewJournalWithClock(func() time.Time { return fixed })
	author := validHumanAuthor()
	if err := j.AppendTraced(ij.TracedEntry{
		Step:   "step-clock-test",
		Note:   "Testing clock injection.",
		Author: author,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !entries[0].At.Equal(fixed) {
		t.Errorf("At: want %v, got %v", fixed, entries[0].At)
	}
}

// TestAppendTracedNilSpanRefsStoredAsNil verifies that when SpanRefs is nil,
// the stored entry's SpanRefs is also nil (no empty-slice coercion).
func TestAppendTracedNilSpanRefsStoredAsNil(t *testing.T) {
	var j ij.Journal
	if err := j.AppendTraced(ij.TracedEntry{
		Step:     "step-nil-refs",
		Note:     "No span refs.",
		Author:   validHumanAuthor(),
		SpanRefs: nil,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if entries[0].SpanRefs != nil {
		t.Errorf("expected nil SpanRefs, got %v", entries[0].SpanRefs)
	}
}

// TestAppendTracedNonNilSpanRefsPropagated verifies that non-nil SpanRefs are
// stored as a defensive copy and values are preserved.
func TestAppendTracedNonNilSpanRefsPropagated(t *testing.T) {
	var j ij.Journal
	refs := []string{"span-001", "span-002"}
	if err := j.AppendTraced(ij.TracedEntry{
		Step:     "step-with-refs",
		Note:     "Has span refs.",
		Author:   validHumanAuthor(),
		SpanRefs: refs,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mutate original — stored copy must not change.
	refs[0] = "mutated"
	entries := j.Entries()
	if entries[0].SpanRefs[0] != "span-001" {
		t.Errorf("SpanRefs[0]: want %q, got %q (defensive copy violated)", "span-001", entries[0].SpanRefs[0])
	}
}

// TestAppendTracedEntryPassesValidate verifies that entries produced by
// AppendTraced satisfy Entry.Validate().
func TestAppendTracedEntryPassesValidate(t *testing.T) {
	var j ij.Journal
	if err := j.AppendTraced(ij.TracedEntry{
		Step:   "validate-test",
		Note:   "Entry should pass Validate.",
		Author: validAIAuthor(),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if err := entries[0].Validate(); err != nil {
		t.Errorf("Entry.Validate() returned error: %v", err)
	}
}

// ── ReplayAsOf ────────────────────────────────────────────────────────────────

// TestReplayAsOfReturnsOnlyEntriesAtOrBefore verifies that ReplayAsOf filters
// entries to those with At <= t.
func TestReplayAsOfReturnsOnlyEntriesAtOrBefore(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(10 * time.Second)
	t3 := t1.Add(20 * time.Second)

	var j ij.Journal
	for _, ts := range []time.Time{t1, t2, t3} {
		e := validEntry()
		e.At = ts
		if err := j.Append(e); err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	got := j.ReplayAsOf(t2)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries at or before t2, got %d", len(got))
	}
	for _, e := range got {
		if e.At.After(t2) {
			t.Errorf("ReplayAsOf returned entry with At=%v which is after cutoff=%v", e.At, t2)
		}
	}
}

// TestReplayAsOfReturnsDefensiveCopy ensures the returned slice is independent
// of the journal's internal state.
func TestReplayAsOfReturnsDefensiveCopy(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.At = epoch
	if err := j.Append(e); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	got := j.ReplayAsOf(epoch.Add(time.Hour))
	if len(got) == 0 {
		t.Fatal("expected entries")
	}
	got[0].Step = "mutated"
	fresh := j.ReplayAsOf(epoch.Add(time.Hour))
	if fresh[0].Step == "mutated" {
		t.Error("ReplayAsOf returned a live reference, not a copy")
	}
}

// TestReplayAsOfEmptyBeforeCutoff verifies nil is returned when no entries
// exist at or before the cutoff.
func TestReplayAsOfEmptyBeforeCutoff(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.At = epoch.Add(time.Hour)
	if err := j.Append(e); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	got := j.ReplayAsOf(epoch) // cutoff is before the single entry
	if got != nil {
		t.Errorf("expected nil for no matching entries, got %v", got)
	}
}

// TestReplayAsOfIncludesBoundary verifies that an entry exactly at the cutoff
// time is included (At == t is included, not excluded).
func TestReplayAsOfIncludesBoundary(t *testing.T) {
	var j ij.Journal
	e := validEntry()
	e.At = epoch
	if err := j.Append(e); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	got := j.ReplayAsOf(epoch)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry at boundary, got %d", len(got))
	}
}
