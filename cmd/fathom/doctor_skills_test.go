package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
)

// TestCheckAgentsSkills pins RP-3 (docs/plans/reach-parity.md): the
// skills_installed row reads `.agents/skills` for every non-Claude harness
// and `.claude/skills` for Claude Code, names each missing skill, and
// reports an unverified descriptor's discovery as unverified.
func TestCheckAgentsSkills(t *testing.T) {
	t.Run("codex reads .agents/skills and names the missing", func(t *testing.T) {
		root := t.TempDir()
		if err := installSkills(root, "codex", &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		c := checkAgentsSkills(root, "codex")
		if c.Status != doctorPass || !strings.Contains(c.Reason, ".agents/skills") || !strings.Contains(c.Reason, "unverified") {
			t.Fatalf("codex all current = %+v", c)
		}
		if err := os.RemoveAll(filepath.Join(root, ".agents", "skills", "fathom-impact")); err != nil {
			t.Fatal(err)
		}
		c = checkAgentsSkills(root, "codex")
		if c.Status != doctorFail || !strings.Contains(c.Reason, "fathom-impact") || strings.Contains(c.Reason, "none of the") {
			t.Fatalf("one missing must be NAMED: %+v", c)
		}
		if !strings.Contains(c.Remedy, "--skills=agents") {
			t.Errorf("remedy = %q", c.Remedy)
		}
	})

	t.Run("nothing installed names every skill", func(t *testing.T) {
		c := checkAgentsSkills(t.TempDir(), "gemini-cli")
		if c.Status != doctorFail {
			t.Fatalf("status = %v", c.Status)
		}
		for _, name := range skillsdist.Names {
			if !strings.Contains(c.Reason, name) {
				t.Errorf("reason must name %s: %s", name, c.Reason)
			}
		}
	})

	t.Run("claude reads .claude/skills", func(t *testing.T) {
		root := t.TempDir()
		if err := installSkills(root, "claude", &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		c := checkAgentsSkills(root, "claude")
		if c.Status != doctorPass || !strings.Contains(c.Reason, ".claude/skills") || strings.Contains(c.Reason, "unverified") {
			t.Fatalf("claude = %+v", c)
		}
		// A hand-authored file by the same name is named, never skipped.
		p := filepath.Join(root, ".claude", "skills", "fathom-plan", "SKILL.md")
		if err := os.WriteFile(p, []byte("mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		c = checkAgentsSkills(root, "claude")
		if c.Status != doctorFail || !strings.Contains(c.Reason, "not fathom-owned") || !strings.Contains(c.Reason, p) {
			t.Fatalf("non-owned = %+v", c)
		}
	})

	t.Run("agents compares the one real copy through the link", func(t *testing.T) {
		root := t.TempDir()
		if err := installSkills(root, "agents", &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, ".agents", "skills", "fathom-verify", "SKILL.md")
		raw, _ := os.ReadFile(p)
		if err := os.WriteFile(p, append(raw, []byte("stale edit\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		// ADR-059: .agents/skills is a link, so the edit landed in the one
		// real copy, and the row names that file.
		real := filepath.Join(root, ".claude", "skills", "fathom-verify", "SKILL.md")
		c := checkAgentsSkills(root, "agents")
		if c.Status != doctorFail || !strings.Contains(c.Reason, "stale") || !strings.Contains(c.Reason, real) {
			t.Fatalf("stale = %+v", c)
		}
	})
}
