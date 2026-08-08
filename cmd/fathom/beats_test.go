package main

// beats_test.go — critic finding F7 (phase-P fix round): beat log
// rotation and the bounded-tail read the Stop hook uses.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateBeatLogIfNeeded_NoFile_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	rotateBeatLogIfNeeded(path) // must not panic or create anything
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file to be created")
	}
}

func TestRotateBeatLogIfNeeded_UnderCap_NotRotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	if err := os.WriteFile(path, []byte(`{"channel":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rotateBeatLogIfNeeded(path)
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("expected no rotation for a small file")
	}
}

func TestRotateBeatLogIfNeeded_OverCap_Rotated(t *testing.T) {
	origCap := beatLogRotateBytes
	beatLogRotateBytes = 100 // shrink the cap rather than writing a real 64MB fixture
	t.Cleanup(func() { beatLogRotateBytes = origCap })

	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	content := strings.Repeat("x", int(beatLogRotateBytes)+1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rotateBeatLogIfNeeded(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected the original path to be renamed away, stat err = %v", err)
	}
	data, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("expected the rotated backup to exist: %v", err)
	}
	if string(data) != content {
		t.Error("rotated backup content mismatch")
	}
}

// TestEmitBeat_RotatesBeforeAppending is the end-to-end proof: emitBeat
// itself rotates an oversize log before the new beat lands, so the log
// never grows past roughly one beat over the cap.
func TestEmitBeat_RotatesBeforeAppending(t *testing.T) {
	origCap := beatLogRotateBytes
	beatLogRotateBytes = 100
	t.Cleanup(func() { beatLogRotateBytes = origCap })

	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(beatLogRotateBytes)+1)), 0o644); err != nil {
		t.Fatal(err)
	}

	emitBeat("test-channel", "test-kind", nil)

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected a rotated backup after emitBeat on an oversize log: %v", err)
	}
	beats, ok := readBeatLog(path)
	if !ok || len(beats) != 1 || beats[0].Channel != "test-channel" {
		t.Fatalf("beats = %+v ok=%v, want exactly the one new beat in the fresh file", beats, ok)
	}
}

// ─── readBeatLogTail ────────────────────────────────────────────────────────

func TestReadBeatLogTail_SmallFile_ReadsEverything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	content := `{"channel":"a","at":"2026-07-30T00:00:00Z"}` + "\n" +
		`{"channel":"b","at":"2026-07-30T00:00:01Z"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	beats, ok := readBeatLogTail(path, 1<<20)
	if !ok || len(beats) != 2 {
		t.Fatalf("beats = %+v ok=%v, want 2", beats, ok)
	}
}

// TestReadBeatLogTail_LargeFile_ReadsOnlyRecentTail proves the bound is
// real: a file far larger than maxBytes yields only the beats within the
// tail window, with the (expectedly torn) first partial line dropped —
// never a full-file read.
func TestReadBeatLogTail_LargeFile_ReadsOnlyRecentTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Many old beats (channel "old"), padded to exceed the tail window,
	// then a handful of recent beats (channel "new") at the very end.
	oldLine := `{"channel":"old","at":"2026-07-30T00:00:00Z"}` + "\n"
	for i := 0; i < 2000; i++ {
		if _, err := f.WriteString(oldLine); err != nil {
			t.Fatal(err)
		}
	}
	newLine := `{"channel":"new","at":"2026-07-30T00:00:01Z"}` + "\n"
	for i := 0; i < 3; i++ {
		if _, err := f.WriteString(newLine); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	// Just enough for the 3 "new" lines plus a few bytes of slack — NOT a
	// full additional "old" line's worth (old and new lines are the same
	// length), so a leak would mean the bound isn't real.
	tailBytes := int64(len(newLine)*3 + 5)
	beats, ok := readBeatLogTail(path, tailBytes)
	if !ok {
		t.Fatal("expected a successful tail read")
	}
	for _, b := range beats {
		if b.Channel == "old" {
			t.Fatalf("tail read leaked an old beat — bound is not real: %+v", beats)
		}
	}
	newCount := 0
	for _, b := range beats {
		if b.Channel == "new" {
			newCount++
		}
	}
	if newCount == 0 {
		t.Fatal("expected at least one 'new' beat from the tail window")
	}
}

func TestReadBeatLogTail_AbsentFile(t *testing.T) {
	if _, ok := readBeatLogTail(filepath.Join(t.TempDir(), "nope.jsonl"), 1024); ok {
		t.Fatal("expected ok=false for an absent file")
	}
}

// TestBeatVersionStamp pins IO-12 (vision 14 §3): every beat's meta carries
// schema_version + fathom_version + project_dir, so the user-global beat
// log becomes filterable by producer semantics and attributable per repo —
// caller keys always win on collision, and the caller's map is not mutated.
func TestBeatVersionStamp(t *testing.T) {
	caller := map[string]string{"phase": "post"}
	got := stampBeatMeta(caller)
	if got["schema_version"] != beatSchemaVersion {
		t.Errorf("schema_version = %q, want %q", got["schema_version"], beatSchemaVersion)
	}
	if got["fathom_version"] == "" {
		t.Error("fathom_version must never be empty")
	}
	if got["project_dir"] == "" {
		t.Error("project_dir should carry the hook's cwd")
	}
	if got["phase"] != "post" {
		t.Error("caller keys must survive")
	}
	if len(caller) != 1 {
		t.Error("caller map must not be mutated")
	}

	t.Run("caller_wins_on_collision", func(t *testing.T) {
		got := stampBeatMeta(map[string]string{"project_dir": "/explicit"})
		if got["project_dir"] != "/explicit" {
			t.Errorf("caller-provided project_dir overridden: %q", got["project_dir"])
		}
	})
}
