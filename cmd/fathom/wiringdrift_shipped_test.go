package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
)

// Every owned skill file carries the bytes `fathom init --skills` would
// write for its target, so wiring_current reads a checkout that holds the
// shipped skill as current even when .fathom/owned.json (per checkout)
// recorded an older release.
func TestOwnedSkillFiles_CarryShippedRendering(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	name := skillsdist.Names[0]
	targets := map[string]string{
		filepath.Join(root, ".claude", "skills", name, "SKILL.md"): skillsTargetClaude,
		filepath.Join(root, ".agents", "skills", name, "SKILL.md"): skillsTargetAgents,
		filepath.Join(home, ".codex", "prompts", name+".md"):       skillsTargetCodexPrompts,
	}
	for path, target := range targets {
		content, err := renderSkill(target, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := ownedSkillFiles(root, home)
	if len(files) != 3 {
		t.Fatalf("owned skill files = %d, want 3: %+v", len(files), files)
	}
	for _, f := range files {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, f.Shipped) {
			t.Errorf("%s: Shipped is not what init --skills writes for %s", f.Label, targets[f.Path])
		}
	}
}
