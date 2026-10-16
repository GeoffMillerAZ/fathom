package strikeledger_test

import (
	"sync"
	"testing"
	"time"

	sl "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/strike-ledger"
)

// ---- helpers ----------------------------------------------------------------

// mockClock is a controllable clock for deterministic tests.
type mockClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock(t time.Time) *mockClock { return &mockClock{now: t} }

func (c *mockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *mockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func defaultCfg() sl.Config {
	return sl.Config{
		Threshold: 3,
		BaseBlock: 1 * time.Minute,
		MaxBlock:  1 * time.Hour,
	}
}

// ---- Strike accrual ---------------------------------------------------------

func TestStrike_Accrual(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	s1 := l.Strike("u1", "spam")
	if s1.Strikes != 1 {
		t.Errorf("expected 1 strike, got %d", s1.Strikes)
	}
	if s1.Blocked {
		t.Error("should not be blocked at 1 strike (threshold=3)")
	}

	s2 := l.Strike("u1", "spam")
	if s2.Strikes != 2 {
		t.Errorf("expected 2 strikes, got %d", s2.Strikes)
	}
	if s2.Blocked {
		t.Error("should not be blocked at 2 strikes")
	}
}

func TestStrike_BlockAtThreshold(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	for i := 0; i < 2; i++ {
		l.Strike("u1", "bad")
	}
	s := l.Strike("u1", "bad") // third strike — hits threshold
	if !s.Blocked {
		t.Fatal("expected blocked at threshold=3")
	}
	if s.Until.IsZero() {
		t.Error("Until should not be zero when blocked")
	}
	if !s.Until.After(epoch) {
		t.Error("Until should be after now")
	}
}

func TestStrike_BlockedQuery(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	for i := 0; i < 3; i++ {
		l.Strike("u1", "bad")
	}
	blocked, until := l.Blocked("u1")
	if !blocked {
		t.Fatal("expected blocked")
	}
	if until.IsZero() {
		t.Error("until must not be zero")
	}
}

// ---- Block expiry -----------------------------------------------------------

func TestStrike_BlockExpiry(t *testing.T) {
	clk := newClock(epoch)
	cfg := sl.Config{Threshold: 2, BaseBlock: 10 * time.Second, MaxBlock: 1 * time.Hour}
	l := sl.New(cfg, clk.Now)

	l.Strike("u1", "bad")
	l.Strike("u1", "bad") // blocks for 10s

	blocked, until := l.Blocked("u1")
	if !blocked {
		t.Fatal("expected blocked")
	}

	// Advance past the block expiry.
	clk.Advance(until.Sub(clk.Now()) + time.Second)

	blocked2, _ := l.Blocked("u1")
	if blocked2 {
		t.Error("block should have expired")
	}
}

// ---- Escalating backoff -----------------------------------------------------

func TestStrike_EscalatingBackoff(t *testing.T) {
	clk := newClock(epoch)
	cfg := sl.Config{Threshold: 2, BaseBlock: 10 * time.Second, MaxBlock: 10 * time.Minute}
	l := sl.New(cfg, clk.Now)

	// First block: BaseBlock * 2^0 = 10s
	l.Strike("u1", "bad")
	l.Strike("u1", "bad")
	_, until1 := l.Blocked("u1")
	dur1 := until1.Sub(epoch)
	if dur1 != 10*time.Second {
		t.Errorf("first block duration: want 10s, got %v", dur1)
	}

	// Advance past first block.
	clk.Advance(dur1 + time.Second)

	// Second block: BaseBlock * 2^1 = 20s
	l.Strike("u1", "again")
	l.Strike("u1", "again")
	_, until2 := l.Blocked("u1")
	dur2 := until2.Sub(clk.Now())
	if dur2 != 20*time.Second {
		t.Errorf("second block duration: want 20s, got %v", dur2)
	}

	// Advance past second block.
	clk.Advance(dur2 + time.Second)

	// Third block: BaseBlock * 2^2 = 40s
	l.Strike("u1", "again")
	l.Strike("u1", "again")
	_, until3 := l.Blocked("u1")
	dur3 := until3.Sub(clk.Now())
	if dur3 != 40*time.Second {
		t.Errorf("third block duration: want 40s, got %v", dur3)
	}
}

func TestStrike_MaxBlockClamped(t *testing.T) {
	clk := newClock(epoch)
	cfg := sl.Config{Threshold: 1, BaseBlock: 1 * time.Hour, MaxBlock: 2 * time.Hour}
	l := sl.New(cfg, clk.Now)

	// First block = 1h (= MaxBlock would need 2h but base is 1h and 2^0=1; fine).
	l.Strike("u1", "bad")
	_, until1 := l.Blocked("u1")
	clk.Advance(until1.Sub(clk.Now()) + time.Second)

	// Second block = min(2h, 1h*2^1) = min(2h, 2h) = 2h
	l.Strike("u1", "again")
	_, until2 := l.Blocked("u1")
	dur2 := until2.Sub(clk.Now())
	if dur2 != 2*time.Hour {
		t.Errorf("second block: want 2h, got %v", dur2)
	}
	clk.Advance(dur2 + time.Second)

	// Third block = min(2h, 1h*2^2) = min(2h, 4h) = 2h (MaxBlock)
	l.Strike("u1", "again")
	_, until3 := l.Blocked("u1")
	dur3 := until3.Sub(clk.Now())
	if dur3 != 2*time.Hour {
		t.Errorf("third block (clamped): want 2h (MaxBlock), got %v", dur3)
	}
}

// ---- Strikes count and Events -----------------------------------------------

func TestStrike_StrikesCount(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	l.Strike("u1", "r1")
	l.Strike("u1", "r2")
	if n := l.Strikes("u1"); n != 2 {
		t.Errorf("Strikes: want 2, got %d", n)
	}
	// Unknown user has 0.
	if n := l.Strikes("unknown"); n != 0 {
		t.Errorf("Strikes for unknown: want 0, got %d", n)
	}
}

func TestStrike_Events(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	l.Strike("u1", "reason-a")
	l.Strike("u1", "reason-b")

	events := l.Events("u1")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Reason != "reason-a" {
		t.Errorf("first event reason: got %q", events[0].Reason)
	}
	if events[1].Reason != "reason-b" {
		t.Errorf("second event reason: got %q", events[1].Reason)
	}
}

func TestStrike_EventsImmutable(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	l.Strike("u1", "original")
	events := l.Events("u1")
	// Mutate the returned slice.
	events[0].Reason = "mutated"

	// Internal state must be unaffected.
	events2 := l.Events("u1")
	if events2[0].Reason != "original" {
		t.Error("Events() must return an immutable copy")
	}
}

func TestStrike_EventsUnknownUser(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)
	events := l.Events("nobody")
	if events == nil {
		t.Error("Events for unknown user should return non-nil empty slice")
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// ---- Append-only guarantee --------------------------------------------------

func TestStrike_AppendOnly(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	l.Strike("u1", "r1")
	snapshot1 := l.Events("u1")
	n1 := len(snapshot1)

	l.Strike("u1", "r2")
	snapshot2 := l.Events("u1")

	if len(snapshot2) <= n1 {
		t.Error("events must only grow (append-only)")
	}
	// First event must be unchanged.
	if snapshot2[0].Reason != "r1" {
		t.Error("prior events must not be modified")
	}
}

// ---- Snapshot / Restore -----------------------------------------------------

func TestStrike_SnapshotRestore(t *testing.T) {
	clk := newClock(epoch)
	cfg := defaultCfg()
	l := sl.New(cfg, clk.Now)

	l.Strike("u1", "r1")
	l.Strike("u1", "r2")
	l.Strike("u1", "r3") // triggers block

	snap := l.Snapshot()

	// Create a new ledger from the snapshot.
	l2 := sl.New(cfg, clk.Now)
	l2.Restore(snap)

	if l2.Strikes("u1") != 3 {
		t.Errorf("restored strikes: want 3, got %d", l2.Strikes("u1"))
	}
	blocked, _ := l2.Blocked("u1")
	if !blocked {
		t.Error("restored ledger must reflect block")
	}
}

func TestStrike_SnapshotRestore_EmptyLedger(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)
	snap := l.Snapshot()

	l2 := sl.New(defaultCfg(), clk.Now)
	l2.Restore(snap)
	if n := l2.Strikes("u1"); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

// ---- Concurrency safety -----------------------------------------------------

func TestStrike_ConcurrentSafety(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Strike("shared", "concurrent")
			l.Blocked("shared")
			l.Strikes("shared")
			l.Events("shared")
		}()
	}
	wg.Wait()

	if l.Strikes("shared") != 50 {
		t.Errorf("expected 50 strikes, got %d", l.Strikes("shared"))
	}
}

// ---- Determinism ------------------------------------------------------------

func TestStrike_Determinism(t *testing.T) {
	cfg := defaultCfg()

	run := func() sl.Status {
		clk := newClock(epoch)
		l := sl.New(cfg, clk.Now)
		l.Strike("u1", "r")
		l.Strike("u1", "r")
		return l.Strike("u1", "r")
	}

	s1 := run()
	s2 := run()

	if s1.Strikes != s2.Strikes || s1.Blocked != s2.Blocked || !s1.Until.Equal(s2.Until) {
		t.Errorf("non-deterministic: %+v vs %+v", s1, s2)
	}
}

// ---- Separate users don't interfere -----------------------------------------

func TestStrike_SeparateUsers(t *testing.T) {
	clk := newClock(epoch)
	l := sl.New(defaultCfg(), clk.Now)

	for i := 0; i < 3; i++ {
		l.Strike("alice", "bad")
	}
	// alice is blocked; bob should not be.
	s := l.Strike("bob", "bad")
	if s.Blocked {
		t.Error("bob should not be blocked because alice exceeded threshold")
	}
}
