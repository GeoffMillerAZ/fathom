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

// isolateDoctorHome also moves fixtures out of inherited TMPDIR/GOTMPDIR:
// they may sit below the real home, whose CLAUDE files ancestorClaudeFiles
// still reads after HOME changes. Child tests inherit this test-owned root.
func isolateDoctorHome(t *testing.T) string {
	t.Helper()
	t.Setenv("TMPDIR", "")
	t.Setenv("GOTMPDIR", "")
	home := t.TempDir()
	t.Setenv("TMPDIR", home)
	t.Setenv("GOTMPDIR", home)
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}

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
	isolateDoctorHome(t)
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
	isolateDoctorHome(t)
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
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

// TestStanzaLoadedSkipNeverClaimsALoadStanzaPresentDenies pins the wording
// pair: where stanza_present WARNs that Claude Code will not load AGENTS.md
// (an ancestor CLAUDE file, the claude-md setting, a release before 2.1.277),
// stanza_loaded's SKIP for an AGENTS.md-only repository names that warning and
// its reasons; it never says Claude Code loads the file. Where nothing warns,
// the named agents-md SKIP is unchanged.
func TestStanzaLoadedSkipNeverClaimsALoadStanzaPresentDenies(t *testing.T) {
	isolateDoctorHome(t)
	cases := []struct {
		name  string
		setup func(t *testing.T) (root string)
		want  string // a fragment of the reason stanza_present gives, which the skip must repeat
	}{
		{"an ancestor CLAUDE.md", func(t *testing.T) string {
			parent, root := agentsRepo(t)
			writeRepoFile(t, parent, "CLAUDE.md", "workspace rules\n")
			withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
			return root
		}, "above the repository switches AGENTS.md off"},
		{"the claude-md setting", func(t *testing.T) string {
			home := t.TempDir()
			writeRepoFile(t, home, ".claude/settings.json", `{"pluginConfigs":{"agents-md@builtin":{"options":{"instructionFiles":"claude-md"}}}}`)
			withClaudeLoadEnv(t, home, "", "2.1.284")
			_, root := agentsRepo(t)
			return root
		}, `so Claude Code does not load AGENTS.md`},
		{"a Claude Code before 2.1.277", func(t *testing.T) string {
			withClaudeLoadEnv(t, t.TempDir(), "", "2.1.270")
			_, root := agentsRepo(t)
			return root
		}, "before 2.1.277"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
			root := tc.setup(t)
			present := checkStanzaPresent(root)
			if present.Status != doctorWarn || !strings.Contains(present.Reason, tc.want) {
				t.Fatalf("stanza_present = %+v, want a WARN carrying %q", present, tc.want)
			}
			c := checkStanzaLoaded(root)
			if c.Status != doctorSkipped {
				t.Fatalf("stanza_loaded = %+v, want SKIP", c)
			}
			if c.Reason == stanzaLoadedNoClaudeReason || strings.Contains(c.Reason, "loads AGENTS.md") {
				t.Errorf("stanza_loaded claims a load stanza_present denies: %q", c.Reason)
			}
			for _, need := range []string{"stanza_present warns Claude Code will not load AGENTS.md", tc.want} {
				if !strings.Contains(c.Reason, need) {
					t.Errorf("stanza_loaded reason %q lacks %q", c.Reason, need)
				}
			}
			if c.Remedy == "" {
				t.Error("stanza_loaded names the warning but no remedy")
			}
		})
	}
	t.Run("nothing warns keeps the named agents-md skip", func(t *testing.T) {
		t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
		withClaudeLoadEnv(t, t.TempDir(), "", "2.1.284")
		_, root := agentsRepo(t)
		if p := checkStanzaPresent(root); p.Status != doctorPass {
			t.Fatalf("stanza_present = %+v, want PASS", p)
		}
		if c := checkStanzaLoaded(root); c.Status != doctorSkipped || c.Reason != stanzaLoadedNoClaudeReason {
			t.Fatalf("stanza_loaded = %+v, want the named agents-md SKIP", c)
		}
	})
}

