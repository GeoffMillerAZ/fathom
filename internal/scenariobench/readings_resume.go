package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
)

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

	// RC-13 (vision 28 §2.4): tokens_to_first_edit for EVERY session,
	// session 1 included — unconditionally, with no NotMeasurable gate,
	// since even one session has a number. TokensToFirstEditS2 below stays
	// session>=2 only BY DESIGN (a session 1 pays discovery once by
	// construction — the comment above this function); a consumer like
	// FactsReused needs the whole population, not just the cold hand-offs.
	for _, s := range sessions {
		reading.TokensToFirstEditAll += s.TokensToFirstEdit
	}

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

// FactsReused is RC-13 (vision 28 §2.4, F11: "nothing carries a planner's
// discoveries to a worker except a plan row's ctx: pointers pasted by
// hand"): among briefPointers — the hand-off pointers (symbol ids, paths)
// a planning session's brief, or a prior session's own end-of-session
// note, named — how many did session's OWN transcript reference, as a
// tool_use input or inside a tool_result it read, before its first
// Edit/Write? "Reused", not merely present: a pointer that surfaces only
// AFTER the first edit was not part of orienting for it, so scanning stops
// there.
//
// 0 (never nil) when briefPointers is empty or session has no raw
// transcript to read: nothing to reuse and nothing found are the same
// answer here, unlike the tri-state readings above — a caller distinguishing
// "not measurable" from "reused nothing" does so via Readings.FactsReused's
// own nil-ness, set only when this function is actually invoked.
func FactsReused(session SessionResult, briefPointers []string) int {
	if len(briefPointers) == 0 || session.RawPath == "" {
		return 0
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return 0
	}

	found := make(map[string]bool, len(briefPointers))
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamToolEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "assistant" && ev.Type != "user" {
			continue
		}
		stopAfterThisLine := false
		for _, c := range ev.Message.Content {
			switch c.Type {
			case "tool_use":
				if editTargetFile(c.Name, c.Input) != "" {
					stopAfterThisLine = true
					continue
				}
				for _, p := range briefPointers {
					if p != "" && bytes.Contains(c.Input, []byte(p)) {
						found[p] = true
					}
				}
			case "tool_result":
				text := rawResultText(c.Content)
				for _, p := range briefPointers {
					if p != "" && strings.Contains(text, p) {
						found[p] = true
					}
				}
			}
		}
		if stopAfterThisLine {
			break
		}
	}
	return len(found)
}
