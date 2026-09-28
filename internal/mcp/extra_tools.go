// Package mcp — extra MCP tool registrations wiring the many implemented-but-
// previously-unexposed analyzers.
//
// registerExtraTools is called from registerTools (tools.go) and adds tools in
// the following groups:
//
//	investigation: blast_radius,
//	               traceability, evidence_summary,
//	               contradiction_scan, review_diff, search
//	shared:        ingest
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
	"strings"
	"time"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/analysis/proto"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/concern"
	"github.com/geoffmilleraz/fathom/internal/context/packer"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// registerExtraTools is the single entry-point that wires every new tool.
// It is called from registerTools immediately after all existing groups are
// registered.
func registerExtraTools(s *Server, backend QueryInterface) {
	// ── investigation ────────────────────────────────────────────────────────
	registerBlastRadius(s, backend)
	// registerDiffImpact (F-16) was unregistered by SR-25 (surface
	// reckoning): it was a thin alias for blast_radius, and review_diff
	// (which already runs blast_radius in-process) is the verb for a
	// diff/PR impact question.
	// registerSpecCoverage (F-35) was unregistered by SR-27 (surface
	// reckoning): review_diff's engine already runs the identical check
	// in-process (runSpecCoverage, internal/analysis/review/engine.go)
	// via the same sdd package this tool called directly.
	registerTraceability(s, backend)
	registerEvidenceSummary(s, backend)
	registerContradictionScan(s, backend)
	registerReferences(s, backend)
	registerSearch(s, backend)
	registerVerifyPlan(s, backend)
	registerReviewDiff(s, backend)
	registerFindingsNear(s, backend)

	// ── coding ───────────────────────────────────────────────────────────────
	// registerArchConformance (F-35) was unregistered by SR-27 (surface
	// reckoning): review_diff's engine already runs the identical check
	// in-process (runArch, internal/analysis/review/engine.go) via the
	// same archconformance/projectrules packages this tool called
	// directly — one caller instead of two, no logic lost.
	// registerStaleDocs (F-56) was unregistered by SR-28 (surface reckoning):
	// its own scan (docdrift.Analyze over every Markdown source) now runs
	// in-process as staleness_report's doc_drift section (stalenessDocDrift,
	// internal/mcp/meta.go) — one tool call instead of two.
	// registerAPIDiff (F-20) was unregistered by SR-26 (surface reckoning):
	// the calibrated API delta stays in `conserve`'s surface plane
	// (internal/conserve/surface.go, ADR-044), which reuses the same
	// internal/analysis/apisurface package this tool called directly.
	// registerDepDrift (F-18) was unregistered by SR-10 (surface reckoning):
	// 0 lifetime calls; internal/analysis/deps and internal/depgraph stay
	// for vision 15's system graph (N1's engine seam).

	// ── shared / security ────────────────────────────────────────────────────
	registerIngest(s, backend)
	registerRemoveService(s, backend)
	registerRenameService(s, backend)

	// ── platform ─────────────────────────────────────────────────────────────
	// registerCrossRepoBlastRadius (F-17) was unregistered by SR-10 (surface
	// reckoning): 0 lifetime calls; the serviceRefHops* helpers below stay
	// (blast_radius and review_diff still reach them via withServiceRefHops).

	// ── context ──────────────────────────────────────────────────────────────
	registerRepoMap(s, backend)
	registerPackContext(s, backend)

	// ── ops / meta ───────────────────────────────────────────────────────────
	registerBatch(s)

	// ── proto ────────────────────────────────────────────────────────────────
	registerProtoConformance(s, backend)

	// ── spec / context ───────────────────────────────────────────────────────
	// registerSpecRefs (F-35) was unregistered by SR-27 (surface
	// reckoning): its one real consumer, the edit hook's spec-line RPC
	// (306 calls), now reads the identical spec_id → locations map from
	// the obligations answer's additive spec_refs section
	// (obligationsSpecRefs, internal/mcp/semantic_tools.go) instead of a
	// second call.
	// registerBrief was unregistered by SR-15 (surface reckoning F-53,
	// refined): the MCP brief tool had 0 lifetime calls and overlaps
	// pack_context; the CLI's renderBriefMD/briefTuning (the console's
	// /brief page) are a separate implementation and are unaffected.

	// registerTraverse (F-22) was unregistered by SR-12 (surface
	// reckoning): 0 lifetime calls; internal/graph's Traverse/Edge types
	// had no other importer and stay unused pending a future consumer.

	// registerHistory (F-22) was unregistered by SR-12 (surface
	// reckoning): 0 lifetime calls; backend.ListCommits stays (used by
	// `fathom purge history` / `fathom plans history`, unrelated CLI
	// verbs that only share the word "history").
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

// fleetServiceRefHops joins services' ingest-extracted routes and client
// calls (RT-89) into hops — rows read from the store, never source. A
// backend without the stored rows (serviceref.RefReader), or a read
// error, yields no hops; unextracted names services whose rows are not
// complete (not re-ingested since RT-89). Each service's rows are read
// once per watermark (memoRefReader), not per call.
func fleetServiceRefHops(ctx context.Context, backend QueryInterface, services []string) (hops []serviceref.Hop, unextracted []string) {
	rr, ok := backend.(serviceref.RefReader)
	if !ok {
		return nil, services
	}
	hops, unextracted, err := serviceref.BuildFromStore(ctx, memoRefReader{backend: backend, rr: rr}, services)
	if err != nil {
		return nil, services
	}
	return hops, unextracted
}

// serviceRefHopsInto reports the service_ref hops (VB-30) whose ToService
// is service and whose ToFile is one of changedFiles: another service's
// client call joins a route THIS change touches, so that OTHER service's
// call site is part of the blast radius even though no same-process call
// graph connects them. Joined fleet-wide each call (serviceref.BuildFromStore
// across every known service) from rows read once per watermark
// (memoRefReader, RT-89). Best-effort: a fleet of fewer than two services,
// or any backend error, degrades to no hops rather than failing the whole
// tool.
//
// unextracted (RT-89) names services whose routes and client calls have
// not been extracted by an ingest yet: their hops cannot be in the answer.
func serviceRefHopsInto(ctx context.Context, backend QueryInterface, service string, changedFiles []string) (out []serviceref.Hop, unextracted []string) {
	services, err := backend.Services(ctx)
	if err != nil || len(services) < 2 {
		return nil, nil
	}
	hops, unextracted := fleetServiceRefHops(ctx, backend, services)
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	for _, h := range hops {
		if h.ToService == service && changed[h.ToFile] {
			out = append(out, h)
		}
	}
	return out, unextracted
}

// withServiceRefHops sets m's service_ref_hops and, when any service's
// routes and calls are not extracted yet, service_ref_unextracted (RT-89).
func withServiceRefHops(m map[string]any, hops []serviceref.Hop, unextracted []string) {
	m["service_ref_hops"] = serviceRefHopsPayload(hops)
	if len(unextracted) > 0 {
		m["service_ref_unextracted"] = unextracted
	}
}

// ─── investigation: blast_radius ─────────────────────────────────────────────

func registerBlastRadius(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "blast_radius",
		Description: `Blast radius of changed files (kind:"change"), every rename call site (kind:"rename"), or a symbol's remaining referents (kind:"delete"). Reach for it when a change, rename, or delete needs who breaks. Required: service.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":           map[string]any{"type": "string", "description": "Service / repo identifier"},
				"kind":              map[string]any{"type": "string", "enum": []string{"change", "rename", "delete"}, "description": `"change" (default, changed_files), "rename"/"delete" (need symbol) — grep gives none of the three.`},
				"changed_files":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Repo-relative file paths that changed"},
				"symbol":            map[string]any{"type": "string", "description": "Symbol name (bare or fully-qualified)"},
				"include_heuristic": map[string]any{"type": "boolean", "description": "Also follow syntax-only edges. Default false."},
				"max_depth":         map[string]any{"type": "integer", "description": "Hops from the changed files (default 2; 0 = unbounded)."},
				"max_symbols":       map[string]any{"type": "integer", "description": "Symbols listed, shallowest first (default 500)."},
			},
			"required": []string{"service"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Service          string   `json:"service"`
			Kind             string   `json:"kind"`
			ChangedFiles     []string `json:"changed_files"`
			Symbol           string   `json:"symbol"`
			IncludeHeuristic bool     `json:"include_heuristic"`
			MaxDepth         *int     `json:"max_depth"`
			MaxSymbols       *int     `json:"max_symbols"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: invalid args"}
		}
		return blastRadiusByKind(ctx, s, backend, p.Kind, p.Service, p.ChangedFiles, p.Symbol, p.IncludeHeuristic, p.MaxDepth, p.MaxSymbols)
	})
}

