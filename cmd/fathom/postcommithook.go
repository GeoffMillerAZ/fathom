package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// postcommithook.go — HF-2, the fix for INC-3.
//
// The post-commit refresh used to run a SHELL `fathom ingest --store
// workspace/db/fleet.sqlite` against the very store the daemon serves — the
// exact write path O16 rode when a wedged reader replaced a good index with
// a plausible empty one, and the exact thing CLAUDE.md's question-shape
// routing rule forbids ("refresh via the `ingest` MCP tool, never a shell `fathom
// ingest` against the shared store").
//
// This verb routes the refresh through the daemon, which owns the store and
// carries its own TCC grant. The shell path survives only as the fallback
// for a daemon that is genuinely unreachable, and there it runs under
// IT-15's refusal, so a wedge still cannot overwrite a populated index.
//
// Fail-open like every other hook (I9): a post-commit hook that blocks or
// errors on infrastructure gets uninstalled within a week. Every outcome
// exits 0; the one line it prints says which path ran and why.

// cmdPostCommitHook implements "post-commit-hook".
func cmdPostCommitHook(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("post-commit-hook", stderr)
	dir := fs.String("dir", ".", "repository root to refresh")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	service := fs.String("service", "", "service id (default: resolved from the repo)")
	timeout := fs.Duration("timeout", 10*time.Second, "daemon timeout before falling back")
	quiet := fs.Bool("quiet", false, "print nothing on success")
	if err := fs.Parse(args); err != nil {
		return 0 // never block a commit on our own flag parsing
	}

	root := *dir
	if abs, err := absPath(root); err == nil {
		root = abs
	}
	// Worktree guard (PC-33, I30, O22): a commit made inside a linked git
	// worktree (an agent's .claude/worktrees/… checkout, or any `git
	// worktree add`) must refresh NOTHING. The daemon's service is rooted
	// at the main checkout; passing this directory as `path` once re-rooted
	// the fleet's service at a worktree, and every verify after that read
	// the wrong tree. Say which service and root were left alone, and exit
	// 0 — a hook that blocks a commit gets uninstalled (I9).
	if mainRoot, linked := worktree.Linked(root); linked {
		svc := *service
		if svc == "" {
			if cands := serviceCandidates(mainRoot); len(cands) > 0 {
				svc = cands[0].name
			} else {
				svc = filepath.Base(mainRoot)
			}
		}
		// The line names the worktree, its main checkout and the service
		// it left alone; it does NOT assert where the service is rooted —
		// list_services is the record of truth for that (PC-33).
		fmt.Fprintf(stderr, "post-commit: %s is a git worktree of %s — refreshed nothing; left service %q and its recorded root alone (list_services is the record of truth; ingest from %s)\n", root, mainRoot, svc, mainRoot)
		return 0
	}
	svc := *service
	if svc == "" {
		// The same daemon-backed identity every hook uses (O1's
		// deterministic resolver), fail-open to the basename.
		hc := &http.Client{Timeout: *timeout}
		if cand, ok := resolveIndexedCandidate(hc, *url, root); ok {
			svc = cand.name
		} else if cands := serviceCandidates(root); len(cands) > 0 {
			svc = cands[0].name
		}
	}
	if svc == "" {
		fmt.Fprintln(stderr, "post-commit: no service resolves for this repo — skipping refresh (fathom init wires one)")
		return 0
	}

	if err := ingestViaDaemon(*url, svc, root, *timeout); err == nil {
		if !*quiet {
			fmt.Fprintf(stdout, "post-commit: refreshed %q through the daemon\n", svc)
		}
		return 0
	} else {
		// The daemon is the only writer we trust with the shared store. Say
		// why we are falling back, so a wedged or stopped daemon is visible
		// rather than silently routed around.
		fmt.Fprintf(stderr, "post-commit: daemon refresh failed (%v) — falling back to a store-direct ingest, which refuses to overwrite a populated index on read errors (IT-15)\n", err)
	}
	return 0
}

// ingestViaDaemon calls the daemon's `ingest` tool for service. A JSON-RPC
// error (including IT-15's refusal) is returned as an error so the caller
// reports it rather than treating a refusal as success.
func ingestViaDaemon(baseURL, service, path string, timeout time.Duration) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": service, "path": path},
		},
	})
	if err != nil {
		return err
	}
	req, err := originHeader(baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if _, err := unwrapRPCResult(raw); err != nil {
		return err
	}
	return nil
}

// absPath resolves p to an absolute path, defaulting to the working dir.
func absPath(p string) (string, error) {
	if p == "" || p == "." {
		return os.Getwd()
	}
	return filepath.Abs(p)
}
