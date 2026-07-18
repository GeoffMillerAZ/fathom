package ratelimiter_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	rl "github.com/geoffmilleraz/agentic-kit/foundations/governance/rate-limiter"
)

// ---- helpers ----------------------------------------------------------------

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func advancingClock(start time.Time) (func() time.Time, func(d time.Duration)) {
	var mu sync.Mutex
	cur := start
	get := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	advance := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cur = cur.Add(d)
	}
	return get, advance
}

// ---- Limiter: burst then deny -----------------------------------------------

func TestLimiter_BurstThenDeny(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	lim := rl.New(1.0, 3, now)

	// Should allow exactly burst=3 tokens immediately.
	for i := 0; i < 3; i++ {
		if !lim.Allow() {
			t.Fatalf("Allow() %d: expected true, got false", i)
		}
	}
	// 4th should be denied (bucket empty).
	if lim.Allow() {
		t.Fatal("4th Allow() should be false (burst exhausted)")
	}
}

// ---- Limiter: AllowN burst --------------------------------------------------

func TestLimiter_AllowN_Burst(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	lim := rl.New(1.0, 5, now)

	// AllowN(5) should consume all 5 tokens.
	if !lim.AllowN(5) {
		t.Fatal("AllowN(5) should be true (full burst)")
	}
	if lim.Allow() {
		t.Fatal("Allow after AllowN(5) should be false")
	}
}

func TestLimiter_AllowN_ExceedsBurst(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	lim := rl.New(1.0, 3, now)

	// AllowN(4) > burst=3: always false.
	if lim.AllowN(4) {
		t.Fatal("AllowN(4) > burst=3 should be false")
	}
}

// ---- Limiter: refill over injected time -------------------------------------

func TestLimiter_Refill(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	lim := rl.New(2.0, 4, now) // 2 tokens/sec, burst 4

	// Drain burst.
	for i := 0; i < 4; i++ {
		lim.Allow()
	}
	if lim.Allow() {
		t.Fatal("should be empty after draining burst")
	}

	// Advance 1 second → 2 tokens refilled.
	advance(time.Second)
	if !lim.Allow() {
		t.Fatal("should allow after 1s refill (2 tokens added)")
	}
	if !lim.Allow() {
		t.Fatal("should allow second token after 1s refill")
	}
	if lim.Allow() {
		t.Fatal("third token not yet available")
	}
}

// ---- Limiter: partial refill (sub-second) ------------------------------------

func TestLimiter_PartialRefill(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	lim := rl.New(10.0, 10, now) // 10 tokens/sec

	for i := 0; i < 10; i++ {
		lim.Allow()
	}
	// Advance 50ms → 0.5 tokens → not enough for 1.
	advance(50 * time.Millisecond)
	if lim.Allow() {
		t.Fatal("50ms at 10/s should not refill a full token")
	}
	// Advance another 50ms → total 100ms → 1 full token.
	advance(50 * time.Millisecond)
	if !lim.Allow() {
		t.Fatal("100ms at 10/s should refill exactly 1 token")
	}
}

// ---- Limiter: determinism ---------------------------------------------------

func TestLimiter_Determinism(t *testing.T) {
	start := time.Unix(1000, 0)
	lim1 := rl.New(1.0, 2, fixedClock(start))
	lim2 := rl.New(1.0, 2, fixedClock(start))

	// Same sequence should produce same results.
	for i := 0; i < 5; i++ {
		a, b := lim1.Allow(), lim2.Allow()
		if a != b {
			t.Fatalf("determinism failed at step %d: %v vs %v", i, a, b)
		}
	}
}

// ---- Limiter: concurrent Allow under -race ----------------------------------

func TestLimiter_ConcurrentAllow(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	lim := rl.New(100.0, 100, now)

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lim.Allow() {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	// At most burst=100 allowed.
	if got := allowed.Load(); got > 100 {
		t.Fatalf("concurrent Allow: expected ≤100 allowed, got %d", got)
	}
}

// ---- KeyedLimiter: per-key isolation ----------------------------------------

func TestKeyedLimiter_PerKeyIsolation(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	kl := rl.NewKeyed(1.0, 3, now, 0)

	// Drain key "a".
	for i := 0; i < 3; i++ {
		if !kl.Allow("a") {
			t.Fatalf("key a: Allow() %d: expected true", i)
		}
	}
	if kl.Allow("a") {
		t.Fatal("key a: 4th Allow() should be false")
	}

	// Key "b" should still have a full bucket.
	if !kl.Allow("b") {
		t.Fatal("key b: Allow() should be true (independent bucket)")
	}
}

// ---- KeyedLimiter: global cap -----------------------------------------------

func TestKeyedLimiter_GlobalCap(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	// rate=10/s, burst=5 per key, globalCap=7.
	kl := rl.NewKeyed(10.0, 5, now, 7)

	var total int
	for i := 0; i < 5; i++ {
		if kl.Allow("a") {
			total++
		}
		if kl.Allow("b") {
			total++
		}
	}
	// Global cap = 7 means at most 7 total allowed across all keys.
	if total > 7 {
		t.Fatalf("global cap=7 violated: %d allowed", total)
	}
}

// ---- KeyedLimiter: idle-bucket GC -------------------------------------------

func TestKeyedLimiter_IdleGC(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	kl := rl.NewKeyed(1.0, 2, now, 0)

	kl.Allow("gc-key")
	advance(2 * time.Minute) // idle for 2 minutes
	kl.GC(now())             // trigger GC

	// After GC, the bucket for "gc-key" should be gone. A new Allow restarts it.
	// We can't directly inspect the map, but we verify GC runs without panic and
	// a subsequent Allow succeeds (key recreated).
	if !kl.Allow("gc-key") {
		t.Fatal("GC-evicted key: first Allow should recreate bucket and succeed")
	}
}

// ---- KeyedLimiter: concurrent Allow under -race -----------------------------

func TestKeyedLimiter_ConcurrentAllow(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	kl := rl.NewKeyed(100.0, 50, now, 0)

	var wg sync.WaitGroup
	keys := []string{"k1", "k2", "k3"}
	for _, k := range keys {
		for i := 0; i < 100; i++ {
			wg.Add(1)
			k := k
			go func() {
				defer wg.Done()
				kl.Allow(k)
			}()
		}
	}
	wg.Wait()
}
