package tenancy

// refreshgate.go — RT-150 (docs/design/responsible-tenant.md §7.1, §2.1,
// §2.3): when auto-refresh may run an ingest. ensure() (internal/mcp) asks
// RefreshDeferralNow before any drift check; a non-empty answer means the
// read is served from the index as it stands, disclosed as
// `refresh: deferred (<reason>)`. Background ingest pauses at shed and
// above, is off in safe mode, runs no inline ingest during warm-up after a
// start, and never re-queues a quarantined service.

import (
	"strings"
	"sync/atomic"
	"time"
)

// DefaultWarmup is [tenancy.breaker].warmup's default (§2.3).
const DefaultWarmup = 120 * time.Second

// WarmupDuration is the configured warm-up, or DefaultWarmup.
func (b BreakerSettings) WarmupDuration() time.Duration {
	if b.Warmup > 0 {
		return b.Warmup
	}
	return DefaultWarmup
}

var (
	safeMode    atomic.Bool
	warmupStart atomic.Int64 // Unix nano; 0 = none
	warmupUntil atomic.Int64 // Unix nano; 0 = none
)

// SafeModeFlag is the process's safe-mode flag: set while paused.json
// holds (RT-6) and, from RT-38, by the breaker. cmd/fathom writes it; the
// refresh gate (and RT-39's refusals) read it.
func SafeModeFlag() *atomic.Bool { return &safeMode }

// StartWarmup opens the warm-up window [at, at+d); a zero d clears it.
func StartWarmup(at time.Time, d time.Duration) {
	if d <= 0 || at.IsZero() {
		warmupStart.Store(0)
		warmupUntil.Store(0)
		return
	}
	warmupStart.Store(at.UnixNano())
	warmupUntil.Store(at.Add(d).UnixNano())
}

// InWarmup reports whether now falls in the warm-up window, and when the
// window ends.
func InWarmup(now time.Time) (bool, time.Time) {
	from, until := warmupStart.Load(), warmupUntil.Load()
	if until == 0 {
		return false, time.Time{}
	}
	n := now.UnixNano()
	if n < from || n >= until {
		return false, time.Time{}
	}
	return true, time.Unix(0, until)
}

// QuarantineOf returns service's quarantine, if it is quarantined.
func QuarantineOf(service string) (Quarantine, bool) {
	quarantineMu.Lock()
	defer quarantineMu.Unlock()
	q, ok := quarantined[service]
	return q, ok
}

// RefreshGate is the state the refresh deferral is decided from.
type RefreshGate struct {
	Level       Level
	SafeMode    bool
	Warmup      bool
	WarmupLeft  time.Duration
	Quarantined bool
	Quarantine  Quarantine
}

// RefreshDeferral names why an auto-refresh ingest must not run now —
// "deferred (<reason>) — …" — or returns "" when it may. Pure. Order: a
// quarantine (the service's own state) first, then safe mode, the ladder
// level, and warm-up.
func RefreshDeferral(g RefreshGate) string {
	const served = " — served from the index as it stands; freshness not checked"
	switch {
	case g.Quarantined:
		reason := g.Quarantine.Reason
		if reason == "" {
			reason = QuarantineReasonIngest
		}
		return "deferred (quarantined: " + reason + ")" + served + "; an explicit `ingest` of " + g.Quarantine.Service + " lifts it"
	case g.SafeMode:
		return "deferred (safe mode)" + served + " — fathom resources"
	case g.Level >= LevelShed:
		return "deferred (" + g.Level.String() + ")" + served + " — fathom resources"
	case g.Warmup:
		left := g.WarmupLeft.Round(time.Second)
		return "deferred (warm-up, " + strings.TrimSpace(left.String()) + " left)" + served
	}
	return ""
}

// RefreshDeferralNow is RefreshDeferral over the process's state for
// service at now.
func RefreshDeferralNow(service string, now time.Time) string {
	g := RefreshGate{Level: CurrentLevel(), SafeMode: safeMode.Load()}
	if in, until := InWarmup(now); in {
		g.Warmup, g.WarmupLeft = true, until.Sub(now)
	}
	if q, ok := QuarantineOf(service); ok {
		g.Quarantined, g.Quarantine = true, q
	}
	return RefreshDeferral(g)
}
