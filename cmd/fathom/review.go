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
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
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
	since := fs.String("since", "", "git ref to diff against (mutually exclusive with --manifest; one of the two is required)")
	manifestPath := fs.String("manifest", "", "explicit changed-path manifest FILE (JSON [{\"path\":\"…\",\"sha256\":\"…\"}], mutually exclusive with --since); each path's CURRENT sha256 must match or the run refuses with \"manifest mismatch: <path>\" and exit 2 — never a silent re-read")
	jsonOut := fs.Bool("json", false, "emit JSON (the ReviewResponse envelope) instead of a human-readable summary")
	format := fs.String("format", "text", "output format: \"text\" (default) or \"github\" (one deterministic Markdown PR comment — changed symbols with resolved callers and tier, call-site completeness, governing rules, a verify line for touched plan rows, affected tests/test gap, coverage; overrides --json); with --receipt, \"sarif\" or \"check\" instead")
	receipt := fs.Bool("receipt", false, "render ONLY the change risk receipt (see docs/design/change-and-compliance.md) through the shared emitter, instead of the full review — requires --format sarif or check. Fathom never posts, approves or blocks on its own authority from this output: the check-run's conclusion is always \"neutral\"; CI decides what a band does.")
	outPath := fs.String("out", "", "write the review report to this file instead of stdout (a GitHub Action step captures --format github's comment body here, then hands the file to the upsert-comment step); stdout then carries only a short confirmation line")
	noPersist := fs.Bool("no-persist", false, "report only: never write findings to the claims store (the pre-push summary hook uses this); equivalent to --mode report")
	mode := fs.String("mode", "", "\"report\" (never writes; --no-persist is an alias) or \"persist\" (default): write findings/idempotency records to the store, pinned by a store-state equality test on report mode including the error path")
	idempotencyKey := fs.String("idempotency-key", "", "persist mode only: same key + same payload digest replays the recorded response and writes nothing else; same key + a different payload digest refuses (exit 2)")
	expectIndexWatermark := fs.String("expect-index-watermark", "", "refuse to trust the index if it disagrees: a mismatch sets stale_index:true in the response (dimensions still run in report mode) and refuses persist")
	budgetSeconds := fs.Int("budget-seconds", 0, "advisory time budget: on excess, print a named skip and exit 0 (requires --mode report / --no-persist — a bounded run must never be a store writer); 0 = unbounded")
	callLogFlag := fs.String("call-log", "", "MCP-format call log (JSONL) to append this run's verdict outcome to (T5-7); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	var coverageProfileFlags profileFlagValues
	fs.Var(&coverageProfileFlags, "coverage-profile", "Go cover profile (go test -coverprofile) to join against changed symbols for the coverage dimension, as `[layer=]path` (repeatable — one per test layer, e.g. --coverage-profile unit=workspace/cov/unit.out --coverage-profile integration=workspace/cov/integration.out); a bare path defaults to layer \"unit\". Absent = coverage dimension reports checked:false.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: review requires --service")
		fs.Usage()
		return 1
	}
	if (*since == "") == (*manifestPath == "") {
		fmt.Fprintln(stderr, "error: review requires exactly one of --since or --manifest")
		fs.Usage()
		return 1
	}
	switch {
	case *receipt && *format != "sarif" && *format != "check":
		fmt.Fprintf(stderr, "error: --receipt requires --format sarif or check, got %q\n", *format)
		return 1
	case !*receipt && *format != "text" && *format != "github":
		fmt.Fprintf(stderr, "error: --format must be \"text\" or \"github\", got %q\n", *format)
		return 1
	}
	effectiveMode := *mode
	switch {
	case effectiveMode != "" && effectiveMode != "report" && effectiveMode != "persist":
		fmt.Fprintf(stderr, "error: --mode must be \"report\" or \"persist\", got %q\n", effectiveMode)
		return 1
	case effectiveMode == "persist" && *noPersist:
		fmt.Fprintln(stderr, "error: --no-persist conflicts with --mode persist")
		return 1
	case effectiveMode == "" && *noPersist:
		effectiveMode = "report"
	case effectiveMode == "":
		effectiveMode = "persist"
	}
	if *idempotencyKey != "" && effectiveMode != "persist" {
		fmt.Fprintln(stderr, "error: --idempotency-key requires --mode persist")
		return 1
	}
	if *budgetSeconds > 0 && effectiveMode != "report" {
		fmt.Fprintln(stderr, "error: --budget-seconds requires --mode report (or --no-persist) — a budget-bounded run can be cut off mid-flight, and a run that can be cut off must never be a store writer")
		return 1
	}

	body := func() int {
		return runReviewBody(gitRunner, *storePath, *service, *since, *manifestPath, effectiveMode, *idempotencyKey, *expectIndexWatermark, *format, *jsonOut, *receipt, *callLogFlag, *outPath, coverageProfileFlags, stdout, stderr)
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
		//fathom:tenancy-allow bare-go CLI: the body raced against its timeout on a buffered channel
		go func() { done <- body() }()
		select {
		case rc := <-done:
			return rc
		case <-time.After(time.Duration(*budgetSeconds) * time.Second):
			refFlag := "--since " + *since
			if *manifestPath != "" {
				refFlag = "--manifest " + *manifestPath
			}
			fmt.Fprintf(stdout, "review: budget exceeded (%ds) — summary skipped this push (advisory, fail-open; run `fathom review --service %s %s --no-persist` directly for the full report)\n", *budgetSeconds, *service, refFlag)
			return 0
		}
	}
	return body()
}

