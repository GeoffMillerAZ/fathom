package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

func TestEditHookPre_GitSafetyNearestConfig(t *testing.T) {
	for _, tc := range []struct {
		name, user, root, nested string
		want                     bool
	}{
		{"nested_opt_out", "", "git_safety = true", "git_safety = false", false},
		{"nested_opt_in", "git_safety = false", "git_safety = false", "git_safety = true", true},
		{"root_opt_out", "", "git_safety = false", "enforce = true", false},
		{"root_opt_in", "git_safety = false", "git_safety = true", "enforce = true", true},
		{"user_opt_out", "git_safety = false", "", "", false},
		{"user_error", "git_safety = [", "git_safety = true", "", false},
		{"nested_error", "", "git_safety = true", "git_safety = [", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("FATHOM_ENFORCE", "")
			t.Setenv("CLAUDE_PROJECT_DIR", "")
			repo := t.TempDir()
			gitInitTempRepo(t, repo)
			nested := filepath.Join(repo, "service")
			for _, layer := range []struct{ dir, body string }{{os.Getenv("HOME"), tc.user}, {repo, tc.root}, {nested, tc.nested}} {
				if err := os.MkdirAll(filepath.Join(layer.dir, ".fathom"), 0755); err != nil {
					t.Fatal(err)
				}
				if layer.body != "" {
					if err := os.WriteFile(filepath.Join(layer.dir, ".fathom", "config.toml"), []byte("[guards]\n"+layer.body+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, advisory := range []bool{false, true} {
				if advisory {
					t.Setenv("FATHOM_ENFORCE", "off")
				}
				var out bytes.Buffer
				cmdEditHookPreCommand(harness.ClaudeCodec(), "git push --no-verify", nested, "", "", false, &out)
				if got := strings.Contains(out.String(), "hooks enabled"); got != tc.want {
					t.Errorf("advisory=%v: git-safety output=%s, want match=%v", advisory, out.String(), tc.want)
				}
				if got := strings.Contains(out.String(), `"permissionDecision":"deny"`); got != (tc.want && !advisory) {
					t.Errorf("advisory=%v: unexpected denial: %s", advisory, out.String())
				}
			}
			// Opting out of the pack must not disable the root's own guard.
			writeTestGuardFile(t, repo, `guards: [{command_scopes: ["make deploy*"], note: "repo deploy guard"}]`)
			var out bytes.Buffer
			cmdEditHookPreCommand(harness.ClaudeCodec(), "make deploy", nested, "", "", false, &out)
			if !strings.Contains(out.String(), "repo deploy guard") {
				t.Fatalf("root guard lost: %s", out.String())
			}
		})
	}
}

func TestReadoutHookGuardSharedCommandMatcher(t *testing.T) {
	for _, tc := range []struct {
		pattern, command string
		want             bool
	}{
		{"go test *", "sudo go test ./...", true},
		{"GOFLAGS=-count=1 go test *", "GOFLAGS=-count=1 go test ./...", true},
		{"go test *", `go "test ./..."`, false},
		{"go test *", "go test ./... && echo -race", true},
		{"go test *", "GOFLAGS=-race go test ./...", false},
		{"go test *", "go test $PACKAGES", false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			gs := guardspec.GuardSet{Guards: []guardspec.Guard{{CommandScopes: []string{tc.pattern}, Requires: []string{"-race"}}}}
			c := shellcmd.ParseCommand(tc.command)
			line := guardLineForCommand(gs, &c, ".fathom/guard.cue", readoutRichLineBudget)
			if (line != "") != tc.want {
				t.Errorf("guard line=%q, want match=%v", line, tc.want)
			}
		})
	}
}
