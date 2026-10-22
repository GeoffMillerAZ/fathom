package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// GDEV-43: project setup can leave the entire user home byte-identical.
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
				if after := snapshotTree(t, home); !reflect.DeepEqual(before, after) {
					t.Errorf("HOME changed: before %v, after %v", before, after)
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
	home, root := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HOME", filepath.Join(home, ".fathom"))
	var out, errOut bytes.Buffer
	if code := cmdInit([]string{"--harness", "none", "--seed-user-ignore", "--path", root, "--service", "fixture", "--url", "://unavailable", "--no-scip"}, &out, &errOut); code != 2 {
		t.Fatalf("init = %d: %s", code, errOut.String())
	}
	for _, rel := range []string{".fathom/config.toml", ".fathom/ignore"} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(rel))); err != nil {
			t.Errorf("explicit user scope omitted %s: %v", rel, err)
		}
	}
}
