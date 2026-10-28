package benchprep

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProvenanceRow is the slice of one scenario ledger row that NP-B1's
// evaluator refusals read: which run and cell it files, the prereg sha its
// cells were run under and the earliest started_at over them.
type ProvenanceRow struct {
	RunID        string
	TaskID       string
	Arm          string
	PreregSHA256 string
	FirstCellAt  string
}

// ProvenanceReading is CheckPreregProvenance's verdict on one run. A zero
// CommittedAt means the prereg has no commit in history; a zero FirstCellAt
// means no row of the run carried a readable first_cell_at.
type ProvenanceReading struct {
	RunID        string
	PreregSHA256 string
	CommittedAt  time.Time
	FirstCellAt  time.Time
	Rows         int
	// Refusals holds every cause that makes the run unreadable, each
	// "<name>: <detail>", sorted. Empty means provenance holds.
	Refusals []string
}

// Refused reports whether any refusal applies.
func (r ProvenanceReading) Refused() bool { return len(r.Refusals) > 0 }

// CheckPreregProvenance decides whether runID's ledger rows can be read
// against the prereg whose file hashes to preregSHA and was last committed
// at committedAt (NP-B1, BX-88). It is pure: the caller reads the commit
// time from git and the rows from the ledger. Every applicable refusal is
// named, never only the first (I38):
//
//   - no-rows: no row carries runID;
//   - missing-prereg-sha256 / missing-first-cell-at: a row lacks a stamp;
//   - unreadable-first-cell-at: a first_cell_at is not RFC3339;
//   - prereg-sha256-mismatch: a row's sha is not the file's, both named;
//   - commit-time-unknown: the prereg is not in history;
//   - committed-after-first-cell: the prereg was last committed after the
//     run's first cell started, the minimum first_cell_at over its rows.
//     A commit at that instant passes.
//
// Rows of every state count, voided ones included: a row's provenance does
// not depend on whether its reading was later voided.
func CheckPreregProvenance(runID string, committedAt time.Time, preregSHA string, rows []ProvenanceRow) ProvenanceReading {
	reading := ProvenanceReading{RunID: runID, PreregSHA256: preregSHA, CommittedAt: committedAt.UTC()}
	if committedAt.IsZero() {
		reading.CommittedAt = time.Time{}
	}
	var noSHA, noFirst, unreadable []string
	mismatched := map[string][]string{}
	for _, r := range rows {
		if r.RunID != runID {
			continue
		}
		reading.Rows++
		cell := r.TaskID + "/" + r.Arm
		switch {
		case r.PreregSHA256 == "":
			noSHA = append(noSHA, cell)
		case r.PreregSHA256 != preregSHA:
			mismatched[r.PreregSHA256] = append(mismatched[r.PreregSHA256], cell)
		}
		if r.FirstCellAt == "" {
			noFirst = append(noFirst, cell)
			continue
		}
		at, err := time.Parse(time.RFC3339, r.FirstCellAt)
		if err != nil {
			unreadable = append(unreadable, fmt.Sprintf("%s %q", cell, r.FirstCellAt))
			continue
		}
		if at = at.UTC(); reading.FirstCellAt.IsZero() || at.Before(reading.FirstCellAt) {
			reading.FirstCellAt = at
		}
	}

	var refusals []string
	if reading.Rows == 0 {
		refusals = append(refusals, fmt.Sprintf("no-rows: no ledger row carries run_id %s", runID))
	}
	if len(noSHA) > 0 {
		refusals = append(refusals, fmt.Sprintf("missing-prereg-sha256: %d row(s) lack prereg_sha256: %s", len(noSHA), provenanceJoin(noSHA)))
	}
	if len(noFirst) > 0 {
		refusals = append(refusals, fmt.Sprintf("missing-first-cell-at: %d row(s) lack first_cell_at: %s", len(noFirst), provenanceJoin(noFirst)))
	}
	if len(unreadable) > 0 {
		refusals = append(refusals, fmt.Sprintf("unreadable-first-cell-at: %d row(s) carry a first_cell_at that is not RFC3339: %s", len(unreadable), provenanceJoin(unreadable)))
	}
	if len(mismatched) > 0 {
		shas := make([]string, 0, len(mismatched))
		for s := range mismatched {
			shas = append(shas, s)
		}
		sort.Strings(shas)
		parts := make([]string, len(shas))
		for i, s := range shas {
			parts[i] = fmt.Sprintf("%s on %s", s, provenanceJoin(mismatched[s]))
		}
		refusals = append(refusals, fmt.Sprintf("prereg-sha256-mismatch: the prereg file hashes to %s, rows carry %s", preregSHA, strings.Join(parts, "; ")))
	}
	if reading.CommittedAt.IsZero() {
		refusals = append(refusals, "commit-time-unknown: the prereg file has no commit in history")
	} else if !reading.FirstCellAt.IsZero() && reading.CommittedAt.After(reading.FirstCellAt) {
		refusals = append(refusals, fmt.Sprintf("committed-after-first-cell: the prereg was last committed at %s, after the run's first cell started at %s",
			reading.CommittedAt.Format(time.RFC3339), reading.FirstCellAt.Format(time.RFC3339)))
	}
	sort.Strings(refusals)
	reading.Refusals = refusals
	return reading
}

func provenanceJoin(items []string) string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return strings.Join(out, ", ")
}
