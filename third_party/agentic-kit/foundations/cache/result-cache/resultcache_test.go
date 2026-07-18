package resultcache_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	resultcache "github.com/geoffmilleraz/agentic-kit/foundations/cache/result-cache"
)

// ---- helpers ----------------------------------------------------------------

var epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func advancingClock(start time.Time) (now func() time.Time, advance func(d time.Duration)) {
	var mu sync.Mutex
	cur := start
	now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	advance = func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cur = cur.Add(d)
	}
	return now, advance
}

func wantStats(t *testing.T, c *resultcache.Cache[string], want resultcache.Stats) {
	t.Helper()
	if got := c.Stats(); got != want {
		t.Fatalf("Stats() = %+v, want %+v", got, want)
	}
}

// waitMisses spins (yielding) until misses() reaches want. A GetOrCompute
// caller records its miss in the same critical section in which it grabs the
// in-flight handle, so once the count is reached every waiter has provably
// joined the flight. The wall-clock deadline only bounds test runtime on
// failure; it is not load-bearing for correctness.
func waitMisses(t *testing.T, misses func() int64, want int64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for misses() < want {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d misses, have %d", want, misses())
		}
		runtime.Gosched()
	}
}

type result struct {
	v   string
	err error
}

// ---- construction -----------------------------------------------------------

func TestNewNilClockPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with nil clock did not panic")
		}
	}()
	resultcache.New[string](resultcache.Config{}, nil)
}

func TestDefaultMaxEntries(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[int](resultcache.Config{}, now) // MaxEntries 0 → 1024

	for i := 0; i < resultcache.DefaultMaxEntries; i++ {
		c.Put(fmt.Sprintf("k%d", i), i)
	}
	if got := c.Stats().Evictions; got != 0 {
		t.Fatalf("evictions before overflow = %d, want 0", got)
	}
	c.Put("overflow", -1)
	if got := c.Stats().Evictions; got != 1 {
		t.Fatalf("evictions after overflow = %d, want 1", got)
	}
	if got := c.Len(); got != resultcache.DefaultMaxEntries {
		t.Fatalf("Len() = %d, want %d", got, resultcache.DefaultMaxEntries)
	}
	if _, ok := c.Get("k0"); ok {
		t.Fatal("oldest entry k0 survived eviction at default capacity")
	}
}

// ---- Get / Put / TTL --------------------------------------------------------

func TestGetMissOnEmpty(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{}, now)

	v, ok := c.Get("absent")
	if ok || v != "" {
		t.Fatalf("Get(absent) = (%q, %v), want zero value and false", v, ok)
	}
	wantStats(t, c, resultcache.Stats{Misses: 1})
}

func TestTTLExpiry(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		advance time.Duration
		wantHit bool
	}{
		{name: "fresh entry hits", ttl: time.Minute, advance: 59 * time.Second, wantHit: true},
		{name: "expires exactly at deadline", ttl: time.Minute, advance: time.Minute, wantHit: false},
		{name: "expired entry misses", ttl: time.Minute, advance: 2 * time.Minute, wantHit: false},
		{name: "zero TTL never expires", ttl: 0, advance: 1000 * time.Hour, wantHit: true},
		{name: "negative TTL treated as no expiry", ttl: -time.Second, advance: 1000 * time.Hour, wantHit: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now, advance := advancingClock(epoch)
			c := resultcache.New[string](resultcache.Config{TTL: tt.ttl}, now)
			c.Put("k", "v")
			advance(tt.advance)
			v, ok := c.Get("k")
			if ok != tt.wantHit {
				t.Fatalf("Get after advance(%v) hit = %v, want %v", tt.advance, ok, tt.wantHit)
			}
			if tt.wantHit && v != "v" {
				t.Fatalf("Get returned %q, want %q", v, "v")
			}
			if !tt.wantHit {
				if got := c.Len(); got != 0 {
					t.Fatalf("expired entry not lazily removed: Len() = %d, want 0", got)
				}
				// Expiry is a miss, never an eviction.
				if got := c.Stats().Evictions; got != 0 {
					t.Fatalf("Evictions = %d, want 0 (expiry must not count)", got)
				}
			}
		})
	}
}

func TestPutRefreshesTTL(t *testing.T) {
	now, advance := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{TTL: time.Minute}, now)

	c.Put("k", "v1")
	advance(45 * time.Second)
	c.Put("k", "v2") // refresh: new deadline is 45s + 60s
	advance(45 * time.Second)

	v, ok := c.Get("k")
	if !ok || v != "v2" {
		t.Fatalf("Get after refresh = (%q, %v), want (v2, true)", v, ok)
	}
}

// ---- LRU --------------------------------------------------------------------

