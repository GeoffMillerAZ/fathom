package main

// obligationsmerge.go — merging obligations across MULTIPLE resolved
// files into one capped, deduped payload. Shared by prompt-hook (O57
// item 4/5a: the paths/packages/symbols a PROMPT names) and edit-hook's
// Bash-write path (O57 item 4/5b: the path(s) a shell command WROTE) —
// the two hooks that must answer for more than one candidate file in a
// single call, where a plain single-file fetchObligations answer is not
// enough.

import (
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// mergedObligations is the union of every resolved file's obligations,
// already capped and ordered for rendering.
type mergedObligations struct {
	rules      []semantic.Obligation
	displaced  int
	stale      bool
	namedFiles []string // files that actually carried >=1 rule, resolution order
}

// mergeObligationsAcrossFiles fetches obligations for every file in files
// (deduplicated by the caller) and merges them: same-rule/same-source
// duplicates collapse to one, the union is ordered fresh-before-flagged
// then by rule text (so which rules survive the cap does not depend on
// which candidate happened to resolve first), and displaced accumulates
// both each call's own cap pressure and whatever the FINAL merge cap cuts.
// onError, when non-nil, is called with each file whose fetch itself
// failed (daemon down, bad response) — one failed candidate never
// silences the others.
func mergeObligationsAcrossFiles(url, service string, files []string, timeout time.Duration, onError func(file string)) mergedObligations {
	type ruleKey struct{ rule, src string }
	seen := map[ruleKey]bool{}
	var m mergedObligations
	for _, f := range files {
		obl, indexStale, ok := fetchObligations(url, service, f, 0, timeout)
		if !ok {
			if onError != nil {
				onError(f)
			}
			continue
		}
		if len(obl.Rules) > 0 {
			m.namedFiles = append(m.namedFiles, f)
		}
		m.stale = m.stale || indexStale
		m.displaced += obl.Displaced
		for _, r := range obl.Rules {
			k := ruleKey{r.Rule, r.Src}
			if seen[k] {
				continue
			}
			seen[k] = true
			m.rules = append(m.rules, r)
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
