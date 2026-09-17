// Package mcp — extra MCP tool registrations wiring the many implemented-but-
// previously-unexposed analyzers.
//
// registerExtraTools is called from registerTools (tools.go) and adds tools in
// the following groups:
//
//	investigation: blast_radius, diff_impact, spec_coverage,
//	               traceability, evidence_summary,
//	               contradiction_scan, review_diff, search
//	coding:        arch_conformance, stale_docs, api_diff, dep_drift
//	shared:        ingest
//	platform:      cross_repo_blast_radius
//	context:       repo_map, pack_context
//	ops/meta:      batch
//	proto:         proto_conformance
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/apisurface"
	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/analysis/deps"
	"github.com/geoffmilleraz/fathom/internal/analysis/docdrift"
	"github.com/geoffmilleraz/fathom/internal/analysis/envkeys"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/proto"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/concern"
	"github.com/geoffmilleraz/fathom/internal/context/packer"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/graph"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// registerExtraTools is the single entry-point that wires every new tool.
// It is called from registerTools immediately after all existing groups are
// registered.
func registerExtraTools(s *Server, backend QueryInterface) {
	// ── investigation ────────────────────────────────────────────────────────
	registerBlastRadius(s, backend)
	registerDiffImpact(s, backend)
	registerSpecCoverage(s, backend)
	registerTraceability(s, backend)
	registerEvidenceSummary(s, backend)
	registerContradictionScan(s, backend)
	registerReferences(s, backend)
	registerSearch(s, backend)
	registerVerifyPlan(s, backend)
	registerReviewDiff(s, backend)
	registerFindingsNear(s, backend)

	// ── coding ───────────────────────────────────────────────────────────────
	registerArchConformance(s, backend)
	registerStaleDocs(s, backend)
	registerAPIDiff(s, backend)
	registerDepDrift(s, backend)

	// ── shared / security ────────────────────────────────────────────────────
	registerIngest(s, backend)
	registerRemoveService(s, backend)
	registerRenameService(s, backend)

	// ── platform ─────────────────────────────────────────────────────────────
	registerCrossRepoBlastRadius(s, backend)

	// ── context ──────────────────────────────────────────────────────────────
	registerRepoMap(s, backend)
	registerPackContext(s, backend)

	// ── ops / meta ───────────────────────────────────────────────────────────
	registerBatch(s)

	// ── proto ────────────────────────────────────────────────────────────────
	registerProtoConformance(s, backend)

	// ── spec / context ───────────────────────────────────────────────────────
	registerSpecRefs(s, backend)
	registerBrief(s, backend)

	// ── graph traversal ──────────────────────────────────────────────────────
	registerTraverse(s, backend)

	// ── investigation: history ───────────────────────────────────────────────────
	registerHistory(s, backend)
}

// ─── platform: service_ref hops (VB-30) ──────────────────────────────────────

// serviceRefHopsPayload renders hops via serviceref.Hop.Payload() — shared
// by blast_radius, cross_repo_blast_radius and review_diff so their
// service_ref reporting never drifts into three slightly different shapes.
// Always kind:"service_ref", tier:"heuristic" per hop (core.
// CrossRepoEdgeConfidenceOf's meaning: a normalized-string match across
// repositories, never confirmed by a compiler-grade resolver).
func serviceRefHopsPayload(hops []serviceref.Hop) []any {
	out := make([]any, len(hops))
	for i, h := range hops {
		out[i] = h.Payload()
	}
	return out
}

// serviceRefHopsInto reports the service_ref hops (VB-30) whose ToService
// is service and whose ToFile is one of changedFiles: another service's
// client call joins a route THIS change touches, so that OTHER service's
// call site is part of the blast radius even though no same-process call
// graph connects them. Computed fleet-wide (serviceref.BuildFromStore
// across every known service) each call — the same no-caching precedent
// cross_repo_blast_radius already sets by re-listing every service's
// symbols on every call. Best-effort: a fleet of fewer than two services,
// or any backend error, degrades to no hops rather than failing the whole
// tool.
func serviceRefHopsInto(ctx context.Context, backend QueryInterface, service string, changedFiles []string) []serviceref.Hop {
	services, err := backend.Services(ctx)
	if err != nil || len(services) < 2 {
		return nil
	}
	hops, err := serviceref.BuildFromStore(ctx, backend, services)
	if err != nil {
		return nil
	}
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	var out []serviceref.Hop
	for _, h := range hops {
		if h.ToService == service && changed[h.ToFile] {
			out = append(out, h)
		}
	}
	return out
}

// ─── investigation: blast_radius ─────────────────────────────────────────────

func registerBlastRadius(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "blast_radius",
		Description: "The answer grep cannot give: blast radius of changed files — the symbols they define plus transitive callers, with affected tests, file paths, and by_depth (depth 0 = changed symbols, 1 = direct callers, …), each labeled edge_confidence. DEFAULTS changed in 0.9: resolved edges only, depth 2, at most 500 symbols — widen with include_heuristic / max_depth / max_symbols (0 = unbounded). Always reports edge_filter, total_symbols, truncated and depth_capped, so a bounded answer never reads as complete. Reach for it when you changed a file and need to know who transitively breaks — no grep query walks a call graph. Required: service, changed_files.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":           map[string]any{"type": "string", "description": "Service / repo identifier"},
				"changed_files":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative file paths that changed"},
				"include_heuristic": map[string]any{"type": "boolean", "description": "Also follow syntax-only (heuristic) edges. Default false."},
				"max_depth":         map[string]any{"type": "integer", "description": "Hops from the changed files (default 2; 0 = unbounded)."},
				"max_symbols":       map[string]any{"type": "integer", "description": "Symbols listed, shallowest first (default 500; 0 = uncapped); total_symbols keeps the full count."},
			},
			"required": []string{"service", "changed_files"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service          string   `json:"service"`
			ChangedFiles     []string `json:"changed_files"`
			IncludeHeuristic bool     `json:"include_heuristic"`
			MaxDepth         *int     `json:"max_depth"`
			MaxSymbols       *int     `json:"max_symbols"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: service required"}
		}
		if len(p.ChangedFiles) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: changed_files required"}
		}
		navs := s.loadNavGraphs(ctx, backend, p.Service)
		if navs == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "blast_radius: no symbols for service " + p.Service}
		}
		// BP-27: bounded, resolved-by-default. The unbounded both-kinds
		// walk (impact.BlastRadiusWithTypedEdges) is untouched and is still
		// what the verify engine's reachability reads.
		res := blastRadiusResolved(navs, p.ChangedFiles, p.IncludeHeuristic,
			intOr(p.MaxDepth, defaultBlastDepth), intOr(p.MaxSymbols, defaultBlastSymbols))
		result := map[string]any{
			"symbols":       stringsToAny(res.Symbols),
			"tests":         stringsToAny(res.Tests),
			"files":         stringsToAny(res.Files),
			"edge_filter":   res.EdgeFilter,
			"total_symbols": res.TotalSymbols,
			"truncated":     res.Truncated,
			"depth_capped":  res.DepthCapped,
			"max_depth":     res.MaxDepthReached,
			// BD-3 (docs/plans/blast-depth.md): the same symbols regrouped
			// by distance from the changed files, each labeled with the
			// confidence of the edge that pulled it in — under the SAME
			// graph and bounds the listing used (BP-27).
			"by_depth": res.Buckets,
			// resolution (VB-19): the same by_depth counts, summed to one
			// envelope-level typed/heuristic mix.
			"resolution": resolutionFromBlastBuckets(res.Buckets),
			// VB-30: other services whose outbound HTTP client call joins a
			// route one of changed_files registers — a hop no same-process
			// call graph can see. Each entry kind:"service_ref",
			// tier:"heuristic". Empty (never omitted) when the fleet has
			// fewer than two services or no such hop exists.
			"service_ref_hops": serviceRefHopsPayload(serviceRefHopsInto(ctx, backend, p.Service, p.ChangedFiles)),
		}
		if ic := indexCommitFor(ctx, backend, p.Service); ic != "" {
			result["index_commit"] = ic
		}
		return result, nil
	})
}

// intOr returns *p when the caller supplied it (0 is a real value meaning
// "unbounded"), else def.
func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	if *p < 0 {
		return def
	}
	return *p
}

// ─── investigation: diff_impact ──────────────────────────────────────────────

func registerDiffImpact(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "diff_impact",
		Description: "Alias for blast_radius suited for diff/PR workflows: compute the impact set of changed files (symbols + callers + tests).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":       map[string]any{"type": "string", "description": "Service / repo identifier"},
				"changed_files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative file paths that changed"},
			},
			"required": []string{"service", "changed_files"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service      string   `json:"service"`
			ChangedFiles []string `json:"changed_files"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "diff_impact: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "diff_impact: service required"}
		}
		if len(p.ChangedFiles) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "diff_impact: changed_files required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Typed Go call edges (ADR-021 §3): additive on the transitive-caller
		// walk — best-effort, a non-Go service contributes nothing.
		typed, _ := backend.TypedEdges(ctx, p.Service)
		result := impact.DiffImpactWithTypedEdges(nodes, p.ChangedFiles, typed)
		return map[string]any{
			"symbols":  stringsToAny(result.Symbols),
			"tests":    stringsToAny(result.Tests),
			"files":    stringsToAny(result.Files),
			"by_depth": blastRadiusByDepth(nodes, p.ChangedFiles, typed), // BD-3, identical to blast_radius
		}, nil
	})
}

