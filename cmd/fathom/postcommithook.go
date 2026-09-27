package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
// carries its own TCC grant. There is no fallback: when the daemon is
// unreachable or refuses, nothing is ingested and the one line printed says
// so and names how to refresh once the daemon is back (RT-170).
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
	timeout := fs.Duration("timeout", 10*time.Second, "daemon timeout for the refresh request (on failure nothing is ingested)")
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
	hc := &http.Client{Timeout: *timeout}
	svc := *service
	if svc == "" {
		// The same daemon-backed identity every hook uses (O1's
		// deterministic resolver), fail-open to the basename.
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

	// Registered-root guard (wave P3 / O82, .fathom/observations.md): a
	// hook must never re-point a service. The daemon's ingest tool now
	// refuses that itself (a differing path needs remap:true), but
	// stumbling into that refusal here would only print a confusing
	// "daemon refresh failed ... falling back" line for what is really
	// "this hook is running inside the wrong checkout" — the exact shape
	// that re-rooted "fathom" at a push clone's path under /private/tmp.
	// Skip proactively and say so plainly. Fail-open on enumeration
	// failure (daemon down, or too old to answer list_services): the
	// ingest call's own refusal below remains the backstop.
	if svcs, err := fetchServices(hc, *url); err == nil {
		if info, ok := svcs[strings.ToLower(svc)]; ok && info.path != "" {
			recCanon, rerr := filepath.EvalSymlinks(info.path)
			rootCanon, cerr := filepath.EvalSymlinks(root)
			if rerr == nil && cerr == nil && recCanon != rootCanon {
				fmt.Fprintf(stderr, "post-commit: %q is registered at %s, not this repo root (%s) — refreshed nothing (a hook must never re-point a service; use `fathom remap` for a deliberate move, or ingest with remap:true)\n", svc, recCanon, rootCanon)
				return 0
			}
		}
	}

	if err := ingestViaDaemon(*url, svc, root, *timeout); err == nil {
		if !*quiet {
			fmt.Fprintf(stdout, "post-commit: refreshed %q through the daemon\n", svc)
		}
		return 0
	} else {
		// The daemon is the only writer we trust with the shared store, and
		// this hook runs no other path. Say exactly that (RT-170, design
		// §12.3): the refresh failed, nothing was ingested, and how to
		// refresh through the daemon once it is back. The line used to
		// announce a store-direct fallback it never ran — a false claim made
		// during exactly the incidents it reports. It does not suggest a
		// shell `fathom ingest`: that writes the shared store directly (the
		// INC-3 path this verb exists to close).
		fmt.Fprintln(stderr, postCommitFailureLine(svc, root, err))
	}
	return 0
}

// postCommitFailureLine is the one stderr line a failed daemon refresh
// prints (RT-170): what failed, that nothing was ingested, and the refresh
// to run through the daemon when it is back.
func postCommitFailureLine(service, root string, err error) string {
	return fmt.Sprintf("post-commit: daemon refresh failed (%v) — nothing was ingested; %q stays at its previously indexed commit. When the daemon is back, refresh through it: fathom post-commit-hook --dir %s (or the ingest MCP tool for service %q)", err, service, root, service)
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
