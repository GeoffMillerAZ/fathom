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
// renders them through the wiring engine (RP-4, docs/plans/reach-parity.md):
// the one real copy in .claude/skills (ADR-059), receipted, a hand-authored
// copy BLOCKED by name.
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
	if !strings.Contains(stdout.String(), "shipped skill file(s)") {
		t.Errorf("dry-run must name the missing-skills finding, got:\n%s", stdout.String())
	}

	stdout.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit %d: %s\n%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "fathom-impact", "SKILL.md")); err != nil {
		t.Errorf("apply must install the shipped skills: %v", err)
	}
	// ADR-059: reconcile never creates the .agents/skills link — which
	// harnesses read it is init's --skills choice — and never writes a file
	// there.
	if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
		t.Errorf("apply created .agents (err=%v); only init --skills chooses the link", err)
	}
	if !strings.Contains(stdout.String(), "receipt:") {
		t.Errorf("skill writes must be receipted, got:\n%s", stdout.String())
	}
	if d := analyzeWiring(root); len(d.MissingSkills) != 0 || len(d.StaleSkills) != 0 || len(d.BlockedSkills) != 0 {
		t.Errorf("second analyze must report no skill drift, got %+v", d)
	}

	// Law 15, both directions: a hand edit UNDER the marker of a file the
	// engine recorded is drift, and a hand-authored copy by a skill's name
	// (no marker) is unproven — both BLOCKED by name, left byte-identical.
	editedPath := filepath.Join(root, ".claude", "skills", "fathom-plan", "SKILL.md")
	raw, _ := os.ReadFile(editedPath)
	edited := append(append([]byte(nil), raw...), []byte("my note under the marker\n")...)
	if err := os.WriteFile(editedPath, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	minePath := filepath.Join(root, ".claude", "skills", "fathom-verify", "SKILL.md")
	mine := []byte("---\nname: fathom-verify\ndescription: mine\n---\nmine\n")
	if err := os.WriteFile(minePath, mine, 0o644); err != nil {
		t.Fatal(err)
	}
	d = analyzeWiring(root)
	if len(d.StaleSkills) != 0 || len(d.BlockedSkills) != 2 ||
		!strings.Contains(d.BlockedSkills[0], ".claude/skills/fathom-plan/SKILL.md was edited by hand") ||
		!strings.Contains(d.BlockedSkills[1], ".claude/skills/fathom-verify/SKILL.md exists without fathom's marker") {
		t.Fatalf("drift buckets = %+v", d)
	}
	stdout.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 1 {
		t.Fatalf("apply with a blocked skill must exit 1, got %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "BLOCKED: .claude/skills/fathom-verify/SKILL.md") || !strings.Contains(stdout.String(), "BLOCKED: .claude/skills/fathom-plan/SKILL.md") {
		t.Errorf("both must be BLOCKED by name:\n%s", stdout.String())
	}
	if got, _ := os.ReadFile(minePath); string(got) != string(mine) {
		t.Errorf("the hand-authored copy was touched")
	}
	if got, _ := os.ReadFile(editedPath); string(got) != string(edited) {
		t.Errorf("the hand-edited owned copy was touched")
	}

	// A pre-engine render (installSkills wrote it; no ledger entry) that is
	// behind the shipped source is fathom's by its marker: stale, rewritten.
	root2 := t.TempDir()
	if err := installSkills(nil, root2, "agents", &stdout); err != nil {
		t.Fatal(err)
	}
	// Before FA-23 installSkills recorded no baseline: drop the ledger it
	// now writes to stand in for a render of that era.
	if err := os.Remove(filepath.Join(root2, ".fathom", "owned.json")); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(root2, ".claude", "skills", "fathom-impact", "SKILL.md")
	current, _ := os.ReadFile(stalePath)
	if err := os.WriteFile(stalePath, append([]byte("stale render\n"), current...), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := analyzeWiring(root2); len(d.StaleSkills) != 1 || d.StaleSkills[0] != ".claude/skills/fathom-impact/SKILL.md" {
		t.Fatalf("stale bucket = %+v", d)
	}
	stdout.Reset()
	if code := cmdFleetReconcile([]string{"--apply", "--repo", root2}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit %d: %s", code, stdout.String())
	}
	if got, _ := os.ReadFile(stalePath); string(got) != string(current) {
		t.Errorf("the stale owned copy was not rewritten")
	}
	// J6/O9 (app-kit dogfood): reconciling a REFRESHED (previously stale)
	// owned skill must record the digest of the bytes it actually wrote,
	// not a pre-refresh one — the very next drift check must read clean
	// immediately, never a false "17 stale digest warnings" the instant
	// after a successful --apply. This is the exact repro the observation
	// names: `fleet reconcile --apply` then `doctor` back to back.
	if d := analyzeWiring(root2); len(d.StaleSkills) != 0 || len(d.BlockedSkills) != 0 {
		t.Fatalf("a freshly-reconciled skill file must show no drift immediately: %+v", d)
	}
}