// ─── investigation: spec_coverage ────────────────────────────────────────────

func registerSpecCoverage(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "spec_coverage",
		Description: "Build a spec coverage matrix: for each declared spec ID report which symbols implement it, which specs are uncovered, and which referenced IDs are orphans (not in the declared set).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":  map[string]any{"type": "string", "description": "Service / repo identifier"},
				"spec_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Declared spec IDs (e.g. [\"[US-001]\",\"[EP-002]\"])"},
				"prefixes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Spec-ID prefixes to scan for (default: US, EP, FR, NFR)"},
			},
			"required": []string{"service", "spec_ids"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service  string   `json:"service"`
			SpecIDs  []string `json:"spec_ids"`
			Prefixes []string `json:"prefixes"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_coverage: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_coverage: service required"}
		}
		// Scan Sources for spec refs; fall back to nodes if Sources unavailable.
		files, _ := backend.Sources(ctx, p.Service)
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Build codeRefs map: symbol → []specID.
		codeRefs := make(map[string][]string)
		// From nodes (claim text may contain spec refs).
		for _, n := range nodes {
			var refs []string
			if len(p.Prefixes) > 0 {
				refs = sdd.ExtractRefsWithPrefixes(n.Signature+" "+n.Name, p.Prefixes)
			} else {
				refs = sdd.ExtractRefs(n.Signature + " " + n.Name)
			}
			if len(refs) > 0 {
				codeRefs[n.Name] = refs
			}
		}
		// From source files.
		for _, f := range files {
			var refs []string
			if len(p.Prefixes) > 0 {
				refs = sdd.ExtractRefsWithPrefixes(string(f.Content), p.Prefixes)
			} else {
				refs = sdd.ExtractRefs(string(f.Content))
			}
			if len(refs) > 0 {
				key := f.Path
				codeRefs[key] = append(codeRefs[key], refs...)
			}
		}
		matrix := sdd.BuildMatrix(p.SpecIDs, codeRefs)
		uncovered := matrix.Uncovered(p.SpecIDs)
		orphans := matrix.Orphans(codeRefs)
		// Build coverage map spec → symbols.
		covered := make(map[string]any, len(p.SpecIDs))
		for _, id := range p.SpecIDs {
			syms := matrix.Coverage(id)
			if syms == nil {
				syms = []string{}
			}
			covered[id] = stringsToAny(syms)
		}
		return map[string]any{
			"coverage":  covered,
			"uncovered": stringsToAny(uncovered),
			"orphans":   stringsToAny(orphans),
		}, nil
	})
}

// ─── investigation: traceability ─────────────────────────────────────────────

func registerTraceability(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "traceability",
		Description: "Extract spec-ID references from source files and build a symbol → spec-ID traceability map.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":  map[string]any{"type": "string", "description": "Service / repo identifier"},
				"prefixes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Spec-ID prefixes (default: US, EP, FR, NFR)"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service  string   `json:"service"`
			Prefixes []string `json:"prefixes"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "traceability: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "traceability: service required"}
		}
		files, _ := backend.Sources(ctx, p.Service)
		refs := make(map[string]any)
		for _, f := range files {
			var specRefs []string
			if len(p.Prefixes) > 0 {
				specRefs = sdd.ExtractRefsWithPrefixes(string(f.Content), p.Prefixes)
			} else {
				specRefs = sdd.ExtractRefs(string(f.Content))
			}
			if len(specRefs) > 0 {
				refs[f.Path] = stringsToAny(specRefs)
			}
		}
		return map[string]any{"refs": refs}, nil
	})
}

// ─── investigation: evidence_summary ─────────────────────────────────────────

func registerEvidenceSummary(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "evidence_summary",
		Description: "Return per-tier observation counts for a service (verified, pre-scan, doc-claimed, sme-stated, assumed).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		svc, rpcErr := serviceArg(args, "evidence_summary")
		if rpcErr != nil {
			return nil, rpcErr
		}
		// Derive observations from ListAllSymbols: convert each SymbolNode's Tier to a fake Observation.
		nodes, err := backend.ListAllSymbols(ctx, svc)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		obs := make([]core.Observation, len(nodes))
		for i, n := range nodes {
			obs[i] = core.Observation{
				ServiceID:    n.Service,
				EvidenceTier: n.Tier,
				Symbol:       n.Name,
			}
		}
		summary := concern.EvidenceSummary(obs)
		result := make(map[string]any, len(summary))
		for tier, count := range summary {
			result[string(tier)] = count
		}
		return map[string]any{"summary": result}, nil
	})
}

// ─── investigation: contradiction_scan ───────────────────────────────────────

