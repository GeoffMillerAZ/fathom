// Package mcp — the scope-required escape hatch (RC-20): for a fan-in so
// large that even a truncated page of rows (fitToolBudget/RC-19) would
// never let an agent page through it — the classic "find_callers String" or
// "references New" against a big fleet — the tool returns the shape of the
// answer instead of a page of it: the true count and the packages driving
// it, so the agent can scope its NEXT call instead of scrolling.
package mcp

import "sort"

// scopeRequiredRowChars is the conservative measured size (in the same char
// units as fitToBudget/defaultCharBudget) of one compact caller/reference
// row on the wire — SB-47/SB-1's {loc, c} shape plus JSON punctuation,
// rounded up. It is an ESTIMATE (mirrors BA-8's convention for any
// bytes-from-count projection), used only to decide, BEFORE rendering any
// rows, whether `count` of them would overrun budget.
const scopeRequiredRowChars = 120

// scopeRequiredBudget is the threshold find_callers and references pass to
// scopeRequired — deliberately much larger than one page's defaultCharBudget
// (16000). fitToolBudget/applyBudget already page a moderately busy symbol
// (hundreds of callers) via truncation + elision; scopeRequired exists for
// the fan-in paging can never resolve — a generic name's matches across a
// whole fleet — so it is reserved for counts roughly 8 pages deep or more.
const scopeRequiredBudget = defaultCharBudget * 8

// packageCount is one package's contribution to a fan-in, as scopeRequired
// reports it: {package, count}.
type packageCount struct {
	Package string `json:"package"`
	Count   int    `json:"count"`
}

// scopeSummary is what scopeRequired returns instead of rows: the true
// fan-in count and the top packages driving it. Tool-specific guidance
// (which argument narrows it) is composed by the caller, not carried here —
// find_callers and references have different scoping vocabularies.
type scopeSummary struct {
	Count       int            `json:"count"`
	TopPackages []packageCount `json:"top_packages"`
}

// scopeRequired reports whether a callers/references fan-in of `count` rows
// would exceed budget so badly that returning rows at all (even a
// fitToBudget-truncated page of them) is not useful — only when it decides
// yes does the caller skip rendering and return the summary instead. A
// non-positive budget uses defaultCharBudget. byPackage tallies `count`
// across each row's owning package; the summary keeps at most 8, sorted by
// count descending then package name ascending (deterministic for ties —
// RC-20's "golden test for a generic name" fixes this order).
func scopeRequired(count, budget int, byPackage map[string]int) (bool, scopeSummary) {
	if budget <= 0 {
		budget = defaultCharBudget
	}
	if count*scopeRequiredRowChars <= budget {
		return false, scopeSummary{}
	}
	return true, scopeSummary{Count: count, TopPackages: topPackages(byPackage, 8)}
}

// topPackages sorts byPackage's tally descending by count, ties broken by
// package name ascending, and caps it at limit.
func topPackages(byPackage map[string]int, limit int) []packageCount {
	rows := make([]packageCount, 0, len(byPackage))
	for pkg, n := range byPackage {
		rows = append(rows, packageCount{Package: pkg, Count: n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Package < rows[j].Package
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}