// TestInstructionFilesSettingReadsBothPluginIDs pins AG-27: Claude Code
// 2.1.277 through 2.1.284 named the plugin agents-md@builtin, and 2.1.285
// renamed it cc-plugin-agents-md@builtin. Doctor reads both keys, the new id
// first, names the key it read, and names a disagreement between the two.
func TestInstructionFilesSettingReadsBothPluginIDs(t *testing.T) {
	isolateDoctorHome(t)
	setting := func(pairs ...string) string {
		var b strings.Builder
		b.WriteString(`{"pluginConfigs":{`)
		for i := 0; i+1 < len(pairs); i += 2 {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`"` + pairs[i] + `":{"options":{"instructionFiles":"` + pairs[i+1] + `"}}`)
		}
		b.WriteString(`}}`)
		return b.String()
	}
	cases := []struct {
		name     string
		settings string
		want     doctorStatus
		has      []string
	}{
		{"only the new id at claude-md warns", setting("cc-plugin-agents-md@builtin", "claude-md"), doctorWarn,
			[]string{`pluginConfigs["cc-plugin-agents-md@builtin"]`, `"claude-md"`}},
		{"only the old id at claude-md warns", setting("agents-md@builtin", "claude-md"), doctorWarn,
			[]string{`pluginConfigs["agents-md@builtin"]`, `"claude-md"`}},
		{"neither passes", `{"pluginConfigs":{}}`, doctorPass, []string{"native (no CLAUDE file, default mode)"}},
		{"the two disagree: the new id is read and the conflict named", setting("cc-plugin-agents-md@builtin", "claude-md", "agents-md@builtin", "claude-md-or-agents-md"), doctorWarn,
			[]string{`pluginConfigs["cc-plugin-agents-md@builtin"]`, "disagree", `"claude-md-or-agents-md"`}},
		{"a disagreement is named on a pass too", setting("cc-plugin-agents-md@builtin", "claude-md-or-agents-md", "agents-md@builtin", "claude-md"), doctorPass,
			[]string{"disagree", "2.1.285"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			writeRepoFile(t, home, ".claude/settings.json", tc.settings)
			withClaudeLoadEnv(t, home, "", "2.1.290")
			_, root := agentsRepo(t)
			c := checkStanzaPresent(root)
			if c.Status != tc.want {
				t.Fatalf("status = %s, want %s: %+v", c.Status, tc.want, c)
			}
			for _, h := range tc.has {
				if !strings.Contains(c.Reason, h) {
					t.Errorf("reason %q lacks %q", c.Reason, h)
				}
			}
		})
	}
}

// TestNoMachineRemedyPrescribesAClaudeImport pins AG-28 (K12, 2026-10-09):
// content lives only in AGENTS.md, so where AGENTS.md exists no remedy for a
// machine-side reason (an ancestor CLAUDE file, the instructionFiles
// setting, an old Claude Code) tells the reader to create a root CLAUDE.md
// or add an @AGENTS.md import. Each names only the native path.
func TestNoMachineRemedyPrescribesAClaudeImport(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string
		want  string // the native path the remedy names
	}{
		{"an ancestor CLAUDE.md", func(t *testing.T) string {
			parent, root := agentsRepo(t)
			writeRepoFile(t, parent, "CLAUDE.md", "workspace rules\n")
			withClaudeLoadEnv(t, t.TempDir(), "", "2.1.290")
			return root
		}, "move "},
		{"the claude-md setting", func(t *testing.T) string {
			home := t.TempDir()
			writeRepoFile(t, home, ".claude/settings.json", `{"pluginConfigs":{"cc-plugin-agents-md@builtin":{"options":{"instructionFiles":"claude-md"}}}}`)
			withClaudeLoadEnv(t, home, "", "2.1.290")
			_, root := agentsRepo(t)
			return root
		}, "set instructionFiles"},
		{"a Claude Code before 2.1.277", func(t *testing.T) string {
			withClaudeLoadEnv(t, t.TempDir(), "", "2.1.270")
			_, root := agentsRepo(t)
			return root
		}, "upgrade Claude Code"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
			root := tc.setup(t)
			reasons, remedies := claudeLoadPath(root, true).warnings()
			if len(reasons) == 0 || len(remedies) == 0 {
				t.Fatalf("no warning for the case: %v / %v", reasons, remedies)
			}
			for _, c := range []doctorCheck{checkStanzaPresent(root), checkStanzaLoaded(root)} {
				remedies = append(remedies, c.Remedy)
			}
			joined := strings.Join(remedies, " | ")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("remedies %q lack the native path %q", joined, tc.want)
			}
			for _, bad := range []string{"@AGENTS.md", "import", "root CLAUDE.md", "fathom init"} {
				if strings.Contains(joined, bad) {
					t.Errorf("a remedy prescribes a CLAUDE import (%q): %q", bad, joined)
				}
			}
		})
	}
}
