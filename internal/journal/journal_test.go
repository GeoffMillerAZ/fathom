package journal_test

import (
	"testing"

	ij "github.com/geoffmilleraz/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/fathom/internal/journal"
)

func TestRecordFindings_Empty(t *testing.T) {
	var j ij.Journal
	if err := journal.RecordFindings(&j, nil); err != nil {
		t.Fatalf("unexpected error for empty findings: %v", err)
	}
	if entries := j.Entries(); len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestRecordFindings_Single(t *testing.T) {
	var j ij.Journal
	findings := []journal.Finding{
		{Step: "analysis", Note: "found 3 clones in pkg/auth"},
	}
	if err := journal.RecordFindings(&j, findings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Step != "analysis" {
		t.Errorf("step: want %q, got %q", "analysis", entries[0].Step)
	}
	if entries[0].Note != "found 3 clones in pkg/auth" {
		t.Errorf("note: want %q, got %q", "found 3 clones in pkg/auth", entries[0].Note)
	}
	if entries[0].At.IsZero() {
		t.Error("expected non-zero At timestamp")
	}
}

func TestRecordFindings_Multiple(t *testing.T) {
	var j ij.Journal
	findings := []journal.Finding{
		{Step: "step-one", Note: "first finding"},
		{Step: "step-two", Note: "second finding"},
		{Step: "step-three", Note: "third finding"},
	}
	if err := journal.RecordFindings(&j, findings); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries := j.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	for i, f := range findings {
		if entries[i].Step != f.Step {
			t.Errorf("entry[%d].Step: want %q, got %q", i, f.Step, entries[i].Step)
		}
		if entries[i].Note != f.Note {
			t.Errorf("entry[%d].Note: want %q, got %q", i, f.Note, entries[i].Note)
		}
	}
}

func TestRecordFindings_EmptyStepReturnsError(t *testing.T) {
	var j ij.Journal
	findings := []journal.Finding{
		{Step: "ok", Note: "fine"},
		{Step: "", Note: "missing step"},
	}
	err := journal.RecordFindings(&j, findings)
	if err == nil {
		t.Fatal("expected error for empty Step, got nil")
	}
	// First finding should have been recorded before the error.
	entries := j.Entries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry before failure, got %d", len(entries))
	}
}

func TestRecordFindings_EmptyNoteReturnsError(t *testing.T) {
	var j ij.Journal
	findings := []journal.Finding{
		{Step: "ok", Note: ""},
	}
	if err := journal.RecordFindings(&j, findings); err == nil {
		t.Fatal("expected error for empty Note, got nil")
	}
}

func TestRecordFindings_AppendToExistingJournal(t *testing.T) {
	var j ij.Journal
	// Pre-populate via the journal API directly.
	if err := journal.RecordFindings(&j, []journal.Finding{
		{Step: "pre-existing", Note: "earlier step"},
	}); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	if err := journal.RecordFindings(&j, []journal.Finding{
		{Step: "new-step", Note: "new finding"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := j.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries total, got %d", len(entries))
	}
	if entries[0].Step != "pre-existing" {
		t.Errorf("entries[0].Step: want %q, got %q", "pre-existing", entries[0].Step)
	}
	if entries[1].Step != "new-step" {
		t.Errorf("entries[1].Step: want %q, got %q", "new-step", entries[1].Step)
	}
}
