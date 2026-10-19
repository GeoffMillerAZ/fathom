package decide

// caps.go — KS-10 (docs/plans/decision-port.md section J, ADR-073 decision
// 7, DMP-17): caps and deadlines. [decide] max_asks_per_site_per_session
// (config.Decide.MaxAsks: default 200, the minimum across layers) counts
// asks in process per (session, site), KS-14's decide-owned Scope; past it
// the site answers on its deterministic path and hears one `capped` line
// per session and site, written the way KS-14 writes `killed`
// (liveGate.discloseLocked). An ask whose path's ceiling its joins already
// spent is not made (askDeadline).
//
// Which sites ask on a hook path: the obligations re-rank alone, on the
// PostToolUse edit hook's obligations RPC, and only under rank_within_tier
// = true, which is off by default (ADR-042's 2026-09-23 amendment). The
// hook's client timeout (cmd/fathom/edithook.go) crosses the loopback as
// X-Fathom-Deadline (internal/hookguard), serve-http's rpcContext makes it
// the request context's deadline, and obligationsCore times its joins on
// its injected clock and asks AskBudget for the re-rank's budget: the
// ceiling less the joins, rerank_timeout_ms only below it. The port applies
// the same rule to every ask whose context carries a ceiling (admit).

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// The site-level closing sources (Gate.Source, decisions.jsonl
// kill_source): the ask cap (KS-10), a ceiling spent before the ask
// (KS-10) and the site's miss kill (KS-12, sitekill.go).
const (
	KillSourceCapped   = "capped"
	KillSourceDeadline = "deadline"
)

// allowAsk counts one ask from scope against cfg's cap and returns the
// cap's verdict (KS-10): open while scope has asked fewer than
// cfg.MaxAsks() times, else closed by KillSourceCapped, naming the layer
// whose value is in effect. A refused ask is not counted. Callers hold the
// lock guarding asks.
func allowAsk(asks map[Scope]int, scope Scope, cfg config.Decide) Gate {
	limit := cfg.MaxAsks()
	if asks[scope] >= limit {
		layer := string(cfg.MaxAsksSource)
		if layer == "" {
			layer = "default"
		}
		return Gate{Closed: true, Source: KillSourceCapped, Layer: layer,
			Reason: fmt.Sprintf("[decide] max_asks_per_site_per_session = %d reached; the rest of the session asks nothing", limit)}
	}
	asks[scope]++
	return Gate{}
}

// askDeadline is how long one ask on a path with a ceiling may run (KS-10,
// ADR-073 decision 7): the ceiling less the time the path's joins already
// took (now is when they finished), and never the site's own constant
// (siteTimeout, such as rerank_timeout_ms; 0 for none) past it. ok is false
// when the joins spent the ceiling: the site then answers on its
// deterministic path without asking.
func askDeadline(ceiling, now time.Time, siteTimeout time.Duration) (time.Duration, bool) {
	left := ceiling.Sub(now)
	if left <= 0 {
		return 0, false
	}
	if siteTimeout > 0 && siteTimeout < left {
		return siteTimeout, true
	}
	return left, true
}

// AskBudget is how long a site on a path with a ceiling may ask once its
// joins are done (KS-10): askDeadline from joinsDone against ctx's
// deadline, the caller's ceiling, with siteTimeout only below it. A ctx with
// no deadline (a CLI verb, a stdio MCP session) has no ceiling and keeps
// siteTimeout. A ceiling the joins spent refuses the ask before it is made:
// the error is CeilingSpent, naming the joins' measured time.
func AskBudget(ctx context.Context, joinsStart, joinsDone time.Time, siteTimeout time.Duration) (time.Duration, error) {
	ceiling, ok := ctx.Deadline()
	if !ok {
		return siteTimeout, nil
	}
	left, ok := askDeadline(ceiling, joinsDone, siteTimeout)
	if !ok {
		return 0, CeilingSpent(joinsDone.Sub(joinsStart))
	}
	return left, nil
}

// CeilingSpent is the refusal of an ask whose path's joins spent its
// ceiling (KS-10): a *KilledError, kill_source deadline, that is
// decision.ErrNoProvider, so the site answers on its deterministic path and
// its decisions.jsonl row names the source.
func CeilingSpent(joins time.Duration) error {
	return &KilledError{Gate: ceilingGate(fmt.Sprintf("the path's joins took %dms and spent its ceiling before the ask", joins.Milliseconds()))}
}

// ceilingGate is the closed verdict of a spent ceiling.
func ceilingGate(reason string) Gate {
	return Gate{Closed: true, Source: KillSourceDeadline, Layer: "context", Reason: reason}
}

// closedVerb names what closed the port for its line and its error:
// `capped` for the ask cap, `out of time` for a spent ceiling, `killed`
// for every kill.
func closedVerb(source string) string {
	switch source {
	case KillSourceCapped:
		return "capped"
	case KillSourceDeadline:
		return "out of time"
	}
	return "killed"
}
