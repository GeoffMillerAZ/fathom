package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
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
	go func() {
		runMemoryScavenger(ctx, 5*time.Millisecond, &syncBuffer{}, func() { calls.Add(1) })
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
	go func() {
		runMemoryScavenger(context.Background(), 0, &syncBuffer{}, func() { called = true })
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
