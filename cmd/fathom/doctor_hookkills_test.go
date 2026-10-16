package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// RT-135 hygiene: a harness kill is an edit-hook invocation that began
// (its `seen` beat carries the invocation id) and never finished (no
// hook-latency row for that id). An invocation that exits silently after
// `seen` — obligations off, a relative path, a deny — finished and is not a
// kill; two invocations in one session on one file never overwrite each
// other; a real kill is counted.
func TestHookBudgetKillsAreUnfinishedInvocations(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	seen := func(inv, phase, target string, at time.Time) heartbeat.Beat {
		return heartbeat.Beat{Channel: "edit-hook", Kind: "seen", At: at, Meta: map[string]string{"session": "s", "file": "a.go", "phase": phase, "target": target, "hook_inv": inv}}
	}
	beats := []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-6 * time.Minute), Meta: map[string]string{"session": "s", "file": "b.go", "phase": "post", "hook_inv": "1-0"}}, // a relative path: silent exit
		seen("1-1", "post", "", now.Add(-5*time.Minute)),       // obligations off: silent exit
		seen("1-2", "pre", "command", now.Add(-4*time.Minute)), // denied, not answered
		{Channel: "edit-hook", Kind: "denied", At: now.Add(-4 * time.Minute), Meta: map[string]string{"session": "s", "phase": "pre", "target": "command", "hook_inv": "1-2"}},
		seen("1-3", "pre", "file", now.Add(-3*time.Minute)), // killed
		seen("1-4", "pre", "file", now.Add(-2*time.Minute)), // same session/file/phase, answered
		{Channel: "edit-hook", Kind: "answered", At: now.Add(-2 * time.Minute), Meta: map[string]string{"session": "s", "file": "a.go", "phase": "pre", "target": "file", "hook_inv": "1-4"}},
	}
	lat := []hookLatencyRow{
		{At: now.Add(-6 * time.Minute), Channel: "edit-hook", MS: 20, BudgetMS: 4000, Inv: "1-0"},
		{At: now.Add(-5 * time.Minute), Channel: "edit-hook", MS: 20, BudgetMS: 4000, Inv: "1-1"},
		{At: now.Add(-4 * time.Minute), Channel: "edit-hook", MS: 20, BudgetMS: 4000, Inv: "1-2"},
		{At: now.Add(-2 * time.Minute), Channel: "edit-hook", MS: 20, BudgetMS: 4000, Inv: "1-4"},
	}
	c := checkHookBudget(lat, beats, now)
	if !strings.Contains(c.Reason, "1 harness kill(s)") {
		t.Fatalf("one invocation was killed, none of the silent exits: %s", c.Reason)
	}

	// The hook guard stamps the id on the beats and the latency row.
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	end, silent := beginHookGuard("edit-hook", "fathom edit-hook", "claude-code", "http://127.0.0.1:1")
	if silent {
		t.Skip("kill switch set in this environment")
	}
	emitBeat("edit-hook", "seen", map[string]string{"phase": "post"})
	end()
	got, _ := readBeatLog(beatLogPath())
	rows := readHookLatency(hookLatencyPath(), time.Now().Add(-time.Hour))
	if len(got) != 1 || got[0].Meta["hook_inv"] == "" || len(rows) != 1 || rows[0].Inv != got[0].Meta["hook_inv"] {
		t.Fatalf("beat %+v and latency %+v must share the invocation id", got, rows)
	}
	_ = os.Getpid
}
