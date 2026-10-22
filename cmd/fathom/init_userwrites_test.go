package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/statedir"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// GDEV-43: user configuration is preserved; operational state is named separately.
func TestInitNoUserWrites(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"harness none", []string{"--harness", "none", "--skills"}},
		{"explicit no user writes", []string{"--no-user-writes", "--skills"}},
		{"explicit beats user flags", []string{"--no-user-writes", "--seed-user-ignore", "--skills=codex-prompts"}},
		{"explicit beats none override", []string{"--harness", "none", "--no-user-writes", "--seed-user-ignore", "--skills"}},
	} {
		for _, existing := range []bool{false, true} {
			name := tc.name + "/empty"
			if existing {
				name = tc.name + "/existing"
			}
			t.Run(name, func(t *testing.T) {
				home, root := t.TempDir(), t.TempDir()
				t.Setenv("HOME", home)
				for _, key := range []string{"XDG_STATE_HOME", "FATHOM_STATE_DIR"} {
					t.Setenv(key, "")
					if err := os.Unsetenv(key); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
				if existing {
					for _, rel := range []string{".fathom/config.toml", ".claude/settings.json", ".codex/config.toml", ".local/bin/fathom"} {
						p := filepath.Join(home, filepath.FromSlash(rel))
						if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(p, []byte("# consumer settings\n"), 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
				before := snapshotTree(t, home)
				args := []string{"--path", root, "--service", "fixture", "--url", "://unavailable", "--no-scip"}
				args = append(args, tc.args...)
				var out, errOut bytes.Buffer
				if code := cmdInit(args, &out, &errOut); code != 2 {
					t.Fatalf("init = %d (want unconfirmed index 2): %s", code, errOut.String())
				}
				state := statedir.Resolve(root)
				after := snapshotTree(t, home)
				for rel := range after {
					abs := rel
					if abs == state || strings.HasPrefix(abs, state+string(filepath.Separator)) {
						delete(after, rel)
					}
				}
				// snapshotTree includes directories. Only state ancestors may be new.
				for dir := filepath.Dir(state); dir != home && strings.HasPrefix(dir, home+string(filepath.Separator)); dir = filepath.Dir(dir) {
					if _, existed := before[dir]; !existed {
						delete(after, dir)
					}
				}
				if !reflect.DeepEqual(before, after) {
					t.Errorf("user configuration changed: before %v, after %v", before, after)
				}
				for _, path := range []string{wiring.LedgerPath(root), wiring.ReceiptsPath(root)} {
					if _, err := os.Stat(path); err != nil {
						t.Errorf("missing operational state %s: %v", path, err)
					}
				}
				if !strings.Contains(out.String(), state) || !strings.Contains(out.String(), "FATHOM_STATE_DIR") || !strings.Contains(out.String(), "XDG_STATE_HOME") {
					t.Errorf("state directory absent from preview: %s", out.String())
				}
				cause := "--no-user-writes"
				if !strings.Contains(strings.Join(tc.args, " "), cause) {
					cause = "--harness none"
				}
				if !strings.Contains(out.String(), "user configuration writes disabled ("+cause+")") {
					t.Errorf("wrong preview cause: %s", out.String())
				}
				if strings.Contains(out.String(), "baselines for") && !strings.Contains(out.String(), "in "+wiring.LedgerPath(root)) {
					t.Errorf("wrong baseline path: %s", out.String())
				}
				manualConfig := strings.Contains(out.String(), "MANUAL: pick a provider")
				if manualConfig != existing {
					t.Errorf("config manual step = %v, existing = %v", manualConfig, existing)
				}
				if !strings.Contains(out.String(), "would ") {
					t.Errorf("missing skipped-write preview: %s", out.String())
				}
				if !strings.Contains(strings.Join(tc.args, " "), "codex-prompts") {
					if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "fathom-plan", "SKILL.md")); err != nil {
						t.Errorf("project skills not staged: %v", err)
					}
				}
			})
		}
	}
}

func TestInitHarnessNoneExplicitUserScope(t *testing.T) {
	for _, flag := range []string{"--seed-user-ignore", "--skills=codex-prompts"} {
		t.Run(flag, func(t *testing.T) {
			home, root := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
			t.Setenv("FATHOM_STATE_DIR", t.TempDir())
			var out, errOut bytes.Buffer
			if code := cmdInit([]string{"--harness", "none", flag, "--path", root, "--service", "fixture", "--url", "://unavailable", "--no-scip"}, &out, &errOut); code != 2 {
				t.Fatalf("init = %d: %s", code, errOut.String())
			}
			for _, rel := range []string{".fathom/config.toml", ".local/bin/fathom", ".claude/settings.json", ".codex/config.toml"} {
				if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(rel))); !os.IsNotExist(err) {
					t.Errorf("unrequested user configuration %s: %v", rel, err)
				}
			}
			ignore := filepath.Join(home, ".fathom", "ignore")
			prompt := filepath.Join(home, ".codex", "prompts", "fathom-plan.md")
			for _, check := range []struct {
				path string
				want bool
			}{{ignore, flag == "--seed-user-ignore"}, {prompt, flag == "--skills=codex-prompts"}} {
				_, err := os.Stat(check.path)
				if (err == nil) != check.want {
					t.Errorf("requested write %s: exists=%v, want=%v", check.path, err == nil, check.want)
				}
			}
		})
	}
}

func TestInitSkippedUserScopeJSON(t *testing.T) {
	for _, flag := range []string{"--no-user-writes", "--harness=none"} {
		t.Run(flag, func(t *testing.T) {
			home, root := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
			t.Setenv("FATHOM_STATE_DIR", t.TempDir())
			var out, errOut bytes.Buffer
			if code := cmdInit([]string{flag, "--json", "--path", root, "--service", "fixture", "--url", "://unavailable", "--no-scip"}, &out, &errOut); code != 2 {
				t.Fatalf("init = %d: %s", code, errOut.String())
			}
			var got initJSON
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			skipped := 0
			for _, w := range got.Writes {
				if strings.HasPrefix(w.Path, "~/") {
					if w.Action != "skipped-user-scope" {
						t.Errorf("user write = %+v", w)
					}
					skipped++
				}
				if strings.HasPrefix(w.Action, "would-") {
					t.Errorf("real run has dry-run action: %+v", w)
				}
			}
			if skipped == 0 {
				t.Error("missing skipped user configuration writes")
			}
		})
	}
}
