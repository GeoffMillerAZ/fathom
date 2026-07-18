// Package budgetguard provides a multi-dimension aggregate quota guard that
// enforces per-minute request counts, per-day request counts, and per-day USD
// spend limits. It is the bill cap for agent workflows.
//
// Design constraints (WASM-safe):
//   - stdlib only: no third-party dependencies.
//   - No time.Now() in logic: callers inject a now func() time.Time.
//   - No direct file I/O: persistence via Snapshot/Restore (plain structs).
//   - Concurrency-safe via sync.Mutex.
//   - Deterministic and fail-closed: when any dimension is exceeded, Charge
//     returns Allowed=false and Tripped=true with the first-exceeded reason.
package budgetguard

import (
	"fmt"
	"sync"
	"time"
)

// Config holds the quota limits for a Guard.
type Config struct {
	// MaxReqPerMin is the maximum number of requests allowed per rolling minute.
	// Must be ≥ 1.
	MaxReqPerMin int

	// MaxReqPerDay is the maximum number of requests allowed per rolling day.
	// Must be ≥ 1.
	MaxReqPerDay int

	// MaxUSDPerDay is the maximum estimated USD spend allowed per rolling day.
	// Must be > 0.
	MaxUSDPerDay float64
}

// Decision is returned by Charge and reports whether the charge was permitted.
type Decision struct {
	// Allowed is true when all dimensions are within their limits.
	Allowed bool

	// Tripped is true when at least one dimension has been exceeded.
	// Always the complement of Allowed.
	Tripped bool

	// Reason is a human-readable description of the first dimension that was
	// exceeded. Empty when Allowed is true.
	Reason string
}

// ChargeID is an opaque identifier for a two-phase charge issued by
// ChargeTracked. The zero value (0) is invalid and is returned on denial.
type ChargeID uint64

// pendingCharge records the estimated cost committed by ChargeTracked,
// awaiting reconciliation via Settle.
type pendingCharge struct {
	estimatedUSD float64
}

// Snapshot is a serializable copy of Guard's runtime counters and window
// boundaries. It can be used for in-pod restart durability via Restore.
// All fields are exported plain types — no io.Reader/Writer needed.
//
// Snapshot captures the committed (possibly adjusted) USDThisDay total.
// The pending map is live-only: a restored guard starts with no pending
// charges, which is correct because a restarted process has no in-flight calls.
type Snapshot struct {
	// ReqThisMin is the number of requests recorded in the current minute window.
	ReqThisMin int

	// ReqThisDay is the number of requests recorded in the current day window.
	ReqThisDay int

	// USDThisDay is the accumulated USD spend in the current day window.
	USDThisDay float64

	// MinuteWindow is the start of the current one-minute rolling window.
	MinuteWindow time.Time

	// DayWindow is the start of the current one-day rolling window.
	DayWindow time.Time
}

// Guard enforces multi-dimension quotas: per-minute requests, per-day requests,
// and per-day USD spend.
//
// It is safe for concurrent use.
type Guard struct {
	mu           sync.Mutex
	cfg          Config
	now          func() time.Time
	reqThisMin   int
	reqThisDay   int
	usdThisDay   float64
	minuteWindow time.Time // start of current minute window
	dayWindow    time.Time // start of current day window
	pending      map[ChargeID]pendingCharge
	nextChargeID uint64
}

// New constructs a Guard with the given configuration and injected clock.
// Both rolling windows are initialised to the current time.
func New(cfg Config, now func() time.Time) *Guard {
	cur := now()
	return &Guard{
		cfg:          cfg,
		now:          now,
		minuteWindow: cur,
		dayWindow:    cur,
		pending:      make(map[ChargeID]pendingCharge),
	}
}

// Charge attempts to record a request with estimated cost estUSD.
// It enforces limits in this order (first-exceeded wins):
//  1. MaxReqPerMin
//  2. MaxReqPerDay
//  3. MaxUSDPerDay
//
// When all dimensions are within limits, counters are incremented and
// Decision.Allowed is true. Otherwise, no counters are modified and
// Decision.Tripped is true with the first-exceeded Reason.
func (g *Guard) Charge(estUSD float64) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	g.advanceWindows(now)

	// Evaluate all three dimensions; fail-closed on the first exceeded.
	if g.reqThisMin+1 > g.cfg.MaxReqPerMin {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason:  fmt.Sprintf("req/min limit exceeded: %d/%d", g.reqThisMin, g.cfg.MaxReqPerMin),
		}
	}
	if g.reqThisDay+1 > g.cfg.MaxReqPerDay {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason:  fmt.Sprintf("req/day limit exceeded: %d/%d", g.reqThisDay, g.cfg.MaxReqPerDay),
		}
	}
	if g.usdThisDay+estUSD > g.cfg.MaxUSDPerDay {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason: fmt.Sprintf("USD/day limit exceeded: %.6f+%.6f > %.6f",
				g.usdThisDay, estUSD, g.cfg.MaxUSDPerDay),
		}
	}

	// All dimensions within limits: commit the charge.
	g.reqThisMin++
	g.reqThisDay++
	g.usdThisDay += estUSD

	return Decision{Allowed: true}
}

