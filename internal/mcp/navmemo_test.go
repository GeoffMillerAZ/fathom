package mcp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// navMemoFake counts full symbol loads; gate holds the first load so
// concurrent cold callers pile up behind it.
type navMemoFake struct {
	refreshFake
	testStore
	loads atomic.Int32
	mark  atomic.Value
	gate  chan struct{}
}

func (f *navMemoFake) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	f.loads.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	return []core.SymbolNode{{Name: "A", FilePath: "a.go", Calls: []string{"B"}}, {Name: "B", FilePath: "b.go"}}, nil
}

func (f *navMemoFake) StoreWatermark(context.Context, string) (string, error) {
	return f.mark.Load().(string), nil
}

// The wave-A incident replay (docs/bench/readings/rt-wave-a-incident-replay-
// 2026-09-26.md): 20 concurrent symbol_context calls each loaded every
// symbol and built the service's call graph (2.45–9.78 GB allocated per
// call, ~6 GiB footprint). One load and one build per (service,
// watermark), shared by concurrent callers.
func TestLoadNavGraphs_ConcurrentColdCallsLoadOnce(t *testing.T) {
	f := &navMemoFake{gate: make(chan struct{})}
	f.mark.Store("w1")
	s := NewServer(f)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if g := s.loadNavGraphs(context.Background(), f, "svc"); g == nil || len(g.Nodes) != 2 {
				t.Errorf("loadNavGraphs = %+v", g)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(f.gate)
	wg.Wait()
	if n := f.loads.Load(); n != 1 {
		t.Fatalf("16 concurrent cold callers loaded all symbols %d times, want 1", n)
	}
	s.loadNavGraphs(context.Background(), f, "svc")
	if n := f.loads.Load(); n != 1 {
		t.Fatalf("a warm call on an unchanged watermark reloaded symbols (%d loads)", n)
	}
	f.mark.Store("w2")
	s.loadNavGraphs(context.Background(), f, "svc")
	if n := f.loads.Load(); n != 2 {
		t.Fatalf("a new watermark must rebuild exactly once, loads = %d", n)
	}
}

// countingFleet records how many fleet-wide scans overlap.
type countingFleet struct {
	cur, peak atomic.Int32
}

func (c *countingFleet) Services() []string { return []string{"a", "b"} }
func (c *countingFleet) SymbolsFor(string) []core.SymbolNode {
	n := c.cur.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	c.cur.Add(-1)
	return nil
}
func (c *countingFleet) ObservationsFor(string) []core.Observation     { return nil }
func (c *countingFleet) UnsupportedLanguagesFor(string) map[string]int { return nil }

// Each fleetCallers call loads every service's symbols and observations;
// concurrent calls multiply that. One scan at a time bounds the peak until
// wave C replaces the scan with a stored reverse index (RT-84).
func TestFleetCallers_OneScanAtATime(t *testing.T) {
	c := &countingFleet{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); fleetCallers(c, "X") }()
	}
	wg.Wait()
	if p := c.peak.Load(); p != 1 {
		t.Fatalf("%d fleet scans overlapped, want 1", p)
	}
}
