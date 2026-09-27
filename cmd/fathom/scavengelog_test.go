package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// fakeScavengeMem returns scripted (total−released, phys_footprint)
// readings: before and after each pass.
type fakeScavengeMem struct {
	mu   sync.Mutex
	seq  []scavengeMem
	next int
}

func (f *fakeScavengeMem) read() scavengeMem {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.seq[f.next%len(f.seq)]
	f.next++
	return m
}

// TestScavengerLogsReleasedAndFootprint pins RT-45 (design §13.3, §2.1):
// the scavenge line reads total−released (runtime/metrics, no
// stop-the-world ReadMemStats) and phys_footprint before and after the
// pass and names what was released — on the periodic tick and on every
// governor event (shed entry, the 30 s cadence while shed, refuse entry,
// the exit-mark halt). HeapSys is the wrong instrument (a high-water mark
// that never falls) and is gone from the line.
func TestScavengerLogsReleasedAndFootprint(t *testing.T) {
	mem := &fakeScavengeMem{seq: []scavengeMem{
		{TotalMinusReleased: 900 * tenancy.MiB, Footprint: 1200 * tenancy.MiB},
		{TotalMinusReleased: 400 * tenancy.MiB, Footprint: 700 * tenancy.MiB},
	}}

	t.Run("tick", func(t *testing.T) {
		var buf strings.Builder
		var mu sync.Mutex
		log := dlog.New(&lockedWriter{w: &buf, mu: &mu}, dlog.LevelInfo, time.Now)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { runMemoryScavenger(ctx, 5*time.Millisecond, log, func() {}, mem.read); close(done) }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			mu.Lock()
			out := buf.String()
			mu.Unlock()
			if strings.Contains(out, "scavenge (tick)") || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
		<-done
		mu.Lock()
		out := buf.String()
		mu.Unlock()
		for _, want := range []string{
			"scavenge (tick)", "released 500 MiB",
			"total_minus_released_before_bytes", "total_minus_released_after_bytes",
			"phys_footprint_before_bytes", "phys_footprint_after_bytes",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("tick line missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "heap_sys") {
			t.Errorf("tick line still reports heap_sys (the wrong instrument):\n%s", out)
		}
	})

	t.Run("events", func(t *testing.T) {
		r := newGovRig(t)
		var lines []string
		r.g.logf = func(s string) { lines = append(lines, s) }
		r.g.memRead = mem.read
		r.host.set(3500*tenancy.MiB, 0) // ≥ shed (3 GiB)
		r.ticks(2)
		r.ticks(15) // 30 s while shed
		r.host.set(4500*tenancy.MiB, 0)
		r.ticks(2)
		var got []string
		for _, l := range lines {
			if strings.HasPrefix(l, "scavenge (") {
				got = append(got, l)
			}
		}
		joined := strings.Join(got, "\n")
		for _, want := range []string{"scavenge (shed entry)", "scavenge (shed, every 30s)", "scavenge (refuse entry)"} {
			if !strings.Contains(joined, want) {
				t.Errorf("no %q line; scavenge lines:\n%s", want, joined)
			}
		}
		for _, l := range got {
			if !strings.Contains(l, "phys_footprint 1.2 GiB → 700 MiB") || !strings.Contains(l, "total−released 900 MiB → 400 MiB") {
				t.Errorf("event line lacks the before/after readings: %s", l)
			}
		}
	})
}

type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
