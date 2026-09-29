package custodian

// agentsmd_test.go — AG-10 (docs/plans/agents-md.md, ADR-059): the
// agents_md_reaches_claude kind holds when Claude Code will load AGENTS.md
// on the repository's own evidence, and allow_import: false admits no CLAUDE
// file at all (test fixtures excepted).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func agentsMDConcern(allowImport *bool) Concern {
	return Concern{ID: "reaches", Want: "Claude Code reaches AGENTS.md", Scope: ScopeProject,
		Check: Check{Kind: KindAgentsMDReachesClaude, AllowImport: allowImport}}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAgentsMDReachesClaude(t *testing.T) {
	no := false
	cases := []struct {
		name   string
		files  map[string]string
		allow  *bool
		want   Status
		detail string
	}{
		{"no AGENTS.md", map[string]string{"CLAUDE.md": "@AGENTS.md\n"}, nil, StatusViolated, "no AGENTS.md"},
		{"AGENTS.md alone loads natively", map[string]string{"AGENTS.md": "# rules\n"}, nil, StatusHolds, "natively"},
		{"a CLAUDE.md without the import switches it off", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "# rules\n"}, nil, StatusViolated, "CLAUDE.md exists without an @AGENTS.md import"},
		{"a fenced import is prose", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "```\n" + "@AGENTS.md\n```\n"}, nil, StatusViolated, "without an @AGENTS.md import"},
		{"CLAUDE.md imports it", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "# rules\n<!-- fathom:begin v3 sha256:x -->\n" + "@AGENTS.md\n<!-- fathom:end -->\n"}, nil, StatusHolds, "CLAUDE.md imports AGENTS.md"},
		{".claude/CLAUDE.md imports it relatively", map[string]string{"AGENTS.md": "x\n", ".claude/CLAUDE.md": "@../AGENTS.md\n"}, nil, StatusHolds, ".claude/CLAUDE.md imports"},
		{".claude/CLAUDE.md naming .claude/AGENTS.md", map[string]string{"AGENTS.md": "x\n", ".claude/CLAUDE.md": "@AGENTS.md\n"}, nil, StatusViolated, ".claude/CLAUDE.md exists without"},
		{"CLAUDE.local.md counts", map[string]string{"AGENTS.md": "x\n", "CLAUDE.local.md": "mine\n"}, nil, StatusViolated, "CLAUDE.local.md exists without"},
		{"the first present file decides", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n", "CLAUDE.local.md": "mine\n"}, nil, StatusHolds, "CLAUDE.md imports"},
		{"allow_import false refuses an importing CLAUDE.md", map[string]string{"AGENTS.md": "x\n", "CLAUDE.md": "@AGENTS.md\n"}, &no, StatusViolated, "allow_import: false"},
		{"allow_import false names a nested CLAUDE.md", map[string]string{"AGENTS.md": "x\n", "internal/CLAUDE.md": "# nested\n"}, &no, StatusViolated, "internal/CLAUDE.md"},
		{"allow_import false ignores test fixtures", map[string]string{"AGENTS.md": "x\n", "internal/x/testdata/fixture/CLAUDE.md": "input\n"}, &no, StatusHolds, "only instruction file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, c.files)
			f := checkAgentsMDReachesClaude(root, agentsMDConcern(c.allow))
			if f.Status != c.want || !strings.Contains(f.Detail, c.detail) {
				t.Fatalf("got %s %q, want %s containing %q", f.Status, f.Detail, c.want, c.detail)
			}
		})
	}
}

// TestAgentsMDReachesClaudeLoadsAndThePackUsesIt: the kind loads from CUE
// with and without allow_import, and the claude-code pack's stanza entry is
// this kind under its old id (law-13 overrides keep working).
func TestAgentsMDReachesClaudeLoadsAndThePackUsesIt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "concerns.cue")
	src := "concerns: {\n\tone: {\n\t\twant: \"one file\"\n\t\tcheck: {kind: \"agents_md_reaches_claude\", allow_import: false}\n\t}\n\ttwo: {\n\t\twant: \"reaches\"\n\t\tcheck: {kind: \"agents_md_reaches_claude\"}\n\t}\n}\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, err := LoadFile(p, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Concern{}
	for _, c := range cs {
		byID[c.ID] = c
	}
	if a := byID["one"].Check.AllowImport; a == nil || *a {
		t.Errorf("allow_import: false did not load: %+v", byID["one"].Check)
	}
	if byID["two"].Check.AllowImport != nil {
		t.Errorf("an absent allow_import must stay nil: %+v", byID["two"].Check)
	}
	pc, err := PackConcern("claude-code", "claude_code_stanza_present")
	if err != nil {
		t.Fatal(err)
	}
	if pc.Check.Kind != KindAgentsMDReachesClaude {
		t.Errorf("claude-code pack's stanza entry = %s, want %s (ADR-059)", pc.Check.Kind, KindAgentsMDReachesClaude)
	}
}
