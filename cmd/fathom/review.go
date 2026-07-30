package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdReview implements the "review" subcommand — the CLI mirror of the
// review_diff MCP tool (C1-1, docs/plans/review-verb.md):
//
//	fathom review --store DB --service S --since origin/main [--no-persist]
//
// Store-direct only, mirroring cmdReport's structure (cmd/fathom/report.go):
// opens --store directly rather than talking to the shared daemon, so it
// works from a git hook context (C3-1's pre-push step) without depending on
// the daemon being up. Resolves changed files via `git diff --name-only
// <ref>` (gitRunner injected for testability, mirroring
// cmd/fathom/ingestdiff.go's runIngestDiff), then runs the SAME
// internal/analysis/review engine the MCP tool uses — same dimensions, same
// honesty rules, same C2-1 findings-bridge write path (review.BuildPersist).
//
// Persistence is DELIBERATE and disclosed: by default a run writes its
// arch_conformance/spec_coverage findings to the claims store (and
// supersedes review-minted claims it no longer emits). --no-persist makes
// the run a pure report — no store write of any kind — which is what the
// non-blocking pre-push summary uses: a hook must never be an undisclosed
// writer to the shared store.
func cmdReview(args []string, stdout, stderr io.Writer) int {
	return runReview(args, defaultGitRunner, stdout, stderr)
}

func runReview(args []string, gitRunner func(args ...string) ([]byte, error), stdout, stderr io.Writer) int {
	fs := newFlagSet("review", stderr)
	storePath := fs.String("store", "workspace/db/fathom.sqlite", "store path (.sqlite/.db)")
	service := fs.String("service", "", "service ID (required)")
	since := fs.String("since", "", "git ref to diff against (required; e.g. origin/main)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	noPersist := fs.Bool("no-persist", false, "report only: never write findings to the claims store (the pre-push summary hook uses this)")
	callLogFlag := fs.String("call-log", "", "MCP-format call log (JSONL) to append this run's verdict outcome to (T5-7); default: [daemon].call_log in config.toml, else "+defaultCallLogPath)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" || *since == "" {
		fmt.Fprintln(stderr, "error: review requires --service and --since")
		fs.Usage()
		return 1
	}

	out, err := gitRunner("diff", "--name-only", *since)
	if err != nil {
		fmt.Fprintf(stderr, "error: git diff --name-only %s: %v\n", *since, err)
		return 1
	}
	var changedFiles []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			changedFiles = append(changedFiles, line)
		}
	}
	if len(changedFiles) == 0 {
		fmt.Fprintf(stdout, "review: no changed files since %s — nothing to review\n", *since)
		return 0
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ctx := context.Background()

	in, sources, err := buildReviewInputFromClient(ctx, c, *service, changedFiles)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	reviewStart := time.Now()
	v := review.Run(in)
	reviewLatency := time.Since(reviewStart)

	// T5-7 (docs/plans/trust-and-tokenomics.md), the forced-channel
	// measurement: this CLI is C3-1's forced channel (lefthook.yml's
	// pre-push review step, `bin/fathom review --no-persist`) — it never
	// goes through the MCP dispatch path (internal/mcp/dispatch.go), so its
	// verdict was otherwise invisible to `instrument --adoption`'s
	// gate-catch accounting. Best-effort and silent on failure: a logging
	// side effect must never fail a hook (§5.3, fail-open convention).
	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}
	appendReviewCallLogEntry(callLogPath, *service, v.Dimensions, reviewLatency, stderr)

	// C2-1 findings bridge, via review.BuildPersist — the one composition
	// shared with the review_diff MCP tool: mint content-addressed claim
	// IDs (hashed cites + scope-fp anchors), label already-refuted
	// findings, and compute the fixed-findings reconcile. With --no-persist
	// the same computation runs for the labels but nothing is written.
	existing, _ := c.ClaimsForService(ctx, *service, review.ClaimsLayer)
	watermark := ""
	if _, indexedCommit, found, merr := c.ServiceMeta(ctx, *service); merr == nil && found {
		watermark = indexedCommit
	}
	content := make(map[string][]byte, len(sources))
	for _, f := range sources {
		content[f.Path] = f.Content
	}
	claims, links, supersede := review.BuildPersist(&v, existing, review.PersistInput{
		Now:       time.Now,
		Watermark: watermark,
		FileContent: func(path string) ([]byte, bool) {
			data, ok := content[path]
			return data, ok
		},
		ScopeFingerprint: func(path string) (string, bool) {
			hash, found, ferr := c.SourceFingerprint(ctx, *service, path)
			return hash, ferr == nil && found
		},
	})
	persisted := false
	if !*noPersist {
		persisted = true
		if len(claims) > 0 {
			if perr := c.PutClaims(ctx, claims, links); perr != nil {
				persisted = false
				fmt.Fprintf(stderr, "warning: persisting review findings failed: %v\n", perr)
			}
		}
		var supersededOK []string
		for _, id := range supersede {
			if serr := c.SetClaimStatus(ctx, *service, id, core.ClaimSuperseded); serr != nil {
				persisted = false
				fmt.Fprintf(stderr, "warning: superseding fixed finding %s failed: %v\n", id, serr)
				continue
			}
			supersededOK = append(supersededOK, id)
		}
		// Q-7 (docs/plans/expert-suite-q.md): record a "superseded"
		// claim_event for every finding this run actually reconciled as
		// fixed — Interventions row 2's real mechanism
		// (docs/design/observability-surfaces.md §1). Best-effort: a
		// logging failure here never flips persisted/the exit code, only
		// SetClaimStatus's own success above does.
		//
		// Resolution classification (docs/design/findings-as-evidence.md
		// §3.4, §6 item 4): resolve looks up the PRE-run claim's own Scope
		// (from `existing`, loaded before this run superseded anything)
		// and classifies via review.ClassifyResolution against the
		// current source set — code-change-in-scope when every scoped
		// file still exists, scope-disappearance when one was deleted.
		existingByID := make(map[string]core.SemanticClaim, len(existing))
		for _, cl := range existing {
			existingByID[cl.ClaimID] = cl
		}
		fileExists := func(path string) bool { _, ok := content[path]; return ok }
		resolve := func(claimID string) (kind, commit string) {
			cl, ok := existingByID[claimID]
			if !ok {
				return "", ""
			}
			return review.ClassifyResolution(cl.Scope, fileExists), watermark
		}
		recordReviewSupersedeEvents(ctx, c, supersededOK, resolve, time.Now, stderr)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v.Payload(persisted)); err != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printReviewSummary(stdout, v)
	return 0
}

