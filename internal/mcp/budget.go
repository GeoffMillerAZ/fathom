package mcp

import (
	"encoding/json"
	"fmt"
)

// defaultCharBudget bounds a list-returning tool's payload so a single result
// can never blow the caller's token budget. Real sessions hit this: a
// query_symbols result of ~57KB was truncated by the client and saved to a file,
// which erodes trust in the tool. ~16KB of JSON is comfortably within budget yet
// holds dozens of symbol/occurrence records.
const defaultCharBudget = 16000

// fitToBudget returns the longest prefix of items whose JSON encodings sum to at
// most maxChars, plus whether any items were dropped. A non-positive maxChars
// uses defaultCharBudget. items must be JSON-marshalable; an item that fails to
// marshal is skipped for sizing (it will still be returned if within the prefix).
func fitToBudget[T any](items []T, maxChars int) ([]T, bool) {
	if maxChars <= 0 {
		maxChars = defaultCharBudget
	}
	total := 0
	for i, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			continue
		}
		total += len(b) + 1 // +1 approximates the separator between items
		if total > maxChars && i > 0 {
			return items[:i], true
		}
	}
	return items, false
}

// applyBudget is the one shared budget-application seam every truncating
// tool calls through (BA-3, BA-4): it returns the longest deterministic
// prefix of items that fits maxChars, plus the elided block describing what
// was cut (nil when nothing was).
//
// Nested/prefix property (BA-3): fitToBudget walks items in the caller's
// own order accumulating size, so a larger budget can only stop at or past
// where a smaller one did — the same query at two budgets always returns
// one prefix nested inside the other, for free, as long as the caller's
// items are themselves deterministically ordered before this call.
//
// Never refuses (BA-4): fitToBudget always keeps at least the first item,
// even when it alone exceeds maxChars — a budget is a target, never a hard
// cap (design §2 rule 1), so the smallest useful response is always
// returned, with the elided block carrying the "budget exceeded" note via
// HowToGetMore rather than an empty result.
func applyBudget[T any](items []T, maxChars int, ordering, howToGetMore string) ([]T, *elided) {
	kept, truncated := fitToBudget(items, maxChars)
	if !truncated {
		return kept, nil
	}
	return kept, elisionFor(len(kept), len(items), ordering, howToGetMore)
}

// fitToolBudget generalizes fitToBudget/applyBudget's row-cutting discipline
// (RC-19) to the tools that had NO budget line at all before this row:
// find_callers, outline, symbol_context, and staleness_report — each
// measured well past defaultCharBudget on a real corpus (find_callers("run")
// 28,206 B, outline(internal/core/model.go) 16,778 B, staleness_report
// 46,567 B). A non-positive maxChars uses defaultCharBudget. Returns the
// kept prefix and how many items were cut — 0 when nothing was — so the
// caller can pair the count with elisionLine to render the one-line
// disclosure. Deterministic (BA-3's nested-prefix property): the cut point
// depends only on items' own order and maxChars, so a stable input ordering
// (already required of every tool here) elides the same tail every time.
func fitToolBudget[T any](items []T, maxChars int) ([]T, int) {
	kept, truncated := fitToBudget(items, maxChars)
	if !truncated {
		return kept, 0
	}
	return kept, len(items) - len(kept)
}

// elisionLine renders fitToolBudget's cut count as the one-line disclosure
// RC-19 asks for: "elided: N more — refine with <refineWith>" — deliberately
// plainer than BA-1's structured `elided` object (elisionFor), since these
// four tools have no existing consumer parsing that shape. refineWith names
// the tool's OWN scoping angle (e.g. "package or file" for find_callers) so
// the hint is never generic filler; it need not be a literal parameter name
// when the tool has none — narrowing the query itself is a valid refinement
// too. Returns "" when elided <= 0 — never attach an empty line.
func elisionLine(elided int, refineWith string) string {
	if elided <= 0 {
		return ""
	}
	return fmt.Sprintf("elided: %d more — refine with %s", elided, refineWith)
}
