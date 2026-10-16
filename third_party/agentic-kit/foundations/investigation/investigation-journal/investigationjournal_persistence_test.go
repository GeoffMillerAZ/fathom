package investigationjournal_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	ij "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
)

// entryEquivalent compares entries field by field, using time.Time.Equal for
// timestamps so JSON round-trips (which drop monotonic clock readings and
// re-derive locations) compare correctly.
func entryEquivalent(a, b ij.Entry) bool {
	if a.Step != b.Step || a.Note != b.Note || !a.At.Equal(b.At) {
		return false
	}
	if !slices.Equal(a.SpanRefs, b.SpanRefs) {
		return false
	}
	if (a.Author == nil) != (b.Author == nil) {
		return false
	}
	if a.Author == nil {
		return true
	}
	x, y := *a.Author, *b.Author
	if x.Kind != y.Kind {
		return false
	}
	if (x.Human == nil) != (y.Human == nil) || (x.AI == nil) != (y.AI == nil) {
		return false
	}
	if x.Human != nil && *x.Human != *y.Human {
		return false
	}
	if x.AI != nil {
		if !x.AI.At.Equal(y.AI.At) {
			return false
		}
		ax, ay := *x.AI, *y.AI
		ax.At, ay.At = time.Time{}, time.Time{}
		if ax != ay {
			return false
		}
	}
	return true
}

// ── AttachSink behavior ───────────────────────────────────────────────────────

// TestAttachSinkReceivesEntriesInOrder verifies the sink observes every
// successful Append and AppendTraced, in journal order, with full content.
func TestAttachSinkReceivesEntriesInOrder(t *testing.T) {
	j := ij.NewJournalWithClock(func() time.Time { return epoch })
	var seen []ij.Entry
	j.AttachSink(func(e ij.Entry) error {
		seen = append(seen, e)
		return nil
	})

	plain := validEntry()
	if err := j.Append(plain); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := j.AppendTraced(ij.TracedEntry{
		Step:     "evidence-collected",
		Note:     "Collected spans.",
		Author:   validHumanAuthor(),
		SpanRefs: []string{"span-001"},
	}); err != nil {
		t.Fatalf("AppendTraced: %v", err)
	}

	want := j.Entries()
	if len(seen) != 2 || len(want) != 2 {
		t.Fatalf("expected sink and journal to both hold 2 entries, got sink=%d journal=%d", len(seen), len(want))
	}
	for i := range want {
		if !entryEquivalent(seen[i], want[i]) {
			t.Errorf("sink entry[%d] diverges from journal: sink=%+v journal=%+v", i, seen[i], want[i])
		}
	}
}

// TestSinkErrorPropagatesAndJournalUnchanged verifies the ordering contract:
// a sink rejection propagates (wrapped, errors.Is-matchable) and the
// in-memory journal does not run ahead of the sink.
func TestSinkErrorPropagatesAndJournalUnchanged(t *testing.T) {
	sentinel := errors.New("disk full")
	var j ij.Journal
	j.AttachSink(func(ij.Entry) error { return sentinel })

	if err := j.Append(validEntry()); !errors.Is(err, sentinel) {
		t.Fatalf("Append: expected errors.Is(err, sentinel), got %v", err)
	}
	if err := j.AppendTraced(ij.TracedEntry{
		Step:   "traced-step",
		Note:   "Should be rejected by sink.",
		Author: validHumanAuthor(),
	}); !errors.Is(err, sentinel) {
		t.Fatalf("AppendTraced: expected errors.Is(err, sentinel), got %v", err)
	}
	if got := j.Entries(); got != nil {
		t.Errorf("journal must be unchanged after sink rejection, got %d entries", len(got))
	}
}

// TestSinkNotInvokedOnValidationFailure verifies validation runs before the
// sink — invalid entries never reach persistence.
func TestSinkNotInvokedOnValidationFailure(t *testing.T) {
	var j ij.Journal
	calls := 0
	j.AttachSink(func(ij.Entry) error { calls++; return nil })

	bad := validEntry()
	bad.Step = ""
	if err := j.Append(bad); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if calls != 0 {
		t.Errorf("sink must not be invoked for invalid entries, got %d calls", calls)
	}
}

