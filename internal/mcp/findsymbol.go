// Package mcp — tiered find_symbol matching.
//
// The exact/key-suffix tier lives in the query engine (symbolMatches). When it
// misses, the MCP layer degrades deterministically over the service's symbol
// list: case-insensitive (same equality rules, case-folded), then substring
// containment, then a concern-ranked did_you_mean via QuerySymbols. The tool
// therefore never punishes not knowing the exact identifier — which is exactly
// when an agent reaches for it.
package mcp

import (
	"sort"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// maxSubstringMatches caps the substring tier so a short query ("get") cannot
// flood the response; truncation is reported explicitly.
const maxSubstringMatches = 20

// maxSuggestions caps the ranked did_you_mean list on a full miss.
const maxSuggestions = 5

// caseFoldMatches returns nodes matching name under the engine's matching
// rules with case folded — the case-insensitive tier — affinity-ranked
// rather than left in plain alphabetical order (P1-3, closing O12(c)).
//
// Group 0 is a NAME match (the original rule: exact, stored "Type.Method"
// vs bare query, or qualified query vs bare stored). Group 1 is a PACKAGE
// match: the query names an entire package (e.g. "heartbeat" for
// foundations/telemetry/heartbeat), whose own exported symbols are never
// literally named "heartbeat" — group 0 alone finds only a same-named TYPE
// in an unrelated package (worker-heartbeat's `Heartbeat` GoType) and
// hides the actual package's members from the caller. Reproduced bug:
// find_symbol(agentic-kit, "heartbeat") returned ONLY the coincidentally
// alphabetized worker-heartbeat GoType, steering the caller away from the
// telemetry/heartbeat package it actually needed.
//
// Both groups are returned TOGETHER — multi-match, never first-hit
// steering — sorted by group, then the pre-existing deterministic
// (name, file, line, kind) order within each group. matched_via stays
// "case_insensitive" for every result regardless of which group matched.
func caseFoldMatches(all []core.SymbolNode, name string) []core.SymbolNode {
	q := strings.ToLower(name)
	type ranked struct {
		node  core.SymbolNode
		group int
	}
	var out []ranked
	for _, n := range all {
		stored := strings.ToLower(n.Name)
		if stored == q || strings.HasSuffix(stored, "."+q) || strings.HasSuffix(q, "."+stored) {
			out = append(out, ranked{n, 0})
			continue
		}
		if pkg := strings.ToLower(n.Package); pkg != "" && (pkg == q || strings.HasSuffix(pkg, "/"+q)) {
			out = append(out, ranked{n, 1})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].group != out[j].group {
			return out[i].group < out[j].group
		}
		return nodeLess(out[i].node, out[j].node)
	})
	nodes := make([]core.SymbolNode, len(out))
	for i, r := range out {
		nodes[i] = r.node
	}
	return nodes
}

// substringMatches returns nodes whose name contains name case-insensitively,
// tightest (shortest) names first, capped at maxSubstringMatches. total is
// the pre-cap match count, so a caller can disclose what was elided
// (BA-1/BA-2) — total == len(nodes) when nothing was cut.
func substringMatches(all []core.SymbolNode, name string) (nodes []core.SymbolNode, total int) {
	q := strings.ToLower(name)
	for _, n := range all {
		if strings.Contains(strings.ToLower(n.Name), q) {
			nodes = append(nodes, n)
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if len(nodes[i].Name) != len(nodes[j].Name) {
			return len(nodes[i].Name) < len(nodes[j].Name)
		}
		return nodeLess(nodes[i], nodes[j])
	})
	total = len(nodes)
	if len(nodes) > maxSubstringMatches {
		return nodes[:maxSubstringMatches], total
	}
	return nodes, total
}

// nodeLess is the deterministic total order for equal-priority nodes:
// name, file, line, kind. Duplicate names in one file (e.g. repeated Markdown
// headings) must not depend on store iteration order.
func nodeLess(a, b core.SymbolNode) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Kind < b.Kind
}

