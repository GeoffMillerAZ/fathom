package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	storePath := fs.String("store", "workspace/db/fathom.sqlite", "store path (.duckdb or .sqlite/.db)")
	service := fs.String("service", "", "service ID (required)")
	since := fs.String("since", "", "git ref to diff against (required; e.g. origin/main)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	noPersist := fs.Bool("no-persist", false, "report only: never write findings to the claims store (the pre-push summary hook uses this)")
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
	v := review.Run(in)

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
		for _, id := range supersede {
			if serr := c.SetClaimStatus(ctx, *service, id, core.ClaimSuperseded); serr != nil {
				persisted = false
				fmt.Fprintf(stderr, "warning: superseding fixed finding %s failed: %v\n", id, serr)
			}
		}
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
