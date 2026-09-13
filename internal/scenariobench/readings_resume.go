package scenariobench

import "sort"

// readings_resume.go is SB-22 (vision 27 §3, M4): does a later session
// resume without re-discovery, and does the change's contract still hold
// after it?

// ResumptionReadings computes M4 over a task's whole session sequence
// (chronological order, as recorded): tokens spent before the first edit
// in every session from the second on (the cost of re-orienting after a
// cold hand-off — session 1 pays discovery once by construction, so it is
// excluded), and edits to rows the caller's `proven` map already says are
// correct — a later checkbox on a row that was already true and stayed
// true is duplicated work, not a correction. `proven` is the caller's
// ground truth (typically FalseDoneRate's verdict inverted per row, from
// the FINAL tree) — this function never re-derives it, so it stays a pure
// read over `sessions`.
func ResumptionReadings(sessions []SessionResult, proven map[string]bool) ResumptionReading {
	reading := ResumptionReading{Sessions: len(sessions)}
	if len(sessions) < 2 {
		reading.NotMeasurable = "fewer than 2 sessions"
	} else {
		for _, s := range sessions[1:] {
			reading.TokensToFirstEditS2 += s.TokensToFirstEdit
		}
	}

	seen := map[string]bool{}
	for _, s := range sessions {
		for _, bc := range s.BoxChecks {
			if bc.RowID == "" {
				continue
			}
			if seen[bc.RowID] && proven[bc.RowID] {
				reading.DuplicatedEdits++
			}
			seen[bc.RowID] = true
		}
	}
	return reading
}

// ContractDrift is M4's second half: after the change, which of the plan's
// own claimed rows (the v2-grammar `symbol:`/`package:` rows planMD
// carries — see extractPlanSymbolRows) no longer hold? A row drifts when
// its symbol vanished from the final tree, or — when a coverprofile was
// captured — its span no longer has a covered block. Without a
// coverprofile only the "vanished" half is checked; the reading still
// reports what it could measure rather than refusing outright.
func ContractDrift(planMD []byte, tree, coverprofile string) DriftReading {
	rows := extractPlanSymbolRows(planMD)
	reading := DriftReading{Rows: len(rows)}
	if len(rows) == 0 {
		reading.NotMeasurable = "no symbol rows found in plan"
		return reading
	}

	var blocks []coverBlock
	haveCoverage := coverprofile != ""
	if haveCoverage {
		b, err := parseCoverProfile(coverprofile)
		if err != nil {
			haveCoverage = false
		} else {
			blocks = b
		}
	}
	if !haveCoverage {
		reading.NotMeasurable = "no coverprofile: only vanished symbols are checked, not test-reachability"
	}

	var drifted []string
	for _, r := range rows {
		label := r.ID
		if label == "" {
			label = r.Symbol
		}
		loc, found, err := findFuncInTree(tree, r.Package, r.Symbol)
		if err != nil || !found {
			drifted = append(drifted, label+": symbol vanished")
			continue
		}
		if haveCoverage && !coveredInSpan(blocks, loc.File, loc.Start, loc.End) {
			drifted = append(drifted, label+": no longer test-reached")
		}
	}
	sort.Strings(drifted)
	reading.Drifted = drifted
	return reading
}
