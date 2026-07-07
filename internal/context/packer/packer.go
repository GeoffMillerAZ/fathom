// Package packer implements a token-budget evidence packer and a budget-bounded
// ranked symbol map for context assembly.
package packer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Evidence is a scored piece of textual evidence to be packed into a context window.
type Evidence struct {
	ID    string
	Text  string
	Score float64
}

// Packed is the result of a Pack call.
type Packed struct {
	Included []Evidence
	Tokens   int
	Dropped  int
}

// tokenEstimate returns a rough token count for s (~1 token per 4 bytes).
func tokenEstimate(s string) int {
	return (len(s) + 3) / 4
}

// Pack greedily selects items from items by Score descending (ties broken by ID
// ascending) until adding the next item would exceed budget tokens.  Items with
// zero or negative Score are still eligible — they are simply ranked lower.
// The result is deterministic for any fixed input.
func Pack(items []Evidence, budget int) Packed {
	if len(items) == 0 || budget <= 0 {
		return Packed{Dropped: len(items)}
	}

	// Copy so we don't mutate the caller's slice.
	sorted := make([]Evidence, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].ID < sorted[j].ID
	})

	var included []Evidence
	total := 0
	for _, ev := range sorted {
		t := tokenEstimate(ev.Text)
		if total+t > budget {
			continue
		}
		included = append(included, ev)
		total += t
	}

	dropped := len(items) - len(included)
	return Packed{
		Included: included,
		Tokens:   total,
		Dropped:  dropped,
	}
}

// RepoMap builds a budget-bounded symbol map string ranked by outbound call-edge
// count (most-connected symbols first, ties broken by Package+Name ascending).
// Each symbol consumes one line; lines are dropped when the cumulative token
// budget is exceeded.
func RepoMap(symbols []core.SymbolNode, budget int) string {
	if len(symbols) == 0 || budget <= 0 {
		return ""
	}

	// Copy and sort: most edges first, then by Package+Name for determinism.
	sorted := make([]core.SymbolNode, len(symbols))
	copy(sorted, symbols)
	sort.SliceStable(sorted, func(i, j int) bool {
		ci := len(sorted[i].Calls)
		cj := len(sorted[j].Calls)
		if ci != cj {
			return ci > cj
		}
		ki := sorted[i].Package + "." + sorted[i].Name
		kj := sorted[j].Package + "." + sorted[j].Name
		return ki < kj
	})

	var sb strings.Builder
	used := 0
	for _, sym := range sorted {
		line := formatSymbolLine(sym)
		t := tokenEstimate(line + "\n")
		if used+t > budget {
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
		used += t
	}
	return sb.String()
}

// formatSymbolLine renders a single SymbolNode as a compact map line.
func formatSymbolLine(s core.SymbolNode) string {
	sig := s.Signature
	if sig == "" {
		sig = string(s.Kind)
	}
	return fmt.Sprintf("%s.%s [edges:%d] %s", s.Package, s.Name, len(s.Calls), sig)
}
