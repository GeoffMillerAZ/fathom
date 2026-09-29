package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/memo"
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
		Description: "Definition-of-done per task. This result IS the answer for whether it's proven; a follow-up call only fixes an unproven row. Reach for it when checking off a row. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":       map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern":       map[string]any{"type": "string"},
				"plan_markdown": map[string]any{"type": "string", "description": "Markdown checklist alt to tasks: checkbox lines, [SPEC-ID]/{symbol} tags"},
				"plan_path":     map[string]any{"type": "string"},
				"tasks": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":                  map[string]any{"type": "string"},
							"title":               map[string]any{"type": "string"},
							"spec_ids":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"symbol":              map[string]any{"type": "string"},
							"package":             map[string]any{"type": "string"},
							"reachable_from":      map[string]any{"type": "string"},
							"skip_test":           map[string]any{"type": "boolean"},
							"skip":                map[string]any{"type": "string", "description": "Justification for skip_test"},
							"allow_contradiction": map[string]any{"type": "boolean"},
							"min_tier":            map[string]any{"type": "string", "enum": []string{"verified", "pre-scan", "doc-claimed", "sme-stated", "assumed"}},
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
			PlanPath     string `json:"plan_path"`
			Tasks        []struct {
				ID                 string   `json:"id"`
				Title              string   `json:"title"`
				SpecIDs            []string `json:"spec_ids"`
				Symbol             string   `json:"symbol"`
				Package            string   `json:"package"`
				ReachableFrom      string   `json:"reachable_from"`
				SkipTest           bool     `json:"skip_test"`
				Skip               string   `json:"skip"`
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

		// Build the call graph once (reused for all reachability / test
		// checks), memoized under the store's watermark in the SAME cache
		// the nav tools use (I21/BP-28, resolved_nav.go's loadNavGraphs).
		//
		// O33: before this, verify_plan rebuilt its graph from scratch on
		// EVERY call via callgraph.Build — on this repository's ~11.5k
		// symbols that alone costs 13-25s (profiled 2026-09-13; every other
		// stage combined is under 200ms), which is why the daemon's
		// verify_plan latency doubled to 10-15s/call and why repeated calls
		// at one watermark cost the same: the watermark-keyed memo existed
		// (BP-28) but nothing on this path ever called into it. A second
		// call at an unchanged watermark now costs microseconds instead of
		// paying the build again.
		// RT-74: the graph store's syntax+typed graph for this watermark,
		// the one every nav tool and the console read.
		ng, lerr := loadNavGraphsErr(ctx, backend, p.Service)
		if lerr != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: lerr.Error()}
		}
		nodes, graph := ng.Nodes, ng.Both

		// What the product entry points reach (the `wired` criterion,
		// ADR-018 amendment 2026-09-28), walked once per watermark over
		// the same graph: the both graph, or the syntax graph this call
		// degraded to when the typed edges could not be read.
		productKind := memo.GraphBoth
		if ng.TypedErr != nil {
			productKind = memo.GraphSyntax
		}
		product, perr := memo.ProductReachFor(ctx, ng.Snapshot, productKind, typedLoader(backend, p.Service))
		if perr != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "verify_plan: product reach: " + perr.Error()}
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
			// O56: self-skip needs one prior plan version to compare
			// against — only available when the caller names the file
			// (plan_path) AND the service's recorded corpus root is a real
			// git checkout on this filesystem. Best-effort; see selfskip.go.
			if p.PlanPath != "" {
				if repoPath, _, found, merr := backend.ServiceMeta(ctx, p.Service); merr == nil && found && repoPath != "" {
					prior := selfSkipPriorPlan(ctx, repoPath, p.PlanPath, p.Service, []byte(p.PlanMarkdown))
					plan = verify.AnnotateSelfSkip(plan, prior)
				}
			}
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
						SkipReason:         t.Skip,
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
			specCov corpus.Coverage
		)
		var allSpecs []string
		for _, t := range plan.Tasks {
			allSpecs = append(allSpecs, t.SpecIDs...)
		}
		if len(allSpecs) > 0 {
			var merr error
			matrix, specCov, merr = buildSpecMatrix(ctx, backend, p.Service, nodes, allSpecs)
			if merr != nil {
				// RT-146 / I38: a hook's whole-tree read was skipped. An
				// empty matrix would fail every spec-backed row falsely, so
				// the call refuses by name instead (the Stop hook renders
				// it as advisory, never as a failing claim).
				return nil, &rpcError{Code: ErrCodeInternal, Message: "verify_plan: " + merr.Error()}
			}
			haveMat = true
		}

		ev := &verifyEvidence{
			ctx: ctx, backend: backend, service: p.Service,
			graph: graph, contras: contras, matrix: matrix, haveMat: haveMat,
			product: product,
		}

		out := verifyReportToMap(verify.VerifyPlan(plan, ev))
		if specCov.LowerBound() {
			// I38: the spec matrix is a lower bound — an "uncovered" spec
			// may sit in a file the budgeted scan never reached.
			out["partial"] = "spec matrix " + partialNote(specCov) + " (an uncovered spec may be in an unscanned file)"
		}
		return out, nil
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
	product *callgraph.ProductReach
}

