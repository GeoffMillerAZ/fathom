package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

// TestMemoryLimitFromEnvOrConfig pins Task 2's precedence: FATHOM_MEMORY_LIMIT
// always wins over [daemon].memory_limit, which wins over defaultMemoryLimit
// — the same env-wins-over-config rule FATHOM_CALL_LOG already follows.
func TestMemoryLimitFromEnvOrConfig(t *testing.T) {
	t.Run("unset env and config falls back to the default", func(t *testing.T) {
		t.Setenv("FATHOM_MEMORY_LIMIT", "")
		if got := memoryLimitFromEnvOrConfig(""); got != defaultMemoryLimit {
			t.Errorf("got %q, want default %q", got, defaultMemoryLimit)
		}
	})
	t.Run("config wins over the default", func(t *testing.T) {
		t.Setenv("FATHOM_MEMORY_LIMIT", "")
		if got := memoryLimitFromEnvOrConfig("1GiB"); got != "1GiB" {
			t.Errorf("got %q, want %q", got, "1GiB")
		}
	})
	t.Run("env wins over config", func(t *testing.T) {
		t.Setenv("FATHOM_MEMORY_LIMIT", "2GiB")
		if got := memoryLimitFromEnvOrConfig("1GiB"); got != "2GiB" {
			t.Errorf("got %q, want %q (env must win)", got, "2GiB")
		}
	})
	t.Run("env wins even over an unset config", func(t *testing.T) {
		t.Setenv("FATHOM_MEMORY_LIMIT", "2GiB")
		if got := memoryLimitFromEnvOrConfig(""); got != "2GiB" {
			t.Errorf("got %q, want %q", got, "2GiB")
		}
	})
}

// TestApplyMemoryLimit_Applies proves the resolved value is actually
// applied through setLimit (never called for "off"), and that the
// startup line matches Task 2's exact wording.
func TestApplyMemoryLimit_Applies(t *testing.T) {
	t.Setenv("FATHOM_MEMORY_LIMIT", "")

	t.Run("a concrete size calls setLimit and prints the soft-limit line", func(t *testing.T) {
		var applied int64 = -999
		var calls int
		setLimit := func(b int64) int64 {
			calls++
			applied = b
			return 0
		}
		var buf syncBuffer
		if err := applyMemoryLimit("3GiB", &buf, setLimit); err != nil {
			t.Fatalf("applyMemoryLimit: %v", err)
		}
		if calls != 1 {
			t.Fatalf("setLimit called %d times, want 1", calls)
		}
		if want := int64(3) << 30; applied != want {
			t.Errorf("applied = %d, want %d", applied, want)
		}
		if got := buf.String(); got != "serve-http: memory limit 3 GiB (soft, runtime.SetMemoryLimit)\n" {
			t.Errorf("startup line = %q", got)
		}
	})

	t.Run(`"off" disables the limit and never calls setLimit`, func(t *testing.T) {
		called := false
		setLimit := func(b int64) int64 {
			called = true
			return 0
		}
		var buf syncBuffer
		if err := applyMemoryLimit("off", &buf, setLimit); err != nil {
			t.Fatalf("applyMemoryLimit: %v", err)
		}
		if called {
			t.Error("setLimit was called with memory_limit = off, want it left untouched")
		}
		if got := buf.String(); got != "serve-http: memory limit off\n" {
			t.Errorf("startup line = %q", got)
		}
	})

	t.Run("garbage is refused and setLimit is never called", func(t *testing.T) {
		called := false
		setLimit := func(b int64) int64 {
			called = true
			return 0
		}
		var buf syncBuffer
		err := applyMemoryLimit("3 gigs", &buf, setLimit)
		if err == nil {
			t.Fatal("applyMemoryLimit(garbage): want error, got none")
		}
		if called {
			t.Error("setLimit was called despite a parse failure")
		}
	})
}

// TestApplyMemoryLimit_ActuallyAppliesToRuntime uses the REAL
// runtime/debug.SetMemoryLimit (Task 2's own test instruction: "the limit
// applied — read it back with debug.SetMemoryLimit(-1)"), restoring
// whatever this process's limit was before the test so it never leaks
// into another test in this binary.
func TestApplyMemoryLimit_ActuallyAppliesToRuntime(t *testing.T) {
	prev := debug.SetMemoryLimit(-1) // query only — a negative input changes nothing
	t.Cleanup(func() { debug.SetMemoryLimit(prev) })

	var buf syncBuffer
	if err := applyMemoryLimit("512MiB", &buf, debug.SetMemoryLimit); err != nil {
		t.Fatalf("applyMemoryLimit: %v", err)
	}
	got := debug.SetMemoryLimit(-1)
	if want := int64(512) << 20; got != want {
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
