package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixtureSkills = []string{"audit", "decide", "design", "doctor", "incident", "init", "prune", "release"}

func put(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	source, root := t.TempDir(), t.TempDir()
	put(t, source, "LICENSE", "license\n")
	put(t, source, "docs/project-intent/README.md", "practice\n")
	put(t, source, "unrelated.md", "never vendor\n")
	for _, name := range fixtureSkills {
		put(t, source, ".agents/skills/intent-"+name+"/SKILL.md", name+"\n")
	}
	return source, root
}

// claudeFixture is a kit checkout in the current layout: the real skills live in
// `.claude/skills` and `.agents/skills` is the one link to them.
func claudeFixture(t *testing.T) (string, string) {
	t.Helper()
	source, root := t.TempDir(), t.TempDir()
	put(t, source, "LICENSE", "license\n")
	put(t, source, "docs/project-intent/README.md", "practice\n")
	for _, name := range fixtureSkills {
		put(t, source, ".claude/skills/intent-"+name+"/SKILL.md", name+"\n")
	}
	if err := os.MkdirAll(filepath.Join(source, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../.claude/skills", filepath.Join(source, ".agents/skills")); err != nil {
		t.Fatal(err)
	}
	return source, root
}

func initial(t *testing.T) (string, string) {
	t.Helper()
	source, root := fixture(t)
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	return source, root
}

func TestSnapshotDeterministicAndOfflineVerifiable(t *testing.T) {
	source, root := initial(t)
	path := filepath.Join(root, ".agentic-kit/manifest.json")
	before, _ := os.ReadFile(path)
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("identical input changed manifest")
	}
	if err := verifySnapshot(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agentic-kit/upstream/unrelated.md")); !os.IsNotExist(err) {
		t.Fatal("copied unrelated source")
	}
	for _, host := range []string{".agents", ".claude"} {
		for _, name := range fixtureSkills {
			got, err := os.ReadFile(filepath.Join(root, host, "skills/intent-"+name+"/SKILL.md"))
			if err != nil || string(got) != name+"\n" {
				t.Fatalf("%s/%s did not resolve: %v", host, name, err)
			}
		}
	}
}

func TestSnapshotPreservesLocalEdits(t *testing.T) {
	source, root := initial(t)
	path := ".agentic-kit/upstream/docs/project-intent/README.md"
	put(t, root, path, "local edit\n")
	if err := verifySnapshot(root); err == nil {
		t.Fatal("tampering passed")
	}
	if err := syncSnapshot(root, source, strings.Repeat("b", 40)); err == nil {
		t.Fatal("overwrote local edit")
	}
	got, _ := os.ReadFile(filepath.Join(root, path))
	if string(got) != "local edit\n" {
		t.Fatal("local edit lost")
	}
}

func TestSnapshotRefusesCollisionBeforeWriting(t *testing.T) {
	source, root := fixture(t)
	put(t, root, ".claude/skills/intent-design/SKILL.md", "existing skill\n")
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err == nil {
		t.Fatal("collision accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".agentic-kit/manifest.json")); !os.IsNotExist(err) {
		t.Fatal("wrote before preflight completed")
	}
}

func TestSnapshotRefusesSymlinkEscapes(t *testing.T) {
	for _, place := range []string{"source", "target"} {
		t.Run(place, func(t *testing.T) {
			source, root := fixture(t)
			path := filepath.Join(source, "docs/project-intent/escape")
			if place == "target" {
				path = filepath.Join(root, ".agentic-kit")
			}
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
			if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err == nil {
				t.Fatal("symlink escape accepted")
			}
		})
	}
}

func TestSnapshotRefusesUnmanagedAndRemovedFiles(t *testing.T) {
	t.Run("unmanaged", func(t *testing.T) {
		source, root := initial(t)
		put(t, root, ".agentic-kit/upstream/extra.md", "local\n")
		if err := verifySnapshot(root); err == nil {
			t.Fatal("unmanaged file passed")
		}
		if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err == nil {
			t.Fatal("unmanaged file ignored")
		}
	})
	t.Run("removed", func(t *testing.T) {
		source, root := fixture(t)
		put(t, source, "docs/project-intent/old.md", "keep\n")
		if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(source, "docs/project-intent/old.md")); err != nil {
			t.Fatal(err)
		}
		if err := syncSnapshot(root, source, strings.Repeat("b", 40)); err == nil {
			t.Fatal("silent deletion accepted")
		}
		if err := verifySnapshot(root); err != nil {
			t.Fatalf("refusal damaged snapshot: %v", err)
		}
	})
}

func TestSnapshotRejectsTraversalAndMissingSkills(t *testing.T) {
	t.Run("manifest", func(t *testing.T) {
		_, root := initial(t)
		path := filepath.Join(root, ".agentic-kit/manifest.json")
		b, _ := os.ReadFile(path)
		b = bytes.ReplaceAll(b, []byte("docs/project-intent/README.md"), []byte("../../outside"))
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
		if err := verifySnapshot(root); err == nil {
			t.Fatal("traversal accepted")
		}
	})
	t.Run("missing skill", func(t *testing.T) {
		source, root := fixture(t)
		if err := os.Remove(filepath.Join(source, ".agents/skills/intent-audit/SKILL.md")); err != nil {
			t.Fatal(err)
		}
		if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err == nil {
			t.Fatal("incomplete install accepted")
		}
	})
}

