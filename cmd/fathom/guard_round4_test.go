package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestEditHookPre_EnforcedGuardBehindAdvisory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_ENFORCE", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	repo := t.TempDir()
	writeTestGuardFile(t, repo, `guards: [
		{command_scopes: ["git push*"], note: "advisory"},
		{command_scopes: ["*--force*"], enforce: true, note: "force forbidden"},
	]`)
	for _, command := range []string{
		"cd sub && git push --force origin feature",
		"make && git push --force origin feature",
	} {
		t.Run(command, func(t *testing.T) {
			var out bytes.Buffer
			cmdEditHookPreCommand(harness.ClaudeCodec(), command, repo, "", "", false, &out)
			if !strings.Contains(out.String(), `"permissionDecision":"deny"`) || !strings.Contains(out.String(), "force forbidden") {
				t.Fatalf("enforced guard hidden by advisory: %s", out.String())
			}
		})
	}
}

func TestEditHookPre_GitSafetyNearestEnforce(t *testing.T) {
	for _, tc := range []struct {
		name, outer, inner string
		deny               bool
	}{
		{"nested_off", "", "enforce = false", false},
		{"nested_off_outer_on", "enforce = true", "enforce = false", false},
		{"nested_on_outer_off", "enforce = false", "enforce = true", true},
		{"outer_off", "enforce = false", "", false},
		{"default_on", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("FATHOM_ENFORCE", "")
			t.Setenv("CLAUDE_PROJECT_DIR", "")
			repo := t.TempDir()
			gitInitTempRepo(t, repo)
			nested := filepath.Join(repo, "service")
			for _, layer := range []struct{ root, body string }{{repo, tc.outer}, {nested, tc.inner}} {
				if err := os.MkdirAll(filepath.Join(layer.root, ".fathom"), 0755); err != nil {
					t.Fatal(err)
				}
				if layer.body != "" {
					if err := os.WriteFile(filepath.Join(layer.root, ".fathom", "config.toml"), []byte("[guards]\n"+layer.body+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			var out bytes.Buffer
			cmdEditHookPreCommand(harness.ClaudeCodec(), "git push --no-verify", nested, "", "", false, &out)
			if strings.Contains(out.String(), `"permissionDecision":"deny"`) != tc.deny || !strings.Contains(out.String(), "hooks enabled") {
				t.Fatalf("want deny=%v with pack note, got %s", tc.deny, out.String())
			}
			// The nearest switch selects pack policy; outer repo guards still govern.
			if tc.inner == "enforce = false" {
				writeTestGuardFile(t, repo, `guards: [{command_scopes: ["make deploy*"], enforce: true, note: "outer deployment guard"}]`)
				out.Reset()
				cmdEditHookPreCommand(harness.ClaudeCodec(), "make deploy", nested, "", "", false, &out)
				if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
					t.Fatalf("outer repo enforcement lost: %s", out.String())
				}
			}
		})
	}
}

func TestEditHookPre_GitSafetyAdvisoryNotes(t *testing.T) {
	for _, source := range []string{"repo", "user"} {
		for _, command := range []string{"eval x; git commit -m x", "git commit --no-verify"} {
			t.Run(source+"/"+command, func(t *testing.T) {
				user := t.TempDir()
				t.Setenv("HOME", user)
				t.Setenv("FATHOM_ENFORCE", "off")
				t.Setenv("CLAUDE_PROJECT_DIR", "")
				repo := t.TempDir()
				gitInitTempRepo(t, repo)
				nested := filepath.Join(repo, "service")
				if err := os.MkdirAll(nested, 0755); err != nil {
					t.Fatal(err)
				}
				guardRoot := repo
				if source == "user" {
					guardRoot = user
				}
				writeTestGuardFile(t, guardRoot, `guards: [{command_scopes: ["*git commit*"], note: "review the commit"}]`)
				var out bytes.Buffer
				cmdEditHookPreCommand(harness.ClaudeCodec(), command, nested, "", "", false, &out)
				text := out.String()
				packNote := "hooks enabled"
				if strings.HasPrefix(command, "eval") {
					packNote = "unknown"
				}
				repoAt, packAt := strings.Index(text, "review the commit"), strings.Index(text, packNote)
				if repoAt < 0 || packAt <= repoAt || strings.Contains(text, `"permissionDecision":"deny"`) {
					t.Fatalf("want %s note followed by pack advisory, got %s", source, text)
				}
			})
		}
	}
}

func TestEditHookPre_GitSafetyRemoteHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_ENFORCE", "")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	repo := t.TempDir()
	for _, tc := range []struct {
		command, want string
		deny          bool
	}{
		{"git push origin :main", "git-safety/push_force_protected", true},
		{"git push -d origin main", "git-safety/push_force_protected", true},
		{"git push --delete origin master", "git-safety/push_force_protected", true},
		{"git push --mirror", "name the branch", true},
		{`git -c alias.publish=push publish --no-verify`, "unknown", false},
		{`git --config-env=core.hooksPath=HOOKS push origin feature`, "unknown", false},
		{`git -c alias.x=y push --no-verify`, "git-safety/git_no_verify", true},
	} {
		t.Run(tc.command, func(t *testing.T) {
			var out bytes.Buffer
			cmdEditHookPreCommand(harness.ClaudeCodec(), tc.command, repo, "", "", false, &out)
			if strings.Contains(out.String(), `"permissionDecision":"deny"`) != tc.deny || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("want deny=%v, %q; got %s", tc.deny, tc.want, out.String())
			}
			if strings.Count(out.String(), "name the branch") > 1 {
				t.Fatalf("pack remedy repeated in deny reason: %s", out.String())
			}
		})
	}
}
