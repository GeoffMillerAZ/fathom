package main

// findings.go — `fathom findings near|status` (docs/design/findings-as-
// evidence.md §3.6/§3.7, §6 items 6/7): the CLI mirror of the
// findings_near MCP tool (internal/mcp/findings_tools.go). Both verbs open
// the store READ-ONLY (store.MountReadOnly, mirroring cmd/fathom/
// metrics_*.go's own discipline — a CLI query must never contend with the
// daemon's own writes) and delegate every computation to
// internal/analysis/findings, which does no I/O of its own.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/findings"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// findingsLayerName is the layer both the LLM-compiled findings/** corpus
// and the deterministic review-verb bridge persist onto (internal/semantic.
// LayerFindings / internal/analysis/review.ClaimsLayer — both "findings";
// this file avoids importing either package just for the literal, mirroring
// review/claims.go's own "mirrored here as a literal constant" convention
// for a leaf-ish CLI file).
const findingsLayerName = "findings"

func cmdFindings(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: findings requires a verb: near | status | import")
		printFindingsUsage(stderr)
		return 1
	}
	switch args[0] {
	case "near":
		return cmdFindingsNear(args[1:], stdout, stderr)
	case "status":
		return cmdFindingsStatus(args[1:], stdout, stderr)
	case "import":
		return cmdFindingsImport(args[1:], stdout, stderr)
	case "candidates":
		return cmdFindingsCandidates(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown findings verb %q (want near | status | import | candidates)\n", args[0])
		printFindingsUsage(stderr)
		return 1
	}
}

func printFindingsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom findings near <symbol|file> --store PATH --service NAME [--hops N] [--include-superseded] [--json]")
	fmt.Fprintln(w, "  fathom findings status --store PATH --service NAME [--json]")
	fmt.Fprintln(w, "  fathom findings import --store PATH --service NAME --subject ORG/REPO/COMPONENT --producer NAME --file PATH [--dry-run] [--json]")
	fmt.Fprintln(w, "  fathom findings candidates --store PATH --service NAME [--min-sites N] [--json]")
}

// openFindingsClient mounts storePath read-only and wraps it in pkg/client
// — the same MountReadOnly discipline cmd/fathom/metrics_*.go's readers
// use (a CLI query must never contend with the daemon's own writes).
func openFindingsClient(storePath string) (*client.Client, func(), error) {
	mounted, err := store.MountReadOnly(storePath, store.LevelL0)
	if err != nil {
		return nil, nil, fmt.Errorf("mount store %s: %w", storePath, err)
	}
	c := client.New(connector.NewLocalConnector(), mounted)
	return c, func() { _ = c.Close() }, nil
}

// ─── findings near ──────────────────────────────────────────────────────────

func cmdFindingsNear(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("findings near", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	hops := fs.Int("hops", 2, "call-graph hop radius")
	includeSuperseded := fs.Bool("include-superseded", false, "include superseded/refuted findings (always labeled)")
	asOf := fs.String("as-of", "", "answer as of this RFC3339 timestamp or commit sha instead of now (SU-19, ADR-041 decision 7)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable table")
	// Accept the positional <symbol|file> before or after the flags (O92's
	// class of friction: the documented order must parse).
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: findings near requires exactly one positional argument, <symbol|file>")
		fs.Usage()
		return 1
	}
	subject := fs.Arg(0)
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}
	if *hops < 1 {
		fmt.Fprintln(stderr, "error: --hops must be >= 1")
		return 1
	}

	c, closeFn, err := openFindingsClient(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer closeFn()

	ctx := context.Background()
	nodes, err := c.ListAllSymbols(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: list symbols: %v\n", err)
		return 1
	}
	typed, _ := c.TypedEdges(ctx, *service)
	claims, err := c.ClaimsForService(ctx, *service, findingsLayerName)
	if err != nil {
		fmt.Fprintf(stderr, "error: load findings: %v\n", err)
		return 1
	}

	// SU-19 (ADR-041 decision 7): --as-of narrows claims to those whose
	// bi-temporal validity window contains the resolved instant BEFORE
	// RankNear ever runs — "was this finding true then", independent of
	// its CURRENT status (a claim later superseded still answers if as-of
	// falls before that event), which is a different question than
	// --include-superseded's "show me what is superseded NOW too".
	asOfLabel := ""
	rankIncludeSuperseded := *includeSuperseded
	if *asOf != "" {
		commits, _ := c.ListCommits(ctx, *service)
		at, aerr := semantic.ResolveAsOf(*asOf, commits)
		if aerr != nil {
			fmt.Fprintf(stderr, "error: %v\n", aerr)
			return 1
		}
		asOfLabel = at.UTC().Format(time.RFC3339)
		valid := claims[:0]
		for _, cl := range claims {
			if cl.ValidAt(at) {
				valid = append(valid, cl)
			}
		}
		claims = valid
		// The bi-temporal filter above has ALREADY decided which claims
		// were valid at asOf; RankNear's own superseded/refuted gate is a
		// CURRENT-status question and must not additionally exclude a
		// since-superseded claim this call just proved was valid then.
		rankIncludeSuperseded = true
	}

	report := buildFindingsNearReport(nodes, typed, claims, subject, *hops, rankIncludeSuperseded)
	report.IncludeSuperseded = *includeSuperseded
	report.AsOf = asOfLabel

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	renderFindingsNear(stdout, report)
	return 0
}

