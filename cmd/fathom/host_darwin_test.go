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
	deadline := time.Now().Add(waitSafety)
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
	case <-time.After(waitSafety):
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

// ownPinnedHost is the REAL darwin host (newTenancyHost's darwinHost,
// whose ReadHost walks the child tree through fathom_read_host) with the
// readings other than the children pinned: own footprint injected, pressure
// normal, swap and the swap volume unknown, so only the children the real
// host sampled can move the ladder.
type ownPinnedHost struct {
	real *darwinHost
	own  int64
}

func (h ownPinnedHost) ReadHost(s *tenancy.Sample) {
	h.real.ReadHost(s)
	s.Footprint = h.own
	s.Pressure = tenancy.PressureNormal
	s.SwapUsed, s.SwapTotal, s.SwapVolumeFree = tenancy.Unknown, tenancy.Unknown, tenancy.Unknown
}

// RT-161 on the product path: the governor's host is newTenancyHost's
// darwinHost, and its ReadHost fills Sample.Children and ChildCount through
// the cgo fathom_read_host every tick (childTreeFootprint is only a test
// wrapper over the same C walk). The real host samples a child tree, the
// reading falls once the tree is killed, and the real host's children count
// toward the shed and the refuse marks.
func TestRealHostChildTreeCountedTowardShedAndRefuse(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal); activeGovernor.Store(nil) })
	h := newTenancyHost(t.TempDir())
	real, ok := h.(*darwinHost)
	if !ok {
		t.Fatalf("newTenancyHost on darwin must build the darwin host, got %T", h)
	}
	rt := tenancy.NewRuntimeReader()
	var s tenancy.Sample
	tenancy.SampleHost(h, rt, time.Now(), 0, &s)
	if s.Children < 0 {
		t.Fatalf("the real host must list the child tree: %+v", s)
	}
	before, n0 := s.Children, s.ChildCount

	// A child with a grandchild: sh → sleep.
	cmd := exec.Command("/bin/sh", "-c", "/bin/sleep 60 & wait")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	deadline := time.Now().Add(waitSafety)
	for time.Now().Before(deadline) {
		tenancy.SampleHost(h, rt, time.Now(), 0, &s)
		if s.ChildCount >= n0+2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if s.ChildCount < n0+2 || s.Children <= before || s.Children <= 0 || s.ChildCount < 1 {
		t.Fatalf("the real host must sample the child tree (sh and its sleep): %d processes, %d bytes (before %d, %d)", s.ChildCount, s.Children, n0, before)
	}
	children, count := s.Children, s.ChildCount

	b, _ := tenancy.ResolveBudget(tenancy.Config{EnvBudget: 64 * tenancy.MiB}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 8, Managed: true})
	// ladder ticks a governor over the real host with own footprint pinned
	// and returns the level after two samples, and its log.
	ladder := func(own int64) (tenancy.Level, string, *governor) {
		tenancy.PublishLevel(tenancy.LevelNormal)
		var logs []string
		g := newGovernor(governorConfig{
			budget: b, host: ownPinnedHost{real: real, own: own}, inflight: tenancy.NewInFlight(nil),
			logf: func(m string) { logs = append(logs, m) }, exit: func(int) { t.Fatalf("no exit expected") },
		})
		now := time.Now()
		g.tick(now, 0)
		g.tick(now.Add(tenancy.SampleEvery), 0)
		return tenancy.CurrentLevel(), strings.Join(logs, "\n"), g
	}

	// Own footprint below the shed mark; own + the real children over it.
	shedOwn := b.Shed - children/2
	lvl, logs, g := ladder(shedOwn)
	if lvl < tenancy.LevelShed {
		t.Fatalf("own %d below shed %d, real children %d: the ladder must count them (level %s)", shedOwn, b.Shed, children, lvl)
	}
	if !strings.Contains(logs, "children") {
		t.Fatalf("the shed transition names the children: %s", logs)
	}
	activeGovernor.Store(g)
	if m := governorMetricLines(); !strings.Contains(m, "fathom_children_footprint_bytes ") || strings.Contains(m, "fathom_children_footprint_bytes 0\n") {
		t.Fatalf("/metrics discloses the real host's children: %s", m)
	}
	if snap := g.snapshot(); snap.Children <= 0 || snap.ChildCount < 2 {
		t.Fatalf("the snapshot (resources) carries the real host's children: %+v", snap)
	}
	activeGovernor.Store(nil)

	// Own footprint below the refuse mark; own + the real children over it.
	refuseOwn := b.Refuse - children/2
	if lvl, logs, _ := ladder(refuseOwn); lvl != tenancy.LevelRefuse {
		t.Fatalf("own %d below refuse %d, real children %d: the ladder must refuse (level %s): %s", refuseOwn, b.Refuse, children, lvl, logs)
	}

	// Killed, the tree's reading falls, and the same own footprint no
	// longer crosses the shed mark: the children were what crossed it.
	if killed := killChildTree(os.Getpid()); killed < 2 {
		t.Fatalf("killChildTree signalled %d, want sh and sleep", killed)
	}
	_, _ = cmd.Process.Wait()
	deadline = time.Now().Add(waitSafety)
	for time.Now().Before(deadline) {
		tenancy.SampleHost(h, rt, time.Now(), 0, &s)
		if s.ChildCount < count && s.Children < children {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if s.ChildCount >= count || s.Children >= children {
		t.Fatalf("after the kill the real host's reading must fall: %d processes, %d bytes (was %d, %d)", s.ChildCount, s.Children, count, children)
	}
	if lvl, logs, _ := ladder(shedOwn); lvl != tenancy.LevelNormal {
		t.Fatalf("without the children own %d stays below shed %d (level %s): %s", shedOwn, b.Shed, lvl, logs)
	}
}