// ChargeTracked is the two-phase entry point for pre-flight cost estimation.
// It behaves identically to Charge but also assigns an opaque ChargeID that
// can later be passed to Settle to reconcile the estimate against the actual
// cost. A denied request returns Decision{Tripped: true} and ChargeID(0)
// (the invalid sentinel); no counters are modified in that case.
//
// Use ChargeTracked + Settle when the actual cost is only known after the
// work completes (e.g. LLM calls). Use Charge for fire-and-forget accounting
// where reconciliation is not needed.
func (g *Guard) ChargeTracked(estUSD float64) (Decision, ChargeID) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	g.advanceWindows(now)

	// Evaluate all three dimensions; fail-closed on the first exceeded.
	if g.reqThisMin+1 > g.cfg.MaxReqPerMin {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason:  fmt.Sprintf("req/min limit exceeded: %d/%d", g.reqThisMin, g.cfg.MaxReqPerMin),
		}, 0
	}
	if g.reqThisDay+1 > g.cfg.MaxReqPerDay {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason:  fmt.Sprintf("req/day limit exceeded: %d/%d", g.reqThisDay, g.cfg.MaxReqPerDay),
		}, 0
	}
	if g.usdThisDay+estUSD > g.cfg.MaxUSDPerDay {
		return Decision{
			Allowed: false,
			Tripped: true,
			Reason: fmt.Sprintf("USD/day limit exceeded: %.6f+%.6f > %.6f",
				g.usdThisDay, estUSD, g.cfg.MaxUSDPerDay),
		}, 0
	}

	// All dimensions within limits: commit the charge and record it as pending.
	g.reqThisMin++
	g.reqThisDay++
	g.usdThisDay += estUSD

	g.nextChargeID++
	id := ChargeID(g.nextChargeID)
	g.pending[id] = pendingCharge{estimatedUSD: estUSD}

	return Decision{Allowed: true}, id
}

// Settle reconciles a prior ChargeTracked to the actual cost. It returns the
// delta (actualUSD - estimatedUSD). Unknown or already-settled IDs are no-ops
// that return 0. usdThisDay is clamped to 0 if the adjustment would push it
// negative (this can happen when the actual cost is lower than the estimate
// and other settled charges have already reduced the total).
//
// Settle is idempotent: a second call with the same ChargeID returns 0 and
// makes no further adjustment.
//
// Settle never errors. If settling pushes usdThisDay over the budget limit,
// subsequent Charge/ChargeTracked calls will fail — this is correct
// fail-closed behaviour (the day's spend is genuinely over budget).
func (g *Guard) Settle(id ChargeID, actualUSD float64) float64 {
	if id == 0 {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	pc, ok := g.pending[id]
	if !ok {
		return 0
	}
	delete(g.pending, id)

	delta := actualUSD - pc.estimatedUSD
	g.usdThisDay += delta
	if g.usdThisDay < 0 {
		g.usdThisDay = 0
	}
	return delta
}

// advanceWindows resets per-minute and/or per-day counters when the rolling
// window has elapsed. Must be called with g.mu held.
func (g *Guard) advanceWindows(now time.Time) {
	if now.Sub(g.minuteWindow) >= time.Minute {
		g.reqThisMin = 0
		g.minuteWindow = now
	}
	if now.Sub(g.dayWindow) >= 24*time.Hour {
		g.reqThisDay = 0
		g.usdThisDay = 0
		g.dayWindow = now
	}
}

// Snapshot returns a serializable copy of the current runtime state.
// It can be persisted and later passed to Restore to resume quota tracking
// across process restarts without any file I/O in the Guard itself.
//
// USDThisDay captures the committed (possibly adjusted) total; the pending
// map is live-only and is not included. A restored Guard starts with no
// pending charges, which is correct since a restarted process has no
// in-flight calls.
func (g *Guard) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return Snapshot{
		ReqThisMin:   g.reqThisMin,
		ReqThisDay:   g.reqThisDay,
		USDThisDay:   g.usdThisDay,
		MinuteWindow: g.minuteWindow,
		DayWindow:    g.dayWindow,
	}
}

// Restore replaces the Guard's runtime counters with those from the snapshot.
// The injected clock is NOT modified; only the quota counters and window
// boundaries are restored. This is idempotent and safe to call at any time.
// The pending charge map is reset to empty (see Snapshot for rationale).
func (g *Guard) Restore(snap Snapshot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.reqThisMin = snap.ReqThisMin
	g.reqThisDay = snap.ReqThisDay
	g.usdThisDay = snap.USDThisDay
	g.minuteWindow = snap.MinuteWindow
	g.dayWindow = snap.DayWindow
	// Reset pending map: a restored Guard has no in-flight calls.
	g.pending = make(map[ChargeID]pendingCharge)
}
