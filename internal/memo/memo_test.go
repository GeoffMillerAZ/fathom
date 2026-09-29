package memo

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// resetDefault gives a test a clean Default registry and restores the
// published ladder level.
func resetDefault(t *testing.T) {
	t.Helper()
	old := Default
	Default = NewRegistry()
	tenancy.PublishLevel(tenancy.LevelNormal)
	t.Cleanup(func() {
		Default = old
		tenancy.PublishLevel(tenancy.LevelNormal)
	})
}

type skey struct{ svc, mark, name string }

func (k skey) MemoScope() (string, string) { return k.svc, k.mark }

// TestGroup_StampedeOneBuildCtxAwareWaiters is RT-72 (§5, I51): 50
// concurrent cold callers cause one build; a waiter whose ctx ends leaves
// at once; when every waiter has left the build is cancelled; a Keep
// group finishes a flight past half its expected cost.
func TestGroup_StampedeOneBuildCtxAwareWaiters(t *testing.T) {
	resetDefault(t)
	g := NewGroup[skey, int]("stampede", Options{WatermarkScoped: true})
	var builds atomic.Int32
	release := make(chan struct{})
	k := skey{"svc", "w1", "graph"}
	var wg sync.WaitGroup
	results := make([]int, 50)
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := g.Get(context.Background(), k, func(ctx context.Context) (int, int64, error) {
				builds.Add(1)
				<-release
				return 42, 8, nil
			})
			if err != nil {
				t.Errorf("Get: %v", err)
			}
			results[i] = v
		}(i)
	}
	for builds.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	// A ctx-bound waiter joins the same flight and leaves at its deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	// The flight's build stays blocked (release closes below), so a waiter
	// that ignored its ctx could never return: returning is "left at its
	// deadline", and the 20 ms is the caller's own input.
	var err error
	returnsUnaided(t, "a ctx-bound waiter on a blocked flight", func() {
		_, err = g.Get(ctx, k, func(context.Context) (int, int64, error) { t.Error("second build"); return 0, 0, nil })
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter err = %v, want deadline", err)
	}
	close(release)
	wg.Wait()
	if n := builds.Load(); n != 1 {
		t.Fatalf("builds = %d, want 1", n)
	}
	for i, v := range results {
		if v != 42 {
			t.Fatalf("result[%d] = %d", i, v)
		}
	}
	// Cached now: no build.
	if v, err := g.Get(context.Background(), k, func(context.Context) (int, int64, error) { t.Error("rebuilt a cached key"); return 0, 0, nil }); err != nil || v != 42 {
		t.Fatalf("cached Get = %d, %v", v, err)
	}

	// Last waiter leaves → the flight is cancelled.
	cancelled := make(chan struct{})
	k2 := skey{"svc", "w1", "other"}
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel2()
	}()
	_, err = g.Get(ctx2, k2, func(bctx context.Context) (int, int64, error) {
		<-bctx.Done()
		close(cancelled)
		return 0, 0, bctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(waitSafety):
		t.Fatal("flight not cancelled when its last waiter left")
	}
	if st := g.stats(); st.Cancelled != 1 || st.Flights != 2 {
		t.Fatalf("stats = %+v", st)
	}

	// Keep: the build survives its last waiter and is cached.
	kg := NewGroup[skey, int]("keep", Options{WatermarkScoped: true, Keep: true, ExpectedCost: time.Nanosecond})
	k3 := skey{"svc", "w1", "kept"}
	started := make(chan struct{})
	finish := make(chan struct{})
	ctx3, cancel3 := context.WithCancel(context.Background())
	go func() { <-started; cancel3() }()
	_, _ = kg.Get(ctx3, k3, func(bctx context.Context) (int, int64, error) {
		close(started)
		select {
		case <-finish:
		case <-bctx.Done():
			return 0, 0, bctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		return 7, 1, bctx.Err()
	})
	deadline := time.Now().Add(waitSafety)
	for {
		if v, ok := kg.Peek(k3); ok && v == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("kept flight was not cached")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(finish)
}

// TestGroup_ByteBudgetAndWatermarkEviction is RT-73 (§5, I51): groups
// share one byte budget with one LRU across them; a newer watermark for a
// service evicts its entries in every group; an empty watermark is built
// but never cached; Drop empties every group.
func TestGroup_ByteBudgetAndWatermarkEviction(t *testing.T) {
	resetDefault(t)
	Default.SetBudget(100, 0)
	a := NewGroup[skey, string]("a", Options{WatermarkScoped: true})
	b := NewGroup[skey, string]("b", Options{WatermarkScoped: true})
	ctx := context.Background()
	put := func(g *Group[skey, string], k skey, size int64) {
		t.Helper()
		if _, err := g.Get(ctx, k, func(context.Context) (string, int64, error) { return k.name, size, nil }); err != nil {
			t.Fatal(err)
		}
	}
	put(a, skey{"s1", "w1", "a1"}, 40)
	put(b, skey{"s2", "w1", "b1"}, 40)
	// Touch a1 so b1 is least recently used.
	put(a, skey{"s1", "w1", "a1"}, 40)
	put(a, skey{"s1", "w1", "a2"}, 40) // 120 > 100: b1 (LRU, other group) goes
	if _, ok := b.Peek(skey{"s2", "w1", "b1"}); ok {
		t.Fatal("LRU entry in the other group survived the shared budget")
	}
	if _, resident := Default.Budget(); resident != 80 {
		t.Fatalf("resident = %d, want 80", resident)
	}
	// A newer watermark for s1 evicts s1's entries in every group.
	put(b, skey{"s1", "w0", "b-old"}, 10) // older-mark build: s1 now at w0, evicting a1/a2
	if a.Len() != 0 {
		t.Fatalf("group a kept %d entries across a watermark move", a.Len())
	}
	put(a, skey{"s1", "w2", "a3"}, 10)
	if _, ok := b.Peek(skey{"s1", "w0", "b-old"}); ok {
		t.Fatal("entry at the superseded watermark survived")
	}
	// An empty watermark is never a key.
	var builds int
	for range 2 {
		_, _ = a.Get(ctx, skey{"s3", "", "nomark"}, func(context.Context) (string, int64, error) { builds++; return "x", 1, nil })
	}
	if builds != 2 || a.Len() != 1 {
		t.Fatalf("empty watermark cached: builds=%d len=%d", builds, a.Len())
	}
	// An entry larger than the budget is served but not cached.
	_, _ = a.Get(ctx, skey{"s1", "w2", "huge"}, func(context.Context) (string, int64, error) { return "h", 1000, nil })
	if _, ok := a.Peek(skey{"s1", "w2", "huge"}); ok {
		t.Fatal("oversize entry cached")
	}
	// EvictService (the refresh path's WatermarkBumped).
	EvictService("s1")
	if a.Len() != 0 {
		t.Fatal("EvictService left entries")
	}
	put(a, skey{"s4", "w1", "x"}, 10)
	put(b, skey{"s5", "w1", "y"}, 10)
	Drop(ReasonRefuse)
	if a.Len()+b.Len() != 0 {
		t.Fatal("Drop left entries")
	}
	if _, resident := Default.Budget(); resident != 0 {
		t.Fatalf("resident after Drop = %d", resident)
	}
	var sawRefuse, sawWatermark bool
	for _, s := range Stats() {
		sawRefuse = sawRefuse || s.Evictions[ReasonRefuse] > 0
		sawWatermark = sawWatermark || s.Evictions[ReasonWatermark] > 0
	}
	if !sawRefuse || !sawWatermark {
		t.Fatalf("evictions not counted by reason: %+v", Stats())
	}
}

// TestShedTrimToHalfSingleRebuildHysteresis is RT-163 (§5): at shed the
// registry trims by LRU to half the cache budget and keeps its resident
// bytes under that while shed, admitting one rebuild flight at a time; a
// daemon whose footprint is its base plus its caches, stepped through the
// real ladder, goes shed → rebuild → normal and does not re-enter shed.
func TestShedTrimToHalfSingleRebuildHysteresis(t *testing.T) {
	resetDefault(t)
	const mib = int64(1 << 20)
	budget := tenancy.Budget{Budget: 1000 * mib, Shed: 750 * mib, Refuse: 900 * mib, Exit: 2000 * mib, Caches: 400 * mib}
	Default.SetBudget(budget.Caches, 0)
	g := NewGroup[skey, int]("graphs", Options{WatermarkScoped: true})
	ctx := context.Background()
	for i := range 8 {
		k := skey{"svc", "w1", string(rune('a' + i))}
		_, _ = g.Get(ctx, k, func(context.Context) (int, int64, error) { return i, 50 * mib, nil })
	}
	if _, resident := Default.Budget(); resident != 400*mib {
		t.Fatalf("resident = %d MiB", resident/mib)
	}

	base := 400 * mib // the daemon's footprint without caches
	footprint := func() int64 { _, r := Default.Budget(); return base + r }
	var st tenancy.LadderState
	at := time.Unix(1_800_000_000, 0)
	step := func() tenancy.Transition {
		at = at.Add(5 * time.Second)
		s := tenancy.Sample{At: at, Footprint: footprint(), Children: 0, SwapUsed: tenancy.Unknown, SwapTotal: tenancy.Unknown, StoreFree: tenancy.Unknown, TmpFree: tenancy.Unknown, SwapVolumeFree: tenancy.Unknown}
		tr := tenancy.StepLadder(&st, &s, &budget)
		if tr.Changed && tr.To != tr.From {
			tenancy.PublishLevel(tr.To)
			if tr.To == tenancy.LevelShed && tr.From < tenancy.LevelShed {
				Trim(ReasonShed, 0)
			}
		}
		return tr
	}
	// 800 MiB > shed mark: two samples → shed, trim to 200 MiB.
	for range 3 {
		step()
	}
	if st.Level != tenancy.LevelShed {
		t.Fatalf("level = %s, want shed (footprint %d MiB)", st.Level, footprint()/mib)
	}
	if _, resident := Default.Budget(); resident > budget.Caches/2 {
		t.Fatalf("shed left %d MiB resident, want ≤ %d", resident/mib, budget.Caches/2/mib)
	}

	// While shed: concurrent rebuilds of evicted keys run one at a time,
	// and the registry never refills past half the cache budget.
	var running, maxRunning atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := skey{"svc", "w1", string(rune('a' + i))}
			_, _ = g.Get(ctx, k, func(context.Context) (int, int64, error) {
				n := running.Add(1)
				for {
					m := maxRunning.Load()
					if n <= m || maxRunning.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				running.Add(-1)
				return i, 50 * mib, nil
			})
		}(i)
	}
	wg.Wait()
	if m := maxRunning.Load(); m != 1 {
		t.Fatalf("%d rebuild flights ran at once while shed, want 1", m)
	}
	if _, resident := Default.Budget(); resident > budget.Caches/2 {
		t.Fatalf("rebuild while shed refilled to %d MiB", resident/mib)
	}
	// The ladder settles: footprint 600 MiB is under shed (750) but above
	// the step-down line (0.6 × 750 = 450): it holds shed, never refuses,
	// and never oscillates back up.
	for range 30 {
		tr := step()
		if tr.To > tenancy.LevelShed {
			t.Fatalf("rebuild pushed the ladder to %s", tr.To)
		}
	}
	// Once the base drops (the request that caused it finished), it steps
	// down and the cache budget is whole again, without re-entering shed.
	base = 100 * mib
	var sawNormal bool
	for range 30 {
		tr := step()
		if tr.To == tenancy.LevelNormal {
			sawNormal = true
		}
		if sawNormal && tr.To == tenancy.LevelShed {
			t.Fatal("re-entered shed after stepping down: no hysteresis")
		}
	}
	if !sawNormal {
		t.Fatalf("never stepped down (footprint %d MiB)", footprint()/mib)
	}
	for i := range 8 {
		k := skey{"svc", "w1", string(rune('i' + i))}
		_, _ = g.Get(ctx, k, func(context.Context) (int, int64, error) { return i, 50 * mib, nil })
		if tr := step(); tr.To != tenancy.LevelNormal {
			t.Fatalf("refilling the cache budget at normal moved the ladder to %s", tr.To)
		}
	}
}