func registerContradictionScan(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "contradiction_scan",
		Description: "Scan query results for contradictions: symbols that appear with conflicting kinds across sources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern": map[string]any{"type": "string", "description": "Concern ID to scope the scan"},
			},
			"required": []string{"service", "concern"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Concern string `json:"concern"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "contradiction_scan: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "contradiction_scan: service required"}
		}
		if p.Concern == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "contradiction_scan: concern required"}
		}
		result, err := backend.QuerySymbols(ctx, core.Query{
			ServiceID: p.Service,
			ConcernID: p.Concern,
			TopN:      200,
		})
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Use embedded contradictions if the backend returns them.
		contras := result.Contradictions
		out := make([]any, len(contras))
		for i, c := range contras {
			out[i] = map[string]any{
				"symbol":       c.Symbol,
				"claim_a":      c.A.Claim,
				"claim_b":      c.B.Claim,
				"tier_a":       string(c.A.EvidenceTier),
				"tier_b":       string(c.B.EvidenceTier),
				"source_ref_a": c.A.SourceRef,
				"source_ref_b": c.B.SourceRef,
			}
		}
		// Also detect kind-conflicts across nodes.
		kindByName := make(map[string]string)
		type conflict struct {
			symbol string
			kindA  string
			kindB  string
		}
		var conflicts []conflict
		for _, n := range result.Nodes {
			if prev, seen := kindByName[n.Name]; seen && prev != string(n.Kind) {
				conflicts = append(conflicts, conflict{n.Name, prev, string(n.Kind)})
			} else {
				kindByName[n.Name] = string(n.Kind)
			}
		}
		for _, cf := range conflicts {
			out = append(out, map[string]any{
				"symbol": cf.symbol,
				"kind_a": cf.kindA,
				"kind_b": cf.kindB,
				"type":   "kind_conflict",
			})
		}
		return map[string]any{"contradictions": out, "count": len(out)}, nil
	})
}

// ─── coding: arch_conformance ─────────────────────────────────────────────────

func registerArchConformance(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "arch_conformance",
		Description: "Check the service call-graph edges against architectural layering rules and report violations. When no inline rules are passed, falls back to the service's own .fathom/rules/archconformance.cue project rule pack; the response's rule_source names which source was used (\"inline\" | \"project\" | \"none\"), and checked is false (with a reason) when no rules were available from either source — never a clean-shaped zero-violation answer for a check that did not run.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"rules": map[string]any{
					"type":        "array",
					"description": "List of {from_pkg,to_pkg,allow} rules. Use \"*\" as wildcard. When omitted or empty, the service's .fathom/rules/archconformance.cue project rule pack is used if present.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"from_pkg": map[string]any{"type": "string"},
							"to_pkg":   map[string]any{"type": "string"},
							"allow":    map[string]any{"type": "boolean"},
						},
					},
				},
				"deny_by_default": map[string]any{"type": "boolean", "description": "Deny edges not matched by any allow rule. When project rules are used, this is OR'd with that rule pack's own deny_by_default."},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service       string `json:"service"`
			DenyByDefault bool   `json:"deny_by_default"`
			Rules         []struct {
				FromPkg string `json:"from_pkg"`
				ToPkg   string `json:"to_pkg"`
				Allow   bool   `json:"allow"`
			} `json:"rules"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "arch_conformance: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "arch_conformance: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Build edges from Calls, both sides normalized to DECLARING
		// packages (P1-4, closes the C4-4 disclosed gap — see schema/
		// rules/archconformance/schema.cue #Rule, now updated to "closed").
		// archconformance.Check derives a rule-matchable "package" as
		// everything before an edge's LAST dot (pkgOf); EdgesFromNodes
		// resolves both FromSymbol and ToSymbol to "declPkg.BareName" so
		// that holds for METHOD-origin and method-TARGET edges too, not
		// only function-to-function ones — see EdgesFromNodes' own doc
		// comment for how. Shared with review_diff (C1-1) via the
		// archconformance package — one implementation, never two that
		// could drift.
		edges := archconformance.EdgesFromNodes(nodes)

		inlineRules := make([]archconformance.Rule, len(p.Rules))
		for i, r := range p.Rules {
			inlineRules[i] = archconformance.Rule{FromPkg: r.FromPkg, ToPkg: r.ToPkg, Allow: r.Allow}
		}

		// C4-2/C4-3: when the caller passes no inline rules, fall back to
		// the service's own project rule pack — and report honestly
		// (checked:false + reason) when neither source has anything to
		// check, rather than the confirmed C4-5 bug of a clean-shaped
		// {violations:[],count:0} indistinguishable from a real scan.
		// projectrules.ResolveRules is the single shared implementation of
		// this three-way split (also used by review_diff, C1-1).
		corpusRoot := ""
		if path, _, found, metaErr := backend.ServiceMeta(ctx, p.Service); metaErr == nil && found {
			corpusRoot = path
		}
		rules, ruleSource, denyByDefault, checked, reason, resolveErr := projectrules.ResolveRules(corpusRoot, inlineRules, p.DenyByDefault)
		if resolveErr != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "arch_conformance: project rules: " + resolveErr.Error()}
		}

		var violations []archconformance.Violation
		if checked {
			var opts []archconformance.Option
			if denyByDefault {
				opts = append(opts, archconformance.DenyByDefault())
			}
			violations = archconformance.Check(edges, rules, opts...)
		}
		result := make([]any, len(violations))
		for i, v := range violations {
			result[i] = map[string]any{
				"from":   v.From,
				"to":     v.To,
				"reason": v.Reason,
			}
		}
		resp := map[string]any{
			"violations":  result,
			"count":       len(violations),
			"rule_source": ruleSource,
			"checked":     checked,
		}
		if !checked {
			resp["reason"] = reason
		}
		return resp, nil
	})
}

// ─── coding: stale_docs ───────────────────────────────────────────────────────

func registerStaleDocs(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "stale_docs",
		Description: "Scan Markdown source files for stale references: inline code spans or link targets that no longer correspond to known symbols or files.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		svc, rpcErr := serviceArg(args, "stale_docs")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		nodes, err := backend.ListAllSymbols(ctx, svc)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Build known-symbol and known-file sets.
		knownSymbols := make(map[string]bool, len(nodes))
		for _, n := range nodes {
			knownSymbols[n.Name] = true
		}
		knownFiles := make(map[string]bool, len(files))
		for _, f := range files {
			knownFiles[f.Path] = true
		}
		var allDrifts []docdrift.Drift
		for _, f := range files {
			if f.Lang != "md" {
				continue
			}
			allDrifts = append(allDrifts, docdrift.Analyze(f.Path, string(f.Content), knownSymbols, knownFiles)...)
		}
		sort.Slice(allDrifts, func(i, j int) bool {
			if allDrifts[i].File != allDrifts[j].File {
				return allDrifts[i].File < allDrifts[j].File
			}
			if allDrifts[i].Line != allDrifts[j].Line {
				return allDrifts[i].Line < allDrifts[j].Line
			}
			return allDrifts[i].Ref < allDrifts[j].Ref
		})
		result := make([]any, len(allDrifts))
		for i, d := range allDrifts {
			result[i] = map[string]any{
				"file": d.File,
				"line": d.Line,
				"ref":  d.Ref,
			}
		}
		return map[string]any{"drifts": result, "count": len(result)}, nil
	})
}

// ─── coding: api_diff ─────────────────────────────────────────────────────────

func registerAPIDiff(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "api_diff",
		Description: "Compute the API surface diff between two services: added, removed, and changed exported symbols.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service_a": map[string]any{"type": "string", "description": "Baseline service identifier"},
				"service_b": map[string]any{"type": "string", "description": "New service identifier"},
			},
			"required": []string{"service_a", "service_b"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			ServiceA string `json:"service_a"`
			ServiceB string `json:"service_b"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "api_diff: invalid args"}
		}
		if p.ServiceA == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "api_diff: service_a required"}
		}
		if p.ServiceB == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "api_diff: service_b required"}
		}
		nodesA, err := backend.ListAllSymbols(ctx, p.ServiceA)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "list service_a: " + err.Error()}
		}
		nodesB, err := backend.ListAllSymbols(ctx, p.ServiceB)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "list service_b: " + err.Error()}
		}
		surfaceA := apisurface.Surface(nodesA)
		surfaceB := apisurface.Surface(nodesB)
		changes := apisurface.Diff(surfaceA, surfaceB)
		added := changes.Added
		if added == nil {
			added = []string{}
		}
		removed := changes.Removed
		if removed == nil {
			removed = []string{}
		}
		changed := changes.Changed
		if changed == nil {
			changed = []string{}
		}
		return map[string]any{
			"added":   stringsToAny(added),
			"removed": stringsToAny(removed),
			"changed": stringsToAny(changed),
		}, nil
	})
}

// ─── coding: dep_drift ────────────────────────────────────────────────────────

func registerDepDrift(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "dep_drift",
		Description: "Compare dependency versions between two services' lockfiles and report packages with version mismatches.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service_a": map[string]any{"type": "string", "description": "Baseline service identifier"},
				"service_b": map[string]any{"type": "string", "description": "Comparison service identifier"},
			},
			"required": []string{"service_a", "service_b"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			ServiceA string `json:"service_a"`
			ServiceB string `json:"service_b"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dep_drift: invalid args"}
		}
		if p.ServiceA == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dep_drift: service_a required"}
		}
		if p.ServiceB == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "dep_drift: service_b required"}
		}
		parseDeps := func(svc string) ([]deps.Dependency, *rpcError) {
			files, err := backend.Sources(ctx, svc)
			if err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "sources " + svc + ": " + err.Error()}
			}
			var all []deps.Dependency
			for _, f := range files {
				parsed, parseErr := deps.Detect(f.Path, f.Content)
				if parseErr != nil {
					continue
				}
				all = append(all, parsed...)
			}
			return all, nil
		}
		depsA, rpcErr := parseDeps(p.ServiceA)
		if rpcErr != nil {
			return nil, rpcErr
		}
		depsB, rpcErr := parseDeps(p.ServiceB)
		if rpcErr != nil {
			return nil, rpcErr
		}
		driftItems := deps.Drift(depsB, depsA) // compare B against A baseline
		result := make([]any, len(driftItems))
		for i, d := range driftItems {
			result[i] = map[string]any{
				"name":     d.Name,
				"current":  d.Current,
				"baseline": d.Baseline,
			}
		}
		return map[string]any{"drift": result, "count": len(result)}, nil
	})
}