// blastRadiusByKind is blast_radius' kind dispatch (SR-24, F-13, design
// §4): "change" (the default) is the tool's own long-standing answer;
// "rename" and "delete" are the retired rename_impact/safe_delete_check
// tools' exact answers, unregistered as standalone tools and reached here
// instead — promotedVerbs() lost rename_impact accordingly.
func blastRadiusByKind(ctx context.Context, s *Server, backend QueryInterface, kind, service string, changedFiles []string, symbol string, includeHeuristic bool, maxDepth, maxSymbols *int) (any, *rpcError) {
	switch kind {
	case "", "change":
		return blastRadiusChangeAnswer(ctx, s, backend, service, changedFiles, includeHeuristic, maxDepth, maxSymbols)
	case "rename":
		return renameImpactAnswer(ctx, backend, service, symbol)
	case "delete":
		return safeDeleteCheckAnswer(ctx, s, backend, service, symbol)
	default:
		return nil, &rpcError{Code: ErrCodeParams, Message: `blast_radius: kind must be "change", "rename", or "delete"`}
	}
}

// blastRadiusChangeAnswer is blast_radius' original (and default,
// kind:"change") answer, unchanged by the SR-24 merge.
func blastRadiusChangeAnswer(ctx context.Context, s *Server, backend QueryInterface, service string, changedFiles []string, includeHeuristic bool, maxDepth, maxSymbols *int) (any, *rpcError) {
	if backend == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
	}
	if service == "" {
		return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: service required"}
	}
	if len(changedFiles) == 0 {
		return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: changed_files required"}
	}
	navs := s.loadNavGraphs(ctx, backend, service)
	if navs == nil {
		return nil, &rpcError{Code: ErrCodeInternal, Message: "blast_radius: no symbols for service " + service}
	}
	// BP-27: bounded, resolved-by-default. The unbounded both-kinds
	// walk (impact.BlastRadiusWithTypedEdges) is untouched and is still
	// what the verify engine's reachability reads.
	res := blastRadiusResolved(navs, changedFiles, includeHeuristic,
		intOr(maxDepth, defaultBlastDepth), intOr(maxSymbols, defaultBlastSymbols))
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
	}
	hops, unextracted := serviceRefHopsInto(ctx, backend, service, changedFiles)
	withServiceRefHops(result, hops, unextracted)
	if ic := indexCommitFor(ctx, backend, service); ic != "" {
		result["index_commit"] = ic
	}
	return withSummary(blastRadiusSummary(res.Buckets, len(res.Tests)), result), nil
}