func TestLRUEvictionOrder(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{MaxEntries: 3}, now)

	c.Put("a", "A")
	c.Put("b", "B")
	c.Put("c", "C")
	if _, ok := c.Get("a"); !ok { // refresh a → LRU order now b, c, a
		t.Fatal("expected hit on a")
	}
	c.Put("d", "D") // evicts b (least recently used)

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted as LRU")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected %q to survive eviction", k)
		}
	}
	if got := c.Stats().Evictions; got != 1 {
		t.Fatalf("Evictions = %d, want 1", got)
	}
}

func TestPutRefreshesRecency(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{MaxEntries: 2}, now)

	c.Put("a", "A")
	c.Put("b", "B")
	c.Put("a", "A2") // overwrite refreshes recency without growing
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	c.Put("c", "C") // evicts b, not a

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	v, ok := c.Get("a")
	if !ok || v != "A2" {
		t.Fatalf("Get(a) = (%q, %v), want (A2, true)", v, ok)
	}
}

// ---- stats ------------------------------------------------------------------

func TestStatsCounters(t *testing.T) {
	now, advance := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{TTL: time.Minute, MaxEntries: 2}, now)

	wantStats(t, c, resultcache.Stats{})

	c.Get("a") // miss
	c.Put("a", "A")
	c.Get("a") // hit
	c.Get("a") // hit
	c.Put("b", "B")
	c.Put("c", "C") // evicts a
	c.Get("a")      // miss (evicted)
	advance(2 * time.Minute)
	c.Get("b") // miss (expired)

	wantStats(t, c, resultcache.Stats{Hits: 2, Misses: 3, Evictions: 1})
}

// ---- GetOrCompute: sequential ------------------------------------------------

func TestGetOrComputeCachesSuccess(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{TTL: time.Minute}, now)
	var calls atomic.Int64

	for i := 0; i < 3; i++ {
		v, err := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
			calls.Add(1)
			return "computed", nil
		})
		if err != nil || v != "computed" {
			t.Fatalf("call %d: GetOrCompute = (%q, %v), want (computed, nil)", i, v, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("compute calls = %d, want 1", got)
	}
	wantStats(t, c, resultcache.Stats{Hits: 2, Misses: 1})
}

func TestGetOrComputeErrorNotCachedThenRecompute(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{}, now)
	wantErr := errors.New("compute failed")
	var calls atomic.Int64

	// First call fails: error returned, nothing cached.
	v, err := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
		calls.Add(1)
		return "partial", wantErr
	})
	if !errors.Is(err, wantErr) || v != "" {
		t.Fatalf("failing GetOrCompute = (%q, %v), want zero value and %v", v, err, wantErr)
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("error was cached: Len() = %d, want 0", got)
	}

	// Second call recomputes and caches the success.
	v, err = c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
		calls.Add(1)
		return "second", nil
	})
	if err != nil || v != "second" {
		t.Fatalf("recompute = (%q, %v), want (second, nil)", v, err)
	}

	// Third call is a pure cache hit.
	v, err = c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
		calls.Add(1)
		return "third", nil
	})
	if err != nil || v != "second" {
		t.Fatalf("cached call = (%q, %v), want (second, nil)", v, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("compute calls = %d, want 2 (fail, recompute, then hit)", got)
	}
}

func TestGetOrComputeRecomputesAfterExpiry(t *testing.T) {
	now, advance := advancingClock(epoch)
	c := resultcache.New[int](resultcache.Config{TTL: time.Minute}, now)
	var calls atomic.Int64
	compute := func(context.Context) (int, error) {
		return int(calls.Add(1)), nil
	}

	v, err := c.GetOrCompute(context.Background(), "k", compute)
	if err != nil || v != 1 {
		t.Fatalf("first GetOrCompute = (%d, %v), want (1, nil)", v, err)
	}
	advance(2 * time.Minute)
	v, err = c.GetOrCompute(context.Background(), "k", compute)
	if err != nil || v != 2 {
		t.Fatalf("post-expiry GetOrCompute = (%d, %v), want (2, nil)", v, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("compute calls = %d, want 2", got)
	}
}

// ---- GetOrCompute: single-flight ---------------------------------------------

// startBlockedLeader launches a leader GetOrCompute on key "k" whose compute
// increments calls, signals it is running, blocks until release closes, then
// returns (v, err). The returned channel yields the leader's result.
func startBlockedLeader(c *resultcache.Cache[string], calls *atomic.Int64, release <-chan struct{}, v string, err error) <-chan result {
	inFlight := make(chan struct{})
	done := make(chan result, 1)
	go func() {
		got, gotErr := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
			calls.Add(1)
			close(inFlight)
			<-release
			return v, err
		})
		done <- result{got, gotErr}
	}()
	<-inFlight // flight is registered before compute runs, so it is visible now
	return done
}