// ─── shared: registry surgery ────────────────────────────────────────────────

func registerRemoveService(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "remove_service",
		Description: "Delete EVERY stored row for a service — registry surgery for duplicate/malformed rows (e.g. a path-like twin). Idempotent: removing an unknown service returns removed=false.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service to remove, exactly as registered (path-like historical ids are accepted here so they can be excised)"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		rs, ok := backend.(registrySurgeryBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "remove_service: this backend does not support registry surgery"}
		}
		var p struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(args, &p); err != nil || p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "remove_service: service required"}
		}
		// NOTE: no ValidServiceID here, deliberately — the verb's first job
		// is excising rows whose ids are exactly the malformed kind the
		// ingest door now refuses ("../geoffmiller-cloud").
		removed, err := rs.RemoveService(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "remove_service: " + err.Error()}
		}
		hookcache.BustIdent()
		return map[string]any{"service": p.Service, "removed": removed}, nil
	})
}

func registerRenameService(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "rename_service",
		Description: "Atomically re-key every stored row from one service id to another. Refuses when the source has no rows or the target already exists. Claim ids keep their minted values (historical evidence); new compiles mint fresh ids.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from": map[string]any{"type": "string", "description": "Current service id (path-like historical ids accepted)"},
				"to":   map[string]any{"type": "string", "description": "New service id — a valid logical name"},
			},
			"required": []string{"from", "to"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		rs, ok := backend.(registrySurgeryBackend)
		if !ok {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "rename_service: this backend does not support registry surgery"}
		}
		var p struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.Unmarshal(args, &p); err != nil || p.From == "" || p.To == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_service: from and to required"}
		}
		if err := core.ValidServiceID(p.To); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "rename_service: the target must be a valid logical name (" + err.Error() + ")"}
		}
		if err := rs.RenameService(ctx, p.From, p.To); err != nil {
			return nil, &rpcError{Code: ErrCodeInvalid, Message: "rename_service: " + err.Error()}
		}
		hookcache.BustIdent()
		return map[string]any{"from": p.From, "to": p.To, "ok": true}, nil
	})
}

// ─── shared: ingest ───────────────────────────────────────────────────────────

