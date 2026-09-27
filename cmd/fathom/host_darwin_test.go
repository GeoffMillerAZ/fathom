//go:build darwin && cgo

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// childHost reads this process's REAL child tree (childTreeFootprint) and
// reports an injected own footprint.
type childHost struct{ own int64 }

func (h childHost) ReadHost(s *tenancy.Sample) {
	s.Footprint = h.own
	s.Children, s.ChildCount, _ = childTreeFootprint(os.Getpid())
	s.Pressure = tenancy.PressureNormal
}

// RT-161 (design §2): `go list`, git and rg run as children; their memory
// is not in the daemon's phys_footprint. The child tree's footprint is
// sampled, disclosed on /metrics, and counted toward the shed and refuse
// marks; the exit rung compares the daemon's own footprint and kills the
// tree first.
func TestChildFootprintCountedTowardShedAndRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal); activeGovernor.Store(nil) })
	before, n0, ok := childTreeFootprint(os.Getpid())
	if !ok {
		t.Fatalf("the child tree must be listable")
	}
	// A child with a grandchild: sh → sleep.
	cmd := exec.Command("/bin/sh", "-c", "/bin/sleep 60 & wait")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	var bytes int64
	var n int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		bytes, n, _ = childTreeFootprint(os.Getpid())
		if n >= n0+2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n < n0+2 || bytes <= before {
		t.Fatalf("the child tree (sh and its sleep) must be summed: %d processes, %d bytes (before %d, %d)", n, bytes, n0, before)
	}

	// Own footprint below the shed mark; own + children over it.
	b, _ := tenancy.ResolveBudget(tenancy.Config{EnvBudget: 64 * tenancy.MiB}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 8, Managed: true})
	own := b.Shed - bytes/2
	var logs []string
	g := newGovernor(governorConfig{
		budget: b, host: childHost{own: own}, inflight: tenancy.NewInFlight(nil),
		logf: func(m string) { logs = append(logs, m) }, exit: func(int) { t.Fatalf("no exit expected") },
	})
	now := time.Now()
	g.tick(now, 0)
	g.tick(now.Add(tenancy.SampleEvery), 0)
	if tenancy.CurrentLevel() < tenancy.LevelShed {
		t.Fatalf("own %d below shed %d, children %d: the ladder must count children (level %s)", own, b.Shed, bytes, tenancy.CurrentLevel())
	}
	if !strings.Contains(strings.Join(logs, "\n"), "children") {
		t.Fatalf("the transition names the children: %v", logs)
	}
	// Disclosed on /metrics.
	activeGovernor.Store(g)
	m := governorMetricLines()
	if !strings.Contains(m, "fathom_children_footprint_bytes ") || strings.Contains(m, "fathom_children_footprint_bytes 0\n") {
		t.Fatalf("/metrics discloses the children's footprint: %s", m)
	}
	if snap := g.snapshot(); snap.Children <= 0 || snap.ChildCount < 2 {
		t.Fatalf("the snapshot (resources) carries the children: %+v", snap)
	}

	// The exit rung kills the tree first: the real killer ends sh and sleep.
	if killed := killChildTree(os.Getpid()); killed < 2 {
		t.Fatalf("killChildTree signalled %d, want sh and sleep", killed)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("the child tree survived killChildTree")
	}
}

// The darwin host reader takes a sample without allocating (RT-171's
// zero-allocation claim, on the real host).
func TestDarwinHostSampleZeroAlloc(t *testing.T) {
	h := newTenancyHost(t.TempDir())
	rt := tenancy.NewRuntimeReader()
	var s tenancy.Sample
	tenancy.SampleHost(h, rt, time.Now(), 0, &s)
	if s.Footprint <= 0 || s.Pressure == tenancy.PressureUnknown || s.SwapTotal < 0 || s.StoreFree <= 0 || s.ChildDiskWritten < 0 {
		t.Fatalf("the darwin host reads footprint, pressure, swap, volume free and the children's bytes written: %+v (unknowns %v)", s, s.Unknowns())
	}
	now := time.Now()
	if a := testing.AllocsPerRun(50, func() { tenancy.SampleHost(h, rt, now, 0, &s) }); a != 0 {
		t.Fatalf("the darwin sampler allocates %.1f per sample", a)
	}
	if hostRAM() <= 0 {
		t.Fatalf("hw.memsize unread")
	}
}
