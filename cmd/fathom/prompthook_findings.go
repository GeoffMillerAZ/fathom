package main

// prompthook_findings.go — FL-21 (docs/vision/30-findings-loop.md §3.4):
// the prompt hook's one-line findings count, "open findings: N across the
// files this prompt names" — computed from the SAME resolved candidates
// the rules block already merged, inside the SAME shared deadline
// (promptHookDeadlineBudget), never a fresh RPC budget of its own.

import (
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// countFindingsAcrossFiles fetches obligations (layer=all) for every file
// in files, bounded by the SAME concurrency cap and shared deadline
// mergeObligationsAcrossFiles uses, and counts the distinct (rule_id, src)
// finding-kind rows seen across all of them. This is a bare COUNT, never a
// rendered list, so it is not subject to semantic.MaxObligations' 6-rule
// cap (mergeObligationsAcrossFiles applies that cap for its own rendering
// purpose; a count line must stay honest past it).
//
// ok=false only when the deadline was already spent before a single file
// could even be attempted — the caller skips the line silently rather than
// rendering a stale or partial zero (the plan's own "skip silently ... if
// the deadline is spent").
func countFindingsAcrossFiles(url, service string, files []string, deadline time.Time) (int, bool) {
	if len(files) == 0 || !time.Now().Before(deadline) {
		return 0, false
	}
	type fileResult struct {
		rows      []semantic.Obligation
		ok        bool
		attempted bool
	}
	results := make([]fileResult, len(files))
	sem := make(chan struct{}, mergeObligationsConcurrency)
	var wg sync.WaitGroup
	for i, f := range files {
		if !time.Now().Before(deadline) {
			break
		}
		wg.Add(1)
		go func(i int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return
			}
			obl, _, ok, _ := fetchObligations(url, service, f, 0, remaining, semantic.ObligationLayerAll)
			results[i] = fileResult{rows: obl.Rules, ok: ok, attempted: true}
		}(i, f)
	}
	wg.Wait()

	attempted := false
	seen := map[string]bool{}
	for _, r := range results {
		if !r.attempted {
			continue
		}
		attempted = true
		if !r.ok {
			continue
		}
		for _, row := range r.rows {
			if row.Kind != "finding" {
				continue
			}
			key := row.RuleID + "\x00" + row.Src
			if seen[key] {
				continue
			}
			seen[key] = true
		}
	}
	if !attempted {
		return 0, false
	}
	return len(seen), true
}