// findingsNearReport is the shared shape both the CLI and the MCP tool
// render (JSON round-trips identically either way).
type findingsNearReport struct {
	Subject           string              `json:"subject"`
	Hops              int                 `json:"hops"`
	IncludeSuperseded bool                `json:"include_superseded"`
	Found             bool                `json:"found"`
	Note              string              `json:"note,omitempty"`
	Findings          []findingsNearEntry `json:"findings"`
	// AsOf (SU-19, ADR-041 decision 7) is set only when --as-of was given —
	// the RFC3339 instant this report answers "was this finding true then"
	// for. Empty (omitted) for the default live report, byte-identical to
	// this report's shape before as_of existed.
	AsOf string `json:"as_of,omitempty"`
}

type findingsNearEntry struct {
	ClaimID    string   `json:"claim_id"`
	Subject    string   `json:"subject"`
	Class      string   `json:"class,omitempty"`
	Rule       string   `json:"rule"`
	Scope      []string `json:"scope"`
	Status     string   `json:"status"`
	Hops       int      `json:"hops"`
	Superseded bool     `json:"superseded"`
	CompiledAt string   `json:"compiled_at,omitempty"`
}

// buildFindingsNearReport is the pure core both the CLI verb and the MCP
// tool call — I/O (ListAllSymbols/TypedEdges/ClaimsForService) already
// happened in the caller.
func buildFindingsNearReport(nodes []core.SymbolNode, typed []core.Edge, claims []core.SemanticClaim, subject string, hops int, includeSuperseded bool) findingsNearReport {
	r := findingsNearReport{Subject: subject, Hops: hops, IncludeSuperseded: includeSuperseded}

	fileHops := findings.FileNeighborhood(nodes, typed, subject, hops)
	if fileHops == nil {
		r.Note = fmt.Sprintf("subject %q not found in the index (no matching symbol or file)", subject)
		return r
	}
	r.Found = true

	entries := findings.RankNear(claims, fileHops, includeSuperseded)
	for _, e := range entries {
		compiledAt := ""
		if !e.Claim.CompiledAt.IsZero() {
			compiledAt = e.Claim.CompiledAt.UTC().Format(time.RFC3339)
		}
		r.Findings = append(r.Findings, findingsNearEntry{
			ClaimID: e.Claim.ClaimID, Subject: e.Claim.Subject, Class: string(e.Claim.Class),
			Rule: e.Claim.Rule, Scope: e.Claim.Scope, Status: string(e.Claim.Status),
			Hops: e.Hops, Superseded: e.Superseded, CompiledAt: compiledAt,
		})
	}
	if len(r.Findings) == 0 {
		r.Note = "no findings within the neighborhood"
	}
	return r
}

func renderFindingsNear(w io.Writer, r findingsNearReport) {
	fmt.Fprintf(w, "findings near %q (hops<=%d)\n", r.Subject, r.Hops)
	if r.AsOf != "" {
		fmt.Fprintf(w, "  as_of=%s\n", r.AsOf)
	}
	if !r.Found {
		fmt.Fprintf(w, "  %s\n", r.Note)
		return
	}
	if r.Note != "" {
		fmt.Fprintf(w, "  %s\n", r.Note)
		return
	}
	for _, f := range r.Findings {
		label := ""
		if f.Superseded {
			label = " [SUPERSEDED — precedent, not live]"
		}
		fmt.Fprintf(w, "  hops=%d [%s] %s%s\n", f.Hops, f.Class, f.Rule, label)
		fmt.Fprintf(w, "    subject=%s scope=%v status=%s\n", f.Subject, f.Scope, f.Status)
	}
}

// ─── findings status ─────────────────────────────────────────────────────────

func cmdFindingsStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("findings status", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}

	c, closeFn, err := openFindingsClient(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer closeFn()

	ctx := context.Background()
	claims, err := c.ClaimsForService(ctx, *service, findingsLayerName)
	if err != nil {
		fmt.Fprintf(stderr, "error: load findings: %v\n", err)
		return 1
	}
	// Store-wide, not service-scoped (claim_events carries no service_id
	// column — the same honesty caveat Store.ClaimEventsSince's own doc
	// comment states, and pkg/client.Client.ClaimEventsSince's mirror of
	// it).
	events, eerr := c.ClaimEventsSince(ctx, time.Unix(0, 0).UTC(), time.Now().UTC())
	if eerr != nil {
		fmt.Fprintf(stderr, "error: load claim events: %v\n", eerr)
		return 1
	}

	// SU-16 (docs/plans/substrate.md §D): telemetry rides the SAME hooks
	// beat log every other `fathom metrics`/instrument reader uses
	// (beatLogPath/readBeatsFor, beats.go/metrics_io.go) — best-effort, an
	// absent/unreadable log degrades to an honest all-zero telemetry, never
	// an error.
	beats := readBeatsFor(beatLogPath())
	report := findings.BuildStatusWithTelemetry(claims, events, beats, time.Now())

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "findings status for %s:\n", *service)
	fmt.Fprintf(stdout, "  resolved findings (independent code-change resolutions): %d\n", report.ResolvedFindingCount)
	fmt.Fprintf(stdout, "  distinct classes: %d\n", len(report.DistinctClasses))
	for _, cl := range report.DistinctClasses {
		fmt.Fprintf(stdout, "    - %s\n", cl)
	}
	fmt.Fprintf(stdout, "  patterns with >=3 resolved members: %d\n", report.PatternsWithThreeResolvedMembers)
	fmt.Fprintf(stdout, "  candidates (>=3 sites): %d\n", report.Candidates)
	fmt.Fprintf(stdout, "  disagreements (by rule shape + anchor path): %d\n", report.DisagreementsByRule)
	fmt.Fprintf(stdout, "  entry condition (product-vision.md §3, %d-%d resolved findings): met=%v\n",
		report.EntryBarLow, report.EntryBarHigh, report.EntryConditionMet)
	fmt.Fprintf(stdout, "  %s\n", report.Note)
	fmt.Fprintf(stdout, "  rules never fired in 30d: %d\n", report.RulesNeverFired30d)
	if len(report.TopFired) > 0 {
		fmt.Fprintln(stdout, "  top fired:")
		for _, f := range report.TopFired {
			fmt.Fprintf(stdout, "    - %s served=%d touched=%d last_fired=%s\n", f.ClaimID, f.Served, f.Touched, f.LastFired)
		}
	}
	return 0
}

// ─── findings candidates ────────────────────────────────────────────────────

func cmdFindingsCandidates(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("findings candidates", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	minSites := fs.Int("min-sites", 3, "minimum distinct sites a pattern must span to emit a candidate")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable report")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}
	if *minSites < 2 {
		fmt.Fprintln(stderr, "error: --min-sites must be >= 2 (a single-site 'pattern' is one finding, not recurrence)")
		return 1
	}

	c, closeFn, err := openFindingsClient(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer closeFn()

	ctx := context.Background()
	claims, err := c.ClaimsForService(ctx, *service, findingsLayerName)
	if err != nil {
		fmt.Fprintf(stderr, "error: load findings: %v\n", err)
		return 1
	}
	events, eerr := c.ClaimEventsSince(ctx, time.Unix(0, 0).UTC(), time.Now().UTC())
	if eerr != nil {
		fmt.Fprintf(stderr, "error: load claim events: %v\n", eerr)
		return 1
	}

	cands := findings.RuleCandidates(claims, events, *minSites)

	if *jsonOut {
		jsonPrint(stdout, cands)
		return 0
	}
	renderFindingsCandidates(stdout, *service, *minSites, cands)
	return 0
}

// renderFindingsCandidates renders one block per candidate: the class,
// shape, drafted sentence, check family, producer set, and every member
// citation — never a table (member citations vary too much in count to
// stay tabular and readable at a terminal width).
func renderFindingsCandidates(w io.Writer, service string, minSites int, cands []findings.RuleCandidate) {
	fmt.Fprintf(w, "rule candidates for %s (>=%d sites): %d\n", service, minSites, len(cands))
	for i, c := range cands {
		fmt.Fprintf(w, "\n%d. [%s] %s\n", i+1, c.Class, c.Shape)
		fmt.Fprintf(w, "   draft: %s\n", c.Draft)
		fmt.Fprintf(w, "   check family: %s\n", c.CheckFamily)
		fmt.Fprintf(w, "   sites: %d, code-change resolutions: %d, producers: %v\n", c.Sites, c.CodeChangeResolutions, c.Producers)
		for _, m := range c.Members {
			fmt.Fprintf(w, "     - %s (producer=%s, claim=%s, scope=%v)\n", m.Subject, m.Producer, m.ClaimID, m.Scope)
		}
	}
	if len(cands) == 0 {
		fmt.Fprintln(w, "  no pattern currently spans enough sites — never a reason to lower the bar without evidence")
	}
}
