package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Review finding 8 (design §13.4): refuse-entry diagnostics are captured at
// most once per 10 minutes, however often the ladder oscillates.
func TestRefuseDiagCaptureRateLimited(t *testing.T) {
	r := newGovRig(t)
	for i := 0; i < 4; i++ { // four shed↔refuse oscillations, ~1 min apart
		r.host.set(4500*tenancy.MiB, 0)
		r.ticks(2)
		if tenancy.CurrentLevel() != tenancy.LevelRefuse {
			t.Fatalf("round %d: level %s", i, tenancy.CurrentLevel())
		}
		r.host.set(2*tenancy.GiB, 0)
		r.ticks(31) // 62 s below 0.6× steps refuse → shed
	}
	if n := r.saw("diag refuse"); n != 1 {
		t.Fatalf("refuse entries within 10 min captured %d diagnostics, want 1: %v", n, r.events)
	}
	r.now = r.now.Add(10 * time.Minute)
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if tenancy.CurrentLevel() == tenancy.LevelRefuse && r.saw("diag refuse") != 2 {
		t.Fatalf("a refuse entry 10 min after the last capture captures again: %v", r.events)
	}
}

// Review finding 8 (design §13.4): the capture directory keeps at most 10
// captures and 200 MiB, oldest pruned first.
func TestDiagCaptureRetention(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		d := filepath.Join(dir, base.Add(time.Duration(i)*time.Minute).Format("20060102T150405Z")+"-refuse")
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "heap.pb.gz"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := base.Add(time.Hour)
	got := captureDiagnostics(dir, "refuse", diagSources{inflight: tenancy.NewInFlight(nil), now: func() time.Time { return now }, metrics: func() string { return "" }})
	if got == "" {
		t.Fatal("capture wrote nothing")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) != 10 || names[9] != filepath.Base(got) || names[0] != base.Add(3*time.Minute).Format("20060102T150405Z")+"-refuse" {
		t.Fatalf("retention kept %d captures %v; want the newest 10 including %s", len(names), names, filepath.Base(got))
	}

	// The byte bound: three 90 MiB captures keep only what fits in 200 MiB.
	dir2 := t.TempDir()
	for i := 0; i < 3; i++ {
		d := filepath.Join(dir2, fmt.Sprintf("20260926T20%02d00Z-refuse", i))
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(d, "heap.pb.gz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(90 * tenancy.MiB); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	pruneDiagCaptures(dir2, 10, 200*tenancy.MiB)
	ents, _ = os.ReadDir(dir2)
	if len(ents) != 2 || ents[0].Name() != "20260926T200100Z-refuse" {
		var n []string
		for _, e := range ents {
			n = append(n, e.Name())
		}
		t.Fatalf("byte retention kept %v; want the newest two (180 MiB)", n)
	}
}

// Review finding 10: compile_layer is ingest-class work the exit rung
// halts, but it is not an ingest — its service is not quarantined as
// "exceeded budget during ingest" (auto-refresh keeps serving it). An
// onboard_service call is an ingest and is quarantined.
func TestExitHaltQuarantinesOnlyIngests(t *testing.T) {
	r := newGovRig(t)
	cctx, done1 := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "compile_layer", "layers-svc", "agent")
	defer done1()
	_, done2 := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "onboard_service", "new-svc", "agent")
	defer done2()
	r.host.set(9*tenancy.GiB, 0)
	r.ticks(2)
	if !errors.Is(context.Cause(cctx), tenancy.ErrHaltedOverBudget) {
		t.Fatalf("compile_layer is halted at the exit mark: cause %v", context.Cause(cctx))
	}
	if tenancy.IsQuarantined("layers-svc") {
		t.Fatal("a halted compile_layer quarantined its service as an ingest")
	}
	if !tenancy.IsQuarantined("new-svc") {
		t.Fatal("a halted onboard_service (an ingest) is quarantined")
	}
}

