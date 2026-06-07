// Package cost provides token estimation and per-tool cost accumulation.
package cost

import (
	"math"
	"sort"
	"sync"
)

// Estimate returns a rough token count for text using the heuristic of
// one token per four characters, rounding up.
func Estimate(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// PriceUSD converts a token count to USD given a per-thousand-token rate.
func PriceUSD(tokens int, ratePerKTok float64) float64 {
	return math.Round(float64(tokens)/1000.0*ratePerKTok*1e9) / 1e9
}

// ToolCost is the accumulated cost record for a single tool.
type ToolCost struct {
	Tool   string
	Tokens int
	USD    float64
}

// Tracker accumulates token counts and USD costs per tool name.
// The zero value is not usable; construct via NewTracker.
type Tracker struct {
	mu    sync.Mutex
	tools map[string]*ToolCost
}

// NewTracker returns an initialised Tracker.
func NewTracker() *Tracker {
	return &Tracker{tools: make(map[string]*ToolCost)}
}

// Record adds tokens and usd to the named tool's running totals.
func (t *Tracker) Record(tool string, tokens int, usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tc, ok := t.tools[tool]
	if !ok {
		tc = &ToolCost{Tool: tool}
		t.tools[tool] = tc
	}
	tc.Tokens += tokens
	tc.USD += usd
}

// Report returns a copy of all per-tool records sorted ascending by tool name.
func (t *Tracker) Report() []ToolCost {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]ToolCost, 0, len(t.tools))
	for _, tc := range t.tools {
		out = append(out, *tc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tool < out[j].Tool })
	return out
}

// Totals returns aggregate tokens and USD across all recorded tools.
func (t *Tracker) Totals() (tokens int, usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, tc := range t.tools {
		tokens += tc.Tokens
		usd += tc.USD
	}
	return
}
