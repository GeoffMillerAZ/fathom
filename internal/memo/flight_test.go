package memo

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights (review finding,
// RT-72): Keep holds a flight only past half a stated expected cost — a
// Keep group with no expected cost cancels like any other — and the refuse
// drop cancels every flight in progress, kept or not, and never caches its
// result.
func TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights(t *testing.T) {
	reg := NewRegistry()
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelNormal })
	g := NewGroupIn[skey, int](reg, "keep-no-cost", Options{WatermarkScoped: true, Keep: true})
	cancelled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	_, _ = g.Get(ctx, skey{"svc", "w1", "a"}, func(bctx context.Context) (int, int64, error) {
		<-bctx.Done()
		close(cancelled)
		return 0, 0, bctx.Err()
	})
	select {
	case <-cancelled:
	case <-time.After(waitSafety):
		t.Fatal("a Keep flight with no expected cost outlived its last waiter")
	}

	// Past half its expected cost the flight is kept… until refuse.
	kg := NewGroupIn[skey, int](reg, "keep-cost", Options{WatermarkScoped: true, Keep: true, ExpectedCost: time.Nanosecond})
	k := skey{"svc", "w1", "b"}
	started := make(chan struct{})
	stopped := make(chan struct{})
	kctx, kcancel := context.WithCancel(context.Background())
	go func() { <-started; kcancel() }()
	_, _ = kg.Get(kctx, k, func(bctx context.Context) (int, int64, error) {
		close(started)
		<-bctx.Done()
		close(stopped)
		return 9, 1, nil
	})
	select {
	case <-stopped:
		t.Fatal("a kept flight was cancelled when its waiter left")
	case <-time.After(30 * time.Millisecond):
	}
	reg.Drop(ReasonRefuse)
	select {
	case <-stopped:
	case <-time.After(waitSafety):
		t.Fatal("refuse did not cancel the flight in progress")
	}
	time.Sleep(10 * time.Millisecond)
	if _, ok := kg.Peek(k); ok {
		t.Fatal("a flight cancelled at refuse was cached")
	}
}
