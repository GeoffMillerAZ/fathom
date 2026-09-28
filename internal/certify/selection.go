package certify

// selection.go — SelectCertifyingFirst (TQ-15, docs/design/test-intelligence.md
// §3 T4): certifying-first test selection for a diff. Pure over its own
// input shape (CertifyingTest), like RiskWeightedCoverage's own factor
// inputs (riskinputs.go) — the store/claims/callgraph gathering a real
// caller needs (portfolio.go's own certs/reach construction is the
// existing precedent for building it) stays the caller's job, so the
// selection algorithm itself is independently testable and never touches
// a store.
//
// I43 applies here exactly as it does to every other certifications-layer
// computation: SelectCertifyingFirst never marks a certificate live — it
// only proposes what to run; only an attested passing run does that
// (TQ-X2, refused explicitly in the plan's own Refused section).

import "sort"

// CertifyingTest is one Test* symbol's selection-relevant facts: which
// LIVE certificate targets it certifies (empty for a test that certifies
// nothing) and which symbols it statically reaches.
type CertifyingTest struct {
	Test    string
	Targets []string
	Reach   []string
}

// Selection is SelectCertifyingFirst's answer.
type Selection struct {
	// Tests is the selected test keys: the greedy set cover over
	// TouchedTargets, plus (when safe) every candidate whose Reach
	// touched changed — sorted, deterministic.
	Tests []string
	// Reproves maps a selected test to the touched targets it re-proves
	// (nil for a safe-only addition that certifies nothing).
	Reproves map[string][]string
	// TouchedTargets is every certificate target the diff touched: its
	// own resolved symbols intersect changed, or a test that certifies it
	// has a Reach intersecting changed (design doc §3 T4). Sorted.
	TouchedTargets []string
	// Uncovered names a touched target no candidate certifies at all —
	// the selection cannot re-prove it; named, never silently dropped.
	Uncovered []string
	// Skipped names every certifying candidate (Targets non-empty) NOT
	// selected: its targets were already covered by another selected
	// test. A safe-only addition is never "skipped" — it was never a
	// cover candidate to begin with.
	Skipped []string
}

func stringSetIntersects(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if set[s] {
			return true
		}
	}
	return false
}

// SelectCertifyingFirst computes the touched targets and the minimal
// test set that re-proves them: a greedy set cover over TouchedTargets —
// each round, the not-yet-selected certifying candidate covering the
// most REMAINING uncovered touched targets is picked, ties broken by
// test name (Test field, ascending) for a result that never depends on
// candidates' input order. When safe is true, every candidate (certifying
// or not) whose Reach intersects changed is unioned into Tests in
// addition to the cover, matching design doc §3 T4's "`--safe` adds every
// reaching test".
func SelectCertifyingFirst(changed []string, candidates []CertifyingTest, targetSymbols map[string][]string, safe bool) Selection {
	sel := Selection{Reproves: map[string][]string{}}
	if len(changed) == 0 {
		return sel
	}

	// certifiers: target -> the candidates that certify it (declaration
	// order irrelevant; only used to detect "no certifier at all").
	certifiers := map[string][]int{}
	for i, c := range candidates {
		for _, tgt := range c.Targets {
			certifiers[tgt] = append(certifiers[tgt], i)
		}
	}

	touched := map[string]bool{}
	for tgt, syms := range targetSymbols {
		if stringSetIntersects(syms, changed) {
			touched[tgt] = true
		}
	}
	for _, c := range candidates {
		if len(c.Targets) == 0 || !stringSetIntersects(c.Reach, changed) {
			continue
		}
		for _, tgt := range c.Targets {
			touched[tgt] = true
		}
	}
	for tgt := range touched {
		sel.TouchedTargets = append(sel.TouchedTargets, tgt)
	}
	sort.Strings(sel.TouchedTargets)

	for _, tgt := range sel.TouchedTargets {
		if len(certifiers[tgt]) == 0 {
			sel.Uncovered = append(sel.Uncovered, tgt)
		}
	}

	// Greedy set cover over sel.TouchedTargets using only candidates that
	// certify at least one target (a non-certifying candidate can never
	// help cover anything).
	remaining := map[string]bool{}
	for _, tgt := range sel.TouchedTargets {
		if len(certifiers[tgt]) > 0 {
			remaining[tgt] = true
		}
	}
	selected := make(map[int]bool, len(candidates))
	for len(remaining) > 0 {
		bestIdx, bestCount := -1, 0
		for i, c := range candidates {
			if selected[i] || len(c.Targets) == 0 {
				continue
			}
			count := 0
			for _, tgt := range c.Targets {
				if remaining[tgt] {
					count++
				}
			}
			if count == 0 {
				continue
			}
			if count > bestCount || (count == bestCount && (bestIdx == -1 || candidates[i].Test < candidates[bestIdx].Test)) {
				bestIdx, bestCount = i, count
			}
		}
		if bestIdx == -1 {
			break // nothing left covers a remaining target (already named in Uncovered)
		}
		selected[bestIdx] = true
		c := candidates[bestIdx]
		var reproved []string
		for _, tgt := range c.Targets {
			if remaining[tgt] {
				reproved = append(reproved, tgt)
				delete(remaining, tgt)
			}
		}
		sort.Strings(reproved)
		sel.Reproves[c.Test] = reproved
	}

	testSet := map[string]bool{}
	for i, c := range candidates {
		if selected[i] {
			testSet[c.Test] = true
		} else if len(c.Targets) > 0 {
			sel.Skipped = append(sel.Skipped, c.Test)
		}
	}
	if safe {
		for _, c := range candidates {
			if stringSetIntersects(c.Reach, changed) {
				testSet[c.Test] = true
			}
		}
		// A --safe addition runs regardless of why — never left listed as
		// "skipped" alongside the tests this selection actually invokes.
		kept := sel.Skipped[:0]
		for _, t := range sel.Skipped {
			if !testSet[t] {
				kept = append(kept, t)
			}
		}
		sel.Skipped = kept
	}
	for t := range testSet {
		sel.Tests = append(sel.Tests, t)
	}
	sort.Strings(sel.Tests)
	sort.Strings(sel.Skipped)
	return sel
}
