package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	wait := fs.Bool("wait", false, "re-index now: run the daemon's ingest tool (an explicit request on its refresh queue) and wait up to --timeout for it, instead of queueing a background refresh")
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
	// A git hook, not a harness hook: no descriptor deadline (its own
	// --timeout bounds each call), but the kill switch and the shared
	// breaker still apply (§9, §11.2).
	endGuard, silent := beginHookGuard("post-commit-hook", "", "", *url)
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()
	hc := &http.Client{Timeout: *timeout}
	// The same cached, daemon-backed identity every hook uses (RT-133,
	// O1's deterministic resolver), fail-open to the basename; it also
	// carries the service's registered root for the guard below.
	svc, registered, registeredKnown := postCommitIdentity(hc, *url, root, *service)
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
	if !registeredKnown {
		// An explicit --service identity cannot answer, or a daemon that
		// predates GET /identity: the full list, as before.
		if svcs, err := fetchServices(hc, *url); err == nil {
			registered = svcs[strings.ToLower(svc)].path
		}
	}
	if registered != "" {
		recCanon, rerr := filepath.EvalSymlinks(registered)
		rootCanon, cerr := filepath.EvalSymlinks(root)
		if rerr == nil && cerr == nil && recCanon != rootCanon {
			fmt.Fprintf(stderr, "post-commit: %q is registered at %s, not this repo root (%s) — refreshed nothing (a hook must never re-point a service; use `fathom remap` for a deliberate move, or ingest with remap:true)\n", svc, recCanon, rootCanon)
			return 0
		}
	}

	how, err := "", error(nil)
	if *wait {
		// I38: a remedy that says "re-index" must re-index — the ingest
		// tool waits for its run on the queue and returns its error.
		how, err = "refreshed", ingestToolViaDaemon(*url, svc, root, *timeout)
	} else {
		how, err = ingestViaDaemon(*url, svc, root, *timeout)
	}
	if err == nil {
		if !*quiet {
			fmt.Fprintf(stdout, "post-commit: %s %q through the daemon\n", how, svc)
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
	if errors.Is(err, errServiceNotIndexed) {
		// RT-111: the hook queues a refresh of a service the daemon
		// indexes; it never onboards one (the ingest tool with a path did,
		// before the refresh queue). Say how to onboard it once.
		return fmt.Sprintf("post-commit: %q is not indexed by the daemon — nothing was ingested; a commit hook never onboards a service. Onboard it once through the daemon: the ingest MCP tool with {\"service\": %q, \"path\": %q} (then each commit queues its refresh)", service, service, root)
	}
	return fmt.Sprintf("post-commit: daemon refresh failed (%v) — nothing was ingested; %q stays at its previously indexed commit. When the daemon is back, refresh through it: fathom post-commit-hook --dir %s (or the ingest MCP tool for service %q)", err, service, root, service)
}

// ingestViaDaemon asks the daemon to refresh service (RT-111, design §12.3):
// POST /refresh-request queues it (202, coalesced with a queued job or the
// running ingest's one follow-up) and returns at once — the ingest runs on
// the daemon's refresh queue, never inside this hook's request. It answers
// what it did ("queued a refresh of"). A daemon that refuses (503: refuse,
// store distress, safe mode, a quarantine) or does not index the service
// (404 with its JSON answer) is an error naming why. Only a daemon that
// predates the route (any other answer) is asked through the `ingest` tool,
// as before ("refreshed").
func ingestViaDaemon(baseURL, service, path string, timeout time.Duration) (string, error) {
	queued, handled, err := enqueueViaDaemon(baseURL, service, timeout)
	if handled {
		if err != nil {
			return "", err
		}
		return queued, nil
	}
	if err := ingestToolViaDaemon(baseURL, service, path, timeout); err != nil {
		return "", err
	}
	return "refreshed", nil
}

// enqueueViaDaemon is POST /refresh-request. handled is false when the
// daemon has no such route (an older binary) — the caller falls back.
func enqueueViaDaemon(baseURL, service string, timeout time.Duration) (what string, handled bool, err error) {
	body, err := json.Marshal(map[string]string{"service": service})
	if err != nil {
		return "", true, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/refresh-request", bytes.NewReader(body))
	if err != nil {
		return "", true, err
	}
	req.Header.Set("Content-Type", "application/json")
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", true, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var ans refreshRequestResponse
	isAnswer := json.Unmarshal(raw, &ans) == nil && ans.State != ""
	switch {
	case resp.StatusCode == http.StatusAccepted && isAnswer:
		what := "queued a refresh of"
		switch {
		case ans.FollowUp:
			what = "queued a follow-up refresh (after the running one) of"
		case ans.Coalesced:
			what = "joined the queued refresh of"
		}
		if ans.Paused != "" {
			what += " (" + ans.Paused + ")"
		}
		return what, true, nil
	case isAnswer && resp.StatusCode == http.StatusNotFound:
		return "", true, fmt.Errorf("%w (HTTP 404): %s", errServiceNotIndexed, ans.Error)
	case isAnswer:
		why := ans.Refused
		if why == "" {
			why = ans.Error
		}
		return "", true, fmt.Errorf("refresh refused (HTTP %d): %s", resp.StatusCode, why)
	}
	return "", false, nil
}

// errServiceNotIndexed is enqueueViaDaemon's error for a service the daemon
// does not index (POST /refresh-request's 404): the hook never onboards one.
var errServiceNotIndexed = errors.New("service not indexed by the daemon")

// ingestToolViaDaemon calls the daemon's `ingest` tool for service — the
// fallback for a daemon that predates POST /refresh-request. A JSON-RPC
// error (including IT-15's refusal) is returned as an error so the caller
// reports it rather than treating a refusal as success.
func ingestToolViaDaemon(baseURL, service, path string, timeout time.Duration) error {
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

// postCommitIdentity resolves the service to refresh for root (explicit
// wins; else the cached identity's root match; else the first basename
// candidate) and, when the identity answered for that service, its
// registered corpus root (known=true; "" when it has none or nothing is
// indexed under that name). known=false leaves the guard to the full list.
func postCommitIdentity(hc *http.Client, baseURL, root, explicit string) (svc, registered string, known bool) {
	if explicit != "" {
		// A named service is not a directory's identity: the guard reads
		// the full list for its registered root, as before.
		return explicit, "", false
	}
	id, err := lookupIdentityCached(hc, baseURL, root)
	if errors.Is(err, errIdentityRouteMissing) {
		if cand, ok := resolveIndexedCandidate(hc, baseURL, root); ok {
			svc = cand.name
		}
	} else if err == nil && id.MatchedBy == identityByRoot {
		svc = id.Service
	}
	if svc == "" {
		if cands := serviceCandidates(root); len(cands) > 0 {
			svc = cands[0].name
		}
	}
	if err != nil || svc == "" {
		return svc, "", false
	}
	switch {
	case strings.EqualFold(id.Service, svc):
		return svc, id.Path, true
	case id.Service == "":
		// Neither root nor any basename candidate names an indexed
		// service: svc (a basename) is not registered anywhere.
		return svc, "", true
	}
	return svc, "", false
}
