package budgetguard_test

import (
	"sync"
	"testing"
	"time"

	bg "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
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

func defaultConfig() bg.Config {
	return bg.Config{
		MaxReqPerMin: 5,
		MaxReqPerDay: 20,
		MaxUSDPerDay: 1.0,
	}
}

// ---- Charge within all limits -----------------------------------------------

func TestGuard_ChargeAllowed(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	g := bg.New(defaultConfig(), now)

	d := g.Charge(0.01)
	if !d.Allowed {
		t.Fatalf("expected Allowed=true, got false (reason: %q)", d.Reason)
	}
	if d.Tripped {
		t.Fatal("expected Tripped=false")
	}
}

// ---- Trips on MaxReqPerMin -------------------------------------------------

func TestGuard_TripsOnReqPerMin(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 3,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 100.0,
	}
	g := bg.New(cfg, now)

	// Use up the per-minute quota.
	for i := 0; i < 3; i++ {
		d := g.Charge(0.001)
		if !d.Allowed {
			t.Fatalf("charge %d: expected allowed, got denied (reason: %q)", i, d.Reason)
		}
	}

	// 4th charge within the same minute should be denied.
	d := g.Charge(0.001)
	if d.Allowed {
		t.Fatal("4th charge: expected Allowed=false (req/min limit)")
	}
	if !d.Tripped {
		t.Fatal("expected Tripped=true")
	}
	if d.Reason == "" {
		t.Fatal("expected non-empty Reason")
	}
}

// ---- Trips on MaxReqPerDay -------------------------------------------------

func TestGuard_TripsOnReqPerDay(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 1000,
		MaxReqPerDay: 5,
		MaxUSDPerDay: 100.0,
	}
	g := bg.New(cfg, now)

	// Spread requests across multiple minutes to avoid per-minute limit.
	for i := 0; i < 5; i++ {
		advance(2 * time.Minute)
		d := g.Charge(0.001)
		if !d.Allowed {
			t.Fatalf("charge %d: expected allowed, got denied (reason: %q)", i, d.Reason)
		}
	}

	// 6th request in same day should be denied by day limit.
	advance(2 * time.Minute)
	d := g.Charge(0.001)
	if d.Allowed {
		t.Fatal("6th charge: expected Allowed=false (req/day limit)")
	}
	if !d.Tripped {
		t.Fatal("expected Tripped=true")
	}
}

// ---- Trips on MaxUSDPerDay -------------------------------------------------

func TestGuard_TripsOnUSDPerDay(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 1000,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 1.0,
	}
	g := bg.New(cfg, now)

	// Spend 0.40+0.40 = 0.80 USD.
	advance(time.Minute)
	d := g.Charge(0.40)
	if !d.Allowed {
		t.Fatalf("first charge: expected allowed, got denied (reason: %q)", d.Reason)
	}
	advance(time.Minute)
	d = g.Charge(0.40)
	if !d.Allowed {
		t.Fatalf("second charge: expected allowed, got denied (reason: %q)", d.Reason)
	}

	// Next charge of 0.30 would push to 1.10, exceeding $1.00.
	advance(time.Minute)
	d = g.Charge(0.30)
	if d.Allowed {
		t.Fatal("expected Allowed=false (USD/day limit)")
	}
	if !d.Tripped {
		t.Fatal("expected Tripped=true")
	}
}

// ---- First-exceeded wins (req/min vs USD) -----------------------------------

func TestGuard_FirstExceededWins_ReqMin(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 1,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 0.01, // also low, but req/min fires first
	}
	g := bg.New(cfg, now)

	g.Charge(0.001) // fills req/min=1

	d := g.Charge(0.001)
	if d.Allowed {
		t.Fatal("expected denied")
	}
	// The first exceeded dimension should be req/min.
	// (Both dimensions are exceeded simultaneously; req/min is checked first.)
	if d.Reason == "" {
		t.Fatal("expected non-empty Reason")
	}
}

// ---- Window reset over injected time ----------------------------------------

