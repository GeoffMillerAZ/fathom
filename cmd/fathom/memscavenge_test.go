package main

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

// TestRunMemoryScavenger_CallsFreeOnEveryTick pins O108's mitigation: the
// scavenger calls free (production: debug.FreeOSMemory) once per interval
// until its context is cancelled, and never before the first interval
// elapses (a scavenge pass forces a full GC — it must not run on every
// request, only on the configured cadence).
func TestRunMemoryScavenger_CallsFreeOnEveryTick(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	go func() {
		runMemoryScavenger(ctx, 5*time.Millisecond, log, func() { calls.Add(1) }, nil)
		close(done)
	}()

	// Let several ticks fire, then cancel and confirm the goroutine exits.
	time.Sleep(45 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMemoryScavenger did not exit after ctx cancellation")
	}

	if got := calls.Load(); got < 3 {
		t.Fatalf("free called %d times in 45ms at a 5ms interval, want at least 3", got)
	}
}

// TestRunMemoryScavenger_ZeroIntervalIsNoOp confirms interval <= 0 returns
// immediately rather than spinning a busy ticker loop (time.NewTicker
// panics on a non-positive duration).
func TestRunMemoryScavenger_ZeroIntervalIsNoOp(t *testing.T) {
	called := false
	done := make(chan struct{})
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	go func() {
		runMemoryScavenger(context.Background(), 0, log, func() { called = true }, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMemoryScavenger(interval<=0) did not return promptly")
	}
	if called {
		t.Error("free was called with a zero interval; want a no-op")
	}
}

// TestRunMemoryScavenger_LogsHeapDelta pins DG-4 as RT-45 restated it: the
// scavenge line must be self-contained evidence of what a pass did, not
// just that one ran. Both the before and after readings of total−released
// and phys_footprint appear in the same line (heap_sys, a high-water mark
// that never falls, was the wrong instrument).
func TestRunMemoryScavenger_LogsHeapDelta(t *testing.T) {
	var buf strings.Builder
	log := dlog.New(&buf, dlog.LevelInfo, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		runMemoryScavenger(ctx, 5*time.Millisecond, log, func() {}, nil)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMemoryScavenger did not exit after ctx cancellation")
	}

	out := buf.String()
	for _, field := range []string{
		"total_minus_released_before_bytes", "total_minus_released_after_bytes",
		"phys_footprint_before_bytes", "phys_footprint_after_bytes",
	} {
		if !strings.Contains(out, field) {
			t.Errorf("scavenge log missing %q; want both before and after readings in the same line:\n%s", field, out)
		}
	}
	if !strings.Contains(out, "scavenge (tick): released") {
		t.Errorf("scavenge log missing the released line:\n%s", out)
	}
}

// TestScavengeIntervalFromEnv_DefaultsAndOverrides pins the env-driven
// cadence: unset/invalid falls back to defaultScavengeInterval; a valid
// duration string overrides it.
func TestScavengeIntervalFromEnv_DefaultsAndOverrides(t *testing.T) {
	t.Run("unset uses the default", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE_INTERVAL", "")
		if got := scavengeIntervalFromEnv(); got != defaultScavengeInterval {
			t.Errorf("got %s, want default %s", got, defaultScavengeInterval)
		}
	})
	t.Run("valid override wins", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE_INTERVAL", "5m")
		if got := scavengeIntervalFromEnv(); got != 5*time.Minute {
			t.Errorf("got %s, want 5m", got)
		}
	})
	t.Run("invalid value falls back to the default", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE_INTERVAL", "not-a-duration")
		if got := scavengeIntervalFromEnv(); got != defaultScavengeInterval {
			t.Errorf("got %s, want default %s", got, defaultScavengeInterval)
		}
	})
	t.Run("non-positive value falls back to the default", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE_INTERVAL", "-5m")
		if got := scavengeIntervalFromEnv(); got != defaultScavengeInterval {
			t.Errorf("got %s, want default %s", got, defaultScavengeInterval)
		}
	})
}

// TestScavengeDisabled pins the FATHOM_SCAVENGE=off escape hatch.
func TestScavengeDisabled(t *testing.T) {
	t.Run("unset means enabled", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE", "")
		if scavengeDisabled() {
			t.Error("scavengeDisabled() = true with FATHOM_SCAVENGE unset, want false")
		}
	})
	t.Run("off disables it", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE", "off")
		if !scavengeDisabled() {
			t.Error("scavengeDisabled() = false with FATHOM_SCAVENGE=off, want true")
		}
	})
	t.Run("case-insensitive", func(t *testing.T) {
		t.Setenv("FATHOM_SCAVENGE", "OFF")
		if !scavengeDisabled() {
			t.Error("scavengeDisabled() = false with FATHOM_SCAVENGE=OFF, want true")
		}
	})
}
