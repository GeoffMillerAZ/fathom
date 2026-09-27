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
	"time"
)

// The safe-mode flag and the warm-up window are lifecycle_state.go's
// (SafeModeNow, SetWarmup/Warmup/InWarmup): one set of process modes, read
// by this gate, the RPC lifecycle gate and /statusz alike.

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
	_, safe := SafeModeNow()
	g := RefreshGate{Level: CurrentLevel(), SafeMode: safe}
	if InWarmup(now) {
		start, d := Warmup()
		g.Warmup, g.WarmupLeft = true, start.Add(d).Sub(now)
	}
	if q, ok := QuarantineOf(service); ok {
		g.Quarantined, g.Quarantine = true, q
	}
	return RefreshDeferral(g)
}