func TestSingleFlightConcurrentCallersShareOneCompute(t *testing.T) {
	const waiters = 32
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{}, now)
	misses := func() int64 { return c.Stats().Misses }

	var calls atomic.Int64
	release := make(chan struct{})
	leaderDone := startBlockedLeader(c, &calls, release, "shared", nil)

	results := make(chan result, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			v, err := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
				calls.Add(1)
				return "rogue", nil
			})
			results <- result{v, err}
		}()
	}
	// Each waiter records a miss in the same critical section in which it
	// grabs the flight, so all 32 have joined once misses reaches 1+32.
	waitMisses(t, misses, 1+waiters)
	close(release)

	for i := 0; i < waiters; i++ {
		if r := <-results; r.err != nil || r.v != "shared" {
			t.Fatalf("waiter %d: got (%q, %v), want (shared, nil)", i, r.v, r.err)
		}
	}
	if r := <-leaderDone; r.err != nil || r.v != "shared" {
		t.Fatalf("leader: got (%q, %v), want (shared, nil)", r.v, r.err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("compute calls = %d, want exactly 1 (single-flight)", got)
	}
	if v, ok := c.Get("k"); !ok || v != "shared" {
		t.Fatalf("Get after flight = (%q, %v), want (shared, true)", v, ok)
	}
}

func TestSingleFlightErrorSharedByAllWaitersAndNotCached(t *testing.T) {
	const waiters = 8
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{}, now)
	misses := func() int64 { return c.Stats().Misses }
	wantErr := errors.New("compute failed")

	var calls atomic.Int64
	release := make(chan struct{})
	leaderDone := startBlockedLeader(c, &calls, release, "partial", wantErr)

	results := make(chan result, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			v, err := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
				calls.Add(1)
				return "rogue", nil
			})
			results <- result{v, err}
		}()
	}
	waitMisses(t, misses, 1+waiters) // all waiters joined the failing flight
	close(release)

	if r := <-leaderDone; !errors.Is(r.err, wantErr) || r.v != "" {
		t.Fatalf("leader: got (%q, %v), want zero value and %v", r.v, r.err, wantErr)
	}
	for i := 0; i < waiters; i++ {
		if r := <-results; !errors.Is(r.err, wantErr) || r.v != "" {
			t.Fatalf("waiter %d: got (%q, %v), want zero value and shared %v", i, r.v, r.err, wantErr)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("compute calls = %d, want 1 (waiters must not recompute)", got)
	}

	// The error was not cached: the next call recomputes.
	v, err := c.GetOrCompute(context.Background(), "k", func(context.Context) (string, error) {
		calls.Add(1)
		return "second", nil
	})
	if err != nil || v != "second" {
		t.Fatalf("recompute after failed flight = (%q, %v), want (second, nil)", v, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("compute calls = %d, want 2 (error must not be cached)", got)
	}
}

func TestSingleFlightWaiterContextCancellation(t *testing.T) {
	now, _ := advancingClock(epoch)
	c := resultcache.New[string](resultcache.Config{}, now)
	misses := func() int64 { return c.Stats().Misses }

	var calls atomic.Int64
	release := make(chan struct{})
	leaderDone := startBlockedLeader(c, &calls, release, "eventual", nil)

	ctx, cancel := context.WithCancel(context.Background())
	waiterDone := make(chan result, 1)
	go func() {
		v, err := c.GetOrCompute(ctx, "k", func(context.Context) (string, error) {
			calls.Add(1)
			return "rogue", nil
		})
		waiterDone <- result{v, err}
	}()
	waitMisses(t, misses, 2) // waiter has joined the flight
	cancel()

	// The flight is still blocked, so the cancelled waiter must give up with
	// its own ctx error — without recomputing and without touching the leader.
	if r := <-waiterDone; !errors.Is(r.err, context.Canceled) || r.v != "" {
		t.Fatalf("cancelled waiter: got (%q, %v), want zero value and context.Canceled", r.v, r.err)
	}

	close(release)
	if r := <-leaderDone; r.err != nil || r.v != "eventual" {
		t.Fatalf("leader after waiter cancel: got (%q, %v), want (eventual, nil)", r.v, r.err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("compute calls = %d, want 1 (waiter cancellation must not retrigger compute)", got)
	}
	if v, ok := c.Get("k"); !ok || v != "eventual" {
		t.Fatalf("Get after flight = (%q, %v), want (eventual, true)", v, ok)
	}
}

// ---- concurrency smoke (exercised under -race) --------------------------------

func TestConcurrentMixedAccess(t *testing.T) {
	now, advance := advancingClock(epoch)
	c := resultcache.New[int](resultcache.Config{TTL: time.Minute, MaxEntries: 16}, now)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("k%d", (g+i)%24)
				switch i % 4 {
				case 0:
					c.Put(key, i)
				case 1:
					c.Get(key)
				case 2:
					_, _ = c.GetOrCompute(context.Background(), key, func(context.Context) (int, error) {
						return i, nil
					})
				case 3:
					c.Stats()
					c.Len()
					advance(time.Second)
				}
			}
		}(g)
	}
	wg.Wait()

	if got := c.Len(); got > 16 {
		t.Fatalf("Len() = %d, want <= MaxEntries (16)", got)
	}
}