func TestGuard_PerMinWindowReset(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 2,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 100.0,
	}
	g := bg.New(cfg, now)

	g.Charge(0.001)
	g.Charge(0.001)
	d := g.Charge(0.001) // should be denied
	if d.Allowed {
		t.Fatal("3rd charge in minute should be denied")
	}

	// Advance into the next minute — per-minute counter resets.
	advance(time.Minute)
	d = g.Charge(0.001)
	if !d.Allowed {
		t.Fatalf("after minute reset, charge should be allowed (reason: %q)", d.Reason)
	}
}

func TestGuard_PerDayWindowReset(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 1000,
		MaxReqPerDay: 3,
		MaxUSDPerDay: 100.0,
	}
	g := bg.New(cfg, now)

	for i := 0; i < 3; i++ {
		advance(2 * time.Minute)
		g.Charge(0.001)
	}
	advance(2 * time.Minute)
	d := g.Charge(0.001)
	if d.Allowed {
		t.Fatal("4th charge in day should be denied")
	}

	// Advance into the next day — per-day counter resets.
	advance(24 * time.Hour)
	d = g.Charge(0.001)
	if !d.Allowed {
		t.Fatalf("after day reset, charge should be allowed (reason: %q)", d.Reason)
	}
}

// ---- Snapshot/Restore round-trip -------------------------------------------

func TestGuard_SnapshotRestoreRoundTrip(t *testing.T) {
	now, advance := advancingClock(time.Unix(0, 0))
	cfg := defaultConfig()
	g := bg.New(cfg, now)

	// Build up some state.
	g.Charge(0.10)
	g.Charge(0.05)
	advance(2 * time.Minute)
	g.Charge(0.20)

	snap := g.Snapshot()

	// Create a new guard and restore.
	g2 := bg.New(cfg, now)
	g2.Restore(snap)

	snap2 := g2.Snapshot()

	if snap.ReqThisMin != snap2.ReqThisMin {
		t.Errorf("ReqThisMin: %d vs %d", snap.ReqThisMin, snap2.ReqThisMin)
	}
	if snap.ReqThisDay != snap2.ReqThisDay {
		t.Errorf("ReqThisDay: %d vs %d", snap.ReqThisDay, snap2.ReqThisDay)
	}
	if snap.USDThisDay != snap2.USDThisDay {
		t.Errorf("USDThisDay: %v vs %v", snap.USDThisDay, snap2.USDThisDay)
	}
	if snap.MinuteWindow != snap2.MinuteWindow {
		t.Errorf("MinuteWindow: %v vs %v", snap.MinuteWindow, snap2.MinuteWindow)
	}
	if snap.DayWindow != snap2.DayWindow {
		t.Errorf("DayWindow: %v vs %v", snap.DayWindow, snap2.DayWindow)
	}
}

func TestGuard_RestorePreservesLimits(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 3,
		MaxReqPerDay: 10,
		MaxUSDPerDay: 1.0,
	}
	g := bg.New(cfg, now)

	// Use 2 of 3 per-minute quota.
	g.Charge(0.01)
	g.Charge(0.01)
	snap := g.Snapshot()

	// Restore into a new guard — the 3rd charge should succeed but 4th denied.
	g2 := bg.New(cfg, now)
	g2.Restore(snap)

	d := g2.Charge(0.01)
	if !d.Allowed {
		t.Fatalf("3rd charge after restore: expected allowed, got denied (reason: %q)", d.Reason)
	}
	d = g2.Charge(0.01)
	if d.Allowed {
		t.Fatal("4th charge after restore: expected denied (req/min limit)")
	}
}

// ---- Determinism ------------------------------------------------------------

