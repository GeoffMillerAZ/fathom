// Package complexity provides analyzers over []core.SymbolNode (or
// []core.Observation) that compute cyclomatic-complexity metrics, hotspot
// ranking, and a Maintainability Index (MI) score.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package complexity

import (
	"math"
	"sort"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// FuncComplexity is the complexity report for a single func or method symbol.
type FuncComplexity struct {
	// Name is the symbol name (e.g. "ValidateToken" or "Handler.ServeHTTP").
	Name string
	// FilePath is the repo-relative source file path.
	FilePath string
	// Line is the 1-based declaration line number.
	Line int
	// Complexity is the cyclomatic complexity value stored on the node.
	Complexity int
}

// isFuncLike returns true when the node kind is a function or method — i.e. the
// node carries a meaningful Complexity value.
func isFuncLike(n core.SymbolNode) bool {
	return n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod
}

// Report returns a []FuncComplexity for every func/method node in nodes, sorted
// ascending by Complexity (lowest first). Non-func/method nodes are excluded.
// The input slice is not modified.
func Report(nodes []core.SymbolNode) []FuncComplexity {
	result := make([]FuncComplexity, 0, len(nodes))
	for _, n := range nodes {
		if !isFuncLike(n) {
			continue
		}
		result = append(result, FuncComplexity{
			Name:       n.Name,
			FilePath:   n.FilePath,
			Line:       n.Line,
			Complexity: n.Complexity,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Complexity != result[j].Complexity {
			return result[i].Complexity < result[j].Complexity
		}
		// Secondary sort by name for determinism when complexity is equal.
		return result[i].Name < result[j].Name
	})
	return result
}

// Hotspots returns the topN func/method nodes with the highest cyclomatic
// complexity, sorted descending (highest first). If topN exceeds the number of
// eligible nodes, all eligible nodes are returned. topN ≤ 0 returns nil.
func Hotspots(nodes []core.SymbolNode, topN int) []FuncComplexity {
	if topN <= 0 {
		return nil
	}
	// Build the full sorted-ascending list and reverse the tail.
	all := Report(nodes)
	if len(all) == 0 {
		return nil
	}
	n := topN
	if n > len(all) {
		n = len(all)
	}
	// Take the last n elements (highest complexity in ascending list) and reverse.
	tail := all[len(all)-n:]
	result := make([]FuncComplexity, n)
	for i, fc := range tail {
		result[n-1-i] = fc
	}
	return result
}

// MaintainabilityIndex computes a Maintainability Index (MI) score for the
// provided symbol nodes, in the range [0, 100].  A higher score indicates more
// maintainable code.
//
// Formula (adapted from SEI/Oman-Hagemeister):
//
//	raw = 171 - 5.2*ln(V) - 0.23*G - 16.2*ln(L)
//	MI  = clamp(raw * 100 / 171, 0, 100)
//
// where:
//
//	V = average cyclomatic complexity of func/method nodes (proxy for Halstead volume)
//	G = average cyclomatic complexity (same; using a single metric keeps it deterministic)
//	L = average LOC estimate (number of nodes used as proxy; minimum 1)
//
// When there are no func/method nodes the function returns 100.0 (no code = perfectly
// maintainable from a complexity standpoint).
func MaintainabilityIndex(nodes []core.SymbolNode) float64 {
	var totalCx float64
	var count int
	for _, n := range nodes {
		if !isFuncLike(n) {
			continue
		}
		cx := float64(n.Complexity)
		if cx < 1 {
			cx = 1 // guard against zero complexity
		}
		totalCx += cx
		count++
	}
	if count == 0 {
		return 100.0
	}

	avgCx := totalCx / float64(count)
	// Use count as the LOC proxy (each function = 1 unit; minimum 1).
	avgLOC := float64(count)
	if avgLOC < 1 {
		avgLOC = 1
	}

	raw := 171.0 - 5.2*math.Log(avgCx) - 0.23*avgCx - 16.2*math.Log(avgLOC)
	mi := raw * 100.0 / 171.0

	// Clamp to [0, 100].
	if mi > 100 {
		mi = 100
	}
	if mi < 0 {
		mi = 0
	}
	return mi
}
