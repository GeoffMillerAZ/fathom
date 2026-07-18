package jsonllog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// readBack opens path and returns every decoded line via ReadAll.
func readBack(t *testing.T, path string) []sample {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open for read: %v", err)
	}
	defer f.Close()

	var got []sample
	err = jsonllog.ReadAll(f, func(raw json.RawMessage) error {
		var s sample
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		got = append(got, s)
		return nil
	})
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return got
}

// TestFileRoundTrip verifies the full file lifecycle: create, append, close,
// reopen (append-only continuation), close, read everything back in order.
func TestFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")

	first := []sample{{Name: "a", N: 1}, {Name: "b", N: 2}}
	f, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	for _, s := range first {
		if err := f.Append(s); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen — O_APPEND must continue the log, not truncate it.
	f2, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile (reopen): %v", err)
	}
	if err := f2.Append(sample{Name: "c", N: 3}); err != nil {
		t.Fatalf("Append after reopen: %v", err)
	}
	if err := f2.Close(); err != nil {
		t.Fatalf("Close (reopen): %v", err)
	}

	got := readBack(t, path)
	want := append(first, sample{Name: "c", N: 3})
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d]: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

// TestOpenFileCreatesMissingFile verifies O_CREATE behavior.
func TestOpenFileCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.jsonl")
	f, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist after OpenFile+Close: %v", err)
	}
}

// TestOpenFileDoesNotTruncate verifies pre-existing content survives reopen.
func TestOpenFileDoesNotTruncate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.jsonl")
	pre := `{"Name":"pre","N":0}` + "\n"
	if err := os.WriteFile(path, []byte(pre), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	f, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := f.Append(sample{Name: "post", N: 1}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := readBack(t, path)
	if len(got) != 2 || got[0].Name != "pre" || got[1].Name != "post" {
		t.Errorf("expected [pre post], got %+v", got)
	}
}

// TestFileCloseTwiceReturnsError verifies the documented call-once contract.
func TestFileCloseTwiceReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.jsonl")
	f, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := f.Close(); err == nil {
		t.Error("expected error from second Close, got nil")
	}
}

// TestFileAppendAfterCloseFails verifies that Append on a closed File surfaces
// the underlying file error rather than silently dropping the entry.
func TestFileAppendAfterCloseFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed.jsonl")
	f, err := jsonllog.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := f.Append(sample{Name: "late", N: 9}); err == nil {
		t.Error("expected error appending to closed file, got nil")
	}
}
