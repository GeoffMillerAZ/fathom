package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAllowedOperatorConfigKey covers the whitelist deriveOperatorGitConfig
// filters the operator's global config through: only keys needed to REACH
// a remote survive (credential helpers, url rewrites, http proxy/CA,
// core.sshCommand) — never the policy keys VB-27's isolation exists to
// keep away from subject preparation (autocrlf, safecrlf, fsck*, hooks,
// aliases, identity, signing, gpg.*).
func TestAllowedOperatorConfigKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"credential.helper", true},
		{"credential.https://example.invalid.helper", true},
		{"url.git@example.invalid:.insteadof", true},
		{"url.git@example.invalid:.pushinsteadof", true},
		{"http.proxy", true},
		{"http.sslcainfo", true},
		{"http.https://example.invalid.sslcainfo", true},
		{"core.sshcommand", true},
		{"core.autocrlf", false},
		{"core.safecrlf", false},
		{"fetch.fsckobjects", false},
		{"transfer.fsckobjects", false},
		{"core.hookspath", false},
		{"alias.co", false},
		{"user.name", false},
		{"user.email", false},
		{"commit.gpgsign", false},
		{"gpg.program", false},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := allowedOperatorConfigKey(c.key); got != c.want {
				t.Errorf("allowedOperatorConfigKey(%q) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

// TestDeriveOperatorGitConfig_CarriesOverOnlyRemoteReachingKeys is the
// defect's own reproduction: an operator global config with a credential
// helper and a url insteadOf rewrite (both needed to reach a private
// remote) alongside core.autocrlf and fetch.fsckObjects (both the kind of
// operator policy VB-27's isolation exists to keep out) must derive a file
// containing EXACTLY the first two.
func TestDeriveOperatorGitConfig_CarriesOverOnlyRemoteReachingKeys(t *testing.T) {
	fakeGlobal := filepath.Join(t.TempDir(), "operator-real-gitconfig")
	if err := os.WriteFile(fakeGlobal, []byte(strings.Join([]string{
		`[credential "https://example.invalid"]`,
		"\thelper = !true",
		`[url "git@example.invalid:"]`,
		"\tinsteadOf = https://example.invalid/",
		"[core]",
		"\tautocrlf = input",
		"[fetch]",
		"\tfsckObjects = true",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", fakeGlobal)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	derived := filepath.Join(t.TempDir(), "derived-gitconfig")
	path, err := deriveOperatorGitConfig(context.Background(), derived)
	if err != nil {
		t.Fatalf("deriveOperatorGitConfig: %v", err)
	}
	if path != derived {
		t.Fatalf("deriveOperatorGitConfig returned %q, want %q", path, derived)
	}

	out, err := exec.Command("git", "config", "--file", derived, "--list", "-z").Output()
	if err != nil {
		t.Fatalf("listing the derived config: %v", err)
	}
	var got []string
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		name, _, _ := strings.Cut(entry, "\n")
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{
		"credential.https://example.invalid.helper",
		"url.git@example.invalid:.insteadof",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("derived config keys = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("derived config keys = %v, want exactly %v", got, want)
		}
	}
}

// TestGitEnvFor_IncludesTerminalPromptOff is fix item (2): every git
// subprocess gets GIT_TERMINAL_PROMPT=0 (and GIT_ASKPASS unset) so a
// missing credential fails fast with exit 128 instead of hanging the run
// at a prompt nothing will ever answer (guards-pilot-7, 2026-09-17).
func TestGitEnvFor_IncludesTerminalPromptOff(t *testing.T) {
	got := gitEnvFor(nil)
	if got["GIT_TERMINAL_PROMPT"] != "0" {
		t.Errorf("gitEnvFor(nil)[GIT_TERMINAL_PROMPT] = %q, want %q", got["GIT_TERMINAL_PROMPT"], "0")
	}
	if v, ok := got["GIT_ASKPASS"]; !ok || v != "" {
		t.Errorf("gitEnvFor(nil)[GIT_ASKPASS] = %q, ok=%v, want \"\", ok=true", v, ok)
	}
	// gitConfigIsolation's own defaults still apply underneath.
	if got["GIT_CONFIG_GLOBAL"] != "/dev/null" {
		t.Errorf("gitEnvFor(nil)[GIT_CONFIG_GLOBAL] = %q, want /dev/null", got["GIT_CONFIG_GLOBAL"])
	}
	if got["GIT_CONFIG_NOSYSTEM"] != "1" {
		t.Errorf("gitEnvFor(nil)[GIT_CONFIG_NOSYSTEM] = %q, want 1", got["GIT_CONFIG_NOSYSTEM"])
	}

	// A caller-supplied GIT_CONFIG_GLOBAL (ensureClone/ensureCommit's own
	// derived-config override) still wins over the /dev/null default.
	override := gitEnvFor(map[string]string{"GIT_CONFIG_GLOBAL": "/tmp/derived-gitconfig"})
	if override["GIT_CONFIG_GLOBAL"] != "/tmp/derived-gitconfig" {
		t.Errorf("gitEnvFor override[GIT_CONFIG_GLOBAL] = %q, want the caller's own value", override["GIT_CONFIG_GLOBAL"])
	}
	if override["GIT_TERMINAL_PROMPT"] != "0" {
		t.Errorf("gitEnvFor override[GIT_TERMINAL_PROMPT] = %q, want 0 even with a caller override present", override["GIT_TERMINAL_PROMPT"])
	}
}

// TestEnsureClone_LocalCloneStillWorksWithDerivedConfigPresent is fix item
// (3)'s last case: with a benign operator global config in play, a clone
// from a local fixture repo still succeeds through ensureClone's new
// derived-config path (operatorGitConfigFile), and that file actually
// lands under root's workspace/tmp as documented.
func TestEnsureClone_LocalCloneStillWorksWithDerivedConfigPresent(t *testing.T) {
	repoPath, _ := gitFixtureRepo(t)

	fakeGlobal := filepath.Join(t.TempDir(), "operator-real-gitconfig")
	if err := os.WriteFile(fakeGlobal, []byte(strings.Join([]string{
		`[credential "https://example.invalid"]`,
		"\thelper = !true",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", fakeGlobal)

	root := t.TempDir()
	cloneDir := filepath.Join(t.TempDir(), "clone")
	if err := ensureClone(context.Background(), repoPath, cloneDir, root); err != nil {
		t.Fatalf("ensureClone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err != nil {
		t.Fatalf("cloneDir has no .git after ensureClone: %v", err)
	}
	derived := filepath.Join(root, "workspace", "tmp", "operator-gitconfig")
	if _, err := os.Stat(derived); err != nil {
		t.Errorf("expected a derived operator config at %s: %v", derived, err)
	}
}
