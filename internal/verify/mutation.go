package verify

// mutation.go — VB-34: `test_reaches` says a Test* statically reaches a
// symbol; it says nothing about whether that test would notice the symbol
// breaking. OverlayMutation re-evaluates every already-computed
// test_reaches Pass against a mutation-testing report: a symbol whose
// mutants ALL survived (score 0) is COVERED but not ASSERTED — real
// evidence of a hollow test, not a static gap. Advisory by default (the
// note is appended to the criterion's detail, status untouched); strict
// downgrades the task to Fail, the same advisory/strict split every other
// criterion in this package uses.
//
// verify does not import internal/analysis (package doc, verify.go) — the
// CLI translates internal/analysis/coverage.SymbolMutation into the
// MutationScore below before calling OverlayMutation, the same boundary
// DetailedEvidence keeps for callgraph.ReachDetail.

import (
	"fmt"
	"strings"
)

// MutationScore is one symbol's joined mutation-testing result — see
// internal/analysis/coverage.SymbolMutation, which this mirrors.
type MutationScore struct {
	File     string
	Line     int
	Mutants  int
	Killed   int
	Survived int
	Score    float64 // Killed/Mutants, 0.0-1.0
}

// mutationLocKey is the (file, line) join key OverlayMutation matches a
// task's already-resolved symbol location against — the location is
// parsed back out of the task's own symbol_present Pass detail (see
// symbolPresentLoc) rather than re-resolving the symbol, so OverlayMutation
// needs no Evidence access at all: it is a pure function of an already
// computed Report, mirroring OverlayLocal's own re-evaluate-in-place shape.
func mutationLocKey(file string, line int) string {
	return fmt.Sprintf("%s:%d", file, line)
}

// symbolPresentLoc extracts the (file, line) a task's Pass'd symbol_present
// criterion recorded — verifyTask's own pass("symbol_present", "%s at
// %s:%d", ...) format — or ok=false when the task has no such criterion
// (an unclaimed/no-contract row, or one whose symbol_present did not pass).
func symbolPresentLoc(criteria []Criterion) (file string, line int, ok bool) {
	for _, c := range criteria {
		if c.Name != "symbol_present" || c.Status != Pass {
			continue
		}
		idx := strings.LastIndexByte(c.Detail, ':')
		if idx < 0 {
			return "", 0, false
		}
		atIdx := strings.Index(c.Detail, " at ")
		if atIdx < 0 || atIdx+4 >= idx {
			return "", 0, false
		}
		file = c.Detail[atIdx+4 : idx]
		var n int
		if _, err := fmt.Sscanf(c.Detail[idx+1:], "%d", &n); err != nil {
			return "", 0, false
		}
		return file, n, true
	}
	return "", 0, false
}

// OverlayMutation re-evaluates every task's Pass'd test_reaches criterion
// in rep against scores (keyed internally by file:line — build it from
// internal/analysis/coverage.JoinMutationSymbols' output). A symbol with no
// entry in scores is untouched: a mutation report speaks only for the
// symbols it ran mutants against, never a fabricated verdict for the rest.
// strict downgrades a score-0 mutation finding from an advisory note to a
// gate-blocking Fail (label "covered-not-asserted"); recomputes Summary,
// mirroring OverlayLocal.
func OverlayMutation(rep *Report, scores []MutationScore, strict bool) {
	if len(scores) == 0 {
		return
	}
	byLoc := make(map[string]MutationScore, len(scores))
	for _, s := range scores {
		byLoc[mutationLocKey(s.File, s.Line)] = s
	}

	for i := range rep.Tasks {
		tv := &rep.Tasks[i]
		file, line, ok := symbolPresentLoc(tv.Criteria)
		if !ok {
			continue
		}
		sc, found := byLoc[mutationLocKey(file, line)]
		if !found {
			continue
		}
		for j := range tv.Criteria {
			c := &tv.Criteria[j]
			if c.Name != "test_reaches" || c.Status != Pass {
				continue
			}
			if sc.Mutants == 0 {
				continue
			}
			if sc.Score <= 0 {
				c.Detail += fmt.Sprintf(" (mutation: %d/%d killed — covered-not-asserted)", sc.Killed, sc.Mutants)
				if strict {
					c.Status = Fail
					tv.Status = Fail
					tv.Label = "covered-not-asserted"
				}
			} else {
				c.Detail += fmt.Sprintf(" (mutation: %d/%d killed, score %.2f)", sc.Killed, sc.Mutants, sc.Score)
			}
		}
	}

	sum := Summary{Total: len(rep.Tasks)}
	for _, v := range rep.Tasks {
		switch v.Status {
		case Pass:
			sum.Pass++
		case Fail:
			sum.Fail++
		default:
			sum.Unknown++
		}
	}
	rep.Summary = sum
}
