package mcp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestIngestRefusesWorktreeReroot is PC-33 (docs/plans/proof-credibility.md;
// invariant I30; O22): the daemon's `ingest` tool must never move an
// EXISTING service onto a git worktree of its own recorded root — the fleet
// "fathom" service was re-rooted at an agent's .claude/worktrees/… checkout
// by that worktree's post-commit hook, the tool answered {ok:true} four
// times, and every "refreshed" verify afterwards read the wrong tree. The
// refusal is a named JSON-RPC error carrying both paths; a successful
// ingest names the root it refreshed; a new service, a refresh at the
// recorded root, and a genuinely different repository all pass.
func TestIngestRefusesWorktreeReroot(t *testing.T) {
	mustDir := func(t *testing.T, parts ...string) string {
		t.Helper()
		p := filepath.Join(parts...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	canon := func(t *testing.T, p string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	call := func(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
		t.Helper()
		return roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1330, Method: "tools/call",
			Params: map[string]any{"name": "ingest", "arguments": args},
		})
	}

	base := t.TempDir()
	repo := mustDir(t, base, "repo")
	wt := mustDir(t, base, "repo", ".claude", "worktrees", "wf_1")

	t.Run("worktree_of_recorded_root_refused_naming_both_paths", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": repo}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom", "path": wt})
		e := xtraError(t, resp)
		msg, _ := e["message"].(string)
		for _, want := range []string{"worktree re-root refused", `"fathom"`, canon(t, repo), canon(t, wt)} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal must carry %q, got %q", want, msg)
			}
		}
		if _, ingested := h.ingestedAt["fathom"]; ingested {
			t.Error("a refused re-root must not reach the backend's IngestServiceAt")
		}
	})

	t.Run("refresh_at_recorded_root_names_the_root", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": repo}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom", "path": repo})
		p := xtraPayload(t, resp)
		if p["ok"] != true {
			t.Fatalf("refresh at the recorded root must succeed: %v", p)
		}
		if p["root"] != repo {
			t.Errorf("the ok envelope must name the root it refreshed (O22): got %v", p["root"])
		}
	})

	t.Run("new_service_may_root_anywhere", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{}}
		srv := mcp.NewServer(h)
		other := mustDir(t, base, "elsewhere")
		resp := call(t, srv, map[string]any{"service": "fresh", "path": other})
		if resp["error"] != nil {
			t.Fatalf("a new service has no recorded root to protect: %v", resp["error"])
		}
	})

	t.Run("plain_refresh_without_path_untouched", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": repo}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom"})
		p := xtraPayload(t, resp)
		if p["ok"] != true || p["root"] != repo {
			t.Errorf("a path-less refresh reports ok and the recorded root: %v", p)
		}
	})
}
