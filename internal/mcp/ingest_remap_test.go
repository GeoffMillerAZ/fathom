package mcp_test

// ingest_remap_test.go — wave P3 defect 3 / O82 (.fathom/observations.md):
// worktree.CheckReroot (O22, PC-33) only refuses re-rooting a service onto a
// LINKED git worktree of its own recorded root. A plain `git clone` — a
// separate physical checkout, not a `git worktree add` — sails straight
// through it: the lefthook post-commit `fathom-refresh` step, firing inside
// a push clone under /private/tmp, called the daemon's `ingest` tool with
// {service:"fathom", path:<push clone>}, and the daemon happily re-pointed
// the fleet's "fathom" service at a directory that vanishes with the clone.
//
// A repository hook must never re-point a service. The daemon-side ingest
// handler must refuse ANY path that differs from the service's recorded
// root unless the caller explicitly says remap:true — the deliberate-move
// escape hatch that parallels the store-direct `fathom remap` verb.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestIngestRefusesPlainCloneReroot(t *testing.T) {
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
			JSONRPC: "2.0", ID: 1340, Method: "tools/call",
			Params: map[string]any{"name": "ingest", "arguments": args},
		})
	}

	base := t.TempDir()
	original := mustDir(t, base, "sextant")
	// A plain `git clone` of original elsewhere — a real, independent
	// checkout, NOT a `git worktree add`. worktree.CheckReroot waves this
	// straight through; it is a different physical directory that shares
	// no git-common-dir and carries no .claude/worktrees/ marker.
	pushClone := mustDir(t, base, "private", "tmp", "pushclone")

	t.Run("refused_without_remap_naming_both_paths_and_the_escape_hatch", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": original}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom", "path": pushClone})
		e := xtraError(t, resp)
		msg, _ := e["message"].(string)
		for _, want := range []string{`"fathom"`, canon(t, original), canon(t, pushClone), "remap"} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal must carry %q, got %q", want, msg)
			}
		}
		if _, ingested := h.ingestedAt["fathom"]; ingested {
			t.Error("a refused re-root must not reach the backend's IngestServiceAt — the exact bug (four {ok:true} re-points)")
		}
	})

	t.Run("remap_true_authorizes_the_move", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": original}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom", "path": pushClone, "remap": true})
		p := xtraPayload(t, resp)
		if p["ok"] != true {
			t.Fatalf("remap:true must be honored as a deliberate move: %v", p)
		}
		if got, want := h.ingestedAt["fathom"], canon(t, pushClone); got != want {
			t.Errorf("IngestServiceAt received %q, want the new root %q", got, want)
		}
	})

	t.Run("same_root_never_needs_remap", func(t *testing.T) {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": original}}
		h.services = []string{"fathom"}
		srv := mcp.NewServer(h)
		resp := call(t, srv, map[string]any{"service": "fathom", "path": original})
		p := xtraPayload(t, resp)
		if p["ok"] != true {
			t.Fatalf("a refresh at the recorded root must never require remap: %v", p)
		}
	})
}
