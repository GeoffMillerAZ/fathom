package scenariobench

import (
	"regexp"
	"sort"
	"strings"
)

// readings_plan.go is SB-20 (vision 27 §3, M3): PlanCoverage scores what an
// agent's own scratch plan named, before its first edit, against the gold
// patch's touched functions. The plan here is whatever the harness captured
// as the agent's plan file during the session (a PLAN.md it wrote, or a
// docs/plans-shaped scratch checklist) — never docs/plans/strengths-bench.md
// itself, so this package deliberately keeps its own minimal copy of the
// checklist/annotation grammar rather than importing internal/verify: an
// agent's session-scoped plan is not a file under that package's push-gate
// contract, and the two must never be able to drift into needing the same
// fix twice (the lesson O4 already paid for once).

// planCheckboxRe matches a Markdown checklist item: "- [ ] title", "* [x]
// title" — the same shape docs/CLAUDE.md's plan grammar v2 and
// internal/verify's taskLineRe both use.
var planCheckboxRe = regexp.MustCompile(`^\s*[-*]\s+\[([ xX~-])\]\s+(.*\S)\s*$`)

// planBacktickRe pulls backticked identifiers out of a checklist row's
// title: `FuncName`, `Type.Method` — the shape a control-arm agent (no v2
// grammar, no fathom stanza) actually writes to name a function.
var planBacktickRe = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)?)`")

// trailingBraceBlock returns the content between the LAST "{" and the LAST
// "}" on the line, or "" when there is no such pair. This is a
// deliberately simple (non-quote-aware) reading of the v2 grammar's
// annotation block — sufficient for a bench reading over an agent's own
// scratch plan, unlike internal/verify/markdown.go's parseAnnotation,
// which must also survive a human hand-authoring docs/plans/*.md under
// push-gate stakes.
func trailingBraceBlock(s string) string {
	open := strings.LastIndex(s, "{")
	closeAt := strings.LastIndex(s, "}")
	if open < 0 || closeAt < open {
		return ""
	}
	return s[open+1 : closeAt]
}

// parseSimpleAnnotation splits a "{key: value, …}" block on commas and
// colons, trimming one pair of surrounding double quotes from each value.
func parseSimpleAnnotation(block string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(block, ",") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

// planSymbolRow is one v2-grammar-annotated checklist row: a `symbol:`
// (and, when present, `package:`) key pulled from the row's trailing
// annotation block.
type planSymbolRow struct {
	ID, Symbol, Package string
}

// extractPlanSymbolRows walks planMD's checklist rows and returns every
// row carrying a v2-grammar `symbol:` annotation key. Rows named only by a
// plain backticked identifier (no annotation) are PlanCoverage's concern,
// not this one: ContractDrift needs a `package:` to search a specific
// directory, which only the annotation form supplies.
func extractPlanSymbolRows(planMD []byte) []planSymbolRow {
	var rows []planSymbolRow
	for _, line := range strings.Split(string(planMD), "\n") {
		m := planCheckboxRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		block := trailingBraceBlock(m[2])
		if block == "" {
			continue
		}
		kv := parseSimpleAnnotation(block)
		if sym := kv["symbol"]; sym != "" {
			rows = append(rows, planSymbolRow{ID: kv["id"], Symbol: sym, Package: kv["package"]})
		}
	}
	return rows
}

// matchGold reports whether named (a symbol mentioned in the plan) refers
// to one of gold's qualified function names, either exactly or by its
// trailing dotted segment — so a plan naming "pkg.Func" matches gold's bare
// "Func", and a plan naming bare "Method" matches gold's "Type.Method".
func matchGold(named string, gold []string) (string, bool) {
	for _, g := range gold {
		if g == named {
			return g, true
		}
		if i := strings.LastIndex(named, "."); i >= 0 && named[i+1:] == g {
			return g, true
		}
		if i := strings.LastIndex(g, "."); i >= 0 && g[i+1:] == named {
			return g, true
		}
	}
	return "", false
}

// PlanCoverage is SB-20 (vision 27 §3, M3): before the first edit, the
// fraction of gold's functions the agent's own plan named, and the rows
// that named something else (spurious). A row's named symbol is its
// v2-grammar `symbol:` value when the row carries one annotation, else
// every backticked identifier on the row — never both, so a v2 row's
// derivation prose (which may backtick unrelated names) cannot inflate its
// own count.
//
// This is a lower bound by construction (vision 27 §3): a different valid
// implementation can score low while passing every test, so the reading is
// reported, never gated, unless a corpus criterion names it explicitly.
func PlanCoverage(planMD []byte, gold Gold) PlanCoverageReading {
	matchedGold := map[string]bool{}
	spuriousSet := map[string]bool{}
	sawRow := false
	for _, line := range strings.Split(string(planMD), "\n") {
		m := planCheckboxRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		sawRow = true
		title := m[2]
		var named []string
		if block := trailingBraceBlock(title); block != "" {
			if sym := parseSimpleAnnotation(block)["symbol"]; sym != "" {
				named = append(named, sym)
			}
		}
		if named == nil {
			for _, bt := range planBacktickRe.FindAllStringSubmatch(title, -1) {
				named = append(named, bt[1])
			}
		}
		for _, sym := range named {
			if g, ok := matchGold(sym, gold.Functions); ok {
				matchedGold[g] = true
			} else {
				spuriousSet[sym] = true
			}
		}
	}

	reading := PlanCoverageReading{GoldFunctions: len(gold.Functions), Named: len(matchedGold)}
	for s := range spuriousSet {
		reading.Spurious = append(reading.Spurious, s)
	}
	sort.Strings(reading.Spurious)

	switch {
	case !sawRow:
		reading.NotMeasurable = "no checklist rows found in plan"
	case len(gold.Functions) == 0:
		reading.NotMeasurable = "gold has no functions"
	default:
		reading.Fraction = float64(reading.Named) / float64(reading.GoldFunctions)
	}
	return reading
}