// buildReviewInputFromClient does every I/O review.Run itself never
// performs, using pkg/client.Client — mirrors internal/mcp/review_tool.go's
// buildReviewInput exactly, over the CLI's direct-store client instead of
// the MCP QueryInterface abstraction. The full source list is also
// returned: the persist step needs it to hash cited lines (semantic.Sweep's
// contract), including arch citations on files OUTSIDE the changed set.
func buildReviewInputFromClient(ctx context.Context, c *client.Client, service string, changedFiles []string) (review.Input, []core.SourceFile, error) {
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return review.Input{}, nil, fmt.Errorf("list symbols: %w", err)
	}
	typed, _ := c.TypedEdges(ctx, service)

	in := review.Input{
		Service:              service,
		ChangedFiles:         changedFiles,
		Nodes:                nodes,
		TypedEdges:           typed,
		ObligationsSupported: true,
		ObligationsByFile:    make(map[string][]review.ObligationRule, len(changedFiles)),
	}
	for _, f := range changedFiles {
		obl, oerr := c.Obligations(ctx, service, f, 0)
		if oerr != nil {
			continue // best-effort per file: a lookup error degrades that file's rules, not the whole dimension
		}
		for _, r := range obl.Rules {
			in.ObligationsByFile[f] = append(in.ObligationsByFile[f], review.ObligationRule{
				File: f, Rule: r.Rule, Src: r.Src, Tier: r.Tier, Kind: r.Kind, Status: r.Status,
			})
		}
	}

	corpusRoot := ""
	if path, _, found, merr := c.ServiceMeta(ctx, service); merr == nil && found {
		corpusRoot = path
	}
	rules, source, deny, checked, reason, rerr := projectrules.ResolveRules(corpusRoot, nil, false)
	if rerr != nil {
		return review.Input{}, nil, fmt.Errorf("project rules: %w", rerr)
	}
	in.ArchRules = rules
	in.ArchRuleSource = source
	in.ArchDenyByDefault = deny
	in.ArchChecked = checked
	in.ArchReason = reason

	sources, serr := c.Sources(ctx, service)
	if serr == nil {
		changedSet := make(map[string]bool, len(changedFiles))
		for _, f := range changedFiles {
			changedSet[f] = true
		}
		for _, f := range sources {
			if changedSet[f.Path] {
				in.ChangedSources = append(in.ChangedSources, f)
			}
		}
	}

	return in, sources, nil
}

// printReviewSummary renders v as a short, human-readable report — the form
// the pre-push hook (C3-1) prints on every push, so it stays scannable in a
// terminal: one line per dimension, findings indented beneath, refuted
// findings marked but not hidden (visible for audit, matching the JSON
// payload's own "carries refuted status, never silently dropped" contract).
func printReviewSummary(w io.Writer, v review.Verdict) {
	fmt.Fprintf(w, "review: %s (%d changed file(s))\n", v.Service, len(v.ChangedFiles))
	fmt.Fprintln(w, v.Summary)
	for _, d := range v.Dimensions {
		mark := "[x]"
		if !d.Checked {
			mark = "[ ]"
		}
		fmt.Fprintf(w, "%s %s: %d active finding(s)\n", mark, d.Name, d.Count)
		if !d.Checked {
			fmt.Fprintf(w, "    reason: %s\n", d.Reason)
			continue
		}
		for _, f := range d.Findings {
			suffix := ""
			if f.Status != "" {
				suffix = fmt.Sprintf(" (%s)", f.Status)
			}
			cite := f.Citation
			if cite == "" {
				cite = f.File
			}
			fmt.Fprintf(w, "  - [%s] %s%s", f.Severity, f.Message, suffix)
			if cite != "" {
				fmt.Fprintf(w, " (%s)", cite)
			}
			fmt.Fprintln(w)
		}
	}
}

