package semantic

// compileorder_test.go — O54 deliverable 1: orderSectionsByGoverningScope
// pinned in isolation (no LLM, no store) before the fuller Compile()-level
// proof in compile_test.go.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestDocCompileTier(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"CLAUDE.md", docTierRoot},
		{"AGENTS.md", docTierRoot},
		{"ADR.md", docTierRoot},
		{"docs/plans/README.md", docTierRoot},
		{"internal/CLAUDE.md", docTierNearestScope},
		{"cmd/fathom/CLAUDE.md", docTierNearestScope},
		{"schema/CLAUDE.md", docTierNearestScope},
		{"internal/mcp/AGENTS.md", docTierNearestScope},
	}
	for _, c := range cases {
		if got := docCompileTier(c.path); got != c.want {
			t.Errorf("docCompileTier(%q) = %d, want %d", c.path, got, c.want)
		}
	}
}

// TestOrderSectionsByGoverningScope_NestedGuidesBeforeRoot: every nested
// guide's section sorts before every root document's section, regardless
// of alphabetical doc order (O54's bug: "ADR.md" < "internal/CLAUDE.md"
// alphabetically, exactly backwards from what a budget stop should leave
// compiled).
func TestOrderSectionsByGoverningScope_NestedGuidesBeforeRoot(t *testing.T) {
	all := []docSections{
		{
			doc:      core.SourceFile{Path: "ADR.md"},
			sections: SplitMarkdownSections("ADR.md", []byte("# Decisions\n\n- root rule\n")),
		},
		{
			doc:      core.SourceFile{Path: "internal/CLAUDE.md"},
			sections: SplitMarkdownSections("internal/CLAUDE.md", []byte("# internal\n\n- nested rule\n")),
		},
	}
	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != 2 {
		t.Fatalf("len(ordered) = %d, want 2", len(ordered))
	}
	if ordered[0].sec.Path != "internal/CLAUDE.md" {
		t.Errorf("ordered[0].Path = %q, want internal/CLAUDE.md (nested guide first, despite sorting after ADR.md alphabetically)", ordered[0].sec.Path)
	}
	if ordered[1].sec.Path != "ADR.md" {
		t.Errorf("ordered[1].Path = %q, want ADR.md (root tail last)", ordered[1].sec.Path)
	}
}

// TestOrderSectionsByGoverningScope_SmallerSectionsFirstWithinATier: two
// nested guides sort smaller-section-first, independent of doc path order.
func TestOrderSectionsByGoverningScope_SmallerSectionsFirstWithinATier(t *testing.T) {
	all := []docSections{
		{
			doc:      core.SourceFile{Path: "zzz/CLAUDE.md"},
			sections: SplitMarkdownSections("zzz/CLAUDE.md", []byte("# zzz\n\n- a\n")),
		},
		{
			doc:      core.SourceFile{Path: "aaa/CLAUDE.md"},
			sections: SplitMarkdownSections("aaa/CLAUDE.md", []byte("# aaa\n\n- a much, much longer rule sentence that pushes this section's byte length well past the short one above\n")),
		},
	}
	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != 2 {
		t.Fatalf("len(ordered) = %d, want 2", len(ordered))
	}
	if ordered[0].sec.Path != "zzz/CLAUDE.md" {
		t.Errorf("ordered[0].Path = %q, want zzz/CLAUDE.md (the SMALLER section, despite sorting after aaa/CLAUDE.md alphabetically)", ordered[0].sec.Path)
	}
}

// TestOrderSectionsByGoverningScope_Deterministic: same input, same order,
// on every call.
func TestOrderSectionsByGoverningScope_Deterministic(t *testing.T) {
	all := []docSections{
		{doc: core.SourceFile{Path: "ADR.md"}, sections: SplitMarkdownSections("ADR.md", []byte("# A\n\n- x\n"))},
		{doc: core.SourceFile{Path: "internal/CLAUDE.md"}, sections: SplitMarkdownSections("internal/CLAUDE.md", []byte("# B\n\n- y\n"))},
		{doc: core.SourceFile{Path: "cmd/fathom/CLAUDE.md"}, sections: SplitMarkdownSections("cmd/fathom/CLAUDE.md", []byte("# C\n\n- z\n"))},
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
