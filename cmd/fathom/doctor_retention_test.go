package main

import (
	"bytes"
	"encoding/json"
	"os"
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
		{Channel: "edit-hook", Kind: "seen", At: now.Add(-3 * time.Minute), Meta: map[string]string{"session": "s", "file": "a.go", "phase": "post", "hook_inv": "9-1"}},
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
	recordHookLatency("edit-hook", "", 1900*time.Millisecond, 4*time.Second, time.Now(), 0, false)
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

// cpuBurnSink keeps burnCPU's arithmetic observable to the compiler.
var cpuBurnSink uint64

// burnCPU spins until processCPUTime has advanced by want since the call
// began. Bounded by a large iteration count as a safety net only: the exit
// is the CPU reader advancing, never a wall clock, so a loaded machine that
// starves the loop still ends with the CPU it was asked to burn.
func burnCPU(t *testing.T, want time.Duration) {
	t.Helper()
	base, ok := processCPUTime()
	if !ok {
		t.Fatal("burnCPU needs a CPU reader")
	}
	for i := uint64(0); i < 1<<32; i++ {
		for j := uint64(0); j < 1<<14; j++ {
			cpuBurnSink += j*i ^ cpuBurnSink>>3
		}
		if now, _ := processCPUTime(); now-base >= want {
			return
		}
	}
	t.Fatalf("the CPU reader never advanced %v", want)
}

// withHookCPUReader installs r as the hook guard's CPU reader for the test.
func withHookCPUReader(t *testing.T, r func() (time.Duration, bool)) {
	t.Helper()
	orig := hookCPUTime
	hookCPUTime = r
	t.Cleanup(func() { hookCPUTime = orig })
}

// hookLatencyLines is the raw hook-latency log, one entry per line.
func hookLatencyLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(hookLatencyPath())
	if err != nil {
		t.Fatalf("hook-latency log: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// guardedInvocation runs one hook invocation through the guard: begin, work,
// end (the closure every hook verb defers).
func guardedInvocation(t *testing.T, channel string, work func()) {
	t.Helper()
	end, silent := beginHookGuard(channel, "fathom "+channel, "claude-code", "http://127.0.0.1:1")
	if silent {
		t.Skip("kill switch set in this environment")
	}
	work()
	end()
}

// EW-29: each hook-latency record carries the invocation's CPU time
// (user+sys, self and reaped children) beside its wall time, so a slow hook
// reads as work or waiting from the log alone. The field is absent where the
// CPU cannot be read — never a fabricated 0 — and the end path adds two CPU
// readings and nothing else: still one append, no read of the log.
func TestRecordHookLatencyCarriesCPUTime(t *testing.T) {
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))

	t.Run("a guarded invocation that burns 50 ms of CPU records at least 50", func(t *testing.T) {
		if _, ok := processCPUTime(); !ok {
			t.Skip("no CPU reader on this platform (the field is absent there: see the unknown case)")
		}
		_ = os.Remove(hookLatencyPath())
		// No upper bound is asserted: the Go runtime is multi-threaded, so
		// the process's CPU can exceed the wall time it took.
		guardedInvocation(t, "session-hook", func() { burnCPU(t, 50*time.Millisecond) })
		lines := hookLatencyLines(t)
		if len(lines) != 1 {
			t.Fatalf("one invocation wrote %d rows, want exactly 1: %q", len(lines), lines)
		}
		var row hookLatencyRow
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatal(err)
		}
		if row.CPUMS == nil || *row.CPUMS < 50 {
			t.Fatalf("cpu_ms = %v after 50 ms of CPU: %s", row.CPUMS, lines[0])
		}
		if row.Channel != "session-hook" || row.Inv == "" {
			t.Fatalf("the row keeps its wall-time fields: %s", lines[0])
		}
	})

	t.Run("an unknown reading leaves the field absent, never 0", func(t *testing.T) {
		unknown := func() (time.Duration, bool) { return 0, false }
		calls := 0
		atBegin := func() (time.Duration, bool) { calls++; return time.Second, calls == 1 }
		atEnd := func() (time.Duration, bool) { calls++; return time.Second, calls != 1 }
		for name, reader := range map[string]func() (time.Duration, bool){
			"never readable":         unknown,
			"readable only at begin": atBegin,
			"readable only at end":   atEnd,
		} {
			calls = 0
			_ = os.Remove(hookLatencyPath())
			withHookCPUReader(t, reader)
			guardedInvocation(t, "edit-hook", func() {})
			lines := hookLatencyLines(t)
			if len(lines) != 1 {
				t.Fatalf("%s: %d rows, want 1", name, len(lines))
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
				t.Fatal(err)
			}
			for _, k := range []string{"cpu_ms", "cpu_user_ms", "cpu_sys_ms"} {
				if _, present := raw[k]; present {
					t.Errorf("%s: %s is present with an unknown CPU reading: %s", name, k, lines[0])
				}
			}
			if _, present := raw["ms"]; !present {
				t.Errorf("%s: the wall time is still recorded: %s", name, lines[0])
			}
		}
	})

	t.Run("the row carries the delta between begin and end, and a measured zero is kept", func(t *testing.T) {
		readings := []time.Duration{2 * time.Second, 2*time.Second + 250*time.Millisecond}
		i := 0
		withHookCPUReader(t, func() (time.Duration, bool) { d := readings[i]; i++; return d, true })
		_ = os.Remove(hookLatencyPath())
		guardedInvocation(t, "stop-hook", func() {})
		var row hookLatencyRow
		if err := json.Unmarshal([]byte(hookLatencyLines(t)[0]), &row); err != nil || row.CPUMS == nil || *row.CPUMS != 250 {
			t.Fatalf("delta of 250 ms: cpu_ms %v, err %v", row.CPUMS, err)
		}

		flat := func() (time.Duration, bool) { return 3 * time.Second, true }
		withHookCPUReader(t, flat)
		_ = os.Remove(hookLatencyPath())
		guardedInvocation(t, "stop-hook", func() {})
		row = hookLatencyRow{}
		if err := json.Unmarshal([]byte(hookLatencyLines(t)[0]), &row); err != nil || row.CPUMS == nil || *row.CPUMS != 0 {
			t.Fatalf("a read that did not advance is a measured 0, kept: cpu_ms %v, err %v", row.CPUMS, err)
		}
	})

	t.Run("the end path makes one append, adds no read, and old rows still parse", func(t *testing.T) {
		// A log of rows written before the field existed.
		old := `{"at":"2026-09-28T03:05:00Z","channel":"edit-hook","ms":3,"budget_ms":4000}` + "\n" +
			`{"at":"2026-09-28T03:05:01Z","channel":"session-hook","ms":940,"budget_ms":4000,"inv":"9-1"}` + "\n"
		if err := os.MkdirAll(filepath.Dir(hookLatencyPath()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hookLatencyPath(), []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		var reads int
		real, _ := processCPUTime()
		withHookCPUReader(t, func() (time.Duration, bool) { reads++; real += time.Millisecond; return real, true })

		guardedInvocation(t, "prompt-hook", func() {})

		if reads != 2 {
			t.Errorf("the guard read the CPU %d times, want 2 (begin and end)", reads)
		}
		after, err := os.ReadFile(hookLatencyPath())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(after, []byte(old)) || bytes.Count(after, []byte("\n")) != 3 {
			t.Fatalf("the invocation must append exactly one line to the log and leave the rest alone: %q", after)
		}
		rows := readHookLatency(hookLatencyPath(), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
		if len(rows) != 3 {
			t.Fatalf("read %d rows, want 3: %+v", len(rows), rows)
		}
		if rows[0].CPUMS != nil || rows[1].CPUMS != nil || rows[0].MS != 3 || rows[1].Inv != "9-1" {
			t.Errorf("rows without the field read back without inventing one: %+v", rows[:2])
		}
		if rows[2].CPUMS == nil {
			t.Errorf("the new row carries its CPU: %+v", rows[2])
		}
	})

	t.Run("doctor names the CPU p50 beside the wall p50 only where rows carry it", func(t *testing.T) {
		cpu := func(ms int64) *int64 { return &ms }
		rows := []hookLatencyRow{
			{Channel: "session-hook", MS: 900, CPUMS: cpu(100), BudgetMS: 4000},
			{Channel: "session-hook", MS: 1000, CPUMS: cpu(200), BudgetMS: 4000},
			{Channel: "session-hook", MS: 1100, CPUMS: cpu(300), BudgetMS: 4000},
			{Channel: "edit-hook", MS: 3, BudgetMS: 4000},
		}
		got := hookLatencySummary(rows)
		if !strings.Contains(got, "session-hook p99 1.1s/4.0s (n=3, p50 wall 1000ms cpu 200ms)") {
			t.Errorf("session-hook line: %s", got)
		}
		if !strings.Contains(got, "edit-hook p99 0.0s/4.0s (n=1)") {
			t.Errorf("a channel with no CPU rows reads exactly as before: %s", got)
		}
		mixed := append(rows[:3:3], hookLatencyRow{Channel: "session-hook", MS: 5, BudgetMS: 4000})
		if got := hookLatencySummary(mixed); !strings.Contains(got, "(n=4, p50 wall 1000ms cpu 200ms of 3 timed)") {
			t.Errorf("a mix names how many rows carry CPU, comparing wall on those same rows: %s", got)
		}
	})
}