// TestAppendUncheckedBypassesSink verifies the documented escape hatch:
// AppendUnchecked cannot propagate a sink error, so it must not invoke it.
func TestAppendUncheckedBypassesSink(t *testing.T) {
	var j ij.Journal
	calls := 0
	j.AttachSink(func(ij.Entry) error { calls++; return nil })

	j.AppendUnchecked(validEntry())
	if calls != 0 {
		t.Errorf("AppendUnchecked must bypass the sink, got %d calls", calls)
	}
	if got := j.Entries(); len(got) != 1 {
		t.Errorf("expected 1 entry in journal, got %d", len(got))
	}
}

// TestSinkReceivesDefensiveCopy verifies the sink may mutate its entry
// without corrupting journal state.
func TestSinkReceivesDefensiveCopy(t *testing.T) {
	var j ij.Journal
	j.AttachSink(func(e ij.Entry) error {
		if e.SpanRefs != nil {
			e.SpanRefs[0] = "mutated"
		}
		return nil
	})

	e := validEntry()
	e.SpanRefs = []string{"span-001"}
	if err := j.Append(e); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := j.Entries()[0].SpanRefs[0]; got != "span-001" {
		t.Errorf("sink mutation leaked into journal: SpanRefs[0] = %q", got)
	}
}

// TestAttachSinkNilDetaches verifies AttachSink(nil) detaches a sink.
func TestAttachSinkNilDetaches(t *testing.T) {
	var j ij.Journal
	calls := 0
	j.AttachSink(func(ij.Entry) error { calls++; return nil })
	j.AttachSink(nil)
	if err := j.Append(validEntry()); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if calls != 0 {
		t.Errorf("detached sink must not be invoked, got %d calls", calls)
	}
}

// ── journal → JSONL → journal round trip ─────────────────────────────────────

// TestJournalJSONLRoundTrip drives the full persistence seam: a journal with
// plain, human-traced, and AI-traced entries (including SpanRefs) is sunk to
// JSONL, loaded back, rebuilt via NewFromEntries, and must compare equal
// entry-for-entry.
func TestJournalJSONLRoundTrip(t *testing.T) {
	clock := epoch
	j := ij.NewJournalWithClock(func() time.Time {
		clock = clock.Add(time.Minute)
		return clock
	})

	var buf bytes.Buffer
	j.AttachSink(ij.JSONLSink(jsonllog.NewWriter(&buf)))

	// Plain entry, no author.
	plain := validEntry()
	plain.SpanRefs = []string{"span-001", "span-002"}
	if err := j.Append(plain); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Human-traced entry.
	if err := j.AppendTraced(ij.TracedEntry{
		Step:   "evidence-reviewed",
		Note:   "Analyst reviewed spans.",
		Author: validHumanAuthor(),
	}); err != nil {
		t.Fatalf("AppendTraced (human): %v", err)
	}
	// AI-traced entry with span refs.
	if err := j.AppendTraced(ij.TracedEntry{
		Step:     "hypothesis-ranked",
		Note:     "Oracle ranked hypotheses.",
		Author:   validAIAuthor(),
		SpanRefs: []string{"span-003"},
	}); err != nil {
		t.Fatalf("AppendTraced (ai): %v", err)
	}

	// One JSON line per committed entry.
	if lines := strings.Count(buf.String(), "\n"); lines != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d:\n%s", lines, buf.String())
	}

	loaded, err := ij.LoadJSONL(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	rebuilt, err := ij.NewFromEntries(loaded)
	if err != nil {
		t.Fatalf("NewFromEntries: %v", err)
	}

	want := j.Entries()
	got := rebuilt.Entries()
	if len(got) != len(want) {
		t.Fatalf("expected %d entries after rebuild, got %d", len(want), len(got))
	}
	for i := range want {
		if !entryEquivalent(got[i], want[i]) {
			t.Errorf("entry[%d] round-trip mismatch:\n got: %+v\nwant: %+v", i, got[i], want[i])
		}
	}
}

// ── LoadJSONL ─────────────────────────────────────────────────────────────────

