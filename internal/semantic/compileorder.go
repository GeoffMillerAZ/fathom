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
// The fix visits sections NEAREST-SCOPE-FIRST, in THREE tiers — a plain
// two-tier (nested guide / everything else root-scoped) split was itself
// a second bug the first review round found: sorting the "everything else
// root-scoped" bucket by byte SIZE put the root CLAUDE.md's own
// Non-negotiables table — the table that governs every file in the repo,
// 1,742 bytes on this repository's own root CLAUDE.md — behind 64 smaller
// root-scoped sections, ranking it 65th of 65 and starving it exactly
// like O54's original bug starved a nested guide:
//
//  1. docTierNestedGuide — a NESTED CLAUDE.md/AGENTS.md below the repo
//     root (scopeFloorDir != ""). Document order, never size: a human
//     author's own section order inside ONE guide is never worth
//     defeating for a byte count.
//  2. docTierRootGuide — the repo ROOT's own CLAUDE.md/AGENTS.md. Same
//     document-order rule as tier 1, and for the same reason: whatever a
//     human put first (a repo's Non-negotiables table, say) is exactly
//     what a reader — human or budget-limited compiler — should reach
//     first, never re-ordered behind smaller sections later in the file.
//  3. docTierRootOther — every other root-scoped document (ADR.md,
//     docs/plans/README.md, a future root-scoped corpus member). Smaller
//     sections first, as before: a budget stop here should still cover as
//     many distinct sections as it can, since nothing about THIS tier's
//     internal order carries the same "author put the important part
//     first" signal a governing instruction file's own structure does.
//
// Within a tier, ties are fully deterministic: document path, then the
// section's own position within that document (StartLine — sections are
// split in document order, so this is never a second, independent
// ordering rule).
import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// docTierNestedGuide, docTierRootGuide and docTierRootOther are the three
// compile-order tiers a governing document falls into (lower sorts
// first) — see this file's doc comment.
const (
	docTierNestedGuide = 0
	docTierRootGuide   = 1
	docTierRootOther   = 2
)

// docCompileTier classifies a governing document path into a compile-order
// tier: nested guide (a CLAUDE.md/AGENTS.md below the repo root,
// scopeFloorDir != ""), root guide (the repo root's OWN CLAUDE.md/
// AGENTS.md — instructionFileBases keyed by the exact bare path, so this
// is true only for literally "CLAUDE.md" or "AGENTS.md", never a nested
// one), or root-other (everything else root-scoped: ADR.md, docs/plans/
// README.md, and any future root-scoped corpus member). This is the
// doc-level shadow of scopeFloorDir's own nested/root distinction (BP-32)
// plus isRootLevelSource's own root-guide check (serve.go) — not a third
// scoping mechanism, just their compile-ORDER consequence.
func docCompileTier(docPath string) int {
	if scopeFloorDir(docPath) != "" {
		return docTierNestedGuide
	}
	if isRootGuidePath(docPath) {
		return docTierRootGuide
	}
	return docTierRootOther
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
// deterministic compile order — see this file's doc comment for the
// three-tier rule and why tiers 1/2 (guides) keep document order while
// tier 3 (root-other) sorts smaller-first. Deterministic: same corpus,
// same order, every call — no clock, no network, no map iteration.
func orderSectionsByGoverningScope(all []docSections) []orderedSection {
	out := make([]orderedSection, 0, len(all))
	for _, ds := range all {
		for _, sec := range ds.sections {
			out = append(out, orderedSection{doc: ds.doc, sec: sec})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ta, tb := docCompileTier(a.sec.Path), docCompileTier(b.sec.Path)
		if ta != tb {
			return ta < tb
		}
		// Root-other (tier 3) only: smaller sections first, so a budget
		// stop covers as many distinct root-scoped sections as it can.
		// Guide tiers (1/2) fall straight through to the document-order
		// tie-break below — a guide's own section order is never
		// defeated by byte count (this file's doc comment).
		if ta == docTierRootOther {
			if la, lb := len(a.sec.Text), len(b.sec.Text); la != lb {
				return la < lb
			}
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
