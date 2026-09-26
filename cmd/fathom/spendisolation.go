package main

// spendisolation.go — IR-4 (docs/design/assess-lanes.md §3, ADR-046),
// pre-registered 2026-09-25 before any assess run: the cost of judging a
// unit never enters that unit's figures. This is the one predicate every
// unit selector consults — armUnitFor and the impact model (the A/B arm),
// threadBetCheck (the continuity bet) — and the flywheel's arm loop reads
// the same label through assess.IsolatedSpendLabel.

import "github.com/geoffmilleraz/fathom/internal/assess"

// spendIsolatedRow reports whether a session-ledger row was written inside
// a judge process `fathom assess` started (its spend_label is set). Any
// non-empty label is isolated, so a future lane's label fails toward
// exclusion rather than leaking into an arm or bet total.
func spendIsolatedRow(e sessionLedgerEntry) bool {
	return assess.IsolatedSpendLabel(e.SpendLabel)
}
