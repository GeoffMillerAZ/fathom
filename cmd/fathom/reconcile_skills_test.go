package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReconcileSkills pins FR-3 (docs/plans/exploration-ledger.md): a repo
// missing shipped skills is a wiring finding — the Unknown-skill session
// failures came from repos wired before a skill existed — and --apply
// installs them through the same installSkills machinery init uses.
func TestReconcileSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	d := analyzeWiring(root)
	if len(d.MissingSkills) == 0 {
		t.Fatal("a repo with no .claude/skills must report missing shipped skills")
	}

	var stdout, stderr strings.Builder
	if code := cmdFleetReconcile([]string{"--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("dry-run exit %d: %s\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "shipped skill(s) for claude") {
		t.Errorf("dry-run must name the missing-skills finding, got:\n%s", stdout.String())
	}

	stdout.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit %d: %s\n%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "fathom-impact", "SKILL.md")); err != nil {
		t.Errorf("apply must install the shipped skills: %v", err)
	}
	if d := analyzeWiring(root); len(d.MissingSkills) != 0 {
		t.Errorf("second analyze must report no missing skills, got %v", d.MissingSkills)
	}
}
