package semantic

// compileorder_test.go — O54 deliverable 1: orderSectionsByGoverningScope
// pinned in isolation (no LLM, no store) before the fuller Compile()-level
// proof in compile_test.go. Second review round: a two-tier (nested guide /
// everything else root-scoped) split sorted the root-scoped bucket by byte
// SIZE, which on this repository's own root CLAUDE.md ranked the
// Non-negotiables table — the table that governs every file — 65th of 65
// sections behind 64 smaller ones. The fix is three tiers: nested guide,
// the root's OWN guide (document order, same as a nested guide), then
// everything else root-scoped (size order, as before).

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestDocCompileTier(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"CLAUDE.md", docTierRootGuide},
		{"AGENTS.md", docTierRootGuide},
		{"ADR.md", docTierRootOther},
		{"docs/plans/README.md", docTierRootOther},
		{"internal/CLAUDE.md", docTierNestedGuide},
		{"cmd/fathom/CLAUDE.md", docTierNestedGuide},
		{"schema/CLAUDE.md", docTierNestedGuide},
		{"internal/mcp/AGENTS.md", docTierNestedGuide},
	}
	for _, c := range cases {
		if got := docCompileTier(c.path); got != c.want {
			t.Errorf("docCompileTier(%q) = %d, want %d", c.path, got, c.want)
		}
	}
}

// TestOrderSectionsByGoverningScope_NestedGuidesBeforeRootGuideBeforeOther:
// the three tiers sort in order regardless of alphabetical doc order (O54's
// bug: "ADR.md" < "CLAUDE.md" < "internal/CLAUDE.md" alphabetically, exactly
// backwards from what a budget stop should leave compiled).
func TestOrderSectionsByGoverningScope_NestedGuidesBeforeRootGuideBeforeOther(t *testing.T) {
	all := []docSections{
		{
			doc:      core.SourceFile{Path: "ADR.md"},
			sections: SplitMarkdownSections("ADR.md", []byte("# Decisions\n\n- root-other rule\n")),
		},
		{
			doc:      core.SourceFile{Path: "CLAUDE.md"},
			sections: SplitMarkdownSections("CLAUDE.md", []byte("# root guide\n\n- root guide rule\n")),
		},
		{
			doc:      core.SourceFile{Path: "internal/CLAUDE.md"},
			sections: SplitMarkdownSections("internal/CLAUDE.md", []byte("# internal\n\n- nested rule\n")),
		},
	}
	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != 3 {
		t.Fatalf("len(ordered) = %d, want 3", len(ordered))
	}
	if ordered[0].sec.Path != "internal/CLAUDE.md" {
		t.Errorf("ordered[0].Path = %q, want internal/CLAUDE.md (nested guide first)", ordered[0].sec.Path)
	}
	if ordered[1].sec.Path != "CLAUDE.md" {
		t.Errorf("ordered[1].Path = %q, want CLAUDE.md (the root's own guide, ahead of root-other)", ordered[1].sec.Path)
	}
	if ordered[2].sec.Path != "ADR.md" {
		t.Errorf("ordered[2].Path = %q, want ADR.md (root-other tail last)", ordered[2].sec.Path)
	}
}

// TestOrderSectionsByGoverningScope_GuideTiersKeepDocumentOrderNeverSize is
// the second review round's own regression: within the nested-guide and
// root-guide tiers, a LARGE section that an author put FIRST (the shape of
// this repository's own root CLAUDE.md "Non-negotiables" table) must sort
// ahead of a smaller section that comes later in the SAME document — size
// must never reorder a guide's own document structure.
func TestOrderSectionsByGoverningScope_GuideTiersKeepDocumentOrderNeverSize(t *testing.T) {
	// The root CLAUDE.md: a large first section (the governing table),
	// then a small second section — document order must survive.
	rootDoc := "# root\n\n" +
		"## Non-negotiables\n\n" +
		"- a long line one that pads this section well past the second section's byte length on its own\n" +
		"- a long line two that pads this section well past the second section's byte length on its own\n\n" +
		"## Small\n\n" +
		"- s\n"
	all := []docSections{
		{doc: core.SourceFile{Path: "CLAUDE.md"}, sections: SplitMarkdownSections("CLAUDE.md", []byte(rootDoc))},
	}
	sections := SplitMarkdownSections("CLAUDE.md", []byte(rootDoc))
	if len(sections) != 3 {
		t.Fatalf("fixture split into %d sections, want 3 (preamble, Non-negotiables, Small)", len(sections))
	}
	if len(sections[1].Text) <= len(sections[2].Text) {
		t.Fatalf("fixture bug: Non-negotiables (%d bytes) must be LARGER than Small (%d bytes)", len(sections[1].Text), len(sections[2].Text))
	}

	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != 3 {
		t.Fatalf("len(ordered) = %d, want 3", len(ordered))
	}
	// Document order, not size: preamble, then Non-negotiables, then Small.
	if ordered[1].sec.Subject != "CLAUDE.md#non-negotiables" {
		t.Fatalf("ordered[1].Subject = %q, want the LARGER Non-negotiables section to sort by document order, not behind Small", ordered[1].sec.Subject)
	}
	if ordered[2].sec.Subject != "CLAUDE.md#small" {
		t.Fatalf("ordered[2].Subject = %q, want Small last (document order)", ordered[2].sec.Subject)
	}
}

