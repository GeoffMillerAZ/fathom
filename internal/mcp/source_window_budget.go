package mcp

import (
	"encoding/json"
	"fmt"
)

// fitSourceWindowReport owns the task prefix used by both the pending receipt
// and its saved final report. The summary remains the full-plan verdict; an
// elision receipt names every task omitted from this response. Other fields
// are preserved, so the measured budget includes their actual JSON encoding.
func fitSourceWindowReport(out map[string]any, budget int) (map[string]any, []byte, error) {
	tasks, ok := out["tasks"].([]any)
	if !ok {
		return nil, nil, fmt.Errorf("current source window: final report could not fit task envelope")
	}
	var prior *elided
	if v, exists := out["elided"]; exists && v != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, nil, err
		}
		var e elided
		if err := json.Unmarshal(b, &e); err != nil || e.Count < 0 || e.Of < len(tasks) || e.Count != e.Of-len(tasks) {
			return nil, nil, fmt.Errorf("current source window: invalid task elision receipt")
		}
		prior = &e
	}
	n, err := jsonSize(out, budget)
	if err != nil {
		return nil, nil, err
	}
	if n <= budget {
		raw, err := json.Marshal(out)
		if err != nil {
			return nil, nil, err
		}
		if len(raw) <= budget {
			return out, raw, nil
		}
	}
	if len(tasks) == 0 {
		return nil, nil, fmt.Errorf("current source window: final report exceeds response budget")
	}
	candidate := make(map[string]any, len(out))
	for key, value := range out {
		candidate[key] = value
	}
	setPrefix := func(n int) {
		candidate["tasks"] = tasks[:n]
		if prior != nil {
			e := *prior
			e.Count += len(tasks) - n
			candidate["elided"] = &e
		} else {
			candidate["elided"] = elisionFor(n, len(tasks), "plan order", "query omitted task IDs with verify_plan")
		}
	}
	best := 0
	for low, high := 1, len(tasks)-1; low <= high; {
		mid := low + (high-low)/2
		setPrefix(mid)
		n, err := jsonSize(candidate, budget)
		if err != nil {
			return nil, nil, err
		}
		if n <= budget {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best == 0 {
		return nil, nil, fmt.Errorf("current source window: final report exceeds response budget")
	}
	setPrefix(best)
	raw, err := json.Marshal(candidate)
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > budget {
		return nil, nil, fmt.Errorf("current source window: final report exceeds response budget")
	}
	return candidate, raw, nil
}