// blastRadiusSummary renders blast_radius' answer-first sentence (wave L1):
// "depth 0: N changed symbols; depth 1: M direct callers (top: …); affected
// tests: K" — built from buckets, the exact by_depth list the response
// returns, and testCount, len(res.Tests) itself.
func blastRadiusSummary(buckets []blastBucket, testCount int) string {
	var d0, d1 blastBucket
	haveD1 := false
	for _, b := range buckets {
		switch b.Depth {
		case 0:
			d0 = b
		case 1:
			d1, haveD1 = b, true
		}
	}
	summary := fmt.Sprintf("depth 0: %d changed symbol%s", d0.Count, pluralS(d0.Count))
	if haveD1 {
		summary += fmt.Sprintf("; depth 1: %d direct caller%s", d1.Count, pluralS(d1.Count))
		if len(d1.Symbols) > 0 {
			n := len(d1.Symbols)
			if n > 3 {
				n = 3
			}
			names := make([]string, n)
			for i, s := range d1.Symbols[:n] {
				names[i] = s.Key
			}
			summary += " (top: " + strings.Join(names, ", ") + ")"
		}
	}
	summary += fmt.Sprintf("; affected tests: %d", testCount)
	return capSummary(summary)
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
		files, cov, _ := collectSources(ctx, backend, p.Service)
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
		return withPartial(map[string]any{"refs": refs}, cov), nil
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
		nodes, err := serviceSymbols(ctx, backend, svc)
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
		Description: "Re-ingests a service to refresh the symbol store; pass path to onboard one. Hash-gated, a clean repo is a no-op. Reach for it when the index lags the tree. Required: service.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"path":    map[string]any{"type": "string", "description": "Absolute corpus root — only for onboarding a new service"},
				"remap":   map[string]any{"type": "boolean", "description": "Authorize re-pointing an existing service's root"},
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
						svc, recCanon, canonical,
					)}
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

		// RT-155: a quarantined service is re-ingested only explicitly —
		// a hook's automated refresh (post-commit, edit) is refused with
		// the quarantine named — and the quarantine is lifted below only
		// once that explicit ingest has completed.
		wasQuarantined := tenancy.IsQuarantined(svc)
		if wasQuarantined && strings.HasSuffix(originFrom(ctx), "-hook") {
			for _, q := range tenancy.QuarantinedNow() {
				if q.Service == svc {
					return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: " + q.Attention() + " — a hook's refresh never lifts a quarantine"}
				}
			}
		}

		var ingestErr error
		var queued *refresh.Status
		if p.Path != "" {
			if _, ok := backend.(ingestAtBackend); !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: this backend does not support onboarding by path"}
			}
		}
		if q := s.RefreshQueue(); q != nil {
			// RT-97/RT-98 (design §7.1): the daemon's ingests share one
			// queue — one per service, one heavy slot. This call waits
			// for the run that serves it (the one follow-up, when the
			// service's ingest was already running); the job runs under
			// the queue's context, so a disconnect never cancels it. The
			// queue's job holds tenancy's ingest slot (RT-160).
			st := q.RequestRefresh(ctx, refresh.Request{Service: svc, Path: p.Path, Reason: refresh.ReasonIngestTool}, true)
			if st.State == refresh.StateRefused {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: " + st.Refused}
			}
			ingestErr = st.Err
			queued = &st
		} else {
			// RT-160: without a queue the ingest runs here, under
			// tenancy's one ingest slot, so a derived-table backfill never
			// runs beside it. A caller whose request ends while another
			// ingest or backfill holds the slot leaves, named.
			releaseSlot, slotErr := tenancy.AcquireIngestSlot(ctx)
			if slotErr != nil {
				return nil, &rpcError{Code: ErrCodeBusy, Message: "ingest: " + svc + " not ingested — the wait for the one ingest slot (held by another ingest or a derived-table backfill) ended: " + slotErr.Error()}
			}
			if p.Path != "" {
				ingestErr = backend.(ingestAtBackend).IngestServiceAt(ctx, svc, p.Path)
			} else {
				ingestErr = backend.IngestService(ctx, svc)
			}
			releaseSlot()
		}
		// RT-41 (§2.1): an ingest that failed writing the store with
		// SQLITE_FULL or SQLITE_IOERR is store distress — the next ingest
		// and refresh are refused, named, instead of admitted to fail.
		tenancy.NoteStoreError(ingestErr, time.Now())
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
		if !(errors.As(ingestErr, &readErr) && readErr.WroteNothing()) {
			hookcache.BustIdent()
			evictServiceCaches(backend, svc)
		}
		// RT-155: the explicit ingest ran to completion (a degraded one
		// included: it indexed every readable file within budget) — lift
		// the quarantine, recorded in the daemon's lifecycle ledger. A
		// refused, halted or failed ingest leaves it in place.
		quarantineCleared := false
		if wasQuarantined && (ingestErr == nil || (readErr != nil && !readErr.WroteNothing())) {
			quarantineCleared = tenancy.ClearQuarantine(svc, "ingest tool")
		}
		if ingestErr != nil {
			return nil, ingestFailure(ingestErr)
		}

		result := map[string]any{"service": svc, "ok": true}
		if queued != nil && queued.Coalesced {
			// The ingest this call waited for was shared with other
			// requests for the service (RT-98) — said, never hidden.
			result["coalesced"] = true
		}
		if quarantineCleared {
			result["quarantine_cleared"] = true
		}
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
		// RT-145 / I38: every directory the default cache list pruned is
		// named here too, not only in the CLI summary.
		if eb, ok := backend.(exclusionsBackend); ok {
			if dirs := eb.DefaultIgnoredDirs(svc); len(dirs) > 0 {
				result["default_ignored_dirs"] = dirs
			}
		}
		// Ingest honesty disclosure (wave P3 / O82, .fathom/observations.md):
		// the SAME TypedEdgeStatus staleness_report/doctor/list_services
		// already read, carried on the ingest response itself — a caller
		// must never need a SEPARATE call to learn a Go tree's typed pass
		// produced nothing to trust. This is the exact gap the 2026-09-16
		// incident hit: the daemon answered {"ok":true} for a push clone
		// whose typed-edge enrichment produced ZERO typed edges (go.work
		// named sibling checkouts that existed only beside the operator's
		// own checkout), and nothing about that response said so —
		// test_impact/verify test_reaches silently read "no test reaches
		// it" and a push was refused with no earlier signal anywhere.
		//
		// ok:false-grade disclosure: the syntax index is kept (IngestService
		// already wrote it, never rolled back here), but `ok` flips to
		// false when the typed pass failed — a caller checking only `ok`
		// (the common case) must not read this as a clean ingest.
		if teb, ok := backend.(typedEdgeStatusBackend); ok {
			if st, known := teb.TypedEdgeStatus(svc); known {
				te := map[string]any{
					"packages_seen":      st.PackagesSeen,
					"packages_processed": st.Packages,
					"edges":              st.EdgesTotal,
					"failed":             st.Failed,
				}
				if st.Reason != "" {
					te["error"] = st.Reason
				}
				if st.Fallback != "" {
					te["fallback"] = st.Fallback
				}
				// finding [4]/DG-10: see platform_tools.go's list_services block
				// -- the SAME distinction (a store-replayed, pre-restart answer
				// vs. one this process just computed) must travel through the
				// ingest response too, or a caller reading only THIS surface has
				// no way to tell them apart.
				if st.Persisted {
					te["persisted"] = true
					te["recorded_at"] = st.RecordedAt.UTC().Format(time.RFC3339)
				}
				result["typed_edges"] = te
				if st.Failed {
					result["ok"] = false
				}
			}
		}
		return result, nil
	})
}