func TestSnapshotAllowsReviewedAdditiveUpdates(t *testing.T) {
	source, root := initial(t)
	put(t, source, "docs/project-intent/README.md", "updated\n")
	put(t, source, "docs/project-intent/new.md", "new\n")
	if err := syncSnapshot(root, source, strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".agentic-kit/upstream/docs/project-intent/README.md"))
	if string(got) != "updated\n" {
		t.Fatal("update not installed")
	}
}

func manifestOf(t *testing.T, root string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".agentic-kit/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The snapshot layout is an adopter-visible format: the source checkout's own skills
// layout (current, or the legacy one with real `.agents/skills`) must not change it.
func TestSnapshotLayoutIsIndependentOfSourceLayout(t *testing.T) {
	legacySource, legacyRoot := fixture(t)
	currentSource, currentRoot := claudeFixture(t)
	sha := strings.Repeat("a", 40)
	if err := syncSnapshot(legacyRoot, legacySource, sha); err != nil {
		t.Fatal(err)
	}
	if err := syncSnapshot(currentRoot, currentSource, sha); err != nil {
		t.Fatalf("current layout: %v", err)
	}
	if !bytes.Equal(manifestOf(t, legacyRoot), manifestOf(t, currentRoot)) {
		t.Fatal("source layout changed the manifest")
	}
	for _, name := range fixtureSkills {
		got, err := os.ReadFile(filepath.Join(currentRoot, ".agentic-kit/upstream/.agents/skills/intent-"+name+"/SKILL.md"))
		if err != nil || string(got) != name+"\n" {
			t.Fatalf("snapshot lost %s at its stable path: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(currentRoot, ".agentic-kit/upstream/.claude")); !os.IsNotExist(err) {
		t.Fatal("snapshot adopted the source's .claude path")
	}
}

// A kit checkout from before the move keeps real skills in `.agents/skills` and links
// in `.claude/skills`; the links are never a source.
func TestSnapshotReadsLegacyCheckoutWithClaudeLinks(t *testing.T) {
	source, root := fixture(t)
	if err := os.MkdirAll(filepath.Join(source, ".claude/skills"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range fixtureSkills {
		if err := os.Symlink("../../.agents/skills/intent-"+name, filepath.Join(source, ".claude/skills/intent-"+name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(root); err != nil {
		t.Fatal(err)
	}
}

// An adopter that keeps one skills copy has `.agents/skills` as a link to
// `.claude/skills`. The `.claude` links reach both hosts, and nothing is written
// through the link.
func TestSnapshotSingleCopyAdopterWritesNothingThroughTheLink(t *testing.T) {
	source, root := claudeFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ".claude/skills"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../.claude/skills", filepath.Join(root, ".agents/skills")); err != nil {
		t.Fatal(err)
	}
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range fixtureSkills {
		want := "../../.agentic-kit/upstream/.agents/skills/intent-" + name
		got, err := os.Readlink(filepath.Join(root, ".claude/skills/intent-"+name))
		if err != nil || got != want {
			t.Fatalf("claude link for %s = %q, %v", name, got, err)
		}
		body, err := os.ReadFile(filepath.Join(root, ".agents/skills/intent-"+name+"/SKILL.md"))
		if err != nil || string(body) != name+"\n" {
			t.Fatalf(".agents/skills/%s did not resolve through the link: %v", name, err)
		}
	}
	if got, err := os.Readlink(filepath.Join(root, ".agents/skills")); err != nil || got != "../.claude/skills" {
		t.Fatalf("the .agents/skills link changed: %q, %v", got, err)
	}
}

func TestSnapshotRefusesAForeignAgentsSkillsLink(t *testing.T) {
	source, root := claudeFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".agents/skills")); err != nil {
		t.Fatal(err)
	}
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err == nil {
		t.Fatal("competing .agents/skills link accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".agentic-kit/manifest.json")); !os.IsNotExist(err) {
		t.Fatal("wrote before refusing")
	}
}

func TestSnapshotAcceptsLicenseMarkdown(t *testing.T) {
	source, root := fixture(t)
	if err := os.Remove(filepath.Join(source, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	put(t, source, "LICENSE.md", "license\n")
	if err := syncSnapshot(root, source, strings.Repeat("a", 40)); err != nil {
		t.Fatalf("a kit shipping only LICENSE.md must snapshot: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, ".agentic-kit/upstream/LICENSE.md")); err != nil || string(b) != "license\n" {
		t.Fatalf("LICENSE.md not vendored: %v %q", err, b)
	}
	if err := verifySnapshot(root); err != nil {
		t.Fatal(err)
	}
}
