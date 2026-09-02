package main

// measuredstale.go — PC-5 (docs/plans/proof-credibility.md, vision 26 §1.1,
// ADR-027): a verdict produced by a stale daemon is a COVERAGE GAP, not a
// refutation. `plans status` and `intent audit` (and the doctor row that
// reads the audit) mark such rows "measured-stale" and count them apart
// from failing — a stale reading says nothing about the tree, so it may
// not be presented as a red row, and it may not be hidden either.
//
// "Stale" has one definition, shared by every reader (measuredStaleReading):
// the daemon's build stamp is known and is not the binary that ran the
// verify — or, inside the fathom source tree only, is not the tree commit
// the reading was taken at (O20's exact shape: the deployed daemon lagged
// HEAD by one commit while the installed CLI lagged with it). Outside the
// fathom checkout the tree commit is another project's history and is
// never compared (O5).

import (
	"net/http"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

// measuredStaleReading reports whether one ledger reading came from a
// stale daemon. An unknown daemon stamp is not stale — it is unknown, and
// the provenance notice already says so.
func measuredStaleReading(r verifyLedgerRow, fathomTree bool) bool {
	if r.DaemonCommit == "" {
		return false
	}
	if r.BinaryCommit != "" && !buildinfo.SameRevision(r.DaemonCommit, r.BinaryCommit) {
		return true
	}
	if fathomTree && r.Commit != "" && !buildinfo.SameRevision(r.DaemonCommit, r.Commit) {
		return true
	}
	return false
}

// markMeasuredStale flags every plan-row state whose plan's newest ledger
// reading is measured-stale and returns how many rows were flagged and the
// sorted plans they belong to. Flagged rows leave U1/U2 (intent.auditPlanRows
// reports them as NOT COMPUTED with the count) — never a failure.
func markMeasuredStale(rows []intent.PlanRowState, newest map[string]verifyLedgerRow, fathomTree bool) (flagged int, plans []string) {
	seen := map[string]bool{}
	for i := range rows {
		r, ok := newest[rows[i].Plan]
		if !ok || !measuredStaleReading(r, fathomTree) {
			continue
		}
		rows[i].MeasuredStale = true
		flagged++
		if !seen[rows[i].Plan] {
			seen[rows[i].Plan] = true
			plans = append(plans, rows[i].Plan)
		}
	}
	sort.Strings(plans)
	return flagged, plans
}

// liveDaemonStale answers the same question for a LIVE run (`plans status`
// over the daemon): is the daemon about to answer stale relative to this
// binary (and, in the fathom checkout, to HEAD)? Returns the verdict and a
// one-line reason for the table footer; false with "" when the daemon is
// current or its stamp is unknown.
func liveDaemonStale(hc *http.Client, baseURL, root string) (bool, string) {
	st, err := fetchStatusz(hc, baseURL)
	if err != nil || st.BinaryCommit == "" {
		return false, ""
	}
	fathomTree, _ := fathomSourceTree(root)
	row := verifyLedgerRow{
		DaemonCommit: stampString(st.BinaryCommit, st.BinaryDirty),
		BinaryCommit: buildinfo.Version(),
	}
	if fathomTree {
		row.Commit = gitHeadShort(root)
	}
	if !measuredStaleReading(row, fathomTree) {
		return false, ""
	}
	why := "daemon binary @ " + shortStamp(row.DaemonCommit) + " ≠ this binary @ " + shortStamp(row.BinaryCommit)
	if fathomTree && row.Commit != "" && !buildinfo.SameRevision(row.DaemonCommit, row.Commit) {
		why = "daemon binary @ " + shortStamp(row.DaemonCommit) + " ≠ HEAD " + shortStamp(row.Commit) + " of the fathom checkout"
	}
	return true, why
}
