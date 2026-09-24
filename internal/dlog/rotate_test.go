package dlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateIfNeeded_NoFile_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if err := RotateIfNeeded(path, 100); err != nil {
		t.Fatalf("RotateIfNeeded on a missing file: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file to be created")
	}
}

func TestRotateIfNeeded_UnderCap_NotRotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(path, []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RotateIfNeeded(path, 100); err != nil {
		t.Fatalf("RotateIfNeeded: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("expected no rotation for a file under the cap")
	}
}

// TestRotateIfNeeded_OverCap_KeepsOneGeneration is DG-6's plan-row contract
// test: a file over maxBytes is renamed aside as ONE prior generation
// (path+".1", overwriting any earlier backup), and the original path is
// then free for the next write to recreate — this is the exact rotation
// cmd/fathom/beats.go's rotateBeatLogIfNeeded proved for the hooks beat
// log, generalized here so cmd/fathom can reuse it verbatim.
func TestRotateIfNeeded_OverCap_KeepsOneGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	const cap = 100
	content := strings.Repeat("x", cap+1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RotateIfNeeded(path, cap); err != nil {
		t.Fatalf("RotateIfNeeded: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("expected the original path to be gone after rotation (renamed aside)")
	}
	backup, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("reading rotated backup: %v", err)
	}
	if string(backup) != content {
		t.Errorf("backup content = %q, want the original %q", backup, content)
	}

	// A second rotation must overwrite the first backup, not accumulate a
	// second generation — "one prior generation," not an archive.
	second := strings.Repeat("y", cap+1)
	if err := os.WriteFile(path, []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RotateIfNeeded(path, cap); err != nil {
		t.Fatalf("RotateIfNeeded (second): %v", err)
	}
	backup2, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("reading second rotated backup: %v", err)
	}
	if string(backup2) != second {
		t.Errorf("second backup content = %q, want %q (the first backup must be overwritten, not kept alongside)", backup2, second)
	}
}