// nameTokens derives keyword hints from a symbol name for the ranked
// fallback: the name itself plus its separator- and camelCase-split segments,
// lowercased and deduplicated.
func nameTokens(name string) []string {
	seen := map[string]bool{}
	tokens := []string{}
	add := func(s string) {
		s = strings.ToLower(s)
		if s != "" && !seen[s] {
			seen[s] = true
			tokens = append(tokens, s)
		}
	}
	add(name)
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	}) {
		add(seg)
		for _, camel := range splitCamel(seg) {
			add(camel)
		}
	}
	return tokens
}

// splitCamel breaks a camelCase / PascalCase segment into its words
// ("ValidateToken" → ["Validate", "Token"]; runs of capitals stay together:
// "HTTPServer" → ["HTTP", "Server"]).
func splitCamel(s string) []string {
	var words []string
	runes := []rune(s)
	start := 0
	for i := 1; i < len(runes); i++ {
		prevUpper := unicode.IsUpper(runes[i-1])
		curUpper := unicode.IsUpper(runes[i])
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if (!prevUpper && curUpper) || (prevUpper && curUpper && nextLower) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	words = append(words, string(runes[start:]))
	if len(words) <= 1 {
		return nil
	}
	return words
}

// fuzzySuggestions returns nodes whose name is within a small edit distance
// of the query — the typo tier. It compares the query against the lowercased
// full name and its last dot-segment, so "authentiction" finds
// "Authentication" and "servehtp" finds "Server.ServeHTTP". Deterministic:
// sorted by distance, then nodeLess; capped at maxSuggestions.
func fuzzySuggestions(all []core.SymbolNode, name string) []core.SymbolNode {
	q := strings.ToLower(name)
	limit := fuzzyLimit(len(q))
	if limit == 0 {
		return nil
	}
	type cand struct {
		node core.SymbolNode
		dist int
	}
	var cands []cand
	for _, n := range all {
		stored := strings.ToLower(n.Name)
		best := editDistanceCapped(q, stored, limit)
		if i := strings.LastIndex(stored, "."); i >= 0 {
			if d := editDistanceCapped(q, stored[i+1:], limit); d >= 0 && (best < 0 || d < best) {
				best = d
			}
		}
		if best >= 0 {
			cands = append(cands, cand{node: n, dist: best})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return nodeLess(cands[i].node, cands[j].node)
	})
	if len(cands) > maxSuggestions {
		cands = cands[:maxSuggestions]
	}
	nodes := make([]core.SymbolNode, len(cands))
	for i, c := range cands {
		nodes[i] = c.node
	}
	return nodes
}

// fuzzyLimit is the maximum edit distance considered a plausible typo for a
// query of length n. Short names get no fuzzy matching — everything is one
// edit away from "Get".
func fuzzyLimit(n int) int {
	switch {
	case n < 4:
		return 0
	case n < 8:
		return 1
	default:
		return 2
	}
}

// editDistanceCapped returns the Levenshtein distance between a and b, or -1
// when it exceeds limit (with a cheap length pre-check).
func editDistanceCapped(a, b string, limit int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > limit || -d > limit {
		return -1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if cur[j] < rowMin {
				rowMin = cur[j]
			}
		}
		if rowMin > limit {
			return -1
		}
		prev, cur = cur, prev
	}
	if prev[len(rb)] > limit {
		return -1
	}
	return prev[len(rb)]
}

// suggestionDTOs renders ranked nodes as slim did_you_mean entries — enough
// to re-query by the right name without bloating a miss response.
func suggestionDTOs(nodes []core.SymbolNode) []map[string]any {
	if len(nodes) > maxSuggestions {
		nodes = nodes[:maxSuggestions]
	}
	out := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		out[i] = map[string]any{
			"name":      n.Name,
			"kind":      string(n.Kind),
			"file_path": n.FilePath,
			"line":      n.Line,
		}
	}
	return out
}