func registerIngest(s *Server, backend QueryInterface) {
	// Ingest honesty (docs/plans/instrument-truth.md IT-15, O16): the
	// pipeline's refusal to replace a populated index with a wedged
	// reader's zero-symbol result — and its degraded "N file(s) could not
	// be read" outcome — arrive as an error whose text names the cause
	// (count, first path, errno, evidence protected). When the error
	// carries an operator hint (the external-volume TCC wedge) it rides
	// along, so the daemon-side caller sees the same fix the CLI prints.
	ingestFailure := func(err error) *rpcError {
		msg := "ingest: " + err.Error()
		var hinted interface{ Hint() string }
		if errors.As(err, &hinted) {
			if h := hinted.Hint(); h != "" {
				msg += " — hint: " + h
			}
		}
		return &rpcError{Code: ErrCodeInternal, Message: msg}
	}
	s.registerTool(toolDescriptor{
		Name:        "ingest",
		Description: "Re-ingest the named service to refresh the symbol store; pass path (absolute repo root) to onboard a NEW service into the index. Hash-gated: a clean repo is a sub-second no-op, so call it at session start and after every commit. A path that differs from an EXISTING service's recorded root is refused unless remap:true is also passed (O82) — a hook must never re-point a service; remap:true (or the `fathom remap` verb) is the deliberate-move escape hatch. Reach for it when staleness_report or a stale-index warning says the index lags the working tree. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"path":    map[string]any{"type": "string", "description": "Absolute corpus root — required only when first onboarding a new service"},
				"remap":   map[string]any{"type": "boolean", "description": "Authorize re-pointing an EXISTING service's recorded root at a different path (O82). Without it, a differing path is refused — a hook must never re-point a service."},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Path    string `json:"path"`
			// Remap authorizes a DELIBERATE move of an existing service's
			// recorded root (wave P3 / O82, .fathom/observations.md): the
			// escape hatch that parallels the store-direct `fathom remap`
			// verb. Without it, a path that differs from the service's
			// recorded root is refused outright — see the re-root refusal
			// below.
			Remap bool `json:"remap"`
		}
		if err := json.Unmarshal(args, &p); err != nil || p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "ingest: service required"}
		}
		svc := p.Service

		// Registration hygiene (plan IO-2; O1 fix d). Enforced HERE, in the
		// daemon's handler — client-side ValidServiceID checks let the
		// ../geoffmiller-cloud twin be re-fed for seven weeks because the
		// daemon accepted whatever arrived.
		if err := core.ValidServiceID(svc); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "ingest: a service id is a logical name, never a path (" + err.Error() + ") — pass the repo root separately via path"}
		}
		if p.Path != "" {
			if !filepath.IsAbs(p.Path) {
				return nil, &rpcError{Code: ErrCodeParams, Message: "ingest: path must be absolute — a relative root resolves against the daemon's working directory, not yours"}
			}
			canonical, cerr := filepath.EvalSymlinks(p.Path)
			if cerr != nil {
				return nil, &rpcError{Code: ErrCodeParams, Message: "ingest: path must exist and resolve: " + cerr.Error()}
			}
			// Duplicate/nested corpus-root refusal: one root, one service.
			// A second service on the same root made resolution a coin flip
			// (O1: 36% of edits silently lost obligations); a nested root
			// double-indexes every symbol underneath it (the
			// crossplane-platform accident).
			if services, serr := backend.Services(ctx); serr == nil {
				sep := string(filepath.Separator)
				for _, other := range services {
					if strings.EqualFold(other, svc) {
						continue
					}
					otherPath, _, found, merr := backend.ServiceMeta(ctx, other)
					if merr != nil || !found || otherPath == "" {
						continue
					}
					otherCanon, oerr := filepath.EvalSymlinks(otherPath)
					if oerr != nil {
						otherCanon = otherPath
					}
					switch {
					case otherCanon == canonical:
						return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: corpus root already claimed by service \"" + other + "\" — one root, one service; rename or remove that service first (rename_service / remove_service)"}
					case strings.HasPrefix(canonical, otherCanon+sep):
						return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: root is nested inside service \"" + other + "\" (" + otherCanon + ") — a nested corpus double-indexes every symbol under it; index the parent once"}
					case strings.HasPrefix(otherCanon, canonical+sep):
						return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: root encloses service \"" + other + "\" (" + otherCanon + ") — remove the nested service first (remove_service) or index the subtree only"}
					}
				}
			}
			// Worktree re-root refusal (PC-33, I30, O22): an EXISTING
			// service is never moved onto a linked git worktree of its own
			// recorded root (or onto any .claude/worktrees/ path). The
			// daemon once accepted exactly that from a worktree's
			// post-commit hook, answered {ok:true} four times, and every
			// "refreshed" verify afterwards read the wrong tree. The same
			// check guards the CLI's store-direct ingest (pkg/client).
			if recorded, _, found, merr := backend.ServiceMeta(ctx, svc); merr == nil && found && recorded != "" {
				if err := worktree.CheckReroot(svc, recorded, canonical); err != nil {
					return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: " + err.Error()}
				}
				// General re-root refusal (wave P3 / O82,
				// .fathom/observations.md): CheckReroot above only catches a
				// LINKED git worktree of the recorded root. A plain `git
				// clone` elsewhere — a separate physical checkout, not
				// `git worktree add` — sails straight through it: a
				// lefthook post-commit `fathom-refresh` step firing inside
				// a push clone called this tool with {service:"fathom",
				// path:<push clone>} and the daemon happily re-pointed the
				// fleet service at a directory that vanishes with the
				// clone. A HOOK must never re-point a service — that is a
				// human (or explicitly authorized) decision. `remap:true`
				// here is the deliberate-move escape hatch, parallel to the
				// store-direct `fathom remap` verb.
				recCanon, rcErr := filepath.EvalSymlinks(recorded)
				if rcErr != nil {
					recCanon = recorded
				}
				if recCanon != canonical && !p.Remap {
					return nil, &rpcError{Code: ErrCodeInvalid, Message: fmt.Sprintf(
						"ingest: service %q is already registered at %s — refusing to re-point it at %s without remap:true (a hook must never re-point a service; pass remap:true for a deliberate move, or use the `fathom remap` verb)",
						svc, recCanon, canonical)}
				}
			}
			p.Path = canonical
		}

		// Snapshot the process-lifetime unclassified/oversize counters
		// (ADR-021 §2 / TI-6) before the ingest, so the delta below reports
		// THIS call's counts — IngestService/IngestServiceAt return only an
		// error, so a before/after snapshot of the cumulative counters is
		// how the tool surfaces per-ingest visibility without changing
		// either method's signature.
		var before struct{ unclassified, oversize int64 }
		sb, hasStats := backend.(statsBackend)
		if hasStats {
			before.unclassified, before.oversize = sb.IngestStats()
		}

		var ingestErr error
		if p.Path != "" {
			ia, ok := backend.(ingestAtBackend)
			if !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: this backend does not support onboarding by path"}
			}
			ingestErr = ia.IngestServiceAt(ctx, svc, p.Path)
		} else {
			ingestErr = backend.IngestService(ctx, svc)
		}
		// Critic finding F2 (BLOCKING, phase-P fix round): bust the
		// CLI's service-identity cache (cmd/fathom/identcache.go) on
		// every ingest via this MCP tool that may have written — a hook
		// re-resolving identity through a daemon this ingest just ran
		// against must never see a cached pre-ingest answer for up to
		// the cache's TTL (the O15 class). A DEGRADED ingest (IT-14)
		// indexed every readable file and is returned as an error below,
		// so the bust must come first; an aborted one may have purged and
		// partially stored before it stopped. Only a REFUSED ingest
		// (IT-15) provably wrote nothing and leaves the cache alone.
		var readErr *fusion.ReadFailuresError
		if !(errors.As(ingestErr, &readErr) && readErr.Refused) {
			hookcache.BustIdent()
		}
		if ingestErr != nil {
			return nil, ingestFailure(ingestErr)
		}

		result := map[string]any{"service": svc, "ok": true}
		// Name the root that was ingested (O22: four ok:true envelopes and
		// not one of them said which tree). Additive; absent when the
		// backend records no root.
		if root, _, found, merr := backend.ServiceMeta(ctx, svc); merr == nil && found && root != "" {
			result["root"] = root
		}
		if hasStats {
			afterUnclassified, afterOversize := sb.IngestStats()
			result["unclassified_files"] = afterUnclassified - before.unclassified
			result["oversize_files"] = afterOversize - before.oversize
		}
		return result, nil
	})
}

// ─── platform: cross_repo_blast_radius ───────────────────────────────────────

func registerCrossRepoBlastRadius(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "cross_repo_blast_radius",
		Description: "Compute the blast radius of a symbol across all known services: finds the symbol in each service plus every cross-service caller.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{"type": "string", "description": "Symbol name or key to blast-radius across all services"},
			},
			"required": []string{"symbol"},
		},
		group: GroupPlatform,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "cross_repo_blast_radius: invalid args"}
		}
		if p.Symbol == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "cross_repo_blast_radius: symbol required"}
		}
		services, err := backend.Services(ctx)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		type hit struct {
			Service string `json:"service"`
			Symbol  string `json:"symbol"`
			File    string `json:"file"`
		}
		var hits []hit
		for _, svc := range services {
			nodes, err := backend.ListAllSymbols(ctx, svc)
			if err != nil {
				continue
			}
			// Find the symbol in this service.
			var seed []string
			for _, n := range nodes {
				if n.Name == p.Symbol || strings.HasSuffix(n.FilePath+"/"+n.Name, p.Symbol) {
					seed = append(seed, n.FilePath)
					hits = append(hits, hit{Service: svc, Symbol: n.Name, File: n.FilePath})
				}
			}
			if len(seed) > 0 {
				// Also compute callers in this service.
				result := impact.BlastRadius(nodes, seed)
				for _, sym := range result.Symbols {
					if sym != p.Symbol {
						hits = append(hits, hit{Service: svc, Symbol: sym})
					}
				}
			}
		}
		out := make([]any, len(hits))
		for i, h := range hits {
			out[i] = map[string]any{
				"service": h.Service,
				"symbol":  h.Symbol,
				"file":    h.File,
			}
		}

		// VB-30: also report every service_ref hop whose ToHandler matches
		// the requested symbol by bare name — the same fleet-wide,
		// bare-name matching style this tool already uses for hits above
		// (the intra-service ListAllSymbols loop), extended across the
		// process boundary a call-graph-only search cannot see.
		var hopHits []serviceref.Hop
		if allHops, err := serviceref.BuildFromStore(ctx, backend, services); err == nil {
			for _, h := range allHops {
				if h.ToHandler == p.Symbol {
					hopHits = append(hopHits, h)
				}
			}
		}

		return map[string]any{
			"hits":             out,
			"count":            len(out),
			"service_ref_hops": serviceRefHopsPayload(hopHits),
		}, nil
	})
}

// ─── context: repo_map ────────────────────────────────────────────────────────

func registerRepoMap(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "repo_map",
		Description: "Build a budget-bounded symbol map ranked by outbound call-edge count (most-connected symbols first), suitable for LLM context assembly. Budgeted; the elision line names the refinement (budget or package). Reach for it when you need the shape of a service you have never seen — the most-connected symbols first, within a token budget. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"budget":  map[string]any{"type": "integer", "description": "Token budget (default 4000)"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Budget  int    `json:"budget"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "repo_map: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "repo_map: service required"}
		}
		budget := p.Budget
		if budget <= 0 {
			budget, _ = budgetDefaultTokens("repo_map", s.budgets) // BA-5/BA-9: project → user → measured default (design §3 "map")
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		mapStr := packer.RepoMap(nodes, budget)
		// packer.RepoMap reports only the packed string, so the included
		// count is recovered from its own one-line-per-symbol output rather
		// than widening that package's signature (out of this row's scope).
		included := strings.Count(mapStr, "\n")
		resp := map[string]any{"map": mapStr, "budget": budget}
		if el := elisionFor(included, len(nodes), "outbound call-edge count, most-connected first, ties by package+name", "re-call with a larger budget"); el != nil {
			resp["elided"] = el
			// RC-19: the same one-line disclosure the other four newly-
			// budgeted tools carry, alongside (not instead of) the existing
			// structured "elided" block above — repo_map already had a
			// working disclosure; this just makes its wording uniform.
			if line := elisionLine(len(nodes)-included, "budget or package"); line != "" {
				resp["elision"] = line
			}
		}
		return resp, nil
	})
}

