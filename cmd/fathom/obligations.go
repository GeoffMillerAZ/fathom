package main

// obligations.go — `fathom obligations` (OC-32, ADR-045 consequence
// "obligations gains an additive --explain"): what governs a file, as the
// obligations tool serves it to an agent — the raw-fallback rows for an
// uncompiled governing guide first, then the compiled rules — and, with
// --explain, why: every scope-matched candidate by scope tier, the ranking
// and the cap, and each displaced rule's reason (semantic.
// ExplainObligations, run through servingExplanation in
// console_explain.go, the same function /explain renders).
//
// CLI only, by the surface rules: the agent surface's argument schemas are
// golden-pinned (TestAgentSurfaceArgSchemaUnchanged) and every byte of
// them is paid by every connected session, while the explanation is read
// by a person. The MCP obligations tool is unchanged; its default payload
// and this verb's default output are the same served list.

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// servedObligationsJSON is the default --json: the list an agent is served.
type servedObligationsJSON struct {
	Service         string                                `json:"service"`
	File            string                                `json:"file"`
	Layer           string                                `json:"layer"`
	Rules           []semantic.Obligation                 `json:"rules"`
	RawWhy          string                                `json:"raw_why,omitempty"`
	Displaced       int                                   `json:"displaced,omitempty"`
	DisplacedByTier map[string]int                        `json:"displaced_by_tier,omitempty"`
	Deduped         int                                   `json:"deduped,omitempty"`
	RankedBy        string                                `json:"ranked_by"`
	Watermark       string                                `json:"compiled_at_watermark,omitempty"`
	Coverage        console.Value[semantic.LayerCoverage] `json:"coverage"`
	Note            string                                `json:"note,omitempty"`
}

func servedJSON(se console.ServingExplanation) servedObligationsJSON {
	return servedObligationsJSON{
		Service: se.Service, File: se.File, Layer: se.Layer, Rules: se.AgentSees, RawWhy: se.RawWhy,
		Displaced: se.Served.Displaced, DisplacedByTier: se.Served.DisplacedByTier, Deduped: se.Deduped,
		RankedBy: se.Served.RankedBy, Watermark: se.Served.Watermark, Coverage: se.Coverage, Note: se.CoverageNote,
	}
}

func cmdObligations(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("obligations", stderr)
	service := fs.String("service", "", "service ID (required)")
	file := fs.String("file", "", "repo-relative file being edited; omit for the root-scope question")
	layer := fs.String("layer", "rules", `which claims to serve: "rules" (the obligations tool's default), "claims" (plan-row bindings) or "all"`)
	explain := fs.Bool("explain", false, "explain the answer: every candidate by scope tier, the ranking, the cap, and each displaced rule's reason")
	format := fs.String("format", "", "output format: text (default) | sarif — SARIF 2.1.0 of the served rules, one rule per tier, located at each rule's own citation (GL-20; overrides --json, --explain unread)")
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		return 1
	}
	switch semantic.ObligationLayerFilter(*layer) {
	case semantic.ObligationLayerRules, semantic.ObligationLayerClaims, semantic.ObligationLayerAll:
	default:
		fmt.Fprintf(stderr, "error: --layer must be rules, claims or all, got %q\n", *layer)
		return 1
	}
	switch *format {
	case "", "sarif":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want sarif\n", *format)
		return 1
	}
	ks := resolveKnowledgeStore(*kf.store, *kf.url)
	st, err := openKnowledgeStore(ks)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	c := knowledgeClient(st)
	defer c.Close()
	se, err := servingExplanation(context.Background(), c, *service, *file, semantic.ObligationLayerFilter(*layer), absStorePath(ks.Path))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	switch {
	case *format == "sarif":
		_, _ = stdout.Write(verify.RenderSARIF("fathom obligations", obligationsSARIF(se.AgentSees)))
		return 0
	case *kf.json && *explain:
		return jsonExit(writeKnowledgeJSON(stdout, se), stderr)
	case *kf.json:
		return jsonExit(writeKnowledgeJSON(stdout, servedJSON(se)), stderr)
	}
	subject := "governs " + se.File
	if se.File == "" {
		subject = "governs the repository root"
	}
	fmt.Fprintf(stdout, "what %s in %s (layer %s; store %s)\n\n", subject, se.Service, se.Layer, ks)
	if len(se.AgentSees) == 0 {
		fmt.Fprintln(stdout, "no rule is served — nothing compiled governs this path")
	}
	for i, r := range se.AgentSees {
		flag := ""
		if r.Status != "" {
			flag = " [" + r.Status + "]"
		}
		fmt.Fprintf(stdout, "%2d. [%s]%s %s\n      %s (%s)\n", i+1, r.Tier, flag, r.Rule, r.Src, r.Layer)
		if r.Why != "" {
			fmt.Fprintf(stdout, "      %s\n", r.Why)
		}
	}
	if se.CoverageNote != "" {
		fmt.Fprintf(stdout, "\nnote: %s\n", se.CoverageNote)
	}
	if !*explain {
		if se.Served.Displaced > 0 {
			fmt.Fprintf(stdout, "\n%d eligible rule(s) displaced by the cap — add --explain to see which and why\n", se.Served.Displaced)
		}
		return 0
	}
	fmt.Fprintf(stdout, "\ncap %d · %d eligible · %d served · %d displaced · %d duplicate(s) not repeated · ranked by %s\n",
		se.Cap, se.Eligible, len(se.Served.Rules), se.Displaced, se.Deduped, strings.Join(se.Ranking, " → "))
	fmt.Fprintf(stdout, "re-rank: %s\n", se.Rerank)
	for _, t := range se.Tiers {
		fmt.Fprintf(stdout, "\n%s — %d candidate(s): %d served, %d displaced, %d duplicate(s)\n", strings.ToUpper(strings.ReplaceAll(t.Tier, "_", " ")), t.Candidates, t.Served, t.Displaced, t.Duplicates)
		for _, cand := range se.Candidates {
			if cand.ScopeTier != t.Tier {
				continue
			}
			state := fmt.Sprintf("served #%d", cand.Position)
			if cand.Reserved {
				state += " (reserved slot)"
			}
			if cand.Displaced != nil {
				state = cand.Displaced.Reason + ": " + cand.Displaced.Why
			}
			fmt.Fprintf(stdout, "  #%-3d %s\n       %s · %s\n", cand.Rank, cand.Rule, cand.Src, state)
		}
	}
	return 0
}
