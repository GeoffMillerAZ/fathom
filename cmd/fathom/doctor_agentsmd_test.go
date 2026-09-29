package main

// doctor_agentsmd_test.go — AG-11 and AG-12 (docs/plans/agents-md.md,
// ADR-059): stanza_present FAILs on the AGENTS.md region and WARNs, with the
// remedy, whenever Claude Code will not load AGENTS.md; stanza_loaded SKIPs
// by name where no root CLAUDE file exists.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withClaudeLoadEnv points claudeLoadEnv at a fake home, managed settings
// file and claude version for one test.
func withClaudeLoadEnv(t *testing.T, home, managed, version string) {
	t.Helper()
	saved := claudeLoadEnv
	t.Cleanup(func() { claudeLoadEnv = saved })
	claudeLoadEnv.home = func() (string, error) { return home, nil }
	claudeLoadEnv.managedSettings = func() []string {
		if managed == "" {
			return nil
		}
		return []string{managed}
	}
	claudeLoadEnv.version = func(context.Context) (string, error) {
		if version == "" {
			return "", errors.New("no claude here")
		}
		return version, nil
	}
}

// agentsRepo makes a repository whose AGENTS.md carries the stanza, under a
// parent directory the test controls (so no stray ancestor CLAUDE file).
func agentsRepo(t *testing.T) (parent, root string) {
	t.Helper()
	parent = t.TempDir()
	root = filepath.Join(parent, "repo")
	writeRepoFile(t, root, "AGENTS.md", "# rules\n\n"+fathomStanza("svc", false)+"\n")
	return parent, root
}

