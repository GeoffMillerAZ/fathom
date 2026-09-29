package intent

// registered.go — PE-25 (docs/design/progress-and-economics.md §11,
// observation O121): scratch-clone runs of `fathom intent audit` appended
// rows keyed by throwaway roots to the intent ledger (91 of 180 on
// 2026-09-28). Every reader of the ledger — the audit's baseline and debt,
// the kill clock, learning announcements, a condition's first-fired moment —
// reads only rows whose repo_root is a registered service root (PE-23's
// ledgerroots rule) and names how many it ignored. A reader never rewrites
// the file, and the writers' dedup reads (StartClock, RecordBaseline,
// LearningRecorded's caller, the enforcement-probe cache) stay raw: a
// filtered dedup would append the same row again.

import "github.com/geoffmilleraz/fathom/internal/ledgerroots"

// registeredLedgerRows keeps the rows whose repo_root is a registered
// service root under roots, and returns what it ignored. An unread registry
// keeps every row and says why (I38).
func registeredLedgerRows(rows []LedgerRow, roots ledgerroots.Roots) ([]LedgerRow, ledgerroots.Ignored) {
	f := roots.NewFilter()
	out := make([]LedgerRow, 0, len(rows))
	for _, r := range rows {
		if f.Keep(r.RepoRoot) {
			out = append(out, r)
		}
	}
	return out, f.Ignored()
}

// ReadRegisteredLedger is ReadLedger kept to roots' registered service
// roots, with what that ignored.
func ReadRegisteredLedger(roots ledgerroots.Roots) ([]LedgerRow, ledgerroots.Ignored) {
	return registeredLedgerRows(ReadLedger(), roots)
}

// RegisteredRows is the same rule over rows a reader already holds (the
// session hook's attention line narrows the ledger to its own root first,
// so it canonicalizes one root, not every row's).
func RegisteredRows(rows []LedgerRow, roots ledgerroots.Roots) ([]LedgerRow, ledgerroots.Ignored) {
	return registeredLedgerRows(rows, roots)
}
