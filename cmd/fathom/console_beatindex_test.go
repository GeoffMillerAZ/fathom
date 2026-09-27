package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSessionPageSeeksIndexedBeats (RT-114, design §14): a session's beats
// come from a per-session offset index into hooks.jsonl and its rotated
// generation, extended by what was appended since the last read — never a
// whole-log read per page. A rotation reuses the rotated file's index; the
// replay and the legacy /observatory/session story read the same index,
// older generation first; a log that cannot be read is an error, never "no
// beats".
func TestSessionPageSeeksIndexedBeats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)
	beat := func(session, at, kind string) string {
		return `{"channel":"edit-hook","kind":"` + kind + `","at":"` + at + `","meta":{"file":"a.go","session":"` + session + `"}}` + "\n"
	}
	mustWrite(t, path+".1", beat("s-a", "2026-09-26T10:00:00Z", "seen")+beat("s-b", "2026-09-26T10:01:00Z", "seen"))
	mustWrite(t, path, beat("s-a", "2026-09-27T10:00:00Z", "emitted")+"not json\n"+beat("s-b", "2026-09-27T10:01:00Z", "seen"))
	ix := &sessionBeatIndex{}

	lines, partial, err := ix.lines(path, "s-a")
	if err != nil || partial != nil || len(lines) != 2 || !strings.Contains(string(lines[0]), "2026-09-26") || !strings.Contains(string(lines[1]), "2026-09-27") {
		t.Fatalf("s-a = %q, %v, %v; want its two beats, the rotated generation first", lines, partial, err)
	}
	indexed := ix.parsed
	if _, _, err := ix.lines(path, "s-b"); err != nil || ix.parsed != indexed {
		t.Fatalf("a second session re-read the log: parsed %d → %d (%v)", indexed, ix.parsed, err)
	}

	// An append is indexed alone.
	app := beat("s-b", "2026-09-27T11:00:00Z", "error")
	fh, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	fh.WriteString(app)
	fh.Close()
	lines, _, _ = ix.lines(path, "s-b")
	if len(lines) != 3 || ix.parsed-indexed != int64(len(app)) {
		t.Fatalf("after an append: %d lines, parsed %d bytes; want 3 lines and only the %d appended bytes", len(lines), ix.parsed-indexed, len(app))
	}

	// Rotation: the current file becomes .1 (its index moves with it) and
	// a new current starts; only the new file is read.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	fresh := beat("s-a", "2026-09-27T12:00:00Z", "seen")
	mustWrite(t, path, fresh)
	before := ix.parsed
	lines, _, _ = ix.lines(path, "s-a")
	if len(lines) != 2 || !strings.Contains(string(lines[0]), "10:00:00") || !strings.Contains(string(lines[1]), "12:00:00") {
		t.Fatalf("after rotation s-a = %q", lines)
	}
	if got := ix.parsed - before; got != int64(len(fresh)) {
		t.Errorf("rotation parsed %d bytes; want only the new current file's %d", got, len(fresh))
	}

	// The replay and the legacy story read the shared index.
	prev := hooksBeatIndex
	hooksBeatIndex = ix
	t.Cleanup(func() { hooksBeatIndex = prev })
	events, _, err := readSessionEvents(path, "s-a")
	if err != nil || len(events) != 2 || events[1].Kind != "seen" {
		t.Fatalf("replay = %+v, %v", events, err)
	}
	st := buildSessionStory("s-b")
	if len(st.Beats) != 2 || st.Beats[1].Kind != "error" {
		t.Fatalf("story beats = %+v", st.Beats)
	}
	if ix.parsed != before+int64(len(fresh)) {
		t.Errorf("a page read re-read the log: parsed %d more bytes", ix.parsed-before-int64(len(fresh)))
	}

	// Missing: not-exist; unreadable current: an error, never no beats.
	if _, _, err := ix.lines(filepath.Join(dir, "none.jsonl"), "s-a"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing log = %v", err)
	}
	if os.Geteuid() != 0 {
		os.Chmod(path, 0o000)
		defer os.Chmod(path, 0o644)
		if _, _, err := ix.lines(path, "s-a"); err == nil {
			t.Error("an unreadable current log must be an error")
		}
	}
}
