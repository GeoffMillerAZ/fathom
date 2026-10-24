package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/provenance"
)

func provenanceGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCmdProvenanceNote(t *testing.T) {
	t.Setenv(fathomhome.Env, t.TempDir())
	for _, flags := range []bool{false, true} {
		t.Run(map[bool]string{false: "environment", true: "flags"}[flags], func(t *testing.T) {
			root := t.TempDir()
			provenanceGit(t, root, "init", "-q")
			provenanceGit(t, root, "config", "user.name", "Test Operator")
			provenanceGit(t, root, "config", "user.email", "operator@example.invalid")
			provenanceGit(t, root, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "-qm", "fixture")
			head := provenanceGit(t, root, "rev-parse", "HEAD")
			for _, key := range []string{"FATHOM_HARNESS", "FATHOM_MODEL", "FATHOM_SESSION_ID", "FATHOM_LANE"} {
				t.Setenv(key, "env-value")
			}
			args := []string{"provenance", "note"}
			want := provenance.Fields{Harness: "env-value", Model: "env-value", HostLabel: "unknown", Session: "env-value", Lane: "env-value"}
			if flags {
				args = append(args, head, "--harness", "flag-harness", "--model", "flag-model", "--host-label", "flag-label", "--session", "flag-session", "--lane", "flag-lane")
				want = provenance.Fields{Harness: "flag-harness", Model: "flag-model", HostLabel: "flag-label", Session: "flag-session", Lane: "flag-lane"}
			}
			code, stdout, stderr := runInDir(t, root, args)
			if code != 0 {
				t.Fatalf("exit %d: %s %s", code, stdout, stderr)
			}
			if wantOut := "provenance note: recorded for " + head + " under refs/notes/fathom/provenance\n"; stdout != wantOut {
				t.Errorf("stdout = %q, want %q", stdout, wantOut)
			}
			raw := provenanceGit(t, root, "notes", "--ref=fathom/provenance", "show", head)
			var got provenance.Fields
			if err := json.Unmarshal([]byte(raw), &got); err != nil || got != want {
				t.Fatalf("note = %s, decode error %v, want %+v", raw, err, want)
			}
		})
	}
}

func TestCmdProvenanceUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"other"}, {"note", "HEAD", "extra"}, {"note", "--no-such-flag"}} {
		code, _, _ := runInDir(t, t.TempDir(), append([]string{"provenance"}, args...))
		if code == 0 {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}

func TestCmdProvenanceHostConfig(t *testing.T) {
	for _, tc := range []struct{ name, user, flag, want string }{
		{"user", "configured-label", "", "configured-label"},
		{"flag", "configured-label", "flag-label", "flag-label"},
		{"absent", "", "", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(fathomhome.Env, t.TempDir())
			if err := os.WriteFile(config.DefaultUserPath(), []byte("[gate]\nhost_label = '"+tc.user+"'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			root := provenanceTestRepo(t)
			// A repository cannot override the operator's user-only label.
			project := projectdir.Config.Path(root)
			if err := os.MkdirAll(filepath.Dir(project), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(project, []byte("[gate]\nhost_label = 'repository-label'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(root, "nested")
			if err := os.Mkdir(nested, 0700); err != nil {
				t.Fatal(err)
			}
			args := []string{"provenance", "note"}
			if tc.flag != "" {
				args = append(args, "--host-label", tc.flag)
			}
			code, out, errOut := runInDir(t, nested, args)
			if code != 0 {
				t.Fatalf("exit %d: %s %s", code, out, errOut)
			}
			raw := provenanceGit(t, root, "notes", "--ref=fathom/provenance", "show", "HEAD")
			var got provenance.Fields
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			if got.HostLabel != tc.want {
				t.Errorf("host label = %q, want %q", got.HostLabel, tc.want)
			}
		})
	}
}

func provenanceTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	provenanceGit(t, root, "init", "-q")
	provenanceGit(t, root, "config", "user.name", "Test Operator")
	provenanceGit(t, root, "config", "user.email", "operator@example.invalid")
	provenanceGit(t, root, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "--allow-empty", "-qm", "fixture")
	return root
}

func TestCmdProvenanceHarnessSessionDefaults(t *testing.T) {
	for _, tc := range []struct{ name, variable, harnessEnv, sessionEnv, flag, wantHarness, wantSession string }{
		{"primary", "CLAUDE_SESSION_ID", "", "", "", "claude", "harness-session"},
		{"alias", "CLAUDE_CODE_SESSION_ID", "", "", "", "claude", "harness-session"},
		{"gemini", "GEMINI_SESSION_ID", "", "", "", "gemini", "harness-session"},
		{"environment", "CLAUDE_CODE_SESSION_ID", "declared-harness", "declared-session", "", "declared-harness", "declared-session"},
		{"flag", "CLAUDE_CODE_SESSION_ID", "declared-harness", "", "flag-harness", "flag-harness", "harness-session"},
		{"absent", "", "", "", "", "unknown", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(fathomhome.Env, t.TempDir())
			for _, d := range harness.All() {
				for _, key := range append(append([]string{}, d.Env.SessionIDAliases...), d.Env.SessionID) {
					if key != "" {
						t.Setenv(key, "")
					}
				}
			}
			t.Setenv("FATHOM_HARNESS", tc.harnessEnv)
			t.Setenv("FATHOM_SESSION_ID", tc.sessionEnv)
			if tc.variable != "" {
				t.Setenv(tc.variable, "harness-session")
			}
			root := provenanceTestRepo(t)
			args := []string{"provenance", "note"}
			if tc.flag != "" {
				args = append(args, "--harness", tc.flag)
			}
			code, out, errOut := runInDir(t, root, args)
			if code != 0 {
				t.Fatalf("exit %d: %s %s", code, out, errOut)
			}
			raw := provenanceGit(t, root, "notes", "--ref=fathom/provenance", "show", "HEAD")
			var got provenance.Fields
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			if got.Harness != tc.wantHarness || got.Session != tc.wantSession {
				t.Errorf("harness/session = %q/%q, want %q/%q", got.Harness, got.Session, tc.wantHarness, tc.wantSession)
			}
		})
	}
}
