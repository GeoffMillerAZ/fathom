package refresh

import (
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TenancyGate is the process's gate (design §7.1, §2.1, §2.3): store
// distress and the refuse rung refuse every refresh; for background work
// (auto-refresh, post-commit, /refresh-file) safe mode and a quarantine
// refuse too, and shed pauses it. An explicit `ingest` passes a quarantine
// (only it lifts one) and safe mode (which refuses tree and fleet reads,
// never ingest — RT-39), and is not paused at shed: admission already
// admitted it in its class.
func TenancyGate(service string, explicit bool, now time.Time) Verdict {
	if why, on := tenancy.StoreDistressNow(now); on {
		return Verdict{Refuse: why}
	}
	lvl := tenancy.CurrentLevel()
	if lvl >= tenancy.LevelRefuse {
		return Verdict{Refuse: "refused (" + lvl.String() + ") — fathom is over its memory budget; fathom resources"}
	}
	if explicit {
		return Verdict{}
	}
	if s, on := tenancy.SafeModeNow(); on {
		return Verdict{Refuse: "refused (safe mode: " + s.Reason + ") — background refreshes are off; an explicit `ingest` still runs"}
	}
	if q, ok := tenancy.QuarantineOf(service); ok {
		return Verdict{Refuse: "refused (" + q.Attention() + ") — a background refresh never lifts a quarantine"}
	}
	if lvl >= tenancy.LevelShed {
		return Verdict{Pause: "paused (" + lvl.String() + ") — background refreshes resume below shed"}
	}
	return Verdict{}
}

func tenancyWarmup(now time.Time) bool { return tenancy.InWarmup(now) }
