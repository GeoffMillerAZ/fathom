package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// registerVerifyPlan registers the verify_plan tool: an evidence-backed
// "definition of done" checker. For each plan task it composes existing
// primitives — find_symbol, callgraph reachability/test-impact, contradiction
// detection, and spec coverage — into a per-task verdict that can DISAGREE with
// a "done" claim, citing file:line and evidence tier. It is read-only and
// strictly static: it never runs a test, and runtime_pass reads only an
// attested run already in test_runs (TC-29), informational.
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
				"plan_path":     map[string]any{"type": "string", "description": "Plan file relative to the service's indexed root, e.g. docs/plans/<plan>.md"},
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
			// PlanTree is the caller's statement of the working tree the
			// plan was read from (HEAD, cleanliness, its root for
			// comparison only — never read): gone: rows hold the index to
			// THAT tree (design §3.1), as `fathom verify --store` does.
			// Undeclared, the plan's tree is taken to be the indexed root.
			PlanTree *goneev.PlanTree `json:"plan_tree"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: service required"}
		}

		// plan_path (O1069) names a file under the service's indexed root:
		// read it when no plan_markdown or tasks came with it. An absolute,
		// escaping or missing path is an error naming the accepted form —
		// never a silent empty plan.
		var (
			repoPath, indexedCommit string
			metaFound, metaRead     bool
			metaErr                 error
		)
		planRel, planFromFile := "", false
		if p.PlanPath != "" {
			repoPath, indexedCommit, metaFound, metaErr = backend.ServiceMeta(ctx, p.Service)
			metaRead = true
			rel, perr := planPathRel(p.PlanPath)
			if perr != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: " + perr.Error()}
			}
			planRel = rel
			if p.PlanMarkdown == "" && len(p.Tasks) == 0 {
				raw, rerr := readPlanUnderRoot(ctx, repoPath, metaFound && metaErr == nil, p.Service, rel)
				if rerr != nil {
					return nil, &rpcError{Code: ErrCodeParams, Message: "verify_plan: " + rerr.Error()}
				}
				p.PlanMarkdown, planFromFile = string(raw), true
			}
		}

		// The evidence stage — the service's symbols, the graph every
		// reachability check walks and what its product entry points reach
		// — from the graph store's watermark-keyed memo (O33, RT-74), waited
		// for no longer than this call's deadline allows (EW-22, O130).
		stage, serr := verifyPlanWithDeadline(ctx, backend, p.Service)
		if serr != nil && !errors.Is(serr, errVerifyStageBuilding) {
			return nil, navLoadError("verify_plan", p.Service, serr)
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
			if planRel != "" && metaErr == nil && metaFound && repoPath != "" {
				prior := selfSkipPriorPlan(ctx, repoPath, planRel, p.Service, []byte(p.PlanMarkdown))
				plan = verify.AnnotateSelfSkip(plan, prior)
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

		if errors.Is(serr, errVerifyStageBuilding) {
			// The graph this index state needs is still building: answer
			// within the deadline, every row elided and the build named —
			// the flights it runs in are kept, so the next call is served
			// from them rather than starting over (O130). At shed or refuse
			// the build may never finish, so the partial names the level
			// and the budget knob instead (EW-79, O1073, I38).
			partial := "the call graph for this index state is still building (the first call after an ingest or a daemon start pays for it); no row was verified — the build keeps running for the next call"
			if lvl := tenancyLevelNote(); lvl != "" {
				partial = "the call graph for this index state did not finish building; no row was verified — " + lvl
			}
			return map[string]any{
				"service": p.Service,
				"summary": map[string]any{"total": len(plan.Tasks), "pass": 0, "fail": 0, "unknown": 0},
				"tasks":   []any{},
				"elided":  elisionFor(0, len(plan.Tasks), "plan order", "call verify_plan again: the next call is served from the build this one left running"),
				"partial": partial,
			}, nil
		}
		nodes, graph, product := stage.nodes, stage.graph, stage.product

		// Contradictions (best-effort; scoped by concern when provided).
		var contras []core.Contradiction
		if res, qErr := backend.QuerySymbols(ctx, core.Query{ServiceID: p.Service, ConcernID: p.Concern, TopN: 500}); qErr == nil {
			contras = res.Contradictions
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
			product: product, nodes: nodes, graphFallback: stage.typedErr,
			metaRead: metaRead, repoPath: repoPath, indexedCommit: indexedCommit, metaFound: metaFound, metaErr: metaErr,
			planTree: p.PlanTree,
		}
		// A plan read from the indexed root has that tree as its working
		// tree: its lexical oracles read there too (FileEvidence), as
		// `fathom verify` reads them from the tree the plan came from.
		var evidence verify.Evidence = ev
		if planFromFile {
			evidence = verifyEvidenceWithFiles{verifyEvidence: ev, DirFiles: verify.DirFiles(repoPath)}
		}

		out := verifyReportToMap(verify.VerifyPlan(plan, evidence))
		// O1162: which graph branch ran rides the verdict_outcome line (a
		// field, never a new record), and a fallback is named in a partial.
		if vo, ok := out["verdict_outcome"].(map[string]any); ok {
			fb := 0
			if stage.typedErr != "" {
				fb = 1
			}
			vo["graph_fallback"] = fb
		}
		if stage.typedErr != "" {
			note := "the typed call graph could not be read (" + stage.typedErr + "); reachability criteria (test_reaches, edge_wired, product reach) that did not pass read Unknown, not failing — " + verify.GraphFallbackRemedy
			if lvl := tenancyLevelNote(); lvl != "" {
				note += "; " + lvl
			}
			out["partial"] = note
		}
		if specCov.LowerBound() {
			// I38: the spec matrix is a lower bound — an "uncovered" spec
			// may sit in a file the budgeted scan never reached.
			note := "spec matrix " + partialNote(specCov) + " (an uncovered spec may be in an unscanned file)"
			if prev, ok := out["partial"].(string); ok {
				note = prev + "; " + note
			}
			out["partial"] = note
		}
		return out, nil
	})
}

// verifyStage is what verify_plan's per-row evaluation reads: the service's
// symbols, the graph the reachability checks walk (syntax plus typed
// edges, or the syntax edges alone when the typed edges could not be read)
// and what that graph's product entry points reach.
type verifyStage struct {
	// typedErr is why the typed edges could not be read ("" when they
	// were): the graph is then the syntax graph and the reachability
	// criteria read Unknown naming it (O1162, I38).
	typedErr string
	nodes    []core.SymbolNode
	graph    *callgraph.Graph
	product  *callgraph.ProductReach
}

// errVerifyStageBuilding is verifyPlanWithDeadline's "not ready within the
// deadline": the stage's builds are still running and the caller answers
// with a named partial rather than letting the deadline abandon the call.
var errVerifyStageBuilding = errors.New("the evidence stage is still building")

// verifyStageReserve is the share of a call's remaining time held back for
// the per-row evaluation and the answer: the stage is waited for until the
// deadline less this reserve, between verifyStageReserveMin and
// verifyStageReserveMax.
const (
	verifyStageReserveShare = 6
	verifyStageReserveMin   = 100 * time.Millisecond
	verifyStageReserveMax   = 5 * time.Second
)

// verifyPlanWithDeadline loads verify_plan's evidence stage (EW-22, O130).
// Each part is a memo flight in the graph store (the snapshot, the typed
// edges, the graph, the product-reach walk), shared with every other
// caller and kept once it has run half its expected cost, so a call that
// gives up waiting leaves it running for the next one. It waits no longer
// than ctx's deadline less a reserve for the rows themselves: when the
// stage is not ready by then it returns errVerifyStageBuilding and the
// caller answers within its deadline, instead of the deadline abandoning
// the call with nothing (the first verify after an ingest or a daemon
// start). Only the one graph verify reads is built — not the nav tools'
// syntax-only and resolved-only graphs.
func verifyPlanWithDeadline(ctx context.Context, backend QueryInterface, service string) (*verifyStage, error) {
	wctx := ctx
	if dl, ok := ctx.Deadline(); ok {
		reserve := min(max(time.Until(dl)/verifyStageReserveShare, verifyStageReserveMin), verifyStageReserveMax)
		var cancel context.CancelFunc
		wctx, cancel = context.WithDeadline(ctx, dl.Add(-reserve))
		defer cancel()
	}
	st, err := loadVerifyStage(wctx, backend, service)
	if err != nil && wctx.Err() != nil && ctx.Err() == nil {
		return nil, errVerifyStageBuilding
	}
	return st, err
}

// verifyTypedLoader is the typed-edge loader verify_plan's stage reads
// (a seam: tests inject a failing read to reach the syntax-graph fallback).
var verifyTypedLoader = typedLoader

// loadVerifyStage builds (or reads from the graph store) the stage at the
// service's current watermark. A typed-edge read failure degrades the call
// to the syntax graph and is never cached: the next call reads the typed
// edges again (I38).
func loadVerifyStage(ctx context.Context, backend QueryInterface, service string) (*verifyStage, error) {
	snap, err := serviceSnapshot(ctx, backend, service)
	if err != nil {
		return nil, err
	}
	typed := verifyTypedLoader(backend, service)
	kind := memo.GraphBoth
	graph, _, typedErr, err := memo.GraphForTyped(ctx, snap, kind, typed)
	if err != nil {
		return nil, err
	}
	fallback := ""
	if typedErr != nil {
		kind, typed = memo.GraphSyntax, nil // graph is the syntax graph
		fallback = typedErr.Error()
	}
	product, err := memo.ProductReachFor(ctx, snap, kind, typed)
	if err != nil {
		return nil, fmt.Errorf("product reach: %w", err)
	}
	return &verifyStage{typedErr: fallback, nodes: snap.Nodes, graph: graph, product: product}, nil
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
	// graphFallback is why the typed edges could not be read ("" when they
	// were): verify.GraphFallbackEvidence.
	graphFallback string
	// now is the clock runtime_pass reads a run's age against (nil: the
	// wall clock); runtime is the service's attested runs, read once on
	// first use, runtimeErr a failed read (TC-29).
	now         func() time.Time
	runtime     *testevidence.RuntimeRuns
	runtimeErr  error
	runtimeRead bool
	// nodes are the stage's symbols (gone: callers' referent locations);
	// repoPath, indexedCommit, metaFound and metaErr the service's ingest
	// record (read on first need: metaRead); planTree the caller's
	// declared plan tree (nil: the indexed root); gone the shared gone:
	// evidence, built on the first gone: row.
	nodes         []core.SymbolNode
	metaRead      bool
	repoPath      string
	indexedCommit string
	metaFound     bool
	metaErr       error
	planTree      *goneev.PlanTree
	gone          *goneev.Evidence
}

// verifyEvidenceWithFiles is verifyEvidence over a known working tree (a
// plan read through plan_path): every capability of verifyEvidence plus
// verify.FileEvidence over that tree.
type verifyEvidenceWithFiles struct {
	*verifyEvidence
	verify.DirFiles
}

// GraphFallback implements verify.GraphFallbackEvidence.
func (e *verifyEvidence) GraphFallback() string { return e.graphFallback }

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

// Reachable implements verify.Evidence over typed edges only (RF-5, I57):
// the same callgraph.ReachableTyped cmd/fathom's cliEvidence reads.
func (e *verifyEvidence) Reachable(from, to string) bool {
	ok, _ := callgraph.ReachableTyped(e.graph, from, to)
	return ok
}

// HeuristicReach implements verify.HeuristicReachEvidence: the heuristic
// edge a refused reach rested on.
func (e *verifyEvidence) HeuristicReach(from, to string) (verify.HeuristicEdge, bool) {
	_, via := callgraph.ReachableTyped(e.graph, from, to)
	if via == nil {
		return verify.HeuristicEdge{}, false
	}
	return heuristicEdgeOf(*via), true
}

// heuristicEdgeOf translates callgraph's heuristic edge into verify's
// (verify imports no analysis package).
func heuristicEdgeOf(h callgraph.HeuristicEdge) verify.HeuristicEdge {
	return verify.HeuristicEdge{From: h.From, To: h.To, Kind: string(h.Kind), Tier: h.Tier().String()}
}

// ProductHeuristicReach implements verify.HeuristicReachEvidence: the
// product entry point that reaches symbol only through a heuristic edge.
func (e *verifyEvidence) ProductHeuristicReach(symbol string) (string, verify.HeuristicEdge, bool) {
	entry, via, ok := e.product.HeuristicReachedBy(symbol)
	if !ok {
		return "", verify.HeuristicEdge{}, false
	}
	return entry, heuristicEdgeOf(via), true
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

// NewestAttestedRuns implements verify.RuntimeEvidence (TC-29): the newest
// attested run of each named test, read once per verify_plan call from the
// store's test_runs through the store-backed backend (conserveBackend's
// Store accessor). A backend with no store reads no attested runs, and
// runtime_pass says so — the daemon never runs a test either.
func (e *verifyEvidence) NewestAttestedRuns(tests []string) (map[string]verify.AttestedRun, error) {
	if !e.runtimeRead {
		e.runtimeRead = true
		if st, ok := storeOf(e.backend); !ok {
			e.runtimeErr = errors.New("this backend holds no store of attested runs")
		} else {
			e.runtime, e.runtimeErr = testevidence.LoadRuntimeRuns(e.ctx, st, e.service, e.clock())
		}
	}
	if e.runtimeErr != nil {
		return nil, e.runtimeErr
	}
	return e.runtime.NewestAttestedRuns(tests)
}

// Now implements verify.RuntimeEvidence: the clock a run's age is read
// against (injected for tests, else the wall clock).
func (e *verifyEvidence) Now() time.Time { return e.clock()() }

func (e *verifyEvidence) clock() func() time.Time {
	if e.now != nil {
		return e.now
	}
	return time.Now
}

var _ verify.RuntimeEvidence = (*verifyEvidence)(nil)

var (
	_ verify.Evidence               = (*verifyEvidence)(nil)
	_ verify.DetailedEvidence       = (*verifyEvidence)(nil)
	_ verify.ProductEvidence        = (*verifyEvidence)(nil)
	_ verify.HeuristicReachEvidence = (*verifyEvidence)(nil)
	_ verify.GraphFallbackEvidence  = (*verifyEvidence)(nil)
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
	// O1160: the same aggregate the CLI gate prints, so the daemon and the
	// CLI agree on why gone: rows are held back.
	if stale, ok := verify.IndexStaleOf([]verify.Report{rep}); ok {
		out["index_stale"] = stale
	}
	// O1162: and the aggregate for rows the graph fallback held back.
	if fb, ok := verify.GraphFallbackOf([]verify.Report{rep}); ok {
		out["graph_fallback"] = fb
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
