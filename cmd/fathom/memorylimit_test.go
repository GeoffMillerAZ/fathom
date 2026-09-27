package main

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-36 (design §2.2): the GC hint is the budget's — the exit mark by
// default, never a fixed 3 GiB — and a managed daemon runs with
// GOMAXPROCS = max(2, NumCPU/2) unless GOMAXPROCS is in its env. The hint
// only keeps the heap from outrunning the exit mark; the ladder bounds.
func TestMemoryLimitDerivedFromBudget(t *testing.T) {
	type call struct {
		limit int64
		procs int
	}
	run := func(b tenancy.Budget, env map[string]string) (call, string) {
		var got call
		var buf syncBuffer
		applyMemoryLimit(b, &buf,
			func(n int64) int64 { got.limit = n; return 0 },
			func(n int) int { got.procs = n; return 0 },
			func(k string) string { return env[k] })
		return got, buf.String()
	}
	managed, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true})
	got, line := run(managed, nil)
	if got.limit != 8*tenancy.GiB {
		t.Fatalf("the GC hint defaults to the exit mark (8 GiB on 32 GiB RAM), got %d", got.limit)
	}
	if got.procs != 6 {
		t.Fatalf("managed GOMAXPROCS max(2, 12/2) = 6, got %d", got.procs)
	}
	if !strings.Contains(line, "8 GiB") || !strings.Contains(line, "exit mark") || !strings.Contains(line, "GOMAXPROCS 6") {
		t.Fatalf("the startup line names the hint, its derivation and GOMAXPROCS: %q", line)
	}
	// GOMAXPROCS in the env wins: left alone.
	got, _ = run(managed, map[string]string{"GOMAXPROCS": "3"})
	if got.procs != 0 {
		t.Fatalf("GOMAXPROCS in the env is left to the runtime, got a set to %d", got.procs)
	}
	// gc_hint = budget.
	b, _ := tenancy.ResolveBudget(tenancy.Config{GCHint: "budget"}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true})
	if got, _ = run(b, nil); got.limit != 4*tenancy.GiB {
		t.Fatalf("gc_hint = budget: %d", got.limit)
	}
	// gc_hint = off: never applied.
	b, _ = tenancy.ResolveBudget(tenancy.Config{GCHint: "off"}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true})
	if got, line = run(b, nil); got.limit != 0 || !strings.Contains(line, "off") {
		t.Fatalf("gc_hint = off: limit %d, line %q", got.limit, line)
	}
	// A migrated [daemon].memory_limit arrives as the configured hint.
	b, _ = tenancy.ResolveBudget(tenancy.Config{GCHint: "3GiB"}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true})
	if got, _ = run(b, nil); got.limit != 3*tenancy.GiB {
		t.Fatalf("migrated memory_limit: %d", got.limit)
	}
	// Unmanaged: the hint follows its own (512 MiB) budget's exit mark; the
	// runtime's GOMAXPROCS stands.
	u, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12})
	if got, _ = run(u, nil); got.limit != tenancy.GiB || got.procs != 0 {
		t.Fatalf("unmanaged: %+v", got)
	}
}

// TestApplyMemoryLimit_ActuallyAppliesToRuntime proves the production
// setter reaches the runtime.
func TestApplyMemoryLimit_ActuallyAppliesToRuntime(t *testing.T) {
	prev := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(prev) })
	b, _ := tenancy.ResolveBudget(tenancy.Config{GCHint: "512MiB"}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12})
	var buf syncBuffer
	applyMemoryLimit(b, &buf, debug.SetMemoryLimit, func(int) int { return 0 }, func(string) string { return "" })
	if got, want := debug.SetMemoryLimit(-1), int64(512)<<20; got != want {
		t.Errorf("runtime memory limit after applyMemoryLimit = %d, want %d", got, want)
	}
}

// TestFormatMemoryLimit pins the startup line's unit-selection: an exact
// multiple of a unit renders as a whole number in that unit; anything else
// falls back to two-decimal GiB.
func TestFormatMemoryLimit(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{3 << 30, "3 GiB"},
		{512 << 20, "512 MiB"},
		{2 << 40, "2 TiB"},
		{1024, "1 KiB"},
		{700, "700 B"},
		{1610612736 + 1, "1.50 GiB"}, // one byte past 1.5GiB: not an exact multiple of any unit
	}
	for _, tc := range cases {
		if got := formatMemoryLimit(tc.bytes); got != tc.want {
			t.Errorf("formatMemoryLimit(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// TestUnlimitedMemoryLimit pins the "off" reading used by both /metrics'
// consumers and doctor's memoryLimitDisplay.
func TestUnlimitedMemoryLimit(t *testing.T) {
	if !unlimitedMemoryLimit(float64(9.3e18)) { // math.MaxInt64's neighborhood
		t.Error("want true for a value at math.MaxInt64's scale")
	}
	if unlimitedMemoryLimit(float64(3 << 30)) {
		t.Error("want false for an ordinary 3GiB reading")
	}
}

// TestMemoryLimitMetricLine proves /metrics' go_memory_limit_bytes line
// reflects whatever the runtime is currently enforcing.
func TestMemoryLimitMetricLine(t *testing.T) {
	prev := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(prev) })
	debug.SetMemoryLimit(7 << 30)

	line := memoryLimitMetricLine()
	if !strings.HasPrefix(line, "go_memory_limit_bytes ") {
		t.Fatalf("memoryLimitMetricLine() = %q, want the go_memory_limit_bytes prefix", line)
	}
	want := "go_memory_limit_bytes 7516192768\n"
	if line != want {
		t.Errorf("memoryLimitMetricLine() = %q, want %q", line, want)
	}
}