// ─── context: pack_context ────────────────────────────────────────────────────

func registerPackContext(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "pack_context",
		Description: "Pack the concern-ranked evidence items for one service into a token budget, returning the highest-scoring items that fit (id, text, score) plus what was dropped. Reach for it when you are assembling context for a task and want the most relevant symbols under a fixed token cost instead of a raw listing. Required: service, concern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern": map[string]any{"type": "string", "description": "Concern ID to scope the query"},
				"budget":  map[string]any{"type": "integer", "description": "Token budget (default 4000)"},
			},
			"required": []string{"service", "concern"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Concern string `json:"concern"`
			Budget  int    `json:"budget"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "pack_context: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "pack_context: service required"}
		}
		if p.Concern == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "pack_context: concern required"}
		}
		budget := p.Budget
		if budget <= 0 {
			budget, _ = budgetDefaultTokens("pack_context", s.budgets) // BA-5/BA-9: project → user → measured default (design §3 "composite")
		}
		result, err := backend.QuerySymbols(ctx, core.Query{
			ServiceID: p.Service,
			ConcernID: p.Concern,
			TopN:      100,
		})
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Convert nodes to packer.Evidence items.
		items := make([]packer.Evidence, len(result.Nodes))
		for i, n := range result.Nodes {
			text := n.Package + "." + n.Name + " " + n.Signature
			items[i] = packer.Evidence{
				ID:    n.Key(),
				Text:  text,
				Score: float64(n.Complexity + 1),
			}
		}
		packed := packer.Pack(items, budget)
		included := make([]any, len(packed.Included))
		for i, ev := range packed.Included {
			included[i] = map[string]any{
				"id":    ev.ID,
				"text":  ev.Text,
				"score": ev.Score,
			}
		}
		resp := map[string]any{
			"included": included,
			"tokens":   packed.Tokens,
			"dropped":  packed.Dropped,
			"budget":   budget,
		}
		if el := elisionFor(len(packed.Included), len(items), "score descending, ties by id ascending", "re-call with a larger budget"); el != nil {
			resp["elided"] = el
		}
		return resp, nil
	})
}

// ─── ops/meta: batch ──────────────────────────────────────────────────────────

// batchCall describes one call inside a batch request.
type batchCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// batchResult is the result of one call in a batch, with a per-call error field.
type batchResult struct {
	Name   string `json:"name"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func registerBatch(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "batch",
		Description: "Execute N tool calls in a single request. Per-call errors are reported in the result rather than aborting the entire batch.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"calls": map[string]any{
					"type":        "array",
					"description": "List of {name, arguments} tool calls to execute",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":      map[string]any{"type": "string"},
							"arguments": map[string]any{"type": "object"},
						},
					},
				},
			},
			"required": []string{"calls"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Calls []batchCall `json:"calls"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "batch: invalid args"}
		}
		if len(p.Calls) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "batch: calls required"}
		}
		results := make([]any, len(p.Calls))
		sc := scopeFrom(ctx)
		// Sub-calls are logged individually (via:"batch") so surface telemetry
		// counts the tools a batch actually exercised, not just "batch".
		logSub := func(name, group string, argsRaw json.RawMessage, offSurface bool, claimsAttached int, verdictOutcome map[string]int, subResult any, latency time.Duration, rpcErr *rpcError) {
			if s.callLogger == nil {
				return
			}
			var sa struct {
				Service string `json:"service"`
			}
			_ = json.Unmarshal(argsRaw, &sa)
			var subErr error
			if rpcErr != nil {
				subErr = &rpcErrWrapper{rpcErr}
			}
			subCount, subVia := outcomeFrom(subResult)
			s.callLogger.log(name, sa.Service, group, s.effectiveSurface(ctx), "batch", originFrom(ctx), offSurface, claimsAttached, verdictOutcome, callOutcome{ResultCount: subCount, MatchedVia: subVia}, latency, subErr)
		}
		for i, call := range p.Calls {
			argsRaw := call.Arguments
			if argsRaw == nil {
				argsRaw = json.RawMessage("{}")
			}
			te, ok := s.findTool(call.Name)
			if !ok {
				results[i] = batchResult{Name: call.Name, Error: "tool not found: " + call.Name}
				continue
			}
			// Hosted-modules security gate (GKE-MH-7, critic finding 2):
			// batch is itself GroupShared, not GroupModules, so it is
			// excluded from hostedModulesTools' allowlist and cannot even
			// be CALLED on the modules surface while hosted (strictFor
			// makes that a hard gate) — but a batch invoked from bare
			// /rpc, or any other named surface, must still be refused
			// from smuggling a GroupModules sub-call through, exactly
			// like handleToolsCallWithCtx's own check (dispatch.go).
			if s.hostedModuleBlocked(sc, te.desc) {
				gateErr := &rpcError{Code: ErrCodeNotFound, Message: "tool " + call.Name + " requires the " + SurfaceModules + " surface (hosted mode is active)"}
				logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, nil, 0, gateErr)
				results[i] = batchResult{Name: call.Name, Error: gateErr.Message}
				continue
			}
			subOff := false
			if sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) {
				if s.strictFor(sc.surface) {
					gateErr := &rpcError{Code: ErrCodeNotFound, Message: "tool " + call.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}
					logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, nil, 0, gateErr)
					results[i] = batchResult{Name: call.Name, Error: gateErr.Message}
					continue
				}
				subOff = true
			}
			// P1-8: batch sub-calls get the same dispatch-level required-field
			// gate handleToolsCallWithCtx applies (validateRequired,
			// dispatch.go) — without this, batch{calls:[...]} was a bypass
			// route straight to the handler, keeping the exact
			// previously-lenient-handler hole (license_policy et al.) open.
			// A failing sub-call errors into ITS OWN batchResult (and is
			// logged like any other sub-call error) — never the whole batch.
			if valErr := validateRequired(te.desc, argsRaw); valErr != nil {
				logSub(call.Name, string(te.desc.group), argsRaw, subOff, 0, nil, nil, 0, valErr)
				results[i] = batchResult{Name: call.Name, Error: valErr.Message}
				continue
			}
			start := time.Now()
			result, rpcErr := te.handler(ctx, argsRaw)
			logSub(call.Name, string(te.desc.group), argsRaw, subOff, claimsAttachedFrom(result), verdictOutcomeFrom(result), result, time.Since(start), rpcErr)
			if rpcErr != nil {
				results[i] = batchResult{Name: call.Name, Error: rpcErr.Message}
				continue
			}
			results[i] = batchResult{Name: call.Name, Result: result}
		}
		return map[string]any{"results": results, "count": len(results)}, nil
	})
}

// ─── proto: proto_conformance ─────────────────────────────────────────────────

func registerProtoConformance(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "proto_conformance",
		Description: "Check .proto files for naming convention violations (PascalCase services/messages/RPCs, snake_case fields).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		svc, rpcErr := serviceArg(args, "proto_conformance")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allIssues []any
		for _, f := range files {
			if !strings.HasSuffix(f.Path, ".proto") {
				continue
			}
			c := proto.Parse(string(f.Content))
			issues := proto.Conformance(c)
			for _, iss := range issues {
				allIssues = append(allIssues, map[string]any{
					"file":     f.Path,
					"location": iss.Location,
					"rule":     iss.Rule,
					"message":  iss.Message,
				})
			}
		}
		return map[string]any{"issues": allIssues, "count": len(allIssues)}, nil
	})
}

// ─── spec / context: spec_refs ────────────────────────────────────────────────

func registerSpecRefs(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name: "spec_refs",
		Description: "Scan service source files for spec-ID references and return a grouped map of spec_id → [{file, line}]. " +
			"Covers bracketed IDs ([US-042], [EP-013]), milestone IDs (M18.2), and doc references (doc 31). " +
			"Results are deterministically sorted (spec_id asc, file asc, line asc).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"pattern": map[string]any{"type": "string", "description": "Optional regex override (default covers [US/EP/FR/NFR-N], MN.N, doc N)"},
				"spec":    map[string]any{"type": "string", "description": "Optional: filter output to a single normalised spec ID (e.g. \"M18.2\")"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Pattern string `json:"pattern"`
			Spec    string `json:"spec"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_refs: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_refs: service required"}
		}
		files, rpcErr := fetchSources(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		// Build path→content map for the pure ExtractFileRefs function.
		fileMap := make(map[string]string, len(files))
		for _, f := range files {
			fileMap[f.Path] = string(f.Content)
		}
		refs, scanned, err := sdd.ExtractFileRefs(fileMap, p.Pattern, p.Spec)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_refs: invalid pattern: " + err.Error()}
		}
		// Group refs by spec_id.
		specsMap := make(map[string][]any)
		for _, r := range refs {
			specsMap[r.SpecID] = append(specsMap[r.SpecID], map[string]any{
				"file": r.File,
				"line": r.Line,
			})
		}
		return map[string]any{
			"specs":         specsMap,
			"total_refs":    len(refs),
			"files_scanned": scanned,
		}, nil
	})
}

