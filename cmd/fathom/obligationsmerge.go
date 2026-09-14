package main

// obligationsmerge.go — merging obligations across MULTIPLE resolved
// files into one capped, deduped payload. Shared by prompt-hook (O57
// item 4/5a: the paths/packages/symbols a PROMPT names) and edit-hook's
// Bash-write path (O57 item 4/5b: the path(s) a shell command WROTE) —
// the two hooks that must answer for more than one candidate file in a
// single call, where a plain single-file fetchObligations answer is not
// enough.
//
// The per-file calls run concurrently, bounded, against ONE shared
// deadline — not a fresh per-call timeout each — so a prompt naming five
// files cannot spend five sequential multiples of a single call's budget
// against its hook's own wall-clock limit (review finding on O57 item
// 4/5a: up to nine sequential 3s calls against a 10s hook budget). When
// the deadline arrives before every file got a definite answer, the merge
// still returns whatever DID resolve — resolved/total on the result let
// the caller print a "partial: N of M resolved" note instead of silently
// under-reporting or blocking past its own budget.

import (
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// mergeObligationsConcurrency bounds how many per-file obligations calls
// run at once — generous enough that the fan-out is the point, small
// enough that one call never means the whole daemon connection pool.
const mergeObligationsConcurrency = 4

// mergedObligations is the union of every resolved file's obligations,
// already capped and ordered for rendering.
type mergedObligations struct {
	rules      []semantic.Obligation
	displaced  int
	stale      bool
	namedFiles []string // files that actually carried >=1 rule, resolution order
	resolved   int      // files whose fetch got a definite answer before the deadline
	total      int      // files the caller asked to resolve (len(files))
}

// mergeObligationsAcrossFiles fetches obligations for every file in files
// (deduplicated by the caller) and merges them: same-rule/same-source
// duplicates collapse to one, the union is ordered fresh-before-flagged
// then by rule text (so which rules survive the cap does not depend on
// which candidate happened to resolve first, or which finished first —
// merge order follows files' own order, never goroutine completion
// order), and displaced accumulates both each call's own cap pressure and
// whatever the FINAL merge cap cuts.
//
// deadline is the one wall-clock point every file's fetch shares: a file
// whose turn comes after it has already passed is never even attempted.
// onError, when non-nil, is called with each file whose fetch was
// attempted but itself failed (daemon down, bad response) — one failed
// candidate never silences the others, and is distinct from one skipped
// outright for time.
func mergeObligationsAcrossFiles(url, service string, files []string, deadline time.Time, onError func(file string)) mergedObligations {
	type fileResult struct {
		obl        semantic.Obligations
		indexStale bool
		ok         bool
		attempted  bool
	}
	results := make([]fileResult, len(files))
	sem := make(chan struct{}, mergeObligationsConcurrency)
	var wg sync.WaitGroup
	for i, f := range files {
		if !time.Now().Before(deadline) {
			break // budget already spent: nothing further is even attempted
		}
		wg.Add(1)
		go func(i int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return // deadline passed while queued behind the concurrency cap
			}
			obl, indexStale, ok := fetchObligations(url, service, f, 0, remaining)
			results[i] = fileResult{obl: obl, indexStale: indexStale, ok: ok, attempted: true}
		}(i, f)
	}
	wg.Wait()

	type ruleKey struct{ rule, src string }
	seen := map[ruleKey]bool{}
	var m mergedObligations
	m.total = len(files)
	for i, r := range results {
		if !r.attempted {
			continue
		}
		if !r.ok {
			if onError != nil {
				onError(files[i])
			}
			continue
		}
		m.resolved++
		if len(r.obl.Rules) > 0 {
			m.namedFiles = append(m.namedFiles, files[i])
		}
		m.stale = m.stale || r.indexStale
		m.displaced += r.obl.Displaced
		for _, rule := range r.obl.Rules {
			k := ruleKey{rule.Rule, rule.Src}
			if seen[k] {
				continue
			}
			seen[k] = true
			m.rules = append(m.rules, rule)
		}
	}
	sort.SliceStable(m.rules, func(i, j int) bool {
		fi, fj := m.rules[i].Status == "", m.rules[j].Status == ""
		if fi != fj {
			return fi // fresh (empty status) before flagged
		}
		return m.rules[i].Rule < m.rules[j].Rule
	})
	if len(m.rules) > semantic.MaxObligations {
		m.displaced += len(m.rules) - semantic.MaxObligations
		m.rules = m.rules[:semantic.MaxObligations]
	}
	return m
}
