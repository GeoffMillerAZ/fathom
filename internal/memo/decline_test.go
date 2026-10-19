package memo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// stepClock is an injected clock a test moves by hand: a build "takes" as
// long as the test advances it, so the recorded build duration is exact.
type stepClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *stepClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

// TestGetDeclinesAFlightItsCallerCannotUse is RT-191 (§5 "No doomed
// flights", O1014, I38): a Get on a Keep group whose ctx deadline is shorter
// than the last completed build of its (group, service) — ExpectedCost when
// none is known — starts no flight unless the flight would be kept (the
// deadline at least half ExpectedCost and the build's size within the limit
// in force). Otherwise it answers ErrWouldNotFinish at once, counts
// Declined and takes no rebuild token. No deadline, or a nested build,
// always starts; a kept build its caller left still completes for the next
// caller; a refuse drop counts RefuseCancelled, never Abandoned (RT-188).
func TestGetDeclinesAFlightItsCallerCannotUse(t *testing.T) {
	clk := &stepClock{at: time.Now()}
	level := tenancy.LevelNormal
	var levelMu sync.Mutex
	setLevel := func(l tenancy.Level) { levelMu.Lock(); level = l; levelMu.Unlock() }
	reg := NewRegistry()
	reg.now = clk.Now
	reg.SetLevelSource(func() tenancy.Level { levelMu.Lock(); defer levelMu.Unlock(); return level })
	const cost = 2 * time.Second
	g := NewGroupIn[skey, int](reg, "decline", Options{WatermarkScoped: true, Keep: true, ExpectedCost: cost})
	within := func(d time.Duration) (context.Context, context.CancelFunc) {
		return context.WithDeadline(context.Background(), clk.Now().Add(d))
	}
	never := func(context.Context) (int, int64, error) {
		t.Error("a declined Get ran its build")
		return 0, 0, nil
	}
	// took is a build that takes the service its 3 s on the clock, so the
	// record stays (3 s, size) after it.
	took := func(v int, size int64) func(context.Context) (int, int64, error) {
		return func(context.Context) (int, int64, error) { clk.Advance(3 * time.Second); return v, size, nil }
	}

	// Unknown build, a deadline under half ExpectedCost: declined at once.
	ctx, cancel := within(cost / 4)
	if _, err := g.Get(ctx, skey{"svc", "w1", "a"}, never); !errors.Is(err, ErrWouldNotFinish) {
		t.Fatalf("unknown build, deadline %v: err = %v, want ErrWouldNotFinish", cost/4, err)
	}
	cancel()
	if st := g.stats(); st.Declined != 1 || st.Flights != 0 || st.Misses != 0 {
		t.Fatalf("after one decline: %+v", st)
	}

	// No deadline always starts; the build takes 3 s on the clock and
	// records (3 s, 10 bytes) for the service.
	if v, err := g.Get(context.Background(), skey{"svc", "w1", "a"}, func(context.Context) (int, int64, error) {
		clk.Advance(3 * time.Second)
		return 1, 10, nil
	}); err != nil || v != 1 {
		t.Fatalf("no-deadline Get = %d, %v", v, err)
	}

	// Known build of 3 s: a deadline of 2.5 s is under it but at least half
	// ExpectedCost, and 10 bytes fit (no budget): the flight starts.
	ctx, cancel = within(2500 * time.Millisecond)
	if v, err := g.Get(ctx, skey{"svc", "w1", "b"}, took(2, 10)); err != nil || v != 2 {
		t.Fatalf("kept-flight deadline: %d, %v", v, err)
	}
	cancel()
	// Under half ExpectedCost and under the last build: declined.
	ctx, cancel = within(cost / 4)
	if _, err := g.Get(ctx, skey{"svc", "w1", "c"}, never); !errors.Is(err, ErrWouldNotFinish) {
		t.Fatalf("known build, short deadline: err = %v", err)
	}
	cancel()

	// The limit in force under the build's size: only a deadline of the
	// whole last build starts it, since the flight could not be kept.
	reg.SetBudget(5, 0)
	ctx, cancel = within(2500 * time.Millisecond)
	if _, err := g.Get(ctx, skey{"svc", "w1", "c"}, never); !errors.Is(err, ErrWouldNotFinish) {
		t.Fatalf("oversize build, deadline under it: err = %v", err)
	}
	cancel()
	ctx, cancel = within(3 * time.Second)
	if v, err := g.Get(ctx, skey{"svc", "w1", "c"}, took(3, 10)); err != nil || v != 3 {
		t.Fatalf("deadline covering the last build: %d, %v", v, err)
	}
	cancel()
	reg.SetBudget(0, 0)
	// Another service's record is its own: unknown, ExpectedCost/2 starts.
	ctx, cancel = within(cost / 2)
	if v, err := g.Get(ctx, skey{"other", "w1", "a"}, took(4, 1)); err != nil || v != 4 {
		t.Fatalf("another service, half ExpectedCost: %d, %v", v, err)
	}
	cancel()

	// A nested build always starts, whatever its deadline.
	inner := NewGroupIn[skey, int](reg, "decline-inner", Options{WatermarkScoped: true, Keep: true, ExpectedCost: cost})
	if v, err := g.Get(context.Background(), skey{"svc", "w1", "outer"}, func(bctx context.Context) (int, int64, error) {
		ictx, icancel := context.WithDeadline(bctx, clk.Now().Add(time.Millisecond))
		defer icancel()
		v, err := inner.Get(ictx, skey{"svc", "w1", "inner"}, took(5, 1))
		return v, 1, err
	}); err != nil || v != 5 {
		t.Fatalf("nested build with a short deadline: %d, %v", v, err)
	}

	// At shed with the rebuild token held elsewhere, a decline answers at
	// once instead of waiting for (or taking) the token.
	setLevel(tenancy.LevelShed)
	reg.rebuild <- struct{}{}
	ctx, cancel = within(cost / 4)
	returnsUnaided(t, "a declined Get at shed", func() {
		if _, err := g.Get(ctx, skey{"svc", "w1", "d"}, never); !errors.Is(err, ErrWouldNotFinish) {
			t.Errorf("declined Get at shed: err = %v", err)
		}
	})
	cancel()
	if len(reg.rebuild) != 1 {
		t.Fatal("the rebuild token's holder lost it")
	}
	<-reg.rebuild
	setLevel(tenancy.LevelNormal)
	if st := g.stats(); st.Declined != 4 {
		t.Fatalf("declined = %d, want 4", st.Declined)
	}

	// A kept build its caller left (past half ExpectedCost) completes for
	// the next caller.
	started, finish, built := make(chan struct{}), make(chan struct{}), make(chan struct{})
	kctx, kcancel := within(2500 * time.Millisecond)
	go func() { <-started; kcancel() }()
	if _, err := g.Get(kctx, skey{"svc", "w1", "kept"}, func(context.Context) (int, int64, error) {
		clk.Advance(cost/2 + time.Millisecond)
		close(started)
		<-finish
		defer close(built)
		return 6, 1, nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("the leaving caller: err = %v", err)
	}
	close(finish)
	<-built
	returnsUnaided(t, "the kept build's insert", func() {
		for {
			if v, ok := g.Peek(skey{"svc", "w1", "kept"}); ok && v == 6 {
				return
			}
			time.Sleep(time.Millisecond)
		}
	})
	if st := g.stats(); st.Abandoned != 0 {
		t.Fatalf("a kept flight counted abandoned: %+v", st)
	}

	// A flight short of half its cost is abandoned when its last waiter
	// leaves.
	actx, acancel := context.WithCancel(context.Background())
	astarted := make(chan struct{})
	go func() { <-astarted; acancel() }()
	_, _ = g.Get(actx, skey{"svc", "w1", "abandon"}, func(bctx context.Context) (int, int64, error) {
		close(astarted)
		<-bctx.Done()
		return 0, 0, bctx.Err()
	})
	returnsUnaided(t, "the abandoned count", func() {
		for g.stats().Abandoned != 1 {
			time.Sleep(time.Millisecond)
		}
	})

	// A refuse drop ends a flight as RefuseCancelled; its waiter leaving
	// afterwards is not an abandonment.
	rctx, rcancel := context.WithCancel(context.Background())
	rstarted, rrelease, rdone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(rdone)
		_, _ = g.Get(rctx, skey{"svc", "w1", "refused"}, func(context.Context) (int, int64, error) {
			close(rstarted)
			<-rrelease // ignores its cancellation until the test lets it end
			return 7, 1, nil
		})
	}()
	<-rstarted
	reg.Drop(ReasonRefuse)
	rcancel()
	<-rdone
	close(rrelease)
	st := g.stats()
	if st.RefuseCancelled != 1 || st.Abandoned != 1 {
		t.Fatalf("refuse drop: refuse_cancelled %d abandoned %d, want 1 and 1 (the earlier abandonment)", st.RefuseCancelled, st.Abandoned)
	}
}

// TestGraphForTypedNeverReadsADeclineAsATypedFailure (RT-191): a typed-edge
// load declined for its caller's deadline is the call's error, never the
// typed-edge read failure that degrades it to the syntax graph (I38).
func TestGraphForTypedNeverReadsADeclineAsATypedFailure(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: "decline-typed", Service: "svc", Watermark: "w1"}
	snap, err := SnapshotFor(context.Background(), sc, func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil })
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(context.Background(), typedCost/10)
	defer cancel()
	g, _, typedErr, err := GraphForTyped(short, snap, GraphBoth, func(context.Context) ([]core.Edge, error) {
		t.Error("a declined typed load ran")
		return nil, nil
	})
	if !errors.Is(err, ErrWouldNotFinish) || typedErr != nil || g != nil {
		t.Fatalf("GraphForTyped = graph %v, typedErr %v, err %v; want ErrWouldNotFinish as err", g != nil, typedErr, err)
	}
}