func runReviewBody(gitRunner func(args ...string) ([]byte, error), storePathV, serviceV, sinceV, manifestPathV, modeV, idempotencyKeyV, expectWatermarkV, formatV string, jsonOutV, receiptV bool, callLogFlagV, outPathV string, coverageProfilesV []string, stdout, stderr io.Writer) int {
	storePath, service, since, format, jsonOut, callLogFlag := &storePathV, &serviceV, &sinceV, &formatV, &jsonOutV, &callLogFlagV

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

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ctx := context.Background()

	// corpusRoot/watermark are resolved BEFORE the changed-file list: a
	// --manifest verification needs corpusRoot to resolve each path's
	// current content, and --expect-index-watermark needs watermark to
	// decide stale_index before running any dimension.
	watermark, corpusRoot := "", ""
	if path, indexedCommit, found, merr := c.ServiceMeta(ctx, *service); merr == nil && found {
		watermark = indexedCommit
		corpusRoot = path
	}

	var changedFiles []string
	if manifestPathV != "" {
		entries, merr := loadReviewManifest(manifestPathV)
		if merr != nil {
			fmt.Fprintf(stderr, "error: %v\n", merr)
			return 1
		}
		files, mismatch := verifyReviewManifest(corpusRoot, entries)
		if mismatch != "" {
			fmt.Fprintf(stderr, "error: manifest mismatch: %s\n", mismatch)
			return 2
		}
		changedFiles = files
	} else {
		out, gerr := gitRunner("diff", "--name-only", *since)
		if gerr != nil {
			fmt.Fprintf(stderr, "error: git diff --name-only %s: %v\n", *since, gerr)
			return 1
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				changedFiles = append(changedFiles, line)
			}
		}
	}
	if len(changedFiles) == 0 {
		ref := *since
		if ref == "" {
			ref = manifestPathV
		}
		fmt.Fprintf(stdout, "review: no changed files since %s — nothing to review\n", ref)
		return 0
	}

	// stale_index (FL-15): a mismatch against --expect-index-watermark
	// never blocks the report — dimensions still run over whatever the
	// index currently holds — but it always refuses persist.
	staleIndex := expectWatermarkV != "" && expectWatermarkV != watermark
	persistThisRun := modeV == "persist" && !staleIndex

	// Idempotency (FL-15): only reachable in persist mode (enforced by the
	// flag validation in runReview) — a repeated key with the SAME payload
	// digest replays the recorded response and writes nothing else; the
	// same key with a DIFFERENT digest refuses loudly (exit 2), never a
	// silent overwrite of a conflicting request.
	payloadDigest := ""
	if idempotencyKeyV != "" {
		payloadDigest = reviewPayloadDigest(*service, modeV, *since, manifestPathV, changedFiles)
		if existing, found, rerr := st.ReviewRun(ctx, *service, idempotencyKeyV); rerr == nil && found {
			if existing.PayloadDigest == payloadDigest {
				fmt.Fprint(reportW, existing.ResponseJSON)
				return finishReview(0)
			}
			fmt.Fprintln(stderr, "error: idempotency key reused with a different payload")
			return 2
		}
	}

	in, sources, err := buildReviewInputFromClient(ctx, c, st, *service, changedFiles, gitRunner, sinceV)
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
	// OC-2: the readers' own resolution (reviewCallLogPath).
	appendReviewCallLogEntry(reviewCallLogPath(*callLogFlag, *service), *service, v.Dimensions, reviewLatency, stderr)

	// C2-1 findings bridge, via review.BuildPersist — the one composition
	// shared with the review_diff MCP tool: mint content-addressed claim
	// IDs (hashed cites + scope-fp anchors), label already-refuted
	// findings, and compute the fixed-findings reconcile. In report mode
	// (or when stale_index refused persist) the same computation runs for
	// the labels but nothing is written — watermark/corpusRoot were already
	// resolved above, before the changed-file list, for the manifest/
	// stale-index checks.
	existing, _ := c.ClaimsForService(ctx, *service, review.ClaimsLayer)
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
	// SU-22 (ADR-041 decision 7): every claim this run mints is asserted by
	// the run's own idempotency key when the caller gave one (--idempotency-
	// key), else its corpus watermark as the run identity surrogate — every
	// claim from the SAME run shares one AssertedBy either way.
	reviewRunID := idempotencyKeyV
	if reviewRunID == "" {
		reviewRunID = watermark
	}
	for i := range claims {
		claims[i].AssertedBy = core.AssertedBy("review", reviewRunID)
	}
	persisted := false
	if persistThisRun {
		persisted = true
		if len(claims) > 0 {
			if perr := c.PutClaims(ctx, claims, links); perr != nil {
				persisted = false
				fmt.Fprintf(stderr, "warning: persisting review findings failed: %v\n", perr)
			}
		}
		// Resolution classification (docs/design/findings-as-evidence.md
		// §3.4, §6 item 4): resolve looks up the PRE-run claim's own Scope
		// (from `existing`, loaded before this run superseded anything)
		// and classifies via review.ClassifyResolution against the
		// current source set — code-change-in-scope when every scoped
		// file still exists, scope-disappearance when one was deleted.
		// Built BEFORE the supersede loop below: SU-20's WriteSupersedes
		// call also needs each predecessor's own Subject to look up its
		// successor.
		existingByID := make(map[string]core.SemanticClaim, len(existing))
		for _, cl := range existing {
			existingByID[cl.ClaimID] = cl
		}

		supersedeAt := time.Now().UTC()
		var supersededOK []string
		for _, id := range supersede {
			if serr := c.SetClaimStatus(ctx, *service, id, core.ClaimSuperseded); serr != nil {
				persisted = false
				fmt.Fprintf(stderr, "warning: superseding fixed finding %s failed: %v\n", id, serr)
				continue
			}
			// SU-19 (ADR-041 decision 7): close the predecessor's
			// bi-temporal validity window at the same instant its status
			// flips — best-effort, mirroring the claim_event recording
			// below: a failure here never flips persisted/the exit code,
			// only SetClaimStatus's own success above does.
			if verr := c.SetClaimValidTo(ctx, *service, id, supersedeAt); verr != nil {
				fmt.Fprintf(stderr, "warning: closing validity window for %s failed: %v\n", id, verr)
			}
			// SU-20: a re-review that re-emits the SAME subject under a
			// DIFFERENT content-addressed claim id (the rule text changed)
			// gets an explicit supersedes link to its predecessor — see
			// review.SupersedeSuccessor's own doc comment for why a
			// genuinely fixed finding (no successor) never calls this.
			if cl, ok := existingByID[id]; ok {
				if successorID, sok := review.SupersedeSuccessor(v, cl.Subject); sok {
					if werr := c.WriteSupersedes(ctx, *service, successorID, id, "review reconciliation: fixed/changed finding"); werr != nil {
						fmt.Fprintf(stderr, "warning: recording supersedes link %s -> %s failed: %v\n", successorID, id, werr)
					}
				}
			}
			supersededOK = append(supersededOK, id)
		}
		// Q-7 (docs/plans/expert-suite-q.md): record a "superseded"
		// claim_event for every finding this run actually reconciled as
		// fixed — Interventions row 2's real mechanism
		// (docs/design/observability-surfaces.md §1). Best-effort: a
		// logging failure here never flips persisted/the exit code, only
		// SetClaimStatus's own success above does.
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

	// FL-16: the P0 response envelope — built on every run regardless of
	// --format/--json, since idempotency recording below always stores the
	// SAME JSON shape a --json caller would see, whatever this run actually
	// prints.
	coverage := review.BuildCoverage(v, false, staleIndex, false)
	resp := review.BuildResponse(
		v, modeV,
		review.ProducerInfo{Name: "fathom", Version: buildinfo.Version(), Build: buildinfo.Read().BuiltAt},
		review.ReviewInputs{
			ManifestDigest:    reviewManifestDigest(changedFiles, content),
			SourceWatermark:   sourceWatermarkLabel(*since, manifestPathV),
			IndexWatermark:    watermark,
			RulesDigest:       reviewRulesDigest(corpusRoot),
			PlansDigest:       reviewPlansDigest(corpusRoot),
			ExtractorVersions: map[string]string{"fathom": buildinfo.Version()},
		},
		persisted, coverage, false, staleIndex, false,
		review.UnanchoredPaths(changedFiles, in.Nodes),
	)
	responsePayload := resp.Payload()
	responsePayload["test_impact"] = gap.Payload()
	responseJSON, jerr := json.Marshal(responsePayload)
	if jerr != nil {
		fmt.Fprintf(stderr, "error: encode response JSON: %v\n", jerr)
		return 1
	}

	// Idempotency recording (FL-15): only when a key was given and stale
	// index didn't refuse persist — a stale-index run's response is never
	// cached for replay, since it did not answer the request the key names.
	if idempotencyKeyV != "" && !staleIndex {
		if rerr := st.RecordReviewRun(ctx, core.ReviewRun{
			ServiceID:      *service,
			IdempotencyKey: idempotencyKeyV,
			PayloadDigest:  payloadDigest,
			ResponseJSON:   string(responseJSON),
			RecordedAt:     time.Now().UTC(),
		}); rerr != nil {
			fmt.Fprintf(stderr, "warning: recording idempotency key %q failed: %v\n", idempotencyKeyV, rerr)
		}
	}

	// CX-6 (docs/plans/change-and-compliance.md, design doc §2): --receipt
	// renders ONLY the change risk receipt, through the shared SARIF/
	// check-run emitter, instead of the full review below — runReview's own
	// flag validation already refused any --format but sarif/check when
	// receiptV is true.
	if receiptV {
		table := riskreview.RiskTableFor(corpusRoot)
		receiptOut := buildCLIRiskReceipt(in, v, table)
		// EVX-3: the SARIF run's provenance — the indexed commit, the
		// store's fingerprints and the service's definitions (ADR-065).
		anchorRun := receipt.Run{Dir: corpusRoot, Rev: watermark, Service: *service, Nodes: in.Nodes,
			RepositoryURIAt: receipt.ConfiguredRepositoryURI,
			Stored: func(p string) (string, bool) {
				fp, found, ferr := st.Fingerprint(ctx, *service, p)
				return fp, ferr == nil && found
			}}
		return finishReview(cmdReviewReceipt(reportW, formatV, anchorRun, receiptOut))
	}

	// VB-22 (docs/plans/verbs.md; docs/vision/29-verbs-not-lookups.md §3
	// wave F): --format github renders ONE deterministic Markdown PR
	// comment over the SAME verdict/gap this run already computed —
	// review.RenderGitHubReview does no I/O itself, so every dependency
	// (changed-symbol/caller detail, touched plan rows) is resolved here
	// first. Unchanged by FL-15/FL-16: the marker-prefix contract
	// (review.GitHubReviewMarker, VB-23's PR-comment upsert step) is
	// load-bearing, so the mode line goes on text output only.
	if *format == "github" {
		diffRange := *since + "..HEAD"
		if *since == "" {
			diffRange = "manifest:" + manifestPathV
		}
		fmt.Fprint(reportW, review.RenderGitHubReview(review.GitHubReviewInput{
			Verdict:         v,
			TestGap:         gap,
			IndexCommit:     watermark,
			DiffRange:       diffRange,
			ChangedSymbols:  review.ChangedSymbolReviews(in.Nodes, in.TypedEdges, in.ChangedFiles),
			TouchedPlanRows: resolveTouchedPlanRows(corpusRoot, in),
		}))
		return finishReview(0)
	}

	if *jsonOut {
		var buf bytes.Buffer
		if err := json.Indent(&buf, responseJSON, "", "  "); err != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", err)
			return 1
		}
		buf.WriteByte('\n')
		if _, err := reportW.Write(buf.Bytes()); err != nil {
			fmt.Fprintf(stderr, "error: write JSON: %v\n", err)
			return 1
		}
		return finishReview(0)
	}

	fmt.Fprintf(reportW, "mode: %s\n", modeV)
	printReviewSummary(reportW, v)
	renderTestGap(reportW, gap)
	return finishReview(0)
}

