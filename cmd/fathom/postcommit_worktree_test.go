package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPostCommitHookLeavesWorktreeAlone is PC-33 (O22, I30): a post-commit
// hook running inside a linked git worktree — an agent's
// .claude/worktrees/… checkout — must refresh NOTHING (no daemon call, no
// store-direct fallback) and print one stderr line naming the service and
// the root it left alone. Passing the worktree as `path` is exactly how the
// fleet's service was once re-rooted at a stale agent tree.
func TestPostCommitHookLeavesWorktreeAlone(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true}"}]}}`))
	}))
	defer srv.Close()

	base := t.TempDir()
	mainRoot := filepath.Join(base, "repo")
	wt := filepath.Join(mainRoot, ".claude", "worktrees", "wf_1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	code := cmdPostCommitHook([]string{"--dir", wt, "--url", srv.URL, "--service", "svc"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("a post-commit hook must never block a commit; exit %d", code)
	}
	if calls != 0 {
		t.Errorf("hook must not call the daemon from a worktree, got %d call(s)", calls)
	}
	msg := errBuf.String()
	for _, want := range []string{"worktree", `"svc"`, "refreshed nothing", filepath.Base(mainRoot)} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr line must carry %q: %q", want, msg)
		}
	}
	if strings.Contains(out.String(), "refreshed") {
		t.Errorf("no success line from a worktree: %q", out.String())
	}
}
