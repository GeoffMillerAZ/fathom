package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// registerVerifyPlan registers the verify_plan tool: an evidence-backed
// "definition of done" checker. For each plan task it composes existing
// primitives — find_symbol, callgraph reachability/test-impact, contradiction
// detection, and spec coverage — into a per-task verdict that can DISAGREE with
// a "done" claim, citing file:line and evidence tier. It is read-only and
// strictly static: it never asserts that a test passes.
func registerVerifyPlan(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "verify_plan",
		Description: "Evidence-backed definition-of-done: per plan task, verify the claimed symbol exists at the required evidence tier, a test statically reaches it, required call-edges are wired, and declared specs are covered. Verdicts cite file:line and can disagree with a 'done' claim. Static only — a reached test is not a passing test. Tasks carry claimed/skipped from checkbox state for gate computation. Reach for it when you are about to check off a plan row: done means proven by the index, not claimed. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":       map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern":       map[string]any{"type": "string", "description": "Optional concern to scope contradiction detection"},
				"plan_markdown": map[string]any{"type": "string", "description": "Markdown checklist alternative to tasks: each '- [ ] title [SPEC-ID] {symbol: Foo, package: dir/of/foo, reachable_from: Bar, min_tier: verified}' line becomes a task. Annotations must sit on the checkbox line itself."},
				"tasks": map[string]any{
					"type":        "array",
					"description": "Plan tasks with acceptance contracts",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":                  map[string]any{"type": "string", "description": "Task ID"},
							"title":               map[string]any{"type": "string", "description": "Task title"},
							"spec_ids":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Spec IDs that must be covered"},
							"symbol":              map[string]any{"type": "string", "description": "Symbol that must exist in the index"},
							"package":             map[string]any{"type": "string", "description": "Optional repo-relative directory the symbol's file must live under (disambiguates a bare name that exists in several packages)"},
							"reachable_from":      map[string]any{"type": "string", "description": "Entrypoint that must reach symbol"},
							"skip_test":           map[string]any{"type": "boolean", "description": "Don't require a test to reach symbol"},
							"allow_contradiction": map[string]any{"type": "boolean", "description": "Contradictions don't fail the task"},
							"min_tier":            map[string]any{"type": "string", "enum": []string{"verified", "pre-scan", "doc-claimed", "sme-stated", "assumed"}, "description": "Minimum evidence tier (default verified)"},
						},
						"required": []string{"id"},
					},
				},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service      string `json:"service"`
			Concern      string `json:"concern"`
			PlanMarkdown string `json:"plan_markdown"`
			Tasks        []struct {
				ID                 string   `json:"id"`
				Title              string   `json:"title"`
				SpecIDs            []string `json:"spec_ids"`
				Symbol             string   `json:"symbol"`
				Package            string   `json:"package"`
				ReachableFrom      string   `json:"reachable_from"`
				SkipTest           bool     `json:"skip_test"`
				AllowContradiction bool     `json:"allow_contradiction"`
				MinTier            string   `json:"min_tier"`
				Claimed            bool     `json:"claimed"`
				Skipped            bool     `json:"skipped"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: service required"}
		}

		// Build the call graph once (reused for all reachability / test checks).
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		graph := callgraph.Build(nodes)

		// Typed Go call edges (ADR-021 §3): additive on top of the syntax
		// graph — best-effort, a non-Go service or a store with no typed
		// edges yet simply contributes nothing.
		if typed, typedErr := backend.TypedEdges(ctx, p.Service); typedErr == nil {
			callgraph.AddTypedEdges(graph, typed)
		}

		// Contradictions (best-effort; scoped by concern when provided).
		var contras []core.Contradiction
		if res, qErr := backend.QuerySymbols(ctx, core.Query{ServiceID: p.Service, ConcernID: p.Concern, TopN: 500}); qErr == nil {
			contras = res.Contradictions
		}

		// Build the plan: from a Markdown checklist when provided, else from the
		// structured tasks.
		var plan verify.Plan
		if p.PlanMarkdown != "" && len(p.Tasks) == 0 {
			plan = verify.ParseMarkdownPlan(p.Service, []byte(p.PlanMarkdown))
		} else {
			plan = verify.Plan{Service: p.Service}
			for _, t := range p.Tasks {
				var mt core.EvidenceTier
				if core.EvidenceTier(t.MinTier).Valid() {
					mt = core.EvidenceTier(t.MinTier)
				}
				plan.Tasks = append(plan.Tasks, verify.Task{
					ID:      t.ID,
					Title:   t.Title,
					SpecIDs: t.SpecIDs,
					Claimed: t.Claimed,
					Skipped: t.Skipped,
					Expect: verify.Expect{
						Symbol:             t.Symbol,
						Package:            t.Package,
						ReachableFrom:      t.ReachableFrom,
						SkipTest:           t.SkipTest,
						AllowContradiction: t.AllowContradiction,
						MinTier:            mt,
					},
				})
			}
		}

		// Spec matrix (built only if some task declares spec IDs).
		var (
			matrix  sdd.Matrix
			haveMat bool
		)
		var allSpecs []string
		for _, t := range plan.Tasks {
			allSpecs = append(allSpecs, t.SpecIDs...)
		}
		if len(allSpecs) > 0 {
			matrix = buildSpecMatrix(ctx, backend, p.Service, nodes, allSpecs)
			haveMat = true
		}

		ev := &verifyEvidence{
			ctx: ctx, backend: backend, service: p.Service,
			graph: graph, contras: contras, matrix: matrix, haveMat: haveMat,
		}

		return verifyReportToMap(verify.VerifyPlan(plan, ev)), nil
	})
}

// verifyEvidence adapts the live MCP backend to the verify.Evidence interface.
// The call graph, contradictions, and spec matrix are computed once per
// verify_plan invocation and reused across all tasks.
type verifyEvidence struct {
	ctx     context.Context
	backend QueryInterface
	service string
	graph   *callgraph.Graph
	contras []core.Contradiction
	matrix  sdd.Matrix
	haveMat bool
}

func (e *verifyEvidence) FindSymbol(name string) []core.SymbolNode {
	nodes, err := e.backend.FindSymbol(e.ctx, e.service, name)
	if err != nil {
		return nil
	}
	return nodes
}

func (e *verifyEvidence) TestReaches(symbol string) []string {
	return callgraph.TestImpact(e.graph, symbol)
}

// TestReachesDetail implements verify.DetailedEvidence (ADR-021 §3): a path
// crossing a typed EdgeIfaceDispatch edge is surfaced distinctly in the
// verdict detail rather than reading identically to a direct static reach.
func (e *verifyEvidence) TestReachesDetail(symbol string) []verify.ReachDetail {
	details := callgraph.TestImpactDetail(e.graph, symbol)
	out := make([]verify.ReachDetail, len(details))
	for i, d := range details {
		out[i] = verify.ReachDetail{Key: d.Key, ViaDispatch: d.ViaDispatch, Implementers: d.Implementers}
	}
	return out
}

func (e *verifyEvidence) Reachable(from, to string) bool {
	return callgraph.Reachable(e.graph, from, to)
}

func (e *verifyEvidence) Contradictions() []core.Contradiction { return e.contras }

func (e *verifyEvidence) SpecImplementors(specID string) []string {
	if !e.haveMat {
		return nil
	}
	return e.matrix.Coverage(specID)
}

var (
	_ verify.Evidence         = (*verifyEvidence)(nil)
	_ verify.DetailedEvidence = (*verifyEvidence)(nil)
)

// buildSpecMatrix builds an sdd coverage matrix from a service's symbols and
// source files, mirroring the spec_coverage tool's extraction.
func buildSpecMatrix(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, specs []string) sdd.Matrix {
	codeRefs := map[string][]string{}
	for _, n := range nodes {
		if refs := sdd.ExtractRefs(n.Signature + " " + n.Name); len(refs) > 0 {
			codeRefs[n.Name] = refs
		}
	}
	if files, err := backend.Sources(ctx, service); err == nil {
		for _, f := range files {
			if refs := sdd.ExtractRefs(string(f.Content)); len(refs) > 0 {
				codeRefs[f.Path] = append(codeRefs[f.Path], refs...)
			}
		}
	}
	return sdd.BuildMatrix(specs, codeRefs)
}

// verifyReportToMap renders a verify.Report as a deterministic JSON-friendly map.
func verifyReportToMap(rep verify.Report) map[string]any {
	tasks := make([]any, 0, len(rep.Tasks))
	for _, t := range rep.Tasks {
		crit := make([]any, 0, len(t.Criteria))
		for _, c := range t.Criteria {
			crit = append(crit, map[string]any{
				"name":   c.Name,
				"status": string(c.Status),
				"detail": c.Detail,
			})
		}
		tm := map[string]any{
			"id":       t.TaskID,
			"title":    t.Title,
			"status":   string(t.Status),
			"label":    t.Label,
			"criteria": crit,
		}
		if t.Claimed {
			tm["claimed"] = true
		}
		if t.Skipped {
			tm["skipped"] = true
		}
		tasks = append(tasks, tm)
	}
	out := map[string]any{
		"service": rep.Service,
		"summary": map[string]any{
			"total":   rep.Summary.Total,
			"pass":    rep.Summary.Pass,
			"fail":    rep.Summary.Fail,
			"unknown": rep.Summary.Unknown,
		},
		"tasks": tasks,
	}
	if vo := verdictOutcome(rep); vo != nil {
		out["verdict_outcome"] = vo
	}
	return out
}

// verdictOutcome computes verify_plan's gate-shaped result summary (T5-7,
// docs/plans/trust-and-tokenomics.md): claimed/verified/failing counts over
// exactly the tasks internal/verify.Gate itself gates on (Claimed and not
// Skipped) — the same population, so the count is directly "what the push/CI
// gate would have caught here." Returns nil when no task is claimed: nothing
// gate-relevant happened in this call, so there is nothing to log (mirrors
// claimsAttachedFrom's own >0 opt-in gate for brief).
func verdictOutcome(rep verify.Report) map[string]any {
	claimed, verifiedN, failing := 0, 0, 0
	for _, t := range rep.Tasks {
		if !t.Claimed || t.Skipped {
			continue
		}
		claimed++
		switch t.Status {
		case verify.Pass:
			verifiedN++
		case verify.Fail:
			failing++
		}
	}
	if claimed == 0 {
		return nil
	}
	return map[string]any{"claimed": claimed, "verified": verifiedN, "failing": failing}
}
