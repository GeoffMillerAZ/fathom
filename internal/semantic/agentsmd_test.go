package semantic

// agentsmd_test.go — ADR-059 (docs/plans/agents-md.md AG-13..AG-15): the
// obligations corpus reads AGENTS.md first-class, never compiles fathom's own
// stanza region, compiles a CLAUDE.md/AGENTS.md twin section once, and reads
// `.claude/{AGENTS,CLAUDE}.md` as root guides.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const stanzaFixture = "<!-- fathom:begin v3 sha256:abc -->\n## fathom code intelligence\n\nRoute by question shape via `find_symbol`.\n<!-- fathom:end -->"

func TestFathomRegionIsNeverCompiledAsARule(t *testing.T) {
	doc := "# Repo\n\n## Rules\n\n- Sort all outputs.\n\n" + stanzaFixture + "\n\n## After\n\n- Keep the gate green.\n"
	secs := SplitMarkdownSections("AGENTS.md", []byte(doc))
	var subjects []string
	for _, s := range secs {
		subjects = append(subjects, s.Subject)
		if strings.Contains(s.Text, "fathom code intelligence") || strings.Contains(s.Text, "find_symbol") {
			t.Errorf("section %s carries fathom's stanza text:\n%s", s.Subject, s.Text)
		}
	}
	if strings.Contains(strings.Join(subjects, ","), "fathom-code-intelligence") {
		t.Errorf("the stanza's heading became a section: %v", subjects)
	}
	for _, s := range secs {
		if s.Subject == "AGENTS.md#after" && s.StartLine != 13 {
			t.Errorf("a section after the region moved: start line %d, want 13 (document coordinates)", s.StartLine)
		}
	}
	// A corrupted pair is left as written: fathom does not guess.
	broken := "## Rules\n\n<!-- fathom:begin -->\n## fathom code intelligence\ntext\n"
	if got := SplitMarkdownSections("AGENTS.md", []byte(broken)); len(got) != 2 {
		t.Errorf("a corrupted region was blanked: %d sections", len(got))
	}
}

func TestDedupeTwinGuideSectionsKeepsAgentsMD(t *testing.T) {
	shared := "# Repo\n\n## Rules\n\n- Sort all outputs.\n"
	corpus := []core.SourceFile{
		{Path: "CLAUDE.md", Content: []byte(shared + "\n## Claude only\n\n- Use the shim.\n")},
		{Path: "AGENTS.md", Content: []byte(shared)},
		{Path: "internal/CLAUDE.md", Content: []byte("## Nested\n\n- Golden tests.\n")},
		{Path: "internal/AGENTS.md", Content: []byte("## Nested\n\n- Golden tests.\n")},
		{Path: "cmd/CLAUDE.md", Content: []byte("## Only here\n\n- No AGENTS.md twin.\n")},
	}
	got := map[string][]string{}
	for _, ds := range corpusSections(corpus) {
		for _, s := range ds.sections {
			got[ds.doc.Path] = append(got[ds.doc.Path], s.Subject)
		}
	}
	if strings.Join(got["CLAUDE.md"], ",") != "CLAUDE.md#claude-only" {
		t.Errorf("root CLAUDE.md sections = %v, want only the one AGENTS.md lacks", got["CLAUDE.md"])
	}
	if len(got["internal/CLAUDE.md"]) != 0 || len(got["internal/AGENTS.md"]) != 1 {
		t.Errorf("nested twin: CLAUDE.md %v, AGENTS.md %v — want the AGENTS.md copy only", got["internal/CLAUDE.md"], got["internal/AGENTS.md"])
	}
	if len(got["cmd/CLAUDE.md"]) != 1 {
		t.Errorf("a CLAUDE.md with no AGENTS.md twin lost its section: %v", got["cmd/CLAUDE.md"])
	}
}

func TestRootGuidePathsUnderDotClaude(t *testing.T) {
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", ".claude/AGENTS.md", ".claude/CLAUDE.md"} {
		if !isRootGuidePath(p) || scopeFloorDir(p) != "" || docCompileTier(p) != docTierRootGuide || !isRootLevelSource(p+"#rules") {
			t.Errorf("%s: root=%v floor=%q tier=%d rootLevel=%v, want a root guide", p, isRootGuidePath(p), scopeFloorDir(p), docCompileTier(p), isRootLevelSource(p+"#rules"))
		}
	}
	for _, p := range []string{"internal/AGENTS.md", "internal/.claude/AGENTS.md", "ADR.md"} {
		if isRootGuidePath(p) {
			t.Errorf("%s read as a root guide", p)
		}
	}
	if scopeFloorDir("internal/AGENTS.md") != "internal" {
		t.Errorf("a nested AGENTS.md must floor to its directory, got %q", scopeFloorDir("internal/AGENTS.md"))
	}

	def, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "AGENTS.md"), []byte("## Rules\n\n- One rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extras := RootGuideExtras(root, def.CorpusInclude, def.CorpusExclude, map[string]bool{})
	if len(extras) != 1 || extras[0].Path != ".claude/AGENTS.md" || !strings.Contains(string(extras[0].Content), "One rule") {
		t.Fatalf("RootGuideExtras = %+v, want .claude/AGENTS.md read once", extras)
	}
	if again := RootGuideExtras(root, def.CorpusInclude, def.CorpusExclude, map[string]bool{".claude/AGENTS.md": true}); len(again) != 0 {
		t.Errorf("RootGuideExtras re-read a guide the corpus already has: %+v", again)
	}
}