// ─── spec / context: brief ────────────────────────────────────────────────────

// defaultBriefBudget is the default char budget for the brief tool (≈4000 tokens × 4 chars/tok).
const defaultBriefBudget = 16000

func registerBrief(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name: "brief",
		Description: "Pre-pack worker context in one call: resolve named symbols, run concern-scoped query, and return a paste-ready markdown brief. " +
			"Respects a char budget; truncated output says so explicitly.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"symbols": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional symbol names to resolve via find_symbol"},
				"concern": map[string]any{"type": "string", "description": "Optional concern keyword for query_symbols hit list"},
				"budget":  map[string]any{"type": "integer", "description": "Character budget (default 16000 ≈ 4000 tokens)"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string   `json:"service"`
			Symbols []string `json:"symbols"`
			Concern string   `json:"concern"`
			Budget  int      `json:"budget"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "brief: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "brief: service required"}
		}
		budget := p.Budget
		if budget <= 0 {
			budget = defaultBriefBudget
		}
		charBudget := budget * 4

		// Fetch staleness summary for the header.
		stalenessNote := "unknown"
		if _, ic, found, _ := backend.ServiceMeta(ctx, p.Service); found {
			if ic != "" {
				stalenessNote = "indexed at commit " + ic[:min8(len(ic), 8)]
			} else {
				stalenessNote = "indexed (no git)"
			}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "## Pre-packed context (fathom brief · service %s · %s)\n", p.Service, stalenessNote)

		// Distinct file paths resolved along the way — the join key D1
		// (docs/plans/expert-depth.md) uses to attach governed claims below;
		// collected here rather than re-resolving symbols/concern a second
		// time.
		var paths []string
		pathSeen := map[string]bool{}
		addPath := func(path string) {
			if path == "" || pathSeen[path] {
				return
			}
			pathSeen[path] = true
			paths = append(paths, path)
		}

		// Symbols section.
		if len(p.Symbols) > 0 {
			fmt.Fprintf(&sb, "\n### Symbols\n")
			for _, sym := range p.Symbols {
				nodes, err := backend.FindSymbol(ctx, p.Service, sym)
				if err != nil || len(nodes) == 0 {
					fmt.Fprintf(&sb, "- `%s` — NOT FOUND — verify before use\n", sym)
					continue
				}
				n := nodes[0]
				sig := n.Signature
				if sig == "" {
					sig = string(n.Kind)
				}
				fmt.Fprintf(&sb, "- `%s` — `%s` — %s:%d\n", n.Name, sig, n.FilePath, n.Line)
				addPath(n.FilePath)
			}
			if sb.Len() > charBudget {
				return truncatedBriefResult(&sb, charBudget), nil
			}
		}

		// Concern section.
		if p.Concern != "" {
			fmt.Fprintf(&sb, "\n### Concern: %s\n", p.Concern)
			result, err := backend.QuerySymbols(ctx, core.Query{
				ServiceID: p.Service,
				ConcernID: p.Concern,
				TopN:      20,
			})
			if err == nil {
				for _, n := range result.Nodes {
					if sb.Len() > charBudget {
						break
					}
					sig := n.Signature
					if sig == "" {
						sig = string(n.Kind)
					}
					fmt.Fprintf(&sb, "- `%s` — %s — %s:%d — %s\n", n.Name, string(n.Kind), n.FilePath, n.Line, sig)
					addPath(n.FilePath)
				}
			}
			if sb.Len() > charBudget {
				return truncatedBriefResult(&sb, charBudget), nil
			}
		}

		// Governing claims section (D1, docs/plans/expert-depth.md): after
		// the deterministic packing above, attach any served semantic claims
		// (obligations layer and beyond) scoped to the paths this brief just
		// resolved — verbatim rule text, citation, and tier, with the
		// serving state labeled whenever it isn't fresh. Reuses
		// obligationsBackend.Obligations (the SAME served-claim selection
		// and scope-glob matching the obligations tool uses) rather than
		// adding a new store surface or matching logic.
		claimsAttached := 0
		if cb, ok := backend.(obligationsBackend); ok && len(paths) > 0 {
			type claimKey struct{ rule, src string }
			seen := map[claimKey]bool{}
			var rules []semantic.Obligation
			for _, path := range paths {
				obl, err := cb.Obligations(ctx, p.Service, path, 0)
				if err != nil {
					continue
				}
				for _, r := range obl.Rules {
					k := claimKey{r.Rule, r.Src}
					if seen[k] {
						continue
					}
					seen[k] = true
					rules = append(rules, r)
				}
			}
			if len(rules) > 0 {
				sort.Slice(rules, func(i, j int) bool {
					if rules[i].Rule != rules[j].Rule {
						return rules[i].Rule < rules[j].Rule
					}
					return rules[i].Src < rules[j].Src
				})
				fmt.Fprintf(&sb, "\n### Governing claims\n")
				for _, r := range rules {
					state := ""
					if r.Status != "" {
						state = " [" + r.Status + "]"
					}
					fmt.Fprintf(&sb, "- %s — src:%s tier:%s%s\n", r.Rule, r.Src, r.Tier, state)
				}
				claimsAttached = len(rules)
				if sb.Len() > charBudget {
					// The truncated shape must still carry the telemetry
					// field: claims WERE attached (and partially served) —
					// dropping the count here would log a claims-serving
					// truncated brief as claims_attached:0 and silently
					// poison D1's 30-day hit-rate kill clock.
					result := truncatedBriefResult(&sb, charBudget)
					result["claims_attached"] = claimsAttached
					return result, nil
				}
			}
		}

		result := map[string]any{"brief": sb.String()}
		if claimsAttached > 0 {
			// D1-2: surfaced to the call log too (claimsAttachedFrom,
			// calllog.go) so claims-block hit rate is visible per call.
			result["claims_attached"] = claimsAttached
		}
		return result, nil
	})
}

// truncatedBriefResult trims the brief to charBudget chars and appends a
// clear truncation notice, then returns the result payload.
func truncatedBriefResult(sb *strings.Builder, charBudget int) map[string]any {
	text := sb.String()
	remaining := len(text) - charBudget
	if remaining < 0 {
		remaining = 0
	}
	trimmed := text[:charBudget]
	// Find a safe line boundary.
	if i := strings.LastIndex(trimmed, "\n"); i > 0 {
		trimmed = trimmed[:i]
	}
	notice := fmt.Sprintf("\n\n…truncated at budget (%d more chars)\n", remaining)
	return map[string]any{"brief": trimmed + notice}
}

// min8 returns the minimum of a and b (avoids importing math for one call).
func min8(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── investigation: history ───────────────────────────────────────────────────

func registerHistory(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "history",
		Description: "Return git commit history for a service, filtered by file path suffix, spec ID, or symbol name.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"file":    map[string]any{"type": "string", "description": "Optional file path suffix filter"},
				"spec":    map[string]any{"type": "string", "description": "Optional spec ID filter (e.g. M18.2)"},
				"symbol":  map[string]any{"type": "string", "description": "Optional symbol name; resolved to its file path for filtering"},
				"limit":   map[string]any{"type": "integer", "description": "Max commits to return (default 20)"},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			File    string `json:"file"`
			Spec    string `json:"spec"`
			Symbol  string `json:"symbol"`
			Limit   int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "history: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "history: service required"}
		}
		limit := p.Limit
		if limit <= 0 {
			limit = 20
		}

		// Resolve symbol → file path filter.
		fileFilter := p.File
		if p.Symbol != "" && fileFilter == "" {
			nodes, err := backend.FindSymbol(ctx, p.Service, p.Symbol)
			if err == nil && len(nodes) > 0 {
				fileFilter = nodes[0].FilePath
			}
		}

		commits, err := backend.ListCommits(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}

		// Filter commits.
		var filtered []CommitInfo
		for _, c := range commits {
			if fileFilter != "" {
				matched := false
				for _, f := range c.Files {
					if strings.HasSuffix(f, fileFilter) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			if p.Spec != "" {
				matched := false
				for _, id := range c.SpecIDs {
					if id == p.Spec {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, c)
			if len(filtered) >= limit {
				break
			}
		}

		// Build output rows.
		rows := make([]any, len(filtered))
		for i, c := range filtered {
			specIDs := c.SpecIDs
			if specIDs == nil {
				specIDs = []string{}
			}
			rows[i] = map[string]any{
				"hash":                c.Hash,
				"date":                c.Date,
				"author":              c.Author,
				"message":             c.Message,
				"files_touched_count": c.FilesTouchedCount,
				"spec_ids":            stringsToAny(specIDs),
			}
		}
		return map[string]any{"commits": rows, "count": len(rows)}, nil
	})
}

// ─── graph traversal: traverse ────────────────────────────────────────────────

func registerTraverse(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name: "traverse",
		Description: "BFS traversal over a heterogeneous edge graph built from symbols (calls), source files (spec-refs, routes, env-keys). " +
			"Returns all paths reachable from start within depth hops, optionally filtered by edge kind.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":   map[string]any{"type": "string", "description": "Service / repo identifier"},
				"start":     map[string]any{"type": "string", "description": "Starting node (symbol name, file path, route, spec ID, or env key)"},
				"kinds":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Edge kinds to traverse: calls, imports, spec-ref, route, env-key (default: all)"},
				"depth":     map[string]any{"type": "integer", "description": "Max BFS hops (default 3, max effective ~10)"},
				"direction": map[string]any{"type": "string", "description": "Traversal direction: out (default), in, or both", "enum": []string{"out", "in", "both"}},
			},
			"required": []string{"service", "start"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service   string   `json:"service"`
			Start     string   `json:"start"`
			Kinds     []string `json:"kinds"`
			Depth     int      `json:"depth"`
			Direction string   `json:"direction"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "traverse: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "traverse: service required"}
		}
		if p.Start == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "traverse: start required"}
		}
		depth := p.Depth
		if depth <= 0 {
			depth = 3
		}
		direction := p.Direction
		if direction == "" {
			direction = "out"
		}

		// Local lang helper (routesLang is unexported in source_tools.go).
		traverseRoutesLang := func(lang string) string {
			switch lang {
			case "go":
				return "go"
			case "js", "ts":
				return "js"
			case "py":
				return "python"
			}
			return ""
		}

		// Build edges from call graph (symbols → Calls).
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "traverse: list symbols: " + err.Error()}
		}
		var edges []graph.Edge
		for _, n := range nodes {
			for _, callee := range n.Calls {
				edges = append(edges, graph.Edge{
					From:    n.Name,
					To:      callee,
					Kind:    graph.KindCalls,
					Service: n.Service,
					File:    n.FilePath,
					Line:    n.Line,
				})
			}
		}

		// Build edges from source files: spec-refs, routes, env-keys.
		srcs, _ := backend.Sources(ctx, p.Service)
		for _, f := range srcs {
			// Spec-ref edges: file → spec_id.
			specRefs := sdd.ExtractRefs(string(f.Content))
			for _, ref := range specRefs {
				edges = append(edges, graph.Edge{
					From:    f.Path,
					To:      ref,
					Kind:    graph.KindSpecRef,
					Service: p.Service,
					File:    f.Path,
				})
			}
			// Route edges: route_path → handler.
			if rlang := traverseRoutesLang(f.Lang); rlang != "" {
				for _, r := range routes.Extract(rlang, f.Path, f.Content) {
					from := r.Method + " " + r.Path
					if from == " " {
						from = r.Path
					}
					to := r.Handler
					if to == "" {
						to = f.Path
					}
					edges = append(edges, graph.Edge{
						From:    from,
						To:      to,
						Kind:    graph.KindRoute,
						Service: p.Service,
						File:    r.File,
						Line:    r.Line,
					})
				}
			}
			// Env-key edges: file → env key name.
			for _, ek := range envkeys.Scan(f.Path, string(f.Content)) {
				edges = append(edges, graph.Edge{
					From:    f.Path,
					To:      ek.Name,
					Kind:    graph.KindEnvKey,
					Service: p.Service,
					File:    ek.File,
					Line:    ek.Line,
				})
			}
		}

		// Resolve start: if no exact match as a From/To node, try suffix match on symbol names.
		startNode := p.Start
		nodeExists := false
		for _, e := range edges {
			if e.From == startNode || e.To == startNode {
				nodeExists = true
				break
			}
		}
		if !nodeExists {
			// Try suffix match on symbol names.
			for _, n := range nodes {
				if strings.HasSuffix(n.Name, p.Start) || strings.HasSuffix(n.FilePath+"/"+n.Name, p.Start) {
					startNode = n.Name
					break
				}
			}
		}

		result := graph.Traverse(edges, startNode, p.Kinds, depth, direction)

		// Serialize paths.
		pathsOut := make([]any, len(result.Paths))
		for i, path := range result.Paths {
			edgesOut := make([]any, len(path.Edges))
			for j, e := range path.Edges {
				edgesOut[j] = map[string]any{
					"from":    e.From,
					"to":      e.To,
					"kind":    e.Kind,
					"service": e.Service,
					"file":    e.File,
					"line":    e.Line,
				}
			}
			pathsOut[i] = map[string]any{"edges": edgesOut}
		}

		// Serialize edge_counts.
		edgeCountsOut := make(map[string]any, len(result.EdgeCounts))
		for k, v := range result.EdgeCounts {
			edgeCountsOut[k] = v
		}

		return map[string]any{
			"paths":       pathsOut,
			"truncated":   result.Truncated,
			"edge_counts": edgeCountsOut,
		}, nil
	})
}
