package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	got := captureExitDiag(dir, tenancy.NewInFlight(nil), func() time.Time { return now })("refuse")
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
