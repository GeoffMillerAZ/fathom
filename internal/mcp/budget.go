package mcp

import "encoding/json"

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
