package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// TestDefaultStorePathNeverTheCwd pins PC-15 / I31: inside a git repo the
// store default lands under <toplevel>/workspace even from a
// subdirectory; the call-log default lands under the repo's state
// directory (O63 item 5 — outside the repo tree by default); outside any
// repo both land under ~/.fathom; neither is ever the bare cwd-relative
// literal (O11/O12).
func TestDefaultStorePathNeverTheCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	repo := t.TempDir()
	repo, _ = filepath.EvalSymlinks(repo)
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	sub := filepath.Join(repo, "deploy")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	wantStore := filepath.Join(repo, "workspace", "db", "fathom.sqlite")
	if got := defaultStorePath(); got != wantStore {
		t.Errorf("in <repo>/deploy: defaultStorePath() = %q, want %q", got, wantStore)
	}
	wantLog := filepath.Join(statedir.Resolve(repo), "logs", "mcp-calls.jsonl")
	if got := defaultCallLogPath(); got != wantLog {
		t.Errorf("in <repo>/deploy: defaultCallLogPath() = %q, want %q", got, wantLog)
	}

	outside := t.TempDir()
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	if top := gitToplevel(outside); top != "" {
		t.Skipf("temp dir %s is inside a git repo (%s); cannot test the no-repo branch here", outside, top)
	}
	if got, want := defaultStorePath(), filepath.Join(home, ".fathom", "db", "fathom.sqlite"); got != want {
		t.Errorf("outside a repo: defaultStorePath() = %q, want %q", got, want)
	}
	if got := defaultCallLogPath(); !strings.HasPrefix(got, filepath.Join(home, ".fathom")) {
		t.Errorf("outside a repo: defaultCallLogPath() = %q, want under ~/.fathom", got)
	}
	for _, got := range []string{defaultStorePath(), defaultCallLogPath()} {
		if !filepath.IsAbs(got) {
			t.Errorf("default %q is relative — the cwd-relative literal is exactly what I31 forbids", got)
		}
	}
}

// TestExplicitStoreAndCallLogStillWin: the resolved defaults replace only
// the literal fallback — a flag, FATHOM_CALL_LOG and [daemon].* still win.
func TestExplicitStoreAndCallLogStillWin(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	t.Setenv("FATHOM_CALL_LOG", "/explicit/calls.jsonl")
	if got := resolveCallLogPath(); got != "/explicit/calls.jsonl" {
		t.Errorf("FATHOM_CALL_LOG must win: %q", got)
	}
	t.Setenv("FATHOM_CALL_LOG", "")
	writeProjectConfig(t, projectDir, "[daemon]\nstore = \"custom/store.sqlite\"\ncall_log = \"custom/calls.jsonl\"\n")
	if got := resolveStorePath(); got != "custom/store.sqlite" {
		t.Errorf("[daemon].store must win: %q", got)
	}
	if got := resolveCallLogPath(); got != "custom/calls.jsonl" {
		t.Errorf("[daemon].call_log must win: %q", got)
	}
}
