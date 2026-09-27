package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

func writeBeats(t *testing.T, path string, beats []heartbeat.Beat, padTo int) {
	t.Helper()
	var b strings.Builder
	for _, bt := range beats {
		j, err := json.Marshal(bt)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(j)
		b.WriteByte('\n')
	}
	// Filler beats after the named ones (newer, not breaker beats).
	last := time.Time{}
	if len(beats) > 0 {
		last = beats[len(beats)-1].At
	}
	for b.Len() < padTo {
		last = last.Add(time.Millisecond)
		j, _ := json.Marshal(heartbeat.Beat{Channel: "discovery", Kind: "seen", At: last, Meta: map[string]string{"pad": strings.Repeat("x", 200)}})
		b.Write(j)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hook_budget's "/24h" figures are read from the beats of the whole 24 h
// window: across the rotated generation (hooks.jsonl.1) and past any fixed
// byte tail. When the retained beats do not reach back 24 h, the row names
// the span they cover instead of presenting it as 24 h (I38).
func TestHookBudgetReadsTheWholeWindowOfBeats(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", path)

	// Just after a rotation: the breaker opened 10 h ago, in hooks.jsonl.1;
	// the live file holds only this hour.
	writeBeats(t, path+".1", []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-30 * time.Hour)},
		{Channel: "edit-hook", Kind: "breaker_open", At: now.Add(-10 * time.Hour)},
	}, 0)
	writeBeats(t, path, []heartbeat.Beat{{Channel: "edit-hook", Kind: "seen", At: now.Add(-time.Minute)}}, 0)
	if c := checkHookBudgetFromFiles(now); !strings.Contains(c.Reason, "breaker open 1 min/24h") || !strings.Contains(c.Reason, "1 edit(s) served while open") {
		t.Fatalf("a breaker beat in the rotated generation is inside the 24 h window: %+v", c)
	}

	// One live file larger than any fixed tail, the breaker beat at its head.
	os.Remove(path + ".1")
	writeBeats(t, path, []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-25 * time.Hour)},
		{Channel: "edit-hook", Kind: "breaker_open", At: now.Add(-20 * time.Hour)},
	}, 6<<20)
	if c := checkHookBudgetFromFiles(now); !strings.Contains(c.Reason, "breaker open 1 min/24h") {
		t.Fatalf("a breaker beat 20 h ago, 6 MiB before the end: %+v", c)
	}

	// The read's byte bound reached before the window: the span is named.
	defer func(v int64) { hookBudgetBeatMaxBytes = v }(hookBudgetBeatMaxBytes)
	hookBudgetBeatMaxBytes = 1 << 20
	c := checkHookBudgetFromFiles(now)
	if strings.Contains(c.Reason, "/24h") || !strings.Contains(c.Reason, "beat log read covers only") {
		t.Fatalf("a truncated read must name the span it covers, never /24h: %+v", c)
	}
}
