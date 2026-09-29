package memo

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestGroup_UpdateRunsOutsideTheRegistryLock (review finding, SU-4 at the
// graph store): a refresh's patch re-indexes a whole snapshot, and while it
// ran under the registry-wide lock every memo Get in the process stalled —
// a hit on an unrelated group included. The patch runs unlocked and is
// committed only if the entry it patched is still the one cached.
func TestGroup_UpdateRunsOutsideTheRegistryLock(t *testing.T) {
	reg := NewRegistry()
	a := NewGroupIn[string, int](reg, "a", Options{})
	b := NewGroupIn[string, int](reg, "b", Options{})
	ctx := context.Background()
	build := func(v int) func(context.Context) (int, int64, error) {
		return func(context.Context) (int, int64, error) { return v, 1, nil }
	}
	if _, err := a.Get(ctx, "k", build(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(ctx, "k", build(2)); err != nil {
		t.Fatal(err)
	}
	inPatch, release := make(chan struct{}), make(chan struct{})
	done := make(chan bool)
	go func() {
		done <- a.Update("k", func(old int) (int, int64) {
			close(inPatch)
			<-release
			return old + 10, -1
		})
	}()
	<-inPatch
	hit := make(chan int)
	go func() {
		v, _ := b.Get(ctx, "k", build(99))
		hit <- v
	}()
	select {
	case v := <-hit:
		if v != 2 {
			t.Fatalf("hit = %d, want 2", v)
		}
	case <-time.After(waitSafety):
		close(release)
		t.Fatal("a Get on another group waited for a patch in progress")
	}
	close(release)
	if !<-done {
		t.Fatal("Update reported nothing cached")
	}
	if v, _ := a.Peek("k"); v != 11 {
		t.Fatalf("patched value = %d, want 11", v)
	}
}

// TestGroup_InvalidationDuringAFlightIsNeverCached (review finding, O99 at
// the graph store): a single-file refresh does not move the watermark, so
// a load that was in flight when the refresh patched (or dropped) the key
// would cache its pre-refresh result under the unchanged watermark and
// serve it until the next ingest. A flight overtaken by Update or Remove
// answers its own waiters but is never cached, and a caller arriving after
// the invalidation starts a fresh build rather than joining it.
func TestGroup_InvalidationDuringAFlightIsNeverCached(t *testing.T) {
	for _, inv := range []struct {
		name string
		do   func(g *Group[skey, int], k skey)
	}{
		{"remove", func(g *Group[skey, int], k skey) { g.Remove(k, ReasonWatermark) }},
		{"update", func(g *Group[skey, int], k skey) { g.Update(k, func(v int) (int, int64) { return v, -1 }) }},
	} {
		t.Run(inv.name, func(t *testing.T) {
			reg := NewRegistry()
			g := NewGroupIn[skey, int](reg, "flight-inv", Options{WatermarkScoped: true})
			k := skey{svc: "svc", mark: "w1", name: "k"}
			ctx := context.Background()
			var builds atomic.Int32
			started, release := make(chan struct{}), make(chan struct{})
			first := make(chan int)
			go func() {
				v, _ := g.Get(ctx, k, func(context.Context) (int, int64, error) {
					builds.Add(1)
					close(started)
					<-release
					return 1, 1, nil // the pre-refresh state
				})
				first <- v
			}()
			<-started
			inv.do(g, k)
			tctx, cancel := context.WithTimeout(ctx, waitSafety)
			defer cancel()
			v2, err := g.Get(tctx, k, func(context.Context) (int, int64, error) {
				builds.Add(1)
				return 2, 1, nil // the post-refresh state
			})
			if err != nil || v2 != 2 {
				close(release)
				t.Fatalf("a caller after the invalidation got %d, %v; want a fresh build (2)", v2, err)
			}
			close(release)
			if v := <-first; v != 1 {
				t.Fatalf("the overtaken flight's waiter got %d, want its own build (1)", v)
			}
			if v, ok := g.Peek(k); !ok || v != 2 {
				t.Fatalf("cached = %d, %v; want the post-refresh value 2 (the overtaken flight must not overwrite it)", v, ok)
			}
			if builds.Load() != 2 {
				t.Fatalf("builds = %d, want 2", builds.Load())
			}
		})
	}
}
