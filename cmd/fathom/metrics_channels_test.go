package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// writeBeatsFixture writes beats as JSONL at a fresh temp file and returns
// its path — heartbeat.NewLog stamps At from its own injected clock, so
// beats are written one at a time with a clock that advances through the
// caller's own timestamps.
func writeBeatsFixture(t *testing.T, beats []heartbeat.Beat) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create beats fixture: %v", err)
	}
	defer f.Close()
	idx := 0
	l, err := heartbeat.NewLog(f, func() time.Time { return beats[idx].At })
	if err != nil {
		t.Fatalf("NewLog: %v", err)
	}
	for i, b := range beats {
		idx = i
		if err := l.Beat(b.Channel, b.Kind, b.Meta); err != nil {
			t.Fatalf("Beat: %v", err)
		}
	}
	return path
}

func TestCmdMetricsChannels_JSON(t *testing.T) {
	now := time.Now().UTC()
	logPath := writeBeatsFixture(t, []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-time.Minute)},
		{Channel: "edit-hook", Kind: "answered", At: now.Add(-30 * time.Second), Meta: map[string]string{"emitted": "true"}},
	})
	var out, errBuf bytes.Buffer
	code := cmdMetricsChannels([]string{"--hooks-log", logPath, "--since", "1h", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["injection_seen"].(float64) != 1 || payload["obligations_served"].(float64) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestCmdMetricsChannels_MissingHooksLog_HonestNotes(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	var out, errBuf bytes.Buffer
	code := cmdMetricsChannels([]string{"--hooks-log", logPath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	json.Unmarshal(out.Bytes(), &payload)
	if payload["injection_seen"].(float64) != 0 {
		t.Fatalf("injection_seen = %v, want 0", payload["injection_seen"])
	}
	if payload["obligations_evicted_note"] == "" {
		t.Error("obligations_evicted_note is empty, want the phase-P verbatim note")
	}
}