// TestOrderSectionsByGoverningScope_SmallerSectionsFirstInRootOtherTier:
// two root-OTHER documents (never a guide) still sort smaller-section-first
// within their tier, independent of doc path order — the one sorting rule
// that survives from before the second review round, now scoped to tier 3
// only.
func TestOrderSectionsByGoverningScope_SmallerSectionsFirstInRootOtherTier(t *testing.T) {
	all := []docSections{
		{
			doc:      core.SourceFile{Path: "docs/plans/README.md"},
			sections: SplitMarkdownSections("docs/plans/README.md", []byte("# zzz\n\n- a\n")),
		},
		{
			doc:      core.SourceFile{Path: "ADR.md"},
			sections: SplitMarkdownSections("ADR.md", []byte("# aaa\n\n- a much, much longer rule sentence that pushes this section's byte length well past the short one above\n")),
		},
	}
	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != 2 {
		t.Fatalf("len(ordered) = %d, want 2", len(ordered))
	}
	if ordered[0].sec.Path != "docs/plans/README.md" {
		t.Errorf("ordered[0].Path = %q, want docs/plans/README.md (the SMALLER section, despite sorting after ADR.md alphabetically)", ordered[0].sec.Path)
	}
}

// TestOrderSectionsByGoverningScope_Deterministic: same input, same order,
// on every call.
func TestOrderSectionsByGoverningScope_Deterministic(t *testing.T) {
	all := []docSections{
		{doc: core.SourceFile{Path: "ADR.md"}, sections: SplitMarkdownSections("ADR.md", []byte("# A\n\n- x\n"))},
		{doc: core.SourceFile{Path: "CLAUDE.md"}, sections: SplitMarkdownSections("CLAUDE.md", []byte("# B\n\n- y\n"))},
		{doc: core.SourceFile{Path: "internal/CLAUDE.md"}, sections: SplitMarkdownSections("internal/CLAUDE.md", []byte("# C\n\n- z\n"))},
		{doc: core.SourceFile{Path: "cmd/fathom/CLAUDE.md"}, sections: SplitMarkdownSections("cmd/fathom/CLAUDE.md", []byte("# D\n\n- w\n"))},
	}
	first := orderSectionsByGoverningScope(all)
	second := orderSectionsByGoverningScope(all)
	if len(first) != len(second) {
		t.Fatalf("len mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].sec.Path != second[i].sec.Path || first[i].sec.Subject != second[i].sec.Subject {
			t.Fatalf("order[%d] differs across calls: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestUncompiledDocScopes(t *testing.T) {
	if got := uncompiledDocScopes(nil); got != nil {
		t.Errorf("uncompiledDocScopes(nil) = %v, want nil", got)
	}
	tail := []orderedSection{
		{sec: Section{Path: "ADR.md"}},
		{sec: Section{Path: "ADR.md"}}, // same doc, two sections — dedupe
		{sec: Section{Path: "docs/plans/README.md"}},
	}
	got := uncompiledDocScopes(tail)
	want := []string{"ADR.md", "docs/plans/README.md"}
	if len(got) != len(want) {
		t.Fatalf("uncompiledDocScopes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uncompiledDocScopes = %v, want %v", got, want)
		}
	}
}