// RT-122 (design §13.4): threshold crossings capture — the first crossing
// of each power of two of the process's footprint, refuse entry and
// pressure critical — each kind at most once per 10 minutes; a capture
// holds the heap, labelled goroutines, the in-flight table, the GC CPU
// fraction and (while shed) the flight recorder's trace; the exit rung's
// capture is only the heap and the in-flight table; retention holds.
func TestDiagCaptureRateLimitedAndRetained(t *testing.T) {
	r := newGovRig(t)
	r.ticks(2) // exactly 1 GiB: not over the first power
	if n := len(r.events); n != 0 {
		t.Fatalf("1 GiB is not a crossing: %v", r.events)
	}
	r.host.set(1500*tenancy.MiB, 0)
	r.ticks(2)
	if r.saw("diag footprint-1GiB") != 1 {
		t.Fatalf("crossing 1 GiB captures once: %v", r.events)
	}
	// Down and back over 1 GiB: not a first crossing.
	r.host.set(512*tenancy.MiB, 0)
	r.ticks(2)
	r.host.set(1500*tenancy.MiB, 0)
	r.ticks(2)
	if r.saw("diag footprint-1GiB") != 1 {
		t.Fatalf("a re-crossing is not a first crossing: %v", r.events)
	}
	// 2 GiB within 10 min of the 1 GiB capture: the footprint kind is limited.
	r.host.set(2500*tenancy.MiB, 0)
	r.ticks(2)
	if r.saw("diag footprint-2GiB") != 0 {
		t.Fatalf("a second footprint capture within 10 min: %v", r.events)
	}
	// Pressure critical is its own kind: captured at once, once.
	r.host.mu.Lock()
	r.host.pressure = tenancy.PressureCritical
	r.host.mu.Unlock()
	r.ticks(3)
	if r.saw("diag pressure-critical") != 1 {
		t.Fatalf("pressure critical captures once on entry: %v", r.events)
	}
	// Refuse entry is its own kind too.
	r.host.mu.Lock()
	r.host.pressure = tenancy.PressureNormal
	r.host.mu.Unlock()
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if r.saw("diag refuse") != 1 {
		t.Fatalf("refuse entry captures: %v", r.events)
	}
	// 10 minutes on, crossing 8 GiB captures again (named for the highest power).
	r.now = r.now.Add(11 * time.Minute)
	r.g.budget.Exit = 64 * tenancy.GiB // keep the exit rung out of this test
	r.host.set(8500*tenancy.MiB, 0)
	r.ticks(1)
	if r.saw("diag footprint-8GiB") != 1 {
		t.Fatalf("crossing 8 GiB after the window captures: %v", r.events)
	}

	// The capture's contents.
	dir := t.TempDir()
	inf := tenancy.NewInFlight(nil)
	_, done := inf.Begin(context.Background(), tenancy.ClassTree, "references", "svc", "agent")
	defer done()
	fr := &flightRecorder{}
	if err := fr.start(); err != nil {
		t.Skipf("flight recorder unavailable here: %v", err)
	}
	defer fr.stop()
	src := diagSources{inflight: inf, now: func() time.Time { return time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC) },
		metrics: func() string { return "fathom_gc_cpu_fraction 0.25\n" }, flight: fr}
	d := captureDiagnostics(dir, "refuse", src)
	for _, f := range []string{"heap.pb.gz", "goroutine.txt", "inflight.json", "metrics.txt", "trace.out"} {
		if st, err := os.Stat(filepath.Join(d, f)); err != nil || st.Size() == 0 {
			t.Fatalf("refuse capture lacks %s: %v", f, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(d, "inflight.json")); !strings.Contains(string(b), "references") {
		t.Fatalf("inflight.json lacks the in-flight call: %s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "metrics.txt")); !strings.Contains(string(b), "fathom_gc_cpu_fraction") {
		t.Fatalf("metrics.txt lacks the GC CPU fraction: %s", b)
	}
	src.now = func() time.Time { return time.Date(2026, 9, 27, 1, 1, 0, 0, time.UTC) }
	start := time.Now()
	e := captureDiagnostics(dir, diagTriggerExit, src)
	if el := time.Since(start); el > time.Second {
		t.Fatalf("the exit capture took %s, over 1 s", el)
	}
	ents, _ := os.ReadDir(e)
	var names []string
	for _, x := range ents {
		names = append(names, x.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "heap.pb.gz,inflight.json" {
		t.Fatalf("the exit capture writes only the heap and in-flight table: %v", names)
	}
	// Retention: at most diagKeep captures.
	for i := 0; i < diagKeep+3; i++ {
		i := i
		src.now = func() time.Time { return time.Date(2026, 9, 27, 2, i, 0, 0, time.UTC) }
		captureDiagnostics(dir, diagTriggerExit, src)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != diagKeep {
		t.Fatalf("retention kept %d captures, want %d", len(ents), diagKeep)
	}
}

// RT-122 hygiene: the production capturer (metrics nil → the governor's own
// /metrics lines) runs outside g.mu. Under the lock, captureDiagnostics →
// governorMetricLines → g.snapshot → g.mu self-deadlocks the governor.
func TestProductionDiagCaptureDoesNotDeadlockGovernor(t *testing.T) {
	r := newGovRig(t)
	dir := t.TempDir()
	r.g.captureDiag = diagCapturer(dir, diagSources{inflight: tenancy.NewInFlight(nil), now: time.Now, flight: &flightRecorder{}})
	prev := activeGovernor.Load()
	activeGovernor.Store(r.g)
	t.Cleanup(func() { activeGovernor.Store(prev) })
	r.host.set(1500*tenancy.MiB, 0)
	done := make(chan struct{})
	go func() { r.ticks(1); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK: tick under g.mu -> captureDiagnostics -> governorMetricLines -> g.snapshot -> g.mu")
	}
	s := tenancyStatus(r.g)
	if s.LastDiag == nil || !strings.Contains(s.LastDiag.Path, "footprint-1GiB") {
		t.Fatalf("the capture is recorded after the tick: %+v", s.LastDiag)
	}
	b, err := os.ReadFile(filepath.Join(s.LastDiag.Path, "metrics.txt"))
	if err != nil || !strings.Contains(string(b), "fathom_footprint_bytes") {
		t.Fatalf("metrics.txt carries the governor's own lines: %v %s", err, b)
	}
}

// A footprint crossing the rate limit suppressed is deferred, not spent: a
// fast 1 → 4 GiB climb inside one window keeps the 1 GiB capture, and when
// the window reopens with the footprint still over 4 GiB, the 4 GiB
// capture is taken — the more informative heap is never lost to the limit.
// A footprint that fell back below every uncaptured power takes nothing.
func TestDiagFootprintCrossingDeferredNotDropped(t *testing.T) {
	r := newGovRig(t)
	r.g.budget.Exit = 64 * tenancy.GiB // keep the exit rung out of this test
	r.host.set(1500*tenancy.MiB, 0)
	r.ticks(1)
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if r.saw("diag footprint-1GiB") != 1 || r.saw("diag footprint-4GiB") != 0 {
		t.Fatalf("inside the window only the first capture: %v", r.events)
	}
	r.now = r.now.Add(diagEvery)
	r.ticks(1)
	if r.saw("diag footprint-4GiB") != 1 {
		t.Fatalf("the window reopened over 4 GiB: the deferred crossing is captured: %v", r.events)
	}
	r.ticks(3)
	if r.saw("diag footprint-4GiB") != 1 {
		t.Fatalf("a deferred crossing is captured once: %v", r.events)
	}

	// Fell back before the window reopened: nothing is captured late.
	r2 := newGovRig(t)
	r2.g.budget.Exit = 64 * tenancy.GiB
	r2.host.set(1500*tenancy.MiB, 0)
	r2.ticks(1)
	r2.host.set(2500*tenancy.MiB, 0)
	r2.ticks(1)
	r2.host.set(900*tenancy.MiB, 0)
	r2.now = r2.now.Add(diagEvery)
	r2.ticks(2)
	if r2.saw("diag footprint-2GiB") != 0 {
		t.Fatalf("a crossing the footprint fell back from is not captured late: %v", r2.events)
	}
}
