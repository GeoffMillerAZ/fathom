package mcp

// budgeted_handler.go — SU-3 (docs/plans/substrate.md §A, ADR-041 decision
// 3): a dispatch-level backstop so a tool handler that forgets to budget
// its own result (O90's defect class: a ~63k-token symbol_context answer
// shipped before that tool had any budget line at all) cannot regress past
// defaultCharBudget either. Every handler passed to registerTool is wrapped
// with budgetedHandler, so this applies uniformly — no per-tool opt-in.
//
// A handler that already keeps its result under budget sees NO difference:
// budgetedResult only touches the marshaled bytes when they exceed
// byteBudget, so an already-disciplined tool's response is byte-identical
// to what it always was (TestBudgetedHandler_UnderBudgetIsByteIdentical).

import (
	"context"
	"encoding/json"
	"sort"
)

// budgetedHandler wraps h so its result passes through the SAME byte
// budget + elision seam fitToolBudget/applyBudget already give disciplined
// handlers (applyBudget in budget.go), applied generically over the
// handler's marshaled JSON result rather than a typed slice.
func budgetedHandler(h func(ctx context.Context, args json.RawMessage) (any, *rpcError)) func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
	return func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		result, rpcErr := h(ctx, args)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return budgetedResult(result, defaultCharBudget), nil
	}
}

// budgetedResult returns result unchanged when its own JSON encoding fits
// byteBudget (the common case: nearly every handler already budgets
// itself, so this is a pure passthrough for them). When it does not, the
// largest top-level JSON array in the result — the result itself if it IS
// an array, else the largest array-valued field of a top-level object — is
// trimmed with fitToBudget's own deterministic nested-prefix discipline,
// and the standard elided block (elisionFor) is attached so the cut is
// never silent. A result with no array shape to trim (a giant scalar or
// deeply nested object with no top-level array) falls back to a labelled
// truncated preview — rare, since every existing over-budget tool this row
// was written against (find_callers, outline, symbol_context,
// staleness_report, query_symbols) returns a list or an object wrapping
// one.
func budgetedResult(result any, byteBudget int) any {
	if byteBudget <= 0 {
		byteBudget = defaultCharBudget
	}
	b, err := json.Marshal(result)
	if err != nil || len(b) <= byteBudget {
		return result
	}

	// Case 1: the result itself is a top-level JSON array.
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err == nil {
		kept, el := applyBudget(arr, byteBudget, "as returned", "narrow the query")
		out := map[string]any{"items": kept}
		if el != nil {
			out["elided"] = el
		}
		return out
	}

	// Case 2: a top-level object — trim its largest array-valued field.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err == nil {
		if field, items, ok := largestArrayField(obj); ok {
			kept, el := applyBudget(items, byteBudget, "as returned", "narrow the query")
			keptRaw, mErr := json.Marshal(kept)
			if mErr == nil {
				obj[field] = keptRaw
				if el != nil {
					if elRaw, eErr := json.Marshal(el); eErr == nil {
						obj["elided"] = elRaw
					}
				}
				return obj
			}
		}
	}

	// Fallback: no array field to trim at all — a truncated, labelled
	// preview rather than silently shipping the oversized payload.
	cut := byteBudget
	if cut > len(b) {
		cut = len(b)
	}
	return map[string]any{
		"truncated": true,
		"note":      "response exceeded the dispatch byte budget and had no array field to trim",
		"preview":   string(b[:cut]),
	}
}

// largestArrayField finds the top-level field of obj whose value is a JSON
// array with the most bytes, returning its name and decoded elements.
// Deterministic: obj's keys are walked in sorted order so a byte-size tie
// always resolves to the same field (CLAUDE.md's "map iteration order must
// never reach a result").
func largestArrayField(obj map[string]json.RawMessage) (field string, items []json.RawMessage, ok bool) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bestSize := -1
	for _, k := range keys {
		var candidate []json.RawMessage
		if err := json.Unmarshal(obj[k], &candidate); err != nil {
			continue
		}
		if len(obj[k]) > bestSize {
			bestSize = len(obj[k])
			field = k
			items = candidate
			ok = true
		}
	}
	return field, items, ok
}
