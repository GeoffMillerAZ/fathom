package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// GDEV-41: declared adaptations belong to the consumer, including when a
// shipped version moves on. Unmarked collisions retain their refusal.
func TestDeclaredSkillAdaptations(t *testing.T) {
	for _, tc := range []struct {
		name, marker string
		status       doctorStatus
		adapted      bool
	}{
		{"current", fmt.Sprintf("<!-- fathom:adapted from=fathom-plan version=%s -->", pluginVersion), doctorPass, true},
		{"build metadata", fmt.Sprintf("<!-- fathom:adapted from=fathom-plan version=%s+local -->", pluginVersion), doctorPass, true},
		{"newer", "<!-- fathom:adapted from=fathom-plan version=999.0.0 -->", doctorWarn, true},
		{"inline", fmt.Sprintf("example <!-- fathom:adapted from=fathom-plan version=%s -->", pluginVersion), doctorFail, false},
		{"suffix", fmt.Sprintf("<!-- fathom:adapted from=fathom-plan version=%s --> example", pluginVersion), doctorFail, false},
		{"older", "<!-- fathom:adapted from=fathom-plan version=0.0.1 -->", doctorWarn, true},
		{"owned and adapted", fmt.Sprintf("<!-- fathom:owned -->\n<!-- fathom:adapted from=fathom-plan version=%s -->", pluginVersion), doctorPass, true},
		{"unmarked", "", doctorFail, false},
		{"wrong skill", fmt.Sprintf("<!-- fathom:adapted from=fathom-verify version=%s -->", pluginVersion), doctorFail, false},
		{"incomplete", "<!-- fathom:adapted from=fathom-plan -->", doctorFail, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := installSkills(nil, root, "all", &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			rel := skillRelPath(skillsTargetClaude, "fathom-plan")
			dest := filepath.Join(root, filepath.FromSlash(rel))
			body := []byte(tc.marker + "\n# Consumer adaptation\nKeep this instruction.\n")
			if err := os.WriteFile(dest, body, 0o644); err != nil {
				t.Fatal(err)
			}
			for _, harness := range []string{"claude", "codex", "agents"} {
				c := checkAgentsSkills(root, harness)
				if c.Status != tc.status {
					t.Errorf("doctor %s = %+v, want %s", harness, c, tc.status)
				}
				if tc.adapted && (!strings.Contains(c.Reason, "adapted (shipped v"+pluginVersion+", adapted from v") || !strings.Contains(c.Reason, "fathom-plan")) {
					t.Errorf("doctor does not describe adaptation: %+v", c)
				}
				if tc.status == doctorWarn && (!strings.Contains(c.Remedy, "diff -u") || !strings.Contains(c.Remedy, "fathom plugin render")) {
					t.Errorf("old adaptation lacks comparison command: %+v", c)
				}
			}
			var out bytes.Buffer
			err := installSkills(nil, root, "all", &out)
			if (err == nil) != tc.adapted {
				t.Errorf("init error = %v, adapted = %v", err, tc.adapted)
			}
			plan, err := wiring.PlanWiring(root, wiring.Spec{Files: skillOwnedFiles(root), Links: skillLinks(root)})
			if err != nil {
				t.Fatal(err)
			}
			blocked := false
			for _, it := range plan.Items {
				if it.File == rel && it.Verdict == wiring.VerdictBlocked {
					blocked = true
				}
			}
			if blocked == tc.adapted {
				t.Errorf("reconcile blocked = %v, adapted = %v", blocked, tc.adapted)
			}
			if tc.adapted {
				if _, err := wiring.ApplyWiring(root, wiring.Spec{Files: skillOwnedFiles(root), Links: skillLinks(root)}, wiring.ApplyOptions{}); err != nil {
					t.Fatal(err)
				}
				for _, f := range ownedSkillFiles(root, "") {
					if f.Path == dest {
						t.Error("adaptation still treated as an owned drift baseline")
					}
				}
			}
			if got, err := os.ReadFile(dest); err != nil || !bytes.Equal(got, body) {
				t.Errorf("consumer file changed: %q, %v", got, err)
			}
		})
	}
}

func TestAdaptedSkillsDirectoryWithoutBaseline(t *testing.T) {
	for _, via := range []string{"init", "reconcile preview", "reconcile apply"} {
		t.Run(via, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("FATHOM_HOME", t.TempDir())
			t.Setenv("FATHOM_STATE_DIR", t.TempDir())
			rel := ".agents/skills/fathom-plan/SKILL.md"
			body := fmt.Sprintf("<!-- fathom:owned -->\n<!-- fathom:adapted from=fathom-plan version=%s -->\n# Keep this adaptation\n", pluginVersion)
			writeRepoFile(t, root, rel, body)
			l, err := wiring.LoadLedger(root)
			if err != nil || len(l.Files) != 0 {
				t.Fatalf("fixture has ledger baseline: %+v, %v", l, err)
			}
			var out bytes.Buffer
			if via == "init" {
				if err := installSkills(nil, root, "all", &out); err != nil {
					t.Fatal(err)
				}
			} else {
				args := []string{"--repo", root, "--url", "://unavailable"}
				if via == "reconcile apply" {
					args = append(args, "--apply")
				}
				var errOut bytes.Buffer
				if code := cmdFleetReconcile(args, &out, &errOut); code != 0 {
					t.Fatalf("reconcile = %d: %s\n%s", code, out.String(), errOut.String())
				}
			}

			if got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err != nil || string(got) != body {
				t.Errorf("adaptation changed: %q, %v", got, err)
			}
			if fi, err := os.Lstat(filepath.Join(root, ".agents", "skills")); err != nil || !fi.IsDir() {
				t.Errorf("adapted directory replaced: %v, %v", fi, err)
			}
			for _, want := range []string{"skip", rel, "diff -u", "fathom plugin render"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		})
	}
}

func TestRenderedSkillsNeverDeclareAdaptation(t *testing.T) {
	for _, target := range []string{skillsTargetClaude, skillsTargetAgents, skillsTargetCodexPrompts} {
		for _, name := range skillsdist.Names {
			body, err := renderSkill(target, name)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(body, []byte("<!-- fathom:adapted")) {
				t.Errorf("shipped %s/%s declares an adaptation", target, name)
			}
		}
	}
}
