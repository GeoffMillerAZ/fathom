package circuitbreaker_test

import (
	"sync"
	"testing"
	"time"

	cb "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/circuit-breaker"
)

// ---- helpers ----------------------------------------------------------------

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

func defaultConfig() cb.Config {
	return cb.Config{
		FailureThreshold: 3,
		OpenCooldown:     5 * time.Second,
		HalfOpenProbes:   2,
	}
}

// ---- Closed state: allows requests ------------------------------------------

func TestBreaker_ClosedAllows(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	if b.State() != "closed" {
		t.Fatalf("new breaker should be closed, got %q", b.State())
	}
	if !b.Allow() {
		t.Fatal("closed breaker should allow requests")
	}
}

// ---- Trips after FailureThreshold -------------------------------------------

func TestBreaker_TripsAfterThreshold(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	// Record threshold-1 failures: still closed.
	for i := 0; i < 2; i++ {
		b.Record(false)
	}
	if b.State() != "closed" {
		t.Fatalf("should still be closed after %d failures, got %q", 2, b.State())
	}
	if !b.Allow() {
		t.Fatal("should allow while still closed")
	}

	// Third failure trips the breaker.
	b.Record(false)
	if b.State() != "open" {
		t.Fatalf("should be open after threshold failures, got %q", b.State())
	}
}

// ---- Open blocks requests ---------------------------------------------------

func TestBreaker_OpenBlocks(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	b.Trip()
	if b.State() != "open" {
		t.Fatalf("after Trip(), state should be open, got %q", b.State())
	}
	if b.Allow() {
		t.Fatal("open breaker should block requests (fail-closed)")
	}
}

// ---- Half-opens after cooldown ----------------------------------------------

func TestBreaker_HalfOpensAfterCooldown(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	b.Trip()
	if b.State() != "open" {
		t.Fatalf("expected open, got %q", b.State())
	}

	// Before cooldown expires: still open.
	advance(4 * time.Second)
	if b.Allow() {
		t.Fatal("should still be open before cooldown expires")
	}

	// Advance past cooldown.
	advance(2 * time.Second) // total 6s > 5s cooldown
	if b.State() != "half-open" {
		t.Fatalf("should be half-open after cooldown, got %q", b.State())
	}
	if !b.Allow() {
		t.Fatal("half-open breaker should allow probe requests")
	}
}

// ---- Recovers and closes after enough half-open successes ------------------

func TestBreaker_ClosesAfterHalfOpenSuccesses(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := cb.Config{
		FailureThreshold: 1,
		OpenCooldown:     1 * time.Second,
		HalfOpenProbes:   3,
	}
	b := cb.New(cfg, now)

	b.Trip()
	advance(2 * time.Second) // past cooldown

	// HalfOpenProbes=3 successes should close the breaker.
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("half-open probe %d should be allowed", i)
		}
		b.Record(true)
	}

	if b.State() != "closed" {
		t.Fatalf("should be closed after %d successful probes, got %q", 3, b.State())
	}
	if !b.Allow() {
		t.Fatal("closed breaker should allow requests after recovery")
	}
}

// ---- Re-opens on half-open failure -----------------------------------------

func TestBreaker_ReOpensOnHalfOpenFailure(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	b.Trip()
	advance(6 * time.Second)

	if b.State() != "half-open" {
		t.Fatalf("expected half-open, got %q", b.State())
	}

	// A single failure in half-open state re-opens the breaker.
	b.Record(false)
	if b.State() != "open" {
		t.Fatalf("should re-open on half-open failure, got %q", b.State())
	}
	if b.Allow() {
		t.Fatal("re-opened breaker should block requests")
	}
}

// ---- ForceOpen kill-switch --------------------------------------------------

func TestBreaker_ForceOpen(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	// Breaker is closed and healthy.
	for i := 0; i < 10; i++ {
		b.Record(true)
	}
	if b.State() != "closed" {
		t.Fatalf("expected closed, got %q", b.State())
	}

	// ForceOpen acts as a kill-switch.
	b.ForceOpen()
	if b.State() != "open" {
		t.Fatalf("ForceOpen should set state to open, got %q", b.State())
	}
	if b.Allow() {
		t.Fatal("force-opened breaker should block all requests")
	}
}

// ---- Reset returns to closed ------------------------------------------------

func TestBreaker_Reset(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	b.Trip()
	if b.State() != "open" {
		t.Fatalf("expected open, got %q", b.State())
	}

	b.Reset()
	if b.State() != "closed" {
		t.Fatalf("Reset() should return to closed, got %q", b.State())
	}
	if !b.Allow() {
		t.Fatal("reset breaker should allow requests")
	}
}

// ---- Determinism ------------------------------------------------------------

func TestBreaker_Determinism(t *testing.T) {
	start := time.Unix(1000, 0)
	cfg := defaultConfig()

	run := func() []string {
		now, advance := advancingClock(start)
		b := cb.New(cfg, now)
		var states []string
		states = append(states, b.State())
		b.Record(false)
		b.Record(false)
		b.Record(false) // should trip
		states = append(states, b.State())
		advance(6 * time.Second)
		b.Allow() // triggers half-open
		states = append(states, b.State())
		b.Record(true)
		b.Record(true) // should close
		states = append(states, b.State())
		return states
	}

	s1 := run()
	s2 := run()
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("determinism failed at step %d: %q vs %q", i, s1[i], s2[i])
		}
	}
}

// ---- Concurrent Allow under -race -------------------------------------------

func TestBreaker_ConcurrentAllow(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	b := cb.New(defaultConfig(), now)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			b.Allow()
			b.Record(i%3 == 0) // mix of successes and failures
		}()
	}
	wg.Wait()
	// Just verify it doesn't race or panic — state can be anything.
	_ = b.State()
}

// ---- Success resets failure counter (closed state) --------------------------

func TestBreaker_SuccessResetsFailureCounter(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := cb.Config{
		FailureThreshold: 3,
		OpenCooldown:     5 * time.Second,
		HalfOpenProbes:   2,
	}
	b := cb.New(cfg, now)

	// 2 failures, then a success, then 2 more failures — should NOT trip yet.
	b.Record(false)
	b.Record(false)
	b.Record(true) // resets counter
	b.Record(false)
	b.Record(false)

	if b.State() != "closed" {
		t.Fatalf("success should reset failure counter; expected closed, got %q", b.State())
	}

	// One more failure now trips it.
	b.Record(false)
	if b.State() != "open" {
		t.Fatalf("should trip after 3 consecutive failures, got %q", b.State())
	}
}