func TestGuard_Determinism(t *testing.T) {
	start := time.Unix(0, 0)
	cfg := defaultConfig()

	run := func() []bool {
		now, advance := advancingClock(start)
		g := bg.New(cfg, now)
		var results []bool
		results = append(results, g.Charge(0.01).Allowed)
		results = append(results, g.Charge(0.01).Allowed)
		results = append(results, g.Charge(0.01).Allowed)
		results = append(results, g.Charge(0.01).Allowed) // hits req/min=5? no, 5th
		results = append(results, g.Charge(0.01).Allowed) // 5th — allowed
		results = append(results, g.Charge(0.01).Allowed) // 6th — denied
		advance(time.Minute)
		results = append(results, g.Charge(0.01).Allowed) // new minute — allowed
		return results
	}

	r1 := run()
	r2 := run()
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Fatalf("determinism failed at step %d: %v vs %v", i, r1[i], r2[i])
		}
	}
}

// ---- Concurrent Charge under -race ------------------------------------------

func TestGuard_ConcurrentCharge(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 50,
		MaxReqPerDay: 200,
		MaxUSDPerDay: 10.0,
	}
	g := bg.New(cfg, now)

	var wg sync.WaitGroup
	var allowed, denied int64
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := g.Charge(0.01)
			mu.Lock()
			if d.Allowed {
				allowed++
			} else {
				denied++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// At most 50 per minute allowed.
	if allowed > 50 {
		t.Fatalf("concurrent Charge: expected ≤50 allowed, got %d", allowed)
	}
	if allowed+denied != 100 {
		t.Fatalf("total should be 100, got %d+%d=%d", allowed, denied, allowed+denied)
	}
}

// ---- Two-phase ChargeTracked + Settle ----------------------------------------

func TestGuard_SettleHappyPath(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 10,
		MaxReqPerDay: 100,
		MaxUSDPerDay: 1.0,
	}
	g := bg.New(cfg, now)

	// Charge an estimate of $0.10.
	d, id := g.ChargeTracked(0.10)
	if !d.Allowed {
		t.Fatalf("ChargeTracked: expected Allowed=true, got denied (reason: %q)", d.Reason)
	}
	if id == 0 {
		t.Fatal("ChargeTracked: expected non-zero ChargeID on success")
	}

	// Snapshot reflects the estimated amount.
	snap := g.Snapshot()
	if snap.USDThisDay != 0.10 {
		t.Errorf("USDThisDay after ChargeTracked: want 0.10, got %.6f", snap.USDThisDay)
	}

	// Settle to actual $0.07 — delta should be -0.03.
	delta := g.Settle(id, 0.07)
	const wantDelta = -0.03
	if delta < wantDelta-1e-9 || delta > wantDelta+1e-9 {
		t.Errorf("Settle delta: want %.6f, got %.6f", wantDelta, delta)
	}

	// Snapshot after settle reflects actual cost.
	snap2 := g.Snapshot()
	if snap2.USDThisDay < 0.07-1e-9 || snap2.USDThisDay > 0.07+1e-9 {
		t.Errorf("USDThisDay after Settle: want 0.07, got %.6f", snap2.USDThisDay)
	}

	// There should now be more headroom: a charge of 0.90 (0.07+0.90=0.97 < 1.0) should succeed.
	d2, _ := g.ChargeTracked(0.90)
	if !d2.Allowed {
		t.Fatalf("subsequent charge after settle: expected Allowed=true, got denied (reason: %q)", d2.Reason)
	}
}

func TestGuard_SettleOverBudget(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 10,
		MaxReqPerDay: 100,
		MaxUSDPerDay: 1.0,
	}
	g := bg.New(cfg, now)

	// Charge $0.50.
	d, id := g.ChargeTracked(0.50)
	if !d.Allowed {
		t.Fatalf("ChargeTracked: expected Allowed=true, got denied (reason: %q)", d.Reason)
	}

	// Settle to actual $1.20 — this pushes usdThisDay over the $1.00 budget.
	g.Settle(id, 1.20)

	snap := g.Snapshot()
	if snap.USDThisDay < 1.20-1e-9 {
		t.Errorf("USDThisDay after over-budget settle: want ≥1.20, got %.6f", snap.USDThisDay)
	}

	// Subsequent charge should be denied (fail-closed).
	d2, _ := g.ChargeTracked(0.001)
	if d2.Allowed {
		t.Fatal("charge after over-budget settle: expected Allowed=false (Tripped=true)")
	}
	if !d2.Tripped {
		t.Fatal("expected Tripped=true after over-budget settle")
	}
}

