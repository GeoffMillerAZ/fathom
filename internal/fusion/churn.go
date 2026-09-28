package fusion

// churn.go — TQ-1 (docs/plans/test-intelligence.md; design doc
// docs/design/test-intelligence.md §3 T1): churn per file, computed once
// at ingest from the service's own git commit history (already collected
// through gitro/realGitLog and stored via SetCommits, see pipeline.go's
// recordIngestMeta) and stored via store.SetFileChurn — never recomputed
// at request time (I48). This is a git fact over already-stored `commits`
// rows, not a re-derived code-churn heuristic: the retired churn and
// cognitive analyzers (docs/plans/retire-unused-analyzers.md) stay
// retired (ADR-051 decision 1, TQ-X1).

import (
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// churnWindow is T1's lookback: commits touching a file in the 180 days
// before the ingest's own clock count toward its churn.
const churnWindow = 180 * 24 * time.Hour

// computeFileChurn returns, for every file touched by at least one commit
// within churnWindow before asOf, the count of DISTINCT such commits that
// touched it — a file touched twice in the same commit counts once for
// that commit, matching "commits per file", not "file mentions". A commit
// with no Date (unparsed, Date == 0) is treated as outside the window: an
// unparseable date must never be silently counted as maximally fresh.
//
// asOf is the caller's own clock (internal/CLAUDE.md determinism: inject
// clocks, never call time.Now() in analysis logic) — recordIngestMeta
// passes the same "now" it stamps the ingest's ServiceMeta with.
// Sorted by path (Store contract: deterministic output order).
func computeFileChurn(commits []store.Commit, asOf time.Time) []store.FileChurn {
	if len(commits) == 0 {
		return nil
	}
	cutoffNano := asOf.Add(-churnWindow).UnixNano()
	counts := make(map[string]int)
	for _, c := range commits {
		if c.Date <= 0 || c.Date < cutoffNano {
			continue
		}
		seen := make(map[string]bool, len(c.Files))
		for _, f := range c.Files {
			if f == "" || seen[f] {
				continue
			}
			seen[f] = true
			counts[f]++
		}
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]store.FileChurn, 0, len(counts))
	for path, n := range counts {
		out = append(out, store.FileChurn{Path: path, Commits180d: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
