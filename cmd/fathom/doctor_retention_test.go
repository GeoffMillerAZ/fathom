package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-135 (design §11.3, §12.1): doctor's hook_budget row judges each
// channel's p99 against its deadline, counts breaker-open minutes, edits
// served while the breaker was open and harness kills; disk_retention
// reports reclaimable bytes by class, the largest classes and the internal
// disk's free space.
func TestDoctorHookBudgetAndDiskRetention(t *testing.T) {
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	var lat []hookLatencyRow
	for i := 0; i < 100; i++ {
		lat = append(lat, hookLatencyRow{At: now.Add(-time.Hour), Channel: "edit-hook", MS: int64(100 + i), BudgetMS: 4000})
		lat = append(lat, hookLatencyRow{At: now.Add(-time.Hour), Channel: "stop-hook", MS: 500, BudgetMS: 8000})
	}
	lat = append(lat, hookLatencyRow{At: now.Add(-48 * time.Hour), Channel: "edit-hook", MS: 99000, BudgetMS: 4000}) // outside 24 h
	c := checkHookBudget(lat, nil, now)
	if c.Name != "hook_budget" || c.Status != doctorPass || !strings.Contains(c.Reason, "edit-hook p99 0.2s/4.0s (n=100)") || !strings.Contains(c.Reason, "stop-hook p99 0.5s/8.0s") {
		t.Fatalf("within budget: %+v", c)
	}
	// Over the deadline at p99.
	slow := append([]hookLatencyRow(nil), lat...)
	for i := 0; i < 5; i++ {
		slow = append(slow, hookLatencyRow{At: now.Add(-time.Minute), Channel: "post-bash", MS: 5017, BudgetMS: 4000})
	}
	if c := checkHookBudget(slow, nil, now); c.Status != doctorWarn || !strings.Contains(c.Remedy, "post-bash") {
		t.Fatalf("p99 over the deadline warns and names the channel: %+v", c)
	}
	// Breaker-open minutes, unguarded edits and a kill.
	beats := []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "breaker_open", At: now.Add(-10 * time.Minute)},
		{Channel: "edit-hook", Kind: "breaker_open", At: now.Add(-10*time.Minute + 20*time.Second)},
		{Channel: "session-hook", Kind: "breaker_open", At: now.Add(-5 * time.Minute)},
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-3 * time.Minute), Meta: map[string]string{"session": "s", "file": "a.go", "phase": "post"}},
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-2 * time.Minute), Meta: map[string]string{"session": "s", "file": "b.go", "phase": "post"}},
		{Channel: "edit-hook", Kind: "answered", At: now.Add(-2*time.Minute + time.Second), Meta: map[string]string{"session": "s", "file": "b.go", "phase": "post"}},
	}
	c = checkHookBudget(lat, beats, now)
	if c.Status != doctorWarn || !strings.Contains(c.Reason, "breaker open 2 min/24h") || !strings.Contains(c.Reason, "2 edit(s) served while open") || !strings.Contains(c.Reason, "1 harness kill(s)") {
		t.Fatalf("breaker minutes, unguarded edits, kills: %+v", c)
	}
	if c := checkHookBudget(nil, nil, now); c.Status != doctorPass || !strings.Contains(c.Reason, "no hook latency recorded") {
		t.Fatalf("no data is disclosed, not judged: %+v", c)
	}

	// The latency log round-trips through the hook guard's writer.
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	recordHookLatency("edit-hook", 1900*time.Millisecond, 4*time.Second, time.Now())
	if rows := readHookLatency(hookLatencyPath(), time.Now().Add(-time.Hour)); len(rows) != 1 || rows[0].MS != 1900 || rows[0].BudgetMS != 4000 {
		t.Fatalf("latency log: %+v", rows)
	}

	// disk_retention.
	rep := gcReport{Reclaimable: map[string]int64{"tmp": 10 << 20, "scenariobench-worktrees": 900 << 20, "diag": 1 << 20, "call-log": 0},
		Items: []gcItem{{Class: "ledgers", Action: "report", Reason: "over 32 MiB; kept whole"}}}
	d := checkDiskRetention(rep, 41*tenancy.GiB, "/Users/x")
	if d.Name != "disk_retention" || d.Status != doctorWarn || !strings.Contains(d.Reason, "largest: scenariobench-worktrees 900 MiB, tmp 10 MiB, diag 1 MiB") ||
		!strings.Contains(d.Reason, "41 GiB free on /Users/x") || !strings.Contains(d.Remedy, "fathom gc --apply") || !strings.Contains(d.Reason, "ledgers: over 32 MiB") {
		t.Fatalf("disk_retention over the warn mark: %+v", d)
	}
	if d := checkDiskRetention(gcReport{Reclaimable: map[string]int64{}}, -1, "/"); d.Status != doctorPass || !strings.Contains(d.Reason, "nothing reclaimable") || !strings.Contains(d.Reason, "free unknown") {
		t.Fatalf("nothing to reclaim, free unknown: %+v", d)
	}
}
