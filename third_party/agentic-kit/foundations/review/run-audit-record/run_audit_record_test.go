package runauditrecord_test

import (
	"testing"
	"time"

	rar "github.com/geoffmilleraz/agentic-kit/foundations/review/run-audit-record"
)

func TestRecord_AppendAndEntries(t *testing.T) {
	var r rar.Record

	e1 := rar.AuditEntry{Step: "init", Detail: "run started", At: time.Now().UTC()}
	e2 := rar.AuditEntry{Step: "scan", Detail: "evidence scanned"}
	e3 := rar.AuditEntry{Step: "score", Detail: "scoring complete", At: time.Now().UTC()}

	for _, e := range []rar.AuditEntry{e1, e2, e3} {
		if err := r.Append(e); err != nil {
			t.Fatalf("Append(%q): %v", e.Step, err)
		}
	}

	entries := r.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	steps := []string{"init", "scan", "score"}
	for i, want := range steps {
		if entries[i].Step != want {
			t.Errorf("entry[%d]: expected step %q, got %q", i, want, entries[i].Step)
		}
	}
}

func TestRecord_EmptyStepRejected(t *testing.T) {
	var r rar.Record
	err := r.Append(rar.AuditEntry{Step: "", Detail: "no step"})
	if err == nil {
		t.Error("expected error for empty Step, got nil")
	}
}

func TestRecord_TimestampFilledIfZero(t *testing.T) {
	var r rar.Record
	entry := rar.AuditEntry{Step: "timestamp-test", Detail: "at is zero"}
	// At is zero value.
	if err := r.Append(entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := r.Entries()
	if entries[0].At.IsZero() {
		t.Error("expected At to be set automatically, got zero time")
	}
}

func TestRecord_ImmutableReplay(t *testing.T) {
	var r rar.Record
	r.Append(rar.AuditEntry{Step: "a", Detail: "first"})  //nolint
	r.Append(rar.AuditEntry{Step: "b", Detail: "second"}) //nolint
	r.Append(rar.AuditEntry{Step: "c", Detail: "third"})  //nolint

	// Replay twice — must produce identical results.
	e1 := r.Entries()
	e2 := r.Entries()

	if len(e1) != len(e2) {
		t.Fatalf("replay lengths differ: %d vs %d", len(e1), len(e2))
	}
	for i := range e1 {
		if e1[i].Step != e2[i].Step {
			t.Errorf("replay[%d]: step differs: %q vs %q", i, e1[i].Step, e2[i].Step)
		}
		if e1[i].Detail != e2[i].Detail {
			t.Errorf("replay[%d]: detail differs: %q vs %q", i, e1[i].Detail, e2[i].Detail)
		}
	}
}

func TestRecord_EntriesReturnsCopy(t *testing.T) {
	var r rar.Record
	r.Append(rar.AuditEntry{Step: "orig", Detail: "original"}) //nolint

	entries := r.Entries()
	entries[0].Step = "mutated"

	entries2 := r.Entries()
	if entries2[0].Step == "mutated" {
		t.Error("Entries() should return a copy; internal state was mutated")
	}
}

func TestRecord_AppendOnlyNoPastMutation(t *testing.T) {
	var r rar.Record

	r.Append(rar.AuditEntry{Step: "step1", Detail: "d1"}) //nolint

	snapshot := r.Entries()
	origStep := snapshot[0].Step

	r.Append(rar.AuditEntry{Step: "step2", Detail: "d2"}) //nolint

	// The first entry must not have changed.
	snapshot2 := r.Entries()
	if snapshot2[0].Step != origStep {
		t.Errorf("past entry was mutated: expected %q, got %q", origStep, snapshot2[0].Step)
	}
	if len(snapshot2) != 2 {
		t.Errorf("expected 2 entries, got %d", len(snapshot2))
	}
}

// TestAppendUsesInjectedClock pins the determinism seam: a zero At is stamped
// from the record's injected clock, never the wall clock.
func TestAppendUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 6, 10, 12, 0, 0, 0, time.FixedZone("MST", -7*3600))
	r := rar.Record{Now: func() time.Time { return fixed }}
	if err := r.Append(rar.AuditEntry{Step: "plan"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got := r.Entries()[0].At
	if !got.Equal(fixed) {
		t.Fatalf("At = %v, want injected clock %v", got, fixed)
	}
	if got.Location() != time.UTC {
		t.Fatalf("At location = %v, want UTC", got.Location())
	}
	// An explicit At is never overwritten.
	explicit := fixed.Add(-time.Hour)
	if err := r.Append(rar.AuditEntry{Step: "act", At: explicit}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := r.Entries()[1].At; !got.Equal(explicit) {
		t.Fatalf("explicit At overwritten: got %v, want %v", got, explicit)
	}
}
