package mcp_test

// ingest_remap_live_test.go — EW-19 (O127): remap:true authorizes a deliberate
// move of a service's root, but a worker that read "pass remap:true" in a
// refusal passed it with its scratch clone's path and captured the owner's
// own fathom service; every answer in the orchestrating session then
// resolved against a tree deleted with the clone. remap:true alone is no
// longer enough while the recorded root is a live checkout of the SAME
// repository as the requested path: the caller must also name that root
// (remap_from), which a caller that never looked at what it replaces cannot.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// gitCheckout runs git in dir (created when absent) with a fixed identity.
func gitCheckout(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// newRepo makes dir a git repository with one commit whose content is
// content: two repositories built in one second from the same content share a
// root commit hash, so a test wanting two repositories gives each its own.
func newRepo(t *testing.T, dir, content string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCheckout(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCheckout(t, dir, "add", "-A")
	gitCheckout(t, dir, "commit", "-q", "-m", "init")
	r, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestIngestRemapRefusesScratchCloneOfLiveRoot(t *testing.T) {
	call := func(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
		t.Helper()
		return roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1341, Method: "tools/call",
			Params: map[string]any{"name": "ingest", "arguments": args},
		})
	}
	backend := func(owner string) *hygieneFakeBackend {
		h := &hygieneFakeBackend{paths: map[string]string{"fathom": owner}}
		h.services = []string{"fathom"}
		return h
	}
	// clone returns a plain `git clone` of owner: the scratch checkout of
	// the same repository a worker holds.
	clone := func(t *testing.T, owner string) string {
		t.Helper()
		dst := filepath.Join(t.TempDir(), "scratch")
		cmd := exec.Command("git", "clone", "-q", owner, dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git clone: %v\n%s", err, out)
		}
		r, err := filepath.EvalSymlinks(dst)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	t.Run("remap_true_alone_is_refused_naming_the_live_root", func(t *testing.T) {
		owner := newRepo(t, filepath.Join(t.TempDir(), "owner"), "package owner\n")
		scratch := clone(t, owner)
		h := backend(owner)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": scratch, "remap": true})
		e := xtraError(t, resp)
		msg, _ := e["message"].(string)
		for _, want := range []string{`"fathom"`, owner, scratch, "live checkout", "remap_from"} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal must carry %q, got %q", want, msg)
			}
		}
		if _, ingested := h.ingestedAt["fathom"]; ingested {
			t.Error("a refused remap reached the backend's IngestServiceAt: the scratch clone captured the service")
		}
	})

	t.Run("a_wrong_remap_from_is_refused", func(t *testing.T) {
		owner := newRepo(t, filepath.Join(t.TempDir(), "owner"), "package owner\n")
		scratch := clone(t, owner)
		h := backend(owner)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": scratch, "remap": true, "remap_from": scratch})
		e := xtraError(t, resp)
		if msg, _ := e["message"].(string); !strings.Contains(msg, owner) || !strings.Contains(msg, "remap_from") {
			t.Errorf("a remap_from that is not the recorded root must be refused naming that root, got %q", msg)
		}
		if _, ingested := h.ingestedAt["fathom"]; ingested {
			t.Error("a remap naming the wrong root reached IngestServiceAt")
		}
	})

	t.Run("naming_the_root_authorizes_the_move", func(t *testing.T) {
		owner := newRepo(t, filepath.Join(t.TempDir(), "owner"), "package owner\n")
		scratch := clone(t, owner)
		h := backend(owner)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": scratch, "remap": true, "remap_from": owner})
		if p := xtraPayload(t, resp); p["ok"] != true {
			t.Fatalf("remap naming the recorded root must be honored: %v", p)
		}
		if got := h.ingestedAt["fathom"]; got != scratch {
			t.Errorf("IngestServiceAt received %q, want the new root %q", got, scratch)
		}
	})

	t.Run("a_root_that_no_longer_exists_needs_only_remap_true", func(t *testing.T) {
		owner := newRepo(t, filepath.Join(t.TempDir(), "owner"), "package owner\n")
		scratch := clone(t, owner)
		if err := os.RemoveAll(owner); err != nil { // the repair case: the recorded root was deleted
			t.Fatal(err)
		}
		h := backend(owner)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": scratch, "remap": true})
		if p := xtraPayload(t, resp); p["ok"] != true {
			t.Fatalf("re-pointing a service whose root is gone must need only remap:true: %v", p)
		}
	})

	t.Run("a_different_repository_needs_only_remap_true", func(t *testing.T) {
		owner := newRepo(t, filepath.Join(t.TempDir(), "owner"), "package owner\n")
		other := newRepo(t, filepath.Join(t.TempDir(), "other"), "package other\n")
		gitCheckout(t, other, "commit", "-q", "--allow-empty", "-m", "second")
		h := backend(owner)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": other, "remap": true})
		if p := xtraPayload(t, resp); p["ok"] != true {
			t.Fatalf("a deliberate move to another repository must need only remap:true: %v", p)
		}
	})

	t.Run("a_recorded_root_that_is_not_a_checkout_needs_only_remap_true", func(t *testing.T) {
		plain := filepath.Join(t.TempDir(), "plain")
		if err := os.MkdirAll(plain, 0o755); err != nil {
			t.Fatal(err)
		}
		plain, _ = filepath.EvalSymlinks(plain)
		target := newRepo(t, filepath.Join(t.TempDir(), "target"), "package target\n")
		h := backend(plain)
		resp := call(t, mcp.NewServer(h), map[string]any{"service": "fathom", "path": target, "remap": true})
		if p := xtraPayload(t, resp); p["ok"] != true {
			t.Fatalf("a recorded root that is no git checkout is not a live checkout of the repository: %v", p)
		}
	})
}
