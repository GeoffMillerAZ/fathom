package docindex

// budget.go — enforceDocBudget (KR-9, docs/design/knowledge-retrieval.md
// §8: "Index size... capped per service by the byte budget, with a named
// `elided` when a service exceeds it"). Pure and deterministic
// (internal/AGENTS.md): given a service's full candidate doc_units set and
// a byte budget, decides which units fit and which are elided — each named
// by path and heading path, never a silent drop (I38's "an unreadable
// input is never a healthy zero" carried over to "an over-budget corpus is
// never a healthy silent trim").
//
// The cut is a deterministic PREFIX boundary over a priority-ordered
// candidate list, mirroring internal/mcp/budget.go's own fitToolBudget
// convention exactly (BA-3's nested-prefix property, BA-4's "never
// refuses": a budget is a target, so the first candidate is always kept
// even alone over budget) — this package does not invent a second, looser
// budget discipline.
//
// Priority order (§3: "stale_by_location... ranks below live docs, never
// silently above them" — applied here too, not only to ranking): live
// docs before stale_by_location ones; within each group, by path then
// heading path, for a total order that never depends on map/slice
// iteration.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// DefaultDocBudgetBytes is the per-service FTS byte cap enforceDocBudget
// applies when the caller passes budgetBytes <= 0 (internal/mcp/budget.go's
// "non-positive means default" convention). It is a generous ceiling meant
// to catch runaway growth, not to bind typical use — §9's KR-M3 is the
// measurement that tunes it against a real corpus; until that reading
// exists, 64 MiB comfortably covers every service this repository's own
// corpus (the largest indexed today) produces many times over.
const DefaultDocBudgetBytes = 64 * 1024 * 1024

// ElidedDocUnit names one candidate enforceDocBudget cut for budget —
// identifying enough (path, heading path, byte count) that a caller can
// report exactly what is missing from the index, never just a count.
type ElidedDocUnit struct {
	Path        string
	HeadingPath string
	Bytes       int
}

// DocBudgetResult is enforceDocBudget's verdict: the candidates that fit
// (in priority order, ready to write) and the ones elided, each side's
// running byte total.
type DocBudgetResult struct {
	Kept        []store.DocUnit
	Elided      []ElidedDocUnit
	KeptBytes   int
	ElidedBytes int
}

// docUnitBudgetBytes is the accounting unit enforceDocBudget charges
// against the budget: heading_path + body, the same two (and only two)
// columns docs_fts indexes (internal/store/docindex.go's own column-order
// comment) — not the whole DocUnit struct's in-memory footprint.
//
// A unit carried without its Body (the ingest's planning stubs) states its
// size in SizeBytes, computed from the same two columns.
func docUnitBudgetBytes(u store.DocUnit) int {
	if u.SizeBytes > 0 {
		return u.SizeBytes
	}
	return len(u.HeadingPath) + len(u.Body)
}

// enforceDocBudget orders candidates live-docs-first, then by path and
// heading path, and keeps that prefix until the next candidate would push
// the running total past budgetBytes — at which point that candidate and
// every one after it (in priority order) are elided, by name. The single
// exception is the first candidate: it is always kept, even alone over
// budget (BA-4's "never refuses" — a caller with nothing indexed for a
// service should get its one biggest doc, not an empty index because the
// budget itself is too tight for even that).
func enforceDocBudget(candidates []store.DocUnit, budgetBytes int) DocBudgetResult {
	if budgetBytes <= 0 {
		budgetBytes = DefaultDocBudgetBytes
	}

	ordered := make([]store.DocUnit, len(candidates))
	copy(ordered, candidates)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.StaleByLocation != b.StaleByLocation {
			return !a.StaleByLocation // live (false) sorts before stale (true)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.HeadingPath < b.HeadingPath
	})

	var res DocBudgetResult
	for i, u := range ordered {
		n := docUnitBudgetBytes(u)
		if i > 0 && res.KeptBytes+n > budgetBytes {
			for _, rest := range ordered[i:] {
				m := docUnitBudgetBytes(rest)
				res.Elided = append(res.Elided, ElidedDocUnit{Path: rest.Path, HeadingPath: rest.HeadingPath, Bytes: m})
				res.ElidedBytes += m
			}
			return res
		}
		res.Kept = append(res.Kept, u)
		res.KeptBytes += n
	}
	return res
}
