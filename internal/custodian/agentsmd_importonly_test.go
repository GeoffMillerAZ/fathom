package custodian

// agentsmd_importonly_test.go — CS-5 (docs/plans/cloud-sessions.md, ADR-059
// amendment 2026-10-02): `import_only: true` admits a CLAUDE.md only when
// its whole content is the `@AGENTS.md` import of the AGENTS.md beside it,
// and, once a root CLAUDE.md puts Claude Code in CLAUDE.md mode, demands
// that every nested AGENTS.md has such an import beside it — the nested
// guides load on demand only through a CLAUDE.md of their own there.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func importOnlyConcern() Concern {
	return Concern{ID: "one_instruction_file", Want: "every CLAUDE.md is only the import", Scope: ScopeProject,
		Check: Check{Kind: KindAgentsMDReachesClaude, ImportOnly: true}}
}

func TestAgentsMDImportOnly(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		want   Status
		detail string
	}{
		{"no CLAUDE file holds", map[string]string{"AGENTS.md": "# rules\n"}, StatusHolds, "no CLAUDE file"},
		{"the root import alone holds", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n"}, StatusHolds, "1 CLAUDE file(s), each only the @AGENTS.md import"},
		{"blank lines around the import hold", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "\n" + "@AGENTS.md\n\n"}, StatusHolds, "each only the @AGENTS.md import"},
		{"root and nested imports hold", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n", "internal/AGENTS.md": "y\n", "internal/CLAUDE.md": "@AGENTS.md\n"}, StatusHolds, "2 CLAUDE file(s)"},
		{"prose beside the import is refused", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n\nAlways run the tests.\n"}, StatusViolated, "CLAUDE.md carries more than the @AGENTS.md import"},
		{"a fenced import is not the import", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "```\n" + "@AGENTS.md\n```\n"}, StatusViolated, "CLAUDE.md carries more than the @AGENTS.md import"},
		{"an import of another file is refused", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@docs/rules.md\n"}, StatusViolated, "CLAUDE.md carries more than the @AGENTS.md import"},
		{"a nested import with no AGENTS.md beside it is refused", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n", "pkg/CLAUDE.md": "@AGENTS.md\n"}, StatusViolated, "pkg/CLAUDE.md imports pkg/AGENTS.md, which does not exist"},
		{"a nested guide the root CLAUDE.md strands is named", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n", "internal/AGENTS.md": "y\n"}, StatusViolated, "internal/AGENTS.md has no CLAUDE.md beside it"},
		{"nested guides need no import without a root CLAUDE.md", map[string]string{"AGENTS.md": "x\n", "internal/AGENTS.md": "y\n"}, StatusHolds, "no CLAUDE file"},
		{".claude/CLAUDE.md is refused", map[string]string{"AGENTS.md": "x\n", ".claude/CLAUDE.md": "@../AGENTS.md\n"}, StatusViolated, ".claude/CLAUDE.md"},
		{"CLAUDE.local.md is refused", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n", "CLAUDE.local.md": "@AGENTS.md\n"}, StatusViolated, "CLAUDE.local.md"},
		{"test fixtures are input, not instruction files", map[string]string{"AGENTS.md": "x\n", "internal/x/testdata/fixture/CLAUDE.md": "anything\n"}, StatusHolds, "no CLAUDE file"},
		{"no AGENTS.md is still a violation", map[string]string{"CLAUDE.md": "@AGENTS.md\n"}, StatusViolated, "no AGENTS.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, c.files)
			f := checkAgentsMDReachesClaude(root, importOnlyConcern())
			if f.Status != c.want || !strings.Contains(f.Detail, c.detail) {
				t.Fatalf("got %s %q, want %s containing %q", f.Status, f.Detail, c.want, c.detail)
			}
		})
	}
}

// TestAgentsMDImportOnlyLoads: import_only loads from CUE on its kind, is
// refused on another kind, and contradicting allow_import: false is refused
// by name.
func TestAgentsMDImportOnlyLoads(t *testing.T) {
	load := func(check string) ([]Concern, error) {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		src := "concerns: {\n\tone: {\n\t\twant: \"only the import\"\n\t\tcheck: " + check + "\n\t}\n}\n"
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return LoadFile(p, ScopeProject)
	}
	cs, err := load(`{kind: "agents_md_reaches_claude", import_only: true}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || !cs[0].Check.ImportOnly {
		t.Fatalf("import_only: true did not load: %+v", cs)
	}
	if _, err := load(`{kind: "max_lines", file: "AGENTS.md", lines: 120, import_only: true}`); err == nil || !strings.Contains(err.Error(), "check.import_only applies to agents_md_reaches_claude only") {
		t.Fatalf("import_only on another kind must be refused by name, got %v", err)
	}
	if _, err := load(`{kind: "agents_md_reaches_claude", import_only: true, allow_import: false}`); err == nil || !strings.Contains(err.Error(), "import_only and allow_import: false contradict") {
		t.Fatalf("import_only beside allow_import: false must be refused by name, got %v", err)
	}
}
