package main

// primer_tenancy.go — the session primer's tenancy attention line (RT-51,
// design §13.3). One line, at most tenancyAttentionBudget bytes, inside
// I12's 3,200 B primer cap, and only when the daemon is not normal: shed
// or refusing, in safe mode, crash-looping, stopped by the breaker, or
// holding a quarantined service. It reads files only (the ring, the
// lifecycle ledger, the switch files) — no RPC on the session-start path —
// and a ring sample older than tenancySampleFresh claims no level: a daemon
// that stopped writing is the identity line's to report.

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// tenancyAttentionBudget caps the line (§13.3).
const tenancyAttentionBudget = 160

// tenancySampleFresh is how recent a ring sample must be to be judged as
// the daemon's current state (the primer's level, resources' verdict,
// doctor's host_pressure): the ring is written every minute, so an older
// one means the daemon stopped writing — it is shown with its age, never
// judged as now.
const tenancySampleFresh = 5 * time.Minute

// primerTenancyLine is what the primer appends (a var so tests replace it).
var primerTenancyLine = func(now time.Time) string {
	env := liveResourcesEnv()
	env.fetch, env.census, env.hooks, env.budget = nil, nil, nil, nil
	env.size = func(string) int64 { return 0 }
	env.now = func() time.Time { return now }
	return tenancyAttentionLine(buildResources(env, "", defaultFathomURL(), time.Hour), now)
}

// tenancyAttentionLine renders the line, or "" when the daemon is normal.
func tenancyAttentionLine(rep resourcesReport, now time.Time) string {
	loc := now.Location()
	hhmm := func(s string) string {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.In(loc).Format("15:04")
		}
		return ""
	}
	l := rep.Lifecycle
	if l.BreakerStopped {
		at := hhmm(l.SwitchSince)
		if at != "" {
			at = " " + at
		}
		return capLine(fmt.Sprintf("fathom stopped by breaker%s — operator: fathom daemon start (first: fathom resources)", at))
	}
	t := rep.Tenancy
	fresh := false
	if t != nil {
		if at, err := time.Parse(time.RFC3339, t.SampleAt); err == nil && now.Sub(at) <= tenancySampleFresh {
			fresh = true
		}
	}
	top := ""
	if fresh && t.Oldest != nil {
		top = "top: " + t.Oldest.Tool
	}
	paren := func(parts ...string) string {
		var keep []string
		for _, p := range parts {
			if p != "" {
				keep = append(keep, p)
			}
		}
		if len(keep) == 0 {
			return ""
		}
		return " (" + strings.Join(keep, "; ") + ")"
	}
	exits := ""
	if l.CountedExits30m > 0 {
		exits = fmt.Sprintf("%d counted exits/30m", l.CountedExits30m)
	}
	if fresh && (t.Level == tenancy.LevelShed.String() || t.Level == tenancy.LevelRefuse.String() || t.Level == tenancy.LevelExit.String()) {
		since := ""
		if s := hhmm(t.Since); s != "" {
			since = " since " + s
		}
		use := ""
		if t.Footprint >= 0 && rep.Budget.Budget > 0 {
			c := t.Footprint
			if t.Children > 0 {
				c += t.Children
			}
			use = fmt.Sprintf("%s of %s budget", tenancy.FormatBytes(c), tenancy.FormatBytes(rep.Budget.Budget))
		}
		return capLine(fmt.Sprintf("fathom: %s%s%s — fathom resources", t.Level, since, paren(use, top)))
	}
	if fresh && t.Mode == "safe" {
		since := ""
		if strings.HasPrefix(l.Switch, "paused") {
			if s := hhmm(l.SwitchSince); s != "" {
				since = " since " + s
			}
		}
		return capLine(fmt.Sprintf("fathom: safe mode%s%s — fathom resources", since, paren(exits, top)))
	}
	if l.CrashLooping {
		return capLine(fmt.Sprintf("fathom: crash-looping%s — fathom resources", paren(exits, top)))
	}
	if len(rep.quarantine) > 0 {
		q := rep.quarantine[0]
		more := ""
		if n := len(rep.quarantine) - 1; n > 0 {
			more = fmt.Sprintf(" +%d more", n)
		}
		return capLine(fmt.Sprintf("fathom: quarantined %s%s (%s) — retry with the daemon's ingest tool", q.Service, more, q.Reason))
	}
	return ""
}

// capLine cuts a line to tenancyAttentionBudget bytes on a rune boundary,
// marking the cut with "…".
func capLine(s string) string {
	if len(s) <= tenancyAttentionBudget {
		return s
	}
	const mark = "…"
	cut := tenancyAttentionBudget - len(mark)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + mark
}