func TestCheckStanzaPresentAgentsMD(t *testing.T) {
	t.Run("no AGENTS.md fails", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		root := t.TempDir()
		writeRepoFile(t, root, "CLAUDE.md", fathomStanza("svc", false)+"\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorFail || !strings.Contains(c.Reason, "AGENTS.md") {
			t.Fatalf("got %+v, want FAIL naming AGENTS.md", c)
		}
	})
	t.Run("corrupted AGENTS.md region fails", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		root := t.TempDir()
		writeRepoFile(t, root, "AGENTS.md", fathomStanzaBegin+"\nno end marker\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorFail || !strings.Contains(c.Reason, "corrupted") {
			t.Fatalf("got %+v, want FAIL naming the corrupted pair", c)
		}
	})
	t.Run("native passes and names the path", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		_, root := agentsRepo(t)
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "native (no CLAUDE file, default mode)") || !strings.Contains(c.Reason, "2.1.284") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("an unresolvable claude is named, never guessed", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "")
		_, root := agentsRepo(t)
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "claude version unknown") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("imported by CLAUDE.md passes", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.200")
		_, root := agentsRepo(t)
		writeRepoFile(t, root, "CLAUDE.md", "# house\n\n"+"@AGENTS.md\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "imported by CLAUDE.md") {
			t.Fatalf("got %+v (an import loads AGENTS.md on any release)", c)
		}
	})
	t.Run("an import-only CLAUDE.md says it can go", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		_, root := agentsRepo(t)
		writeRepoFile(t, root, "CLAUDE.md", fathomStanzaBegin+"\n"+"@AGENTS.md\n"+fathomStanzaEnd+"\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "deleting it lets Claude Code read AGENTS.md natively") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("a CLAUDE.md linked to AGENTS.md passes and names the link", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.200")
		_, root := agentsRepo(t)
		if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
			t.Fatal(err)
		}
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "CLAUDE.md is AGENTS.md itself") {
			t.Fatalf("got %+v (a linked CLAUDE.md carries the stanza on any release; it is not a legacy copy)", c)
		}
	})
	t.Run("an AGENTS.md region holding an import, not the stanza, fails", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		root := t.TempDir()
		writeRepoFile(t, root, "AGENTS.md", "# rules\n\n"+fathomStanzaBegin+"\n"+"@AGENTS.md\n"+fathomStanzaEnd+"\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorFail || !strings.Contains(c.Reason, "no stanza") {
			t.Fatalf("got %+v, want FAIL: the region is well-formed but carries no stanza", c)
		}
	})
	t.Run("a CLAUDE.md without the import warns", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		_, root := agentsRepo(t)
		writeRepoFile(t, root, "CLAUDE.md", "# house rules\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, "CLAUDE.md exists without an @AGENTS.md import") || !strings.Contains(c.Remedy, "fathom init --path") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("a legacy stanza copy warns", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		_, root := agentsRepo(t)
		writeRepoFile(t, root, "CLAUDE.md", "@AGENTS.md\n\n"+fathomStanza("svc", false)+"\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, "CLAUDE.md still carries the fathom stanza") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("an ancestor CLAUDE.md warns; the user memory file does not", func(t *testing.T) {
		parent, root := agentsRepo(t)
		home := parent
		writeRepoFile(t, home, ".claude/CLAUDE.md", "user memory\n")
		withClaudeLoadEnv(t, home, "", "2.1.284")
		if c := checkStanzaPresent(root); c.Status != doctorPass {
			t.Fatalf("~/.claude/CLAUDE.md counted as an ancestor: %+v", c)
		}
		writeRepoFile(t, parent, "CLAUDE.md", "workspace rules\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, filepath.Join(parent, "CLAUDE.md")) || !strings.Contains(c.Remedy, "move") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("the claude-md setting warns, managed first", func(t *testing.T) {
		home := t.TempDir()
		writeRepoFile(t, home, ".claude/settings.json", `{"pluginConfigs":{"agents-md@builtin":{"options":{"instructionFiles":"claude-md-or-agents-md"}}}}`)
		managed := filepath.Join(t.TempDir(), "managed-settings.json")
		writeRepoFile(t, filepath.Dir(managed), filepath.Base(managed), `{"pluginConfigs":{"agents-md@builtin":{"options":{"instructionFiles":"claude-md"}}}}`)
		withClaudeLoadEnv(t, home, managed, "2.1.284")
		_, root := agentsRepo(t)
		c := checkStanzaPresent(root)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, `"claude-md"`) || !strings.Contains(c.Reason, managed) || !strings.Contains(c.Remedy, "never writes it") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("mode claude-md-and-agents-md passes by name", func(t *testing.T) {
		home := t.TempDir()
		writeRepoFile(t, home, ".claude/settings.json", `{"pluginConfigs":{"agents-md@builtin":{"options":{"instructionFiles":"claude-md-and-agents-md"}}}}`)
		withClaudeLoadEnv(t, home, "", "2.1.284")
		_, root := agentsRepo(t)
		writeRepoFile(t, root, "CLAUDE.md", "# house rules, no import\n")
		c := checkStanzaPresent(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "mode claude-md-and-agents-md") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("repo-only never spawns claude", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "")
		claudeLoadEnv.version = func(context.Context) (string, error) {
			t.Error("doctor --repo-only asked the claude binary for its version")
			return "", errors.New("unexpected")
		}
		_, root := agentsRepo(t)
		c := checkStanzaPresentRepoOnly(root)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "not probed") {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("a Claude Code before 2.1.277 warns", func(t *testing.T) {
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.270")
		_, root := agentsRepo(t)
		c := checkStanzaPresent(root)
		if c.Status != doctorWarn || !strings.Contains(c.Reason, "2.1.270, before 2.1.277") {
			t.Fatalf("got %+v", c)
		}
	})
}

// TestStanzaLoadedSkipsByNameWithNoClaudeFile pins AG-12's stanza_loaded
// half: no root CLAUDE file and no beat is a SKIP naming why no evidence can
// exist; a beat for AGENTS.md (arriving through a CLAUDE.md import) passes.
func TestStanzaLoadedSkipsByNameWithNoClaudeFile(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	_, root := agentsRepo(t)
	c := checkStanzaLoaded(root)
	if c.Status != doctorSkipped || c.Reason != stanzaLoadedNoClaudeReason {
		t.Fatalf("no CLAUDE file = %+v, want the named SKIP", c)
	}

	withClaude := t.TempDir()
	writeRepoFile(t, withClaude, "CLAUDE.md", "@AGENTS.md\n")
	if c := checkStanzaLoaded(withClaude); c.Status != doctorSkipped || c.Reason == stanzaLoadedNoClaudeReason {
		t.Fatalf("a CLAUDE file with no beat = %+v, want the ordinary SKIP", c)
	}
	line := `{"channel":"instructions-loaded","kind":"loaded","at":"2026-09-29T10:00:00Z","meta":{"file_path":"` + filepath.Join(withClaude, "AGENTS.md") + `","load_reason":"include","memory_type":"Project"}}`
	if err := os.WriteFile(beatsPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := checkStanzaLoaded(withClaude); c.Status != doctorPass || !strings.Contains(c.Reason, "AGENTS.md loaded by a session") {
		t.Fatalf("an AGENTS.md beat through the import = %+v, want PASS", c)
	}
}