// TestLoadJSONLRejectsInvalidEntry verifies the rebuild path enforces the
// same invariants as Append (here: empty Step) with a line-numbered error.
func TestLoadJSONLRejectsInvalidEntry(t *testing.T) {
	input := `{"Step":"ok","Note":"fine","At":"2024-01-01T00:00:00Z","Author":null,"SpanRefs":null}` + "\n" +
		`{"Step":"","Note":"missing step","At":"2024-01-01T00:01:00Z","Author":null,"SpanRefs":null}` + "\n"
	entries, err := ij.LoadJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected line number in error, got %q", err.Error())
	}
	if entries != nil {
		t.Errorf("expected nil entries on validation failure, got %d", len(entries))
	}
}

// TestLoadJSONLRejectsMalformedJSON verifies decode failures are fatal.
func TestLoadJSONLRejectsMalformedJSON(t *testing.T) {
	input := `{"Step":"ok","Note":"fine","At":"2024-01-01T00:00:00Z"}` + "\n" + `{"Step":42}` + "\n"
	if _, err := ij.LoadJSONL(strings.NewReader(input)); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

// TestLoadJSONLPartialTrailingLineRecovery verifies the surfaced recovery
// mode: a torn tail returns the intact entries AND the sentinel error.
func TestLoadJSONLPartialTrailingLineRecovery(t *testing.T) {
	input := `{"Step":"one","Note":"first","At":"2024-01-01T00:00:00Z"}` + "\n" +
		`{"Step":"two","Note":"second","At":"2024-01-01T00:01:00Z"}` + "\n" +
		`{"Step":"thr` // torn mid-append
	entries, err := ij.LoadJSONL(strings.NewReader(input))
	if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		t.Fatalf("expected errors.Is(err, jsonllog.ErrPartialTrailingLine), got %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 recovered entries, got %d", len(entries))
	}
	if entries[0].Step != "one" || entries[1].Step != "two" {
		t.Errorf("recovered entries corrupted: %+v", entries)
	}
	// Recovered entries must still rebuild a usable journal.
	rebuilt, rerr := ij.NewFromEntries(entries)
	if rerr != nil {
		t.Fatalf("NewFromEntries on recovered entries: %v", rerr)
	}
	if got := rebuilt.Entries(); len(got) != 2 {
		t.Errorf("expected rebuilt journal with 2 entries, got %d", len(got))
	}
}

// ── NewFromEntries ────────────────────────────────────────────────────────────

// TestNewFromEntriesValidates verifies an invalid entry fails the rebuild
// with its index.
func TestNewFromEntriesValidates(t *testing.T) {
	bad := validEntry()
	bad.Note = ""
	_, err := ij.NewFromEntries([]ij.Entry{validEntry(), bad})
	if err == nil {
		t.Fatal("expected error for invalid entry, got nil")
	}
	if !strings.Contains(err.Error(), "entry 1") {
		t.Errorf("expected entry index in error, got %q", err.Error())
	}
}

// TestNewFromEntriesEmpty verifies an empty slice yields an empty, usable
// journal.
func TestNewFromEntriesEmpty(t *testing.T) {
	j, err := ij.NewFromEntries(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := j.Entries(); got != nil {
		t.Errorf("expected empty journal, got %d entries", len(got))
	}
	if err := j.Append(validEntry()); err != nil {
		t.Errorf("rebuilt journal must accept appends: %v", err)
	}
}

// TestNewFromEntriesDefensiveCopy verifies mutations of the input slice after
// construction do not leak into the journal.
func TestNewFromEntriesDefensiveCopy(t *testing.T) {
	src := validEntry()
	src.SpanRefs = []string{"span-001"}
	in := []ij.Entry{src}
	j, err := ij.NewFromEntries(in)
	if err != nil {
		t.Fatalf("NewFromEntries: %v", err)
	}
	in[0].SpanRefs[0] = "mutated"
	in[0].Step = "mutated"
	got := j.Entries()[0]
	if got.SpanRefs[0] != "span-001" || got.Step != "hypothesis-formed" {
		t.Errorf("input mutation leaked into journal: %+v", got)
	}
}
