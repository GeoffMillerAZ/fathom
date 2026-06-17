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
