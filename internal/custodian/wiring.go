package custodian

// wiring.go — the `wiring_current` check kind (PC-30, vision 26 §1.4): the
// custodian's "instructed means true" applied to the wiring fathom trusts.
// The evaluator lives in cmd/fathom (it needs the harness registry and the
// hook table, which this package must not import) and is injected with
// WithWiringDrift; without one the concern is unable-to-check, never
// violated. It reads files only — never the daemon.

import (
	"fmt"
	"strings"
)

// WiringDriftFunc evaluates one repo: the drift lines (kind path — reason),
// the files that could not be read, and how many entries were checked.
type WiringDriftFunc func(root string) (drifts, unreadable []string, checked int)

// WithWiringDrift injects the wiring evaluator.
func WithWiringDrift(f WiringDriftFunc) Option {
	return func(e *auditEnv) { e.wiringDrift = f }
}

func checkWiringCurrent(root string, c Concern, env auditEnv) Finding {
	f := Finding{Concern: c}
	if env.wiringDrift == nil {
		f.Status = StatusUnable
		f.Detail = "no wiring evaluator wired into this audit (cmd/fathom injects it) — cannot check"
		return f
	}
	drifts, unreadable, checked := env.wiringDrift(root)
	switch {
	case len(drifts) > 0:
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s drifted since fathom recorded %s: %s", countNoun(len(drifts), "wiring entry"), pluralIt(len(drifts)), strings.Join(drifts, "; "))
		f.Evidence = append(f.Evidence, drifts...)
		if len(unreadable) > 0 {
			f.Detail += fmt.Sprintf("; %s could NOT BE READ: %s", countNoun(len(unreadable), "file"), strings.Join(unreadable, "; "))
		}
	case len(unreadable) > 0:
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("%s could NOT BE READ and may carry drift: %s", countNoun(len(unreadable), "file"), strings.Join(unreadable, "; "))
		f.Evidence = append(f.Evidence, unreadable...)
	default:
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("no drift across %s (hooks, stanza regions, MCP entries, owned skills)", countNoun(checked, "checked entry"))
	}
	return f
}

func pluralIt(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}
