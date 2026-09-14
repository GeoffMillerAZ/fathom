package semantic

// compileorder.go — O54 deliverable 1 (a second reading of pilot
// vs-pilot-3): the obligations compile visited sections in plain
// alphabetical DOC order (Compile's `sort.Slice(docs, ... Path < Path)`).
// For the obligations layer's own corpus (["CLAUDE.md", "ADR.md",
// "docs/plans/README.md", "**/CLAUDE.md"]) that put ADR.md and the root
// CLAUDE.md ahead of every NESTED package guide ('A'/'C' sort before the
// lowercase directory names a nested guide lives under) — so a mid-run
// budget stop (compile.go's enforceBudgets) always exhausted the run on
// the root long tail and never reached a single package guide. Exactly
// the shape the pilot cell saw: sections_total 60, sections_compiled 18,
// budget_stopped=true, and the internal/CLAUDE.md rule that governs
// internal/extractor/goextract.go was never among the 18.
//
// The fix visits sections NEAREST-SCOPE-FIRST: every path-scoped guide
// (a nested CLAUDE.md/AGENTS.md — the same nested/root distinction BP-32's
// scopeFloorDir already draws for claim scoping) before every root-scoped
// document (ADR.md, docs/plans/README.md, the root CLAUDE.md's own long
// tail), and within a tier, smaller sections first — so a budget stop
// always lands after covering as many distinct governing guides as the
// budget allows, never stalling mid-way through one large root document
// while every guide sits unread later in doc order.
import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// docTierNearestScope and docTierRoot are the two compile-order tiers a
// governing document falls into (lower sorts first).
const (
	docTierNearestScope = 0
	docTierRoot         = 1
)

// docCompileTier classifies a governing document path into a compile-order
// tier: nearest-scope (a NESTED CLAUDE.md/AGENTS.md, scopeFloorDir != "")
// or root (everything else — the root CLAUDE.md/AGENTS.md, ADR.md,
// docs/plans/README.md, and any future root-scoped corpus member). This is
// the doc-level shadow of scopeFloorDir's own nested/root distinction
// (BP-32) — not a second scoping mechanism, just its compile-ORDER
// consequence.
func docCompileTier(docPath string) int {
	if scopeFloorDir(docPath) != "" {
		return docTierNearestScope
	}
	return docTierRoot
}

// orderedSection pairs one section with the doc it was split from, for the
// FLAT, globally sorted compile order the section loop walks — replacing
// the old doc-then-section nested walk whose doc order was merely
// alphabetical.
type orderedSection struct {
	doc core.SourceFile
	sec Section
}

// orderSectionsByGoverningScope flattens all's per-doc sections into ONE
// deterministic compile order:
//
//  1. tier (docCompileTier): every nested-guide section before every
//     root-scoped section, regardless of which doc either came from.
//  2. within a tier, section byte size ascending ("smaller sections
//     first") — a budget stop then lands after covering as many distinct
//     small guides/sections as the budget allows, rather than stalling
//     mid-way through one large section.
//  3. a fully deterministic tie-break (doc path, then start line) so two
//     equal-tier, equal-size sections always compile in the same order on
//     every machine and every run.
//
// Deterministic: same corpus, same order, every call — no clock, no
// network, no map iteration.
func orderSectionsByGoverningScope(all []docSections) []orderedSection {
	out := make([]orderedSection, 0, len(all))
	for _, ds := range all {
		for _, sec := range ds.sections {
			out = append(out, orderedSection{doc: ds.doc, sec: sec})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ta, tb := docCompileTier(a.sec.Path), docCompileTier(b.sec.Path); ta != tb {
			return ta < tb
		}
		if la, lb := len(a.sec.Text), len(b.sec.Text); la != lb {
			return la < lb
		}
		if a.sec.Path != b.sec.Path {
			return a.sec.Path < b.sec.Path
		}
		return a.sec.StartLine < b.sec.StartLine
	})
	return out
}

// uncompiledDocScopes returns the sorted, deduplicated set of document
// paths represented in tail — the ordered sections a compile run never
// reached this call (O54 deliverable 2's layer_coverage.uncompiled_scopes).
// Empty/nil tail (the run finished the whole corpus) yields nil, never an
// empty-but-non-nil slice, so the JSON field is omitted rather than
// rendered as `[]`.
func uncompiledDocScopes(tail []orderedSection) []string {
	if len(tail) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, os := range tail {
		if !seen[os.sec.Path] {
			seen[os.sec.Path] = true
			out = append(out, os.sec.Path)
		}
	}
	sort.Strings(out)
	return out
}
