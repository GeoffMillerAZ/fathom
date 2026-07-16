package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

var instT0 = time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)

func TestReportInstrument_CoverageAndAlarms(t *testing.T) {
	// Sessions ran, edits seen, but the channel answered only one of two —
	// and one seen edit is unanswered inside the 24h window → alarm-free
	// (answered exists in window) with 50% coverage.
	beats := []heartbeat.Beat{
		{Channel: "session-hook", Kind: "fired", At: instT0},
		{Channel: "edit-hook", Kind: "seen", At: instT0.Add(time.Hour)},
		{Channel: "edit-hook", Kind: "answered", At: instT0.Add(time.Hour + time.Second)},
		{Channel: "edit-hook", Kind: "seen", At: instT0.Add(2 * time.Hour)},
	}
	var buf bytes.Buffer
	reportInstrument(&buf, beats, instT0.Add(3*time.Hour), 7*24*time.Hour)
	out := buf.String()
	if !strings.Contains(out, "2/2") && !strings.Contains(out, "1/2 = 50%") {
		// answered/seen = 1/2.
		if !strings.Contains(out, "1/2") {
			t.Errorf("coverage line missing 1/2: %s", out)
		}
	}
	if !strings.Contains(out, "liveness: all standing expectations met") {
		t.Errorf("expected no alarms: %s", out)
	}

	// The four-dark-days shape: sessions fired, zero edit beats → alarm.
	dark := []heartbeat.Beat{{Channel: "session-hook", Kind: "fired", At: instT0}}
	buf.Reset()
	reportInstrument(&buf, dark, instT0.Add(3*time.Hour), 7*24*time.Hour)
	if !strings.Contains(buf.String(), "liveness alarms") || !strings.Contains(buf.String(), "edit-hook.seen") {
		t.Errorf("dark channel must alarm: %s", buf.String())
	}
}

func TestCmdInstrument_EndToEnd(t *testing.T) {
	// emitBeat → cmdInstrument round-trip through FATHOM_HOOKS_LOG (GI-1/GI-3).
	dir := t.TempDir()
	logPath := filepath.Join(dir, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", logPath)
	emitBeat("edit-hook", "seen", nil)
	emitBeat("edit-hook", "answered", map[string]string{"emitted": "true"})

	var stdout, stderr bytes.Buffer
	if code := cmdInstrument([]string{"--log", logPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0 (advisory always)", code)
	}
	if !strings.Contains(stdout.String(), "1/1 = 100%") {
		t.Errorf("coverage missing: %s", stdout.String())
	}
	// Missing log is advisory, exit 0.
	if code := cmdInstrument([]string{"--log", filepath.Join(dir, "nope.jsonl")}, &stdout, &stderr); code != 0 {
		t.Error("missing log must still exit 0")
	}
	// Beat failures are swallowed (unwritable path: a directory component
	// that is a regular file) — must not panic, must not error out.
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(logPath, "sub", "hooks.jsonl"))
	emitBeat("edit-hook", "seen", nil)
	_ = os.Remove(logPath)
}