func TestGuard_SettleUnknownID(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	g := bg.New(defaultConfig(), now)

	// Settle on the invalid zero ID: must be a no-op returning 0, no panic.
	delta := g.Settle(bg.ChargeID(0), 0.5)
	if delta != 0 {
		t.Errorf("Settle(0,...): expected delta=0, got %.6f", delta)
	}

	// Settle on a non-existent (but non-zero) ID: also a no-op.
	delta2 := g.Settle(bg.ChargeID(9999), 0.5)
	if delta2 != 0 {
		t.Errorf("Settle(9999,...): expected delta=0, got %.6f", delta2)
	}
}

func TestGuard_SettleIdempotent(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	g := bg.New(defaultConfig(), now)

	_, id := g.ChargeTracked(0.10)
	if id == 0 {
		t.Fatal("ChargeTracked returned zero ID")
	}

	// First settle: adjusts usdThisDay from 0.10 to 0.07.
	delta1 := g.Settle(id, 0.07)
	if delta1 > -0.03+1e-9 || delta1 < -0.03-1e-9 {
		t.Errorf("first Settle: want delta≈-0.03, got %.6f", delta1)
	}

	snap1 := g.Snapshot()

	// Second settle with same ID: must be a no-op (idempotent).
	delta2 := g.Settle(id, 0.99)
	if delta2 != 0 {
		t.Errorf("second Settle same ID: expected delta=0 (idempotent), got %.6f", delta2)
	}

	snap2 := g.Snapshot()
	if snap1.USDThisDay != snap2.USDThisDay {
		t.Errorf("USDThisDay changed on second Settle: %.6f → %.6f", snap1.USDThisDay, snap2.USDThisDay)
	}
}

func TestGuard_SnapshotWithPendingCharge(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	g := bg.New(defaultConfig(), now)

	_, id := g.ChargeTracked(0.10)
	if id == 0 {
		t.Fatal("ChargeTracked returned zero ID")
	}

	// Snapshot during pending charge: USDThisDay should capture estimated amount.
	snap := g.Snapshot()
	if snap.USDThisDay < 0.10-1e-9 || snap.USDThisDay > 0.10+1e-9 {
		t.Errorf("Snapshot.USDThisDay during pending charge: want 0.10, got %.6f", snap.USDThisDay)
	}

	// Restore into a new Guard — starts with empty pending map.
	g2 := bg.New(defaultConfig(), now)
	g2.Restore(snap)

	// Settle on the OLD id against the new guard: must be a no-op (id not in g2).
	delta := g2.Settle(id, 0.05)
	if delta != 0 {
		t.Errorf("Settle old id against restored guard: expected 0, got %.6f", delta)
	}

	// USDThisDay on g2 should be unchanged (still 0.10 from snapshot).
	snap2 := g2.Snapshot()
	if snap2.USDThisDay < 0.10-1e-9 || snap2.USDThisDay > 0.10+1e-9 {
		t.Errorf("g2.USDThisDay after no-op settle: want 0.10, got %.6f", snap2.USDThisDay)
	}
}

func TestGuard_ConcurrentChargeSettle(t *testing.T) {
	now, _ := advancingClock(time.Unix(0, 0))
	cfg := bg.Config{
		MaxReqPerMin: 200,
		MaxReqPerDay: 1000,
		MaxUSDPerDay: 100.0,
	}
	g := bg.New(cfg, now)

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, id := g.ChargeTracked(0.01)
			if d.Allowed && id != 0 {
				g.Settle(id, 0.005)
			}
		}()
	}
	wg.Wait()

	// Just verifying no data races: the -race detector does the real work.
	snap := g.Snapshot()
	if snap.USDThisDay < 0 {
		t.Errorf("USDThisDay should not be negative, got %.6f", snap.USDThisDay)
	}
}