// reviewVerdictOutcome computes review's gate-shaped result summary (T5-7,
// docs/plans/trust-and-tokenomics.md): active-finding counts keyed by
// dimension name, for CHECKED dimensions only — an unchecked dimension
// (checked:false, e.g. no arch rule pack configured) never ran and has no
// finding count to report, so it is omitted rather than reported as a
// misleading 0. Returns nil when nothing was checked at all: nothing
// gate-relevant to log.
func reviewVerdictOutcome(dims []review.Dimension) map[string]int {
	out := make(map[string]int, len(dims))
	for _, d := range dims {
		if !d.Checked {
			continue
		}
		out[d.Name] = d.Count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// appendReviewCallLogEntry appends one call-log-shaped JSONL row for this
// CLI `fathom review` run (T5-7) — C3-1's forced channel (lefthook.yml's
// pre-push review step, `bin/fathom review --no-persist`), which never goes
// through the MCP dispatch path (internal/mcp/dispatch.go) and so was
// otherwise invisible to `instrument --adoption`'s gate-catch accounting.
// Uses the same JSONL wire shape internal/mcp/calllog.go's CallLogger
// writes (adoptionCallEntry, instrument_adoption.go's own decoupled mirror
// of that shape) so readCallLog reads CLI and MCP rows identically —
// Surface "cli" distinguishes this forced channel from an MCP surface in
// the adoption report's surface-traffic table. Best-effort and silent
// on failure beyond a stderr warning: a logging side effect must never
// fail the caller (§5.3's fail-open convention — the pre-push hook this
// feeds always exits 0 regardless).
func appendReviewCallLogEntry(path, service string, dims []review.Dimension, latency time.Duration, stderr io.Writer) {
	outcome := reviewVerdictOutcome(dims)
	if outcome == nil {
		return
	}
	entry := adoptionCallEntry{
		TS:             time.Now().UTC().Format(time.RFC3339Nano),
		Tool:           "review",
		Service:        service,
		Surface:        "cli",
		VerdictOutcome: outcome,
		LatencyMS:      float64(latency.Milliseconds()),
	}
	b, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(stderr, "warning: encoding review call-log entry: %v\n", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "warning: review call-log dir %q: %v\n", filepath.Dir(path), err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "warning: opening review call log %q: %v\n", path, err)
		return
	}
	defer f.Close()
	b = append(b, '\n')
	if _, err := f.Write(b); err != nil {
		fmt.Fprintf(stderr, "warning: writing review call log %q: %v\n", path, err)
	}
}

// recordReviewSupersedeEvents (Q-7, docs/plans/expert-suite-q.md; resolution
// classification added by docs/design/findings-as-evidence.md §3.4, §6 item
// 4) records one "superseded" claim_event per ID in supersededIDs —
// findings runReview just reconciled as fixed (SetClaimStatus already
// succeeded for every ID passed here; this is the durable "raised, then
// reconciled" evidence docs/design/observability-surfaces.md §1's
// Interventions row 2 names as its mechanism). resolve (nil-safe) resolves a
// claim ID to its ResolutionKind/ResolutionCommit — the caller
// (runReview) builds it from review.ClassifyResolution over the pre-run
// claim's own Scope plus the current source set, so recordReviewSupersedeEvents
// itself stays a thin, injectable write path. Best-effort, mirroring
// appendReviewCallLogEntry's own discipline: a write failure is logged and
// never returned as an error — the review run's exit code depends only on
// SetClaimStatus's own success, never on this instrumentation. Returns the
// count actually recorded, for tests.
func recordReviewSupersedeEvents(ctx context.Context, c *client.Client, supersededIDs []string, resolve func(claimID string) (kind, commit string), now func() time.Time, stderr io.Writer) int {
	recorded := 0
	for _, id := range supersededIDs {
		var kind, commit string
		if resolve != nil {
			kind, commit = resolve(id)
		}
		ev := core.ClaimEvent{
			ClaimID:          id,
			Verdict:          "superseded",
			Author:           "review",
			Note:             "finding fixed — no longer emitted by a review run (fathom review's fixed-finding reconcile)",
			At:               now().UTC(),
			ResolutionKind:   kind,
			ResolutionCommit: commit,
		}
		if err := c.AppendClaimEvent(ctx, ev); err != nil {
			fmt.Fprintf(stderr, "warning: recording superseded claim_event for %s failed: %v\n", id, err)
			continue
		}
		recorded++
	}
	return recorded
}
