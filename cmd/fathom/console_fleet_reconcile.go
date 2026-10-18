// console_fleet_reconcile.go — SR-55 (docs/plans/surface-reckoning.md,
// design doc §5 row SR-55, F-57): the fleet page's own reconcile trigger.
// consoleReconcileAction runs the SAME dry-run wiring analysis
// (analyzeWiringFor, reconcile.go) `fathom fleet reconcile` runs before
// --apply — read-only, never a file write — across every project the
// fleet page already knows about, so drift renders right there instead of
// requiring a separate CLI run.
//
// User-triggered only (GET /fleet?reconcile=1 — the fleet page's own
// "Check wiring drift" button, console_fleet.go): it runs when a person
// loads that URL, never on a timer or a background feeder — fathom
// schedules no fleet-wide reconcile of its own. Applying drift (--apply)
// stays a CLI-only, explicitly-invoked action: the console SHOWS drift
// and names the command that fixes it, the same "remedy command, never a
// button that mutates a repository" discipline every existing console
// Actions link already keeps (they are all either a GET to another page
// or a printed CLI command — never a write). A GET is also the
// technically correct verb here: this analysis has no side effect to
// protect against, so it needs no new mutating route on the console
// router (which otherwise refuses every method but GET/HEAD,
// console_router.go's pageHandler).
package main

import "sort"

// consoleReconcileAction is SR-55's anchor (F-57): analyzeWiringFor over
// every (root, service) pair, sorted by root so the fleet page's render
// is deterministic regardless of the caller's own ordering (a Go map
// range, most commonly).
func consoleReconcileAction(roots []string, svcByRoot map[string]string) []wiringDiff {
	sorted := append([]string(nil), roots...)
	sort.Strings(sorted)
	out := make([]wiringDiff, 0, len(sorted))
	for _, root := range sorted {
		out = append(out, analyzeWiringFor(root, svcByRoot[root]))
	}
	return out
}

// wiringDiffHasDrift mirrors cmdFleetReconcile's own hasDrift test exactly
// (reconcile.go) — the fleet page and the CLI must always agree on what
// counts as drift, so a repo the console shows red is never one the CLI
// would call clean, or the reverse.
func wiringDiffHasDrift(d wiringDiff) bool {
	return len(d.Missing) > 0 || len(d.StaleCmds) > 0 || len(d.MissingSkills) > 0 || len(d.StaleSkills) > 0 ||
		len(d.BlockedSkills) > 0 || d.SettingsNote != "" || len(d.StaleStanzas) > 0 ||
		len(d.DriftedStanzas) > 0 || len(d.WiringDrift) > 0 || len(d.WiringUnreadable) > 0
}
