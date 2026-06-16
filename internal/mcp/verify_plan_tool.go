package mcp

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/sextant/internal/analysis/callgraph"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/sdd"
	"github.com/geoffmilleraz/sextant/internal/verify"
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
		Description: "Evidence-backed definition-of-done. For each plan task, verify the claimed symbol exists at a sufficient evidence tier, a test statically reaches it, a required call-edge is wired, declared specs are covered, and there is no contradiction. Returns a per-task verdict (verified | claimed-but-absent | present-but-untested | edge-not-wired | contradicted | likely-unconfirmed | spec-uncovered | no-contract) with cited evidence. STATIC only: test_reaches means a test reaches the symbol via the call graph, NOT that it passes; runtime_pass is always 'unknown' until CI/coverage evidence is ingested.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":       map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern":       map[string]any{"type": "string", "description": "Optional concern to scope contradiction detection"},
				"plan_markdown": map[string]any{"type": "string", "description": "Alternative to tasks: a Markdown checklist (e.g. the contents of TRACKING.md). Each '- [ ] title [SPEC-ID] {symbol: Foo, reachable_from: Bar, min_tier: verified}' line becomes a task; a line with no annotation/spec verifies as no-contract."},
				"tasks": map[string]any{
					"type":        "array",
					"description": "Plan tasks, each with an acceptance contract",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":                  map[string]any{"type": "string", "description": "Task identifier"},
							"title":               map[string]any{"type": "string", "description": "Human-readable task title"},
							"spec_ids":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Declared spec IDs (e.g. [\"[US-042]\"]) that must be covered"},
							"symbol":              map[string]any{"type": "string", "description": "Claimed symbol that must exist in the index"},
							"reachable_from":      map[string]any{"type": "string", "description": "Optional entrypoint that must reach symbol via the call graph"},
							"skip_test":           map[string]any{"type": "boolean", "description": "If true, do not require a test to reach symbol (default false = test required)"},
							"allow_contradiction": map[string]any{"type": "boolean", "description": "If true, contradictions do not fail the task (default false)"},
							"min_tier":            map[string]any{"type": "string", "description": "Minimum evidence tier: verified|pre-scan|doc-claimed|sme-stated|assumed (default verified)"},
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
				ReachableFrom      string   `json:"reachable_from"`
				SkipTest           bool     `json:"skip_test"`
				AllowContradiction bool     `json:"allow_contradiction"`
				MinTier            string   `json:"min_tier"`
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
					Expect: verify.Expect{
						Symbol:             t.Symbol,
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

var _ verify.Evidence = (*verifyEvidence)(nil)

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
		tasks = append(tasks, map[string]any{
			"id":       t.TaskID,
			"title":    t.Title,
			"status":   string(t.Status),
			"label":    t.Label,
			"criteria": crit,
		})
	}
	return map[string]any{
		"service": rep.Service,
		"summary": map[string]any{
			"total":   rep.Summary.Total,
			"pass":    rep.Summary.Pass,
			"fail":    rep.Summary.Fail,
			"unknown": rep.Summary.Unknown,
		},
		"tasks": tasks,
	}
}