// ─── context: repo_map ────────────────────────────────────────────────────────

// repoMapEntrypointCap bounds repo_map's "entrypoints" header (SR-22,
// F-10, design §4): the same symbolnav.Entrypoints detector the retired
// `entrypoints` tool used, capped so a service with hundreds of HTTP
// handlers never turns the header into its own unbounded listing.
const repoMapEntrypointCap = 10

// repoMapEntrypoints returns repo_map's header entrypoints — capped at
// repoMapEntrypointCap — and how many were cut beyond the cap (0 when
// everything fit). Nil input or no detected entrypoints returns (nil, 0)
// so the caller can omit the field entirely rather than serve an empty one.
func repoMapEntrypoints(nodes []core.SymbolNode) ([]symbolNodeDTO, int) {
	eps := symbolnav.Entrypoints(nodes)
	total := len(eps)
	if total > repoMapEntrypointCap {
		eps = eps[:repoMapEntrypointCap]
	}
	return toNodeDTOs(eps), total - len(eps)
}

func registerRepoMap(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "repo_map",
		Description: "Budget-bounded symbol map ranked by outbound call-edge count, for LLM context. Reach for it when you need the shape of an unseen service. Required: service.",
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
		nodes, err := serviceSymbols(ctx, backend, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		mapStr := packer.RepoMap(nodes, budget)
		// packer.RepoMap reports only the packed string, so the included
		// count is recovered from its own one-line-per-symbol output rather
		// than widening that package's signature (out of this row's scope).
		included := strings.Count(mapStr, "\n")
		// SR-41 (F-07, design §5): repo_map is an orientation tool, never
		// the exhaustive answer — "label" says so in every response
		// regardless of budget, so a caller that widened the budget still
		// sees the same steer toward query_symbols/pack_context for
		// anything more specific.
		resp := map[string]any{"map": mapStr, "budget": budget, "label": "orientation-only"}
		// SR-22 (F-10, design §4): the header carries likely entrypoints —
		// the same detector the unregistered `entrypoints` tool used —
		// capped at repoMapEntrypointCap, honest-absence when none exist.
		if eps, more := repoMapEntrypoints(nodes); len(eps) > 0 {
			resp["entrypoints"] = eps
			if more > 0 {
				resp["entrypoints_more"] = more
			}
		}
		if el := elisionFor(included, len(nodes), "outbound call-edge count, most-connected first, ties by package+name", "re-call with a larger budget"); el != nil {
			resp["elided"] = el
			// RC-19: the same one-line disclosure the other four newly-
			// budgeted tools carry, alongside (not instead of) the existing
			// structured "elided" block above — repo_map already had a
			// working disclosure; this just makes its wording uniform.
			if line := elisionLine(len(nodes)-included, "budget"); line != "" {
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
		Description: "Packs concern-ranked evidence into a token budget: top items plus what was dropped. Reach for it when assembling task context. Required: service, concern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern": map[string]any{"type": "string", "description": "Concern ID to scope the query — a short identifier"},
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
		// J3(d) (app-kit O28): a caller that hands pack_context a
		// natural-language sentence instead of a concern ID used to get a
		// silent, indistinguishable-from-real empty pack (budget spent,
		// included:[], dropped:0) — nothing said the input itself was the
		// problem. A concern ID is a short identifier; whitespace is the
		// cheap, always-available tell that free text landed here instead.
		// Refuse loudly rather than run a query that can only ever score
		// zero.
		if strings.ContainsFunc(p.Concern, unicode.IsSpace) {
			return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf(
				"pack_context: %q is not a concern ID — it contains whitespace. Pass a short concern identifier (e.g. the concern argument query_symbols expects), not a sentence", p.Concern,
			)}
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

// batchMaxCalls caps a batch's sub-calls (RT-60, design §3.3).
const batchMaxCalls = 16

// admitBatchCall admits one batch sub-call in its own class under the
// batch's context (and so its deadline); a refusal is that sub-call's
// named error, never the batch's 503 (RT-60).
func (s *Server) admitBatchCall(ctx context.Context, te toolEntry, args json.RawMessage) (context.Context, *tenancy.Ticket, *rpcError) {
	return s.admitToolCall(ctx, te, args, false)
}

// batchResult is the result of one call in a batch, with a per-call error field.
type batchResult struct {
	Name   string `json:"name"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	// Budget is a stopped sub-call's partial block (RT-57): an entry
	// never looks complete when its producer was stopped.
	Budget map[string]any `json:"budget,omitempty"`
}

func registerBatch(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "batch",
		Description: "Execute up to 16 tool calls in a single request. Per-call errors are reported in the result rather than aborting the entire batch.",
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
		// RT-60 (design §3.3): at most batchMaxCalls sub-calls; the batch
		// holds no ticket, each sub-call is admitted in its own class.
		if len(p.Calls) > batchMaxCalls {
			return nil, &rpcError{Code: ErrCodeParams, Message: fmt.Sprintf("batch: at most %d calls per batch (got %d) — split it", batchMaxCalls, len(p.Calls))}
		}
		results := make([]any, len(p.Calls))
		sc := scopeFrom(ctx)
		// Sub-calls are logged individually (via:"batch") so surface telemetry
		// counts the tools a batch actually exercised, not just "batch".
		logSub := func(name, group string, argsRaw json.RawMessage, offSurface bool, claimsAttached int, verdictOutcome map[string]int, subResult any, latency time.Duration, rpcErr *rpcError, meta callMeta) {
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
			s.callLogger.log(name, sa.Service, group, s.effectiveSurface(ctx), "batch", originFrom(ctx), offSurface, claimsAttached, verdictOutcome, s.recordCallOutcome(ctx, meta, callOutcome{ResultCount: subCount, MatchedVia: subVia}), latency, subErr)
		}
		for i, call := range p.Calls {
			// RT-60: the batch stops when its context ends — nothing after
			// the cancellation runs, and each unrun call says so.
			if err := ctx.Err(); err != nil {
				results[i] = batchResult{Name: call.Name, Error: "not run: the batch's context ended (" + err.Error() + ")"}
				continue
			}
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
				logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, nil, 0, gateErr, callMeta{})
				results[i] = batchResult{Name: call.Name, Error: gateErr.Message}
				continue
			}
			subOff := false
			if sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) {
				if s.strictFor(sc.surface) {
					gateErr := &rpcError{Code: ErrCodeNotFound, Message: "tool " + call.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}
					logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, nil, 0, gateErr, callMeta{})
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
				logSub(call.Name, string(te.desc.group), argsRaw, subOff, 0, nil, nil, 0, valErr, callMeta{})
				results[i] = batchResult{Name: call.Name, Error: valErr.Message}
				continue
			}
			start := time.Now()
			subCtx, tk, busy := s.admitBatchCall(ctx, te, argsRaw)
			if busy != nil {
				logSub(call.Name, string(te.desc.group), argsRaw, subOff, 0, nil, nil, 0, busy, callMeta{class: costClassOf(te, argsRaw), outcome: string(tenancy.OutcomeRefused)})
				results[i] = batchResult{Name: call.Name, Error: busy.Message}
				continue
			}
			result, rpcErr := runHandler(subCtx, te, argsRaw)
			result, rpcErr, partial, meta := s.settleCall(subCtx, call.Name, tk, result, rpcErr)
			meta.class = costClassOf(te, argsRaw)
			logSub(call.Name, string(te.desc.group), argsRaw, subOff, claimsAttachedFrom(result), verdictOutcomeFrom(result), result, time.Since(start), rpcErr, meta)
			if rpcErr != nil {
				results[i] = batchResult{Name: call.Name, Error: rpcErr.Message}
				continue
			}
			results[i] = batchResult{Name: call.Name, Result: result, Budget: partial}
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
		files, cov, rpcErr := fetchSources(ctx, backend, svc)
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
		return withPartial(map[string]any{"issues": allIssues, "count": len(allIssues)}, cov), nil
	})
}