// sourceWatermarkLabel names what this run diffed against — ReviewInputs'
// source_watermark — without shelling out to git a second time: the --since
// ref itself when given, or "manifest:<path>" when the changed set came
// from an explicit --manifest instead of a ref.
func sourceWatermarkLabel(since, manifestPath string) string {
	if since != "" {
		return since
	}
	if manifestPath != "" {
		return "manifest:" + manifestPath
	}
	return ""
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
func buildReviewInputFromClient(ctx context.Context, c *client.Client, st store.Store, service string, changedFiles []string, gitRunner func(args ...string) ([]byte, error), since string) (review.Input, []core.SourceFile, error) {
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

	// certifications (vision 33 / ADR-043 stage 1, TC-10): *client.Client
	// is a direct store passthrough, so ClaimsForService is always present
	// here (unlike the MCP path's optional certificationsBackend
	// assertion) — Supported is unconditionally true, mirroring
	// ObligationsSupported above. Declared reflects the LAYER's own
	// service-wide population (design doc §4's "checked:false when the
	// layer is empty" — not merely "nothing in this diff").
	in.CertificationsSupported = true
	if certClaims, cerr := c.ClaimsForService(ctx, service, certify.Layer); cerr == nil {
		records := certify.Records(certClaims)
		in.CertificationsDeclared = len(records) > 0
		if in.CertificationsDeclared {
			changedSet := make(map[string]bool, len(changedFiles))
			for _, f := range changedFiles {
				changedSet[f] = true
			}
			in.CertificationsByFile = make(map[string][]review.CertificationRecord, len(changedFiles))
			for _, r := range records {
				for _, f := range r.Scope {
					if !changedSet[f] {
						continue
					}
					in.CertificationsByFile[f] = append(in.CertificationsByFile[f], review.CertificationRecord{
						Subject: r.Subject, Test: r.Test, Strength: r.Strength, State: r.State,
						Tier: r.Tier, Rule: r.Rule, Status: r.Status, Scope: r.Scope,
					})
				}
			}
		}
	}

	corpusRoot := ""
	if path, _, found, merr := c.ServiceMeta(ctx, service); merr == nil && found {
		corpusRoot = path
	}
	// CV-13 (ADR-044 §1.6): Root is always filled; Base only when --since
	// was given AND resolves — a bad ref fails open (Base stays ""), the
	// SAME discipline internal/mcp's reviewDiffWithBase uses, so a review
	// run never blocks on a base-aware dimension alone.
	in.Root = corpusRoot
	if since != "" {
		if out, gerr := gitRunner("rev-parse", since); gerr == nil {
			in.Base = strings.TrimSpace(string(out))
		}
	}
	// CV-12 (ADR-044 §1.6): the "integrity" conserve plane, over exactly
	// the base just resolved — a no-op (leaves IntegrityFindings empty)
	// when Base is still "", so DimIntegrity's own "needs a base ref" gate
	// is the only thing a caller with no --since ever sees.
	applyIntegrityDimension(ctx, &in, gitRunner, corpusRoot, st, service)
	// DimConservation (ADR-044 §1.6/§2, wave 2): the same base just
	// resolved, run through every registered conserve.Plane — a no-op
	// (leaves ConservationSupported false) when Base is still "".
	applyConservationDimension(ctx, &in, st, corpusRoot, service)
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
	if path == "" {
		return // the daemon runs with logging off: there is no log to append to
	}
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

// reviewRunner runs `fathom review` (registered below; EW-45).
type reviewRunner struct{}

func (reviewRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdReview(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "review", Group: "review", Order: 20, Run: reviewRunner{},
		Summary: "Store-direct review of files changed since a git ref (arch conformance, spec coverage); --mode report|persist (--no-persist = report only), --manifest FILE, --idempotency-key, --expect-index-watermark, --budget-seconds = advisory bound"})
}
