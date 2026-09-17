package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
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
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	service := fs.String("service", "", "service ID (required)")
	since := fs.String("since", "", "git ref to diff against (required; e.g. origin/main)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	format := fs.String("format", "text", "output format: \"text\" (default) or \"github\" (VB-22: one deterministic Markdown PR comment — changed symbols with resolved callers and tier, call-site completeness, governing rules, a verify line for touched plan rows, affected tests/test gap, coverage; overrides --json)")
	outPath := fs.String("out", "", "write the review report to this file instead of stdout (VB-23: a GitHub Action step captures --format github's comment body here, then hands the file to the upsert-comment step — docs/adoption/github-action.md); stdout then carries only a short confirmation line")
	noPersist := fs.Bool("no-persist", false, "report only: never write findings to the claims store (the pre-push summary hook uses this)")
	budgetSeconds := fs.Int("budget-seconds", 0, "advisory time budget: on excess, print a named skip and exit 0 (requires --no-persist — a bounded run must never be a store writer); 0 = unbounded")
	callLogFlag := fs.String("call-log", "", "MCP-format call log (JSONL) to append this run's verdict outcome to (T5-7); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	var coverageProfileFlags profileFlagValues
	fs.Var(&coverageProfileFlags, "coverage-profile", "Go cover profile (go test -coverprofile) to join against changed symbols for the coverage dimension, as `[layer=]path` (repeatable — one per test layer, e.g. --coverage-profile unit=workspace/cov/unit.out --coverage-profile integration=workspace/cov/integration.out); a bare path defaults to layer \"unit\". Absent = coverage dimension reports checked:false.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" || *since == "" {
		fmt.Fprintln(stderr, "error: review requires --service and --since")
		fs.Usage()
		return 1
	}
	if *format != "text" && *format != "github" {
		fmt.Fprintf(stderr, "error: --format must be \"text\" or \"github\", got %q\n", *format)
		return 1
	}
	if *budgetSeconds > 0 && !*noPersist {
		fmt.Fprintln(stderr, "error: --budget-seconds requires --no-persist — a budget-bounded run can be cut off mid-flight, and a run that can be cut off must never be a store writer")
		return 1
	}

	body := func() int {
		return runReviewBody(gitRunner, *storePath, *service, *since, *format, *jsonOut, *noPersist, *callLogFlag, *outPath, coverageProfileFlags, stdout, stderr)
	}
	if *budgetSeconds > 0 {
		// The whole body races the budget. On excess the process prints a
		// named advisory skip and exits 0 — the pre-push hook's lefthook
		// output is buffered, so an unbounded slow review is
		// indistinguishable from a hang to whoever is watching the push
		// (O9's actual root cause: a 22s+ review plus buffering read as a
		// stall and got killed). The abandoned goroutine dies with the
		// process; it holds no store write path (--no-persist enforced
		// above).
		done := make(chan int, 1)
		go func() { done <- body() }()
		select {
		case rc := <-done:
			return rc
		case <-time.After(time.Duration(*budgetSeconds) * time.Second):
			fmt.Fprintf(stdout, "review: budget exceeded (%ds) — summary skipped this push (advisory, fail-open; run `fathom review --service %s --since %s --no-persist` directly for the full report)\n", *budgetSeconds, *service, *since)
			return 0
		}
	}
	return body()
}

func runReviewBody(gitRunner func(args ...string) ([]byte, error), storePathV, serviceV, sinceV, formatV string, jsonOutV, noPersistV bool, callLogFlagV, outPathV string, coverageProfilesV []string, stdout, stderr io.Writer) int {
	storePath, service, since, format, jsonOut, noPersist, callLogFlag := &storePathV, &serviceV, &sinceV, &formatV, &jsonOutV, &noPersistV, &callLogFlagV

	// --out (VB-23): redirect the report body (github Markdown, JSON, or the
	// human text summary — never the operational notices/errors above, which
	// always go to the real stdout/stderr) into a buffer, then flush it to
	// outPathV on every exit path via finishReview. A CI step then hands the
	// file straight to the comment-upsert step instead of scraping stdout.
	reportW := stdout
	var reportBuf *bytes.Buffer
	if outPathV != "" {
		reportBuf = &bytes.Buffer{}
		reportW = reportBuf
	}
	finishReview := func(code int) int {
		if reportBuf == nil {
			return code
		}
		if werr := os.WriteFile(outPathV, reportBuf.Bytes(), 0o644); werr != nil {
			fmt.Fprintf(stderr, "error: writing --out %q: %v\n", outPathV, werr)
			return 1
		}
		fmt.Fprintf(stdout, "review: wrote %s report to %s\n", *format, outPathV)
		return code
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
	if len(coverageProfilesV) > 0 {
		if err := applyCoverageProfile(&in, coverageProfilesV); err != nil {
			fmt.Fprintf(stderr, "error: --coverage-profile %v: %v\n", coverageProfilesV, err)
			return 1
		}
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
	watermark, corpusRoot := "", ""
	if path, indexedCommit, found, merr := c.ServiceMeta(ctx, *service); merr == nil && found {
		watermark = indexedCommit
		corpusRoot = path
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

	// BD-5 (docs/plans/blast-depth.md): the same test_impact composition
	// review_diff emits (internal/mcp's reviewTestGap → impact.TestGap),
	// over the same inputs, so the CLI mirror names affected tests and the
	// test gap too — --json carries the identical block, text gets
	// renderTestGap.
	gap := impact.TestGap(in.Nodes, in.ChangedFiles, in.TypedEdges)

	// VB-22 (docs/plans/verbs.md; docs/vision/29-verbs-not-lookups.md §3
	// wave F): --format github renders ONE deterministic Markdown PR
	// comment over the SAME verdict/gap this run already computed —
	// review.RenderGitHubReview does no I/O itself, so every dependency
	// (changed-symbol/caller detail, touched plan rows) is resolved here
	// first.
	if *format == "github" {
		fmt.Fprint(reportW, review.RenderGitHubReview(review.GitHubReviewInput{
			Verdict:         v,
			TestGap:         gap,
			IndexCommit:     watermark,
			DiffRange:       *since + "..HEAD",
			ChangedSymbols:  review.ChangedSymbolReviews(in.Nodes, in.TypedEdges, in.ChangedFiles),
			TouchedPlanRows: resolveTouchedPlanRows(corpusRoot, in),
		}))
		return finishReview(0)
	}

	if *jsonOut {
		payload := v.Payload(persisted)
		payload["test_impact"] = gap.Payload()
		enc := json.NewEncoder(reportW)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", err)
			return 1
		}
		return finishReview(0)
	}

	printReviewSummary(reportW, v)
	renderTestGap(reportW, gap)
	return finishReview(0)
}

// renderTestGap prints the test-impact block after the dimension summary
// (BD-5): the tests that statically reach the change (run these), each
// with its file:line and the changed symbols it reaches, then the changed
// symbols no test reaches (the gap — unprotected code, cited so a reviewer
// can open it). An unchecked result prints its reason, mirroring the
// dimension lines' own "[ ] … reason:" shape.
func renderTestGap(w io.Writer, gap impact.TestGapResult) {
	if !gap.Checked {
		fmt.Fprintf(w, "[ ] test_impact: not checked\n    reason: %s\n", gap.Reason)
		return
	}
	fmt.Fprintf(w, "[x] test_impact: %d affected test(s), %d changed symbol(s) with no test\n", len(gap.AffectedTests), len(gap.Gap))
	for _, t := range gap.AffectedTests {
		via := ""
		if t.ViaDispatch {
			via = " via interface dispatch"
		}
		fmt.Fprintf(w, "  - run %s (%s) reaches %s%s\n", t.Test, t.Citation, strings.Join(t.Reaches, ", "), via)
	}
	for _, s := range gap.Gap {
		fmt.Fprintf(w, "  - gap %s (%s): no test reaches it\n", s.Symbol, s.Citation)
	}
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

// applyCoverageProfile opens and parses the Go cover profile at path
// (coverage.ParseProfile), joins it against in.Nodes (review.ResolveCoverage
// — the same composition the review_diff MCP tool uses), and sets the
// review.Input coverage fields (RC-41) so DimTestCoverage reports a real
// verdict instead of checked:false. Unlike an ABSENT --coverage-profile
// (an honest "not supplied"), a SUPPLIED-but-unreadable-or-malformed file is
// a hard error: the caller explicitly asked for coverage and got a bad
// file, which must fail loudly rather than silently degrade to
// checked:false.
func applyCoverageProfile(in *review.Input, rawProfiles []string) error {
	labeled, err := loadLabeledProfiles(rawProfiles)
	if err != nil {
		return err
	}
	in.CoverageChecked = true
	in.CoverageLayered = review.ResolveCoverage(labeled, in.Nodes)
	return nil
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
