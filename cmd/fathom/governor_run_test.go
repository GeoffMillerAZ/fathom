package main

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Review finding 11: lateness is how far a sample is behind its schedule —
// the later of the previous tick's start plus the period and the previous
// tick's end — so the governor's own slow tick (FreeOSMemory, the heap
// capture, cache drops) never counts as a host stall.
func TestSampleLatenessExcludesOwnSlowTick(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	p := tenancy.SampleEvery
	for _, tc := range []struct {
		name        string
		prevEnd, at time.Time
		want        time.Duration
	}{
		{"on time", t0.Add(10 * time.Millisecond), t0.Add(p), 0},
		{"host stall after a fast tick", t0.Add(10 * time.Millisecond), t0.Add(p + 7*time.Second), 7 * time.Second},
		{"own 9 s tick, buffered tick received at once", t0.Add(9 * time.Second), t0.Add(9*time.Second + time.Millisecond), time.Millisecond},
		{"own slow tick, then a stall", t0.Add(9 * time.Second), t0.Add(16 * time.Second), 7 * time.Second},
		{"early", t0.Add(time.Millisecond), t0.Add(p - time.Millisecond), 0},
	} {
		if got := sampleLateness(t0, tc.prevEnd, tc.at); got != tc.want {
			t.Errorf("%s: lateness %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Review finding 11: runServeHTTP returning (here: a bad --addr, after the
// GC hint was applied) restores the process's memory limit — an in-process
// test never inherits the daemon's GC hint — and leaves no governor active.
func TestRunServeHTTPRestoresMemoryLimitAndStopsGovernor(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	before := debug.SetMemoryLimit(-1)
	var stdout, stderr bytes.Buffer
	code := runServeHTTP([]string{"--store", filepath.Join(projectDir, "s.sqlite"), "--addr", "127.0.0.1:999999", "--auto-refresh=false"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("bad addr must fail: %s %s", stdout.String(), stderr.String())
	}
	if got := debug.SetMemoryLimit(-1); got != before {
		t.Fatalf("memory limit after runServeHTTP returned = %d, want the prior %d", got, before)
	}
	if activeGovernor.Load() != nil {
		t.Fatal("a returned runServeHTTP left its governor active")
	}
}

// Review finding 11: startGovernor's stop ends sampling and unpublishes
// the governor and its level.
func TestStartGovernorStop(t *testing.T) {
	dir := t.TempDir()
	b, notes := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 8})
	g, stop := startGovernor(b, notes, filepath.Join(dir, "s.sqlite"), filepath.Join(dir, "daemon", "t", "record.json"), "", ladderActions{}, dlog.New(io.Discard, dlog.LevelDebug, time.Now))
	if activeGovernor.Load() != g {
		t.Fatal("startGovernor publishes its governor")
	}
	tenancy.PublishLevel(tenancy.LevelShed)
	stop()
	if activeGovernor.Load() != nil || tenancy.CurrentLevel() != tenancy.LevelNormal {
		t.Fatalf("stop leaves governor %v, level %s", activeGovernor.Load(), tenancy.CurrentLevel())
	}
}
