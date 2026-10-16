package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

func loadBeatsAt(t *testing.T, path string) []heartbeat.Beat {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true)
	if err != nil {
		t.Fatalf("beat log: %v", err)
	}
	return beats
}

// TestSessionHookEventArg pins the pre-parse route: every spelling of
// --event is seen before cmdSessionHook emits its "fired" beat.
func TestSessionHookEventArg(t *testing.T) {
	cases := map[string][]string{
		"instructions-loaded": {"--event", "instructions-loaded"},
		"x":                   {"--url", "http://h", "-event=x"},
		"y":                   {"--event=y", "--dir", "/r"},
		"":                    {"--url", "http://h"},
	}
	for want, args := range cases {
		if got := sessionHookEventArg(args); got != want {
			t.Errorf("sessionHookEventArg(%v) = %q, want %q", args, got, want)
		}
	}
	if got := sessionHookEventArg([]string{"--", "--event", "z"}); got != "" {
		t.Errorf("args after -- must not be read as flags, got %q", got)
	}
}

// TestRecordInstructionsLoaded pins PC-23: the InstructionsLoaded envelope
// becomes ONE silent beat {file_path, memory_type, load_reason} (+ session)
// on its own channel, stdout stays empty, exit 0, and the session-hook
// "fired" counter is not touched.
func TestRecordInstructionsLoaded(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	t.Setenv("FATHOM_CHANNELS", "")
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())

	stdin := strings.NewReader(`{"session_id":"s-9","transcript_path":"/t.jsonl","cwd":"/r","hook_event_name":"InstructionsLoaded","file_path":"/r/CLAUDE.md","memory_type":"Project","load_reason":"session_start"}`)
	var stdout, stderr bytes.Buffer
	if code := cmdSessionHook([]string{"--event", "instructions-loaded"}, stdin, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty (the event has no decision control), got %q", stdout.String())
	}
	beats := loadBeatsAt(t, beatsPath)
	if len(beats) != 1 {
		t.Fatalf("beats = %+v, want exactly one (no session-hook fired beat)", beats)
	}
	b := beats[0]
	if b.Channel != instructionsLoadedChannel || b.Kind != "loaded" {
		t.Errorf("beat = %s/%s, want %s/loaded", b.Channel, b.Kind, instructionsLoadedChannel)
	}
	for k, want := range map[string]string{"file_path": "/r/CLAUDE.md", "memory_type": "Project", "load_reason": "session_start", "session": "s-9"} {
		if b.Meta[k] != want {
			t.Errorf("meta[%s] = %q, want %q", k, b.Meta[k], want)
		}
	}
	if _, ok := b.Meta["file"]; ok {
		t.Error("meta carries the edit-hook 'file' key; the contract names file_path")
	}

	// Absent fields are omitted, never recorded as "".
	stdin = strings.NewReader(`{"session_id":"s-9","hook_event_name":"InstructionsLoaded","file_path":"/r/.claude/rules/x.md"}`)
	if code := cmdSessionHook([]string{"--event=instructions-loaded"}, stdin, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	beats = loadBeatsAt(t, beatsPath)
	if len(beats) != 2 {
		t.Fatalf("beats = %d, want 2", len(beats))
	}
	if _, ok := beats[1].Meta["memory_type"]; ok {
		t.Error("an absent memory_type was recorded")
	}
	if _, ok := beats[1].Meta["load_reason"]; ok {
		t.Error("an absent load_reason was recorded")
	}

	// Malformed stdin: nothing recorded, still exit 0.
	if code := cmdSessionHook([]string{"--event", "instructions-loaded"}, strings.NewReader("{not json"), &stdout, &stderr); code != 0 {
		t.Errorf("malformed stdin exit = %d, want 0", code)
	}
	if got := len(loadBeatsAt(t, beatsPath)); got != 2 {
		t.Errorf("malformed stdin wrote a beat: %d", got)
	}
	// Unknown event: one stderr line, exit 0, nothing recorded.
	stderr.Reset()
	if code := cmdSessionHook([]string{"--event", "nope"}, strings.NewReader("{}"), &stdout, &stderr); code != 0 || !strings.Contains(stderr.String(), "unknown event") {
		t.Errorf("unknown event: exit %d, stderr %q", code, stderr.String())
	}
	if got := len(loadBeatsAt(t, beatsPath)); got != 2 {
		t.Errorf("unknown event wrote a beat: %d", got)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty throughout", stdout.String())
	}
}

// TestRecordInstructionsLoaded_KillSwitch: FATHOM_CHANNELS=off silences
// the event beat exactly like every other hook.
func TestRecordInstructionsLoaded_KillSwitch(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	t.Setenv("FATHOM_CHANNELS", "off")
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(`{"session_id":"s","hook_event_name":"InstructionsLoaded","file_path":"/r/CLAUDE.md"}`)
	if code := cmdSessionHook([]string{"--event", "instructions-loaded"}, stdin, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if _, err := os.Stat(beatsPath); err == nil {
		t.Error("kill switch on, yet a beat was written")
	}
}