// ProductEntryCount implements verify.ProductEvidence.
func (e *verifyEvidence) ProductEntryCount() int { return len(e.product.Entries()) }

// ProductReachedBy implements verify.ProductEvidence.
func (e *verifyEvidence) ProductReachedBy(symbol string) (string, bool) {
	return e.product.ReachedBy(symbol)
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

// certificationsBackend is the optional backend capability
// CertificationsByTest needs (vision 33 / ADR-043 stage 1, TC-9) —
// QueryInterface implementations that don't serve semantic layers simply
// don't implement it, and CertificationsByTest degrades to "no
// certificate" for every row rather than failing the whole verify_plan
// call.
type certificationsBackend interface {
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)
}

// CertificationsByTest implements verify.CertificationEvidence: reads
// every compiled certifications-layer claim for the service and groups it
// by its provenance test's bare name via certify.ByTest — the SAME
// resolution cmd/fathom's cliEvidence uses, so `fathom verify` and
// verify_plan resolve a certified_by contract identically.
func (e *verifyEvidence) CertificationsByTest(tests []string) map[string][]verify.Certification {
	cb, ok := e.backend.(certificationsBackend)
	if !ok {
		return nil
	}
	claims, err := cb.ClaimsForService(e.ctx, e.service, certify.Layer)
	if err != nil {
		return nil
	}
	return certify.ByTest(claims, tests)
}

var _ verify.CertificationEvidence = (*verifyEvidence)(nil)

var (
	_ verify.Evidence         = (*verifyEvidence)(nil)
	_ verify.DetailedEvidence = (*verifyEvidence)(nil)
	_ verify.ProductEvidence  = (*verifyEvidence)(nil)
)

// buildSpecMatrix builds an sdd coverage matrix from a service's symbols and
// source files, mirroring the spec_coverage tool's extraction.
//
// A skipped whole-tree read (core.ErrSourcesBusy, RT-146) is returned, never
// folded into a symbol-only matrix; any other Sources error keeps the old
// best-effort behaviour. The returned coverage names a scan that stopped
// early (the matrix is then a lower bound, I38).
func buildSpecMatrix(ctx context.Context, backend QueryInterface, service string, nodes []core.SymbolNode, specs []string) (sdd.Matrix, corpus.Coverage, error) {
	codeRefs := map[string][]string{}
	for _, n := range nodes {
		if refs := sdd.ExtractRefs(n.Signature + " " + n.Name); len(refs) > 0 {
			codeRefs[n.Name] = refs
		}
	}
	files, cov, err := collectSources(ctx, backend, service)
	if errors.Is(err, core.ErrSourcesBusy) {
		return sdd.Matrix{}, corpus.Coverage{}, err
	}
	if err != nil {
		cov = corpus.Coverage{}
	} else {
		for _, f := range files {
			if refs := sdd.ExtractRefs(string(f.Content)); len(refs) > 0 {
				codeRefs[f.Path] = append(codeRefs[f.Path], refs...)
			}
		}
	}
	return sdd.BuildMatrix(specs, codeRefs), cov, nil
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
