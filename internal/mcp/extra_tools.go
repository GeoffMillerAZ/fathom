// Package mcp — extra MCP tool registrations wiring the many implemented-but-
// previously-unexposed analyzers.
//
// registerExtraTools is called from registerTools (tools.go) and adds tools in
// the following groups:
//
//	investigation: blast_radius, diff_impact, spec_coverage, spec_status,
//	               traceability, propagate_concern, evidence_summary,
//	               contradiction_scan, review_diff
//	coding:        arch_conformance, doc_coverage, stale_docs, naming,
//	               struct_tags, err_audit, sql_map, field_usage, clones,
//	               migration_targets, get_go_symbols, api_diff, dep_drift,
//	               package_deps, hotspots
//	shared:        license, license_policy, security_rollup, ingest
//	platform:      cross_repo_blast_radius
//	context:       repo_map, pack_context
//	ops/meta:      estimate_cost, cost_report, batch
//	proto:         proto_contract, proto_conformance
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/apisurface"
	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/analysis/clone"
	"github.com/geoffmilleraz/fathom/internal/analysis/complexity"
	"github.com/geoffmilleraz/fathom/internal/analysis/deps"
	"github.com/geoffmilleraz/fathom/internal/analysis/doccoverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/docdrift"
	"github.com/geoffmilleraz/fathom/internal/analysis/envkeys"
	"github.com/geoffmilleraz/fathom/internal/analysis/erraudit"
	"github.com/geoffmilleraz/fathom/internal/analysis/fieldusage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/license"
	"github.com/geoffmilleraz/fathom/internal/analysis/migrationtargets"
	"github.com/geoffmilleraz/fathom/internal/analysis/naming"
	"github.com/geoffmilleraz/fathom/internal/analysis/proto"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/analysis/schemaexport"
	"github.com/geoffmilleraz/fathom/internal/analysis/security"
	"github.com/geoffmilleraz/fathom/internal/analysis/sqlmap"
	"github.com/geoffmilleraz/fathom/internal/analysis/structtags"
	"github.com/geoffmilleraz/fathom/internal/concern"
	"github.com/geoffmilleraz/fathom/internal/context/packer"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/graph"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// costTracker is a package-level cost tracker shared across all cost_report
// calls.  It is safe for concurrent use.
var costTracker = cost.NewTracker()

// registerExtraTools is the single entry-point that wires every new tool.
// It is called from registerTools immediately after all existing groups are
// registered.
func registerExtraTools(s *Server, backend QueryInterface) {
	// ── investigation ────────────────────────────────────────────────────────
	registerBlastRadius(s, backend)
	registerDiffImpact(s, backend)
	registerSpecCoverage(s, backend)
	registerSpecStatus(s, backend)
	registerTraceability(s, backend)
	registerPropagateConcern(s, backend)
	registerEvidenceSummary(s, backend)
	registerContradictionScan(s, backend)
	registerReferences(s, backend)
	registerVerifyPlan(s, backend)
	registerReviewDiff(s, backend)
	registerFindingsNear(s, backend)

	// ── coding ───────────────────────────────────────────────────────────────
	registerArchConformance(s, backend)
	registerDocCoverage(s, backend)
	registerStaleDocs(s, backend)
	registerNaming(s, backend)
	registerStructTags(s, backend)
	registerErrAudit(s, backend)
	registerSQLMap(s, backend)
	registerFieldUsage(s, backend)
	registerClones(s, backend)
	registerMigrationTargets(s, backend)
	registerGetGoSymbols(s, backend)
	registerAPIDiff(s, backend)
	registerDepDrift(s, backend)
	registerPackageDeps(s, backend)
	registerHotspots(s, backend)

	// ── shared / security ────────────────────────────────────────────────────
	registerLicense(s, backend)
	registerLicensePolicy(s, backend)
	registerSecurityRollup(s, backend)
	registerIngest(s, backend)

	// ── platform ─────────────────────────────────────────────────────────────
	registerCrossRepoBlastRadius(s, backend)

	// ── context ──────────────────────────────────────────────────────────────
	registerRepoMap(s, backend)
	registerPackContext(s, backend)

	// ── ops / meta ───────────────────────────────────────────────────────────
	registerEstimateCost(s)
	registerCostReport(s)
	registerBatch(s)

	// ── proto ────────────────────────────────────────────────────────────────
	registerProtoContract(s, backend)
	registerProtoConformance(s, backend)

	// ── schema ───────────────────────────────────────────────────────────────
	registerSchemaExport(s, backend)

	// ── spec / context ───────────────────────────────────────────────────────
	registerSpecRefs(s, backend)
	registerBrief(s, backend)

	// ── graph traversal ──────────────────────────────────────────────────────
	registerTraverse(s, backend)

	// ── investigation: history ───────────────────────────────────────────────────
	registerHistory(s, backend)
}

// ─── investigation: blast_radius ─────────────────────────────────────────────

func registerBlastRadius(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "blast_radius",
		Description: "Compute the blast radius of a set of changed files: symbols defined in those files plus every transitive caller, with affected test functions and file paths.",
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
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: service required"}
		}
		if len(p.ChangedFiles) == 0 {
			return nil, &rpcError{Code: ErrCodeParams, Message: "blast_radius: changed_files required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Typed Go call edges (ADR-021 §3): additive on the transitive-caller
		// walk — best-effort, a non-Go service contributes nothing.
		typed, _ := backend.TypedEdges(ctx, p.Service)
		result := impact.BlastRadiusWithTypedEdges(nodes, p.ChangedFiles, typed)
		return map[string]any{
			"symbols": stringsToAny(result.Symbols),
			"tests":   stringsToAny(result.Tests),
			"files":   stringsToAny(result.Files),
		}, nil
	})
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
			"symbols": stringsToAny(result.Symbols),
			"tests":   stringsToAny(result.Tests),
			"files":   stringsToAny(result.Files),
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

// ─── investigation: spec_status ──────────────────────────────────────────────

func registerSpecStatus(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "spec_status",
		Description: "Return the coverage status (\"covered\" or \"uncovered\") for each spec ID in the list.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":  map[string]any{"type": "string", "description": "Service / repo identifier"},
				"spec_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Spec IDs to check"},
			},
			"required": []string{"service", "spec_ids"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string   `json:"service"`
			SpecIDs []string `json:"spec_ids"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_status: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "spec_status: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		codeRefs := make(map[string][]string)
		for _, n := range nodes {
			refs := sdd.ExtractRefs(n.Signature + " " + n.Name)
			if len(refs) > 0 {
				codeRefs[n.Name] = refs
			}
		}
		matrix := sdd.BuildMatrix(p.SpecIDs, codeRefs)
		statuses := make(map[string]any, len(p.SpecIDs))
		for _, id := range p.SpecIDs {
			statuses[id] = sdd.SpecStatus(matrix, id)
		}
		return map[string]any{"statuses": statuses}, nil
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

// ─── investigation: propagate_concern ────────────────────────────────────────

func registerPropagateConcern(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "propagate_concern",
		Description: "Walk the call graph backward from a seed symbol collecting all symbols whose name/package contains the concern token, plus their transitive callers.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"concern": map[string]any{"type": "string", "description": "Concern token (case-insensitive substring match on package.name)"},
				"seed":    map[string]any{"type": "string", "description": "Seed symbol key (package.Name) to start from"},
			},
			"required": []string{"service", "concern", "seed"},
		},
		group: GroupInvestigation,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Concern string `json:"concern"`
			Seed    string `json:"seed"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "propagate_concern: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "propagate_concern: service required"}
		}
		if p.Concern == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "propagate_concern: concern required"}
		}
		if p.Seed == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "propagate_concern: seed required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		related := concern.PropagateConcern(nodes, p.Concern, p.Seed)
		return map[string]any{"symbols": stringsToAny(related)}, nil
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

// ─── coding: doc_coverage ─────────────────────────────────────────────────────

func registerDocCoverage(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "doc_coverage",
		Description: "Report documentation coverage of exported symbols: which are documented, which are not, and the overall percentage.",
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
		svc, rpcErr := serviceArg(args, "doc_coverage")
		if rpcErr != nil {
			return nil, rpcErr
		}
		nodes, err := backend.ListAllSymbols(ctx, svc)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Determine documented: a symbol is documented when its Signature field is
		// non-empty (the extractor stores the doc comment / signature there).
		documented := make(map[string]bool, len(nodes))
		for _, n := range nodes {
			if strings.TrimSpace(n.Signature) != "" {
				documented[n.Name] = true
			}
		}
		report := doccoverage.Coverage(nodes, documented)
		return map[string]any{
			"covered":   stringsToAny(report.Covered),
			"uncovered": stringsToAny(report.Uncovered),
			"pct":       report.Pct,
		}, nil
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

// ─── coding: naming ───────────────────────────────────────────────────────────

func registerNaming(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "naming",
		Description: "Check Go symbol naming conventions (exported PascalCase, no underscores, correct acronym caps, interface -er suffix).",
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
		svc, rpcErr := serviceArg(args, "naming")
		if rpcErr != nil {
			return nil, rpcErr
		}
		nodes, err := backend.ListAllSymbols(ctx, svc)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		issues := naming.Check(nodes)
		result := make([]any, len(issues))
		for i, iss := range issues {
			result[i] = map[string]any{
				"symbol":  iss.Symbol,
				"rule":    iss.Rule,
				"message": iss.Message,
			}
		}
		return map[string]any{"issues": result, "count": len(issues)}, nil
	})
}

// ─── coding: struct_tags ──────────────────────────────────────────────────────

func registerStructTags(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "struct_tags",
		Description: "Audit Go struct tags in source files: missing json tags on exported fields, malformed tag syntax.",
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
		svc, rpcErr := serviceArg(args, "struct_tags")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allIssues []structtags.Issue
		for _, f := range files {
			if f.Lang != "go" {
				continue
			}
			allIssues = append(allIssues, structtags.Scan(f.Path, string(f.Content))...)
		}
		sort.Slice(allIssues, func(i, j int) bool {
			if allIssues[i].File != allIssues[j].File {
				return allIssues[i].File < allIssues[j].File
			}
			return allIssues[i].Line < allIssues[j].Line
		})
		result := make([]any, len(allIssues))
		for i, iss := range allIssues {
			result[i] = map[string]any{
				"file":    iss.File,
				"line":    iss.Line,
				"field":   iss.Field,
				"message": iss.Message,
			}
		}
		return map[string]any{"issues": result, "count": len(result)}, nil
	})
}

// ─── coding: err_audit ────────────────────────────────────────────────────────

func registerErrAudit(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "err_audit",
		Description: "Audit Go error-handling patterns: ignored errors, log-only handlers, naked error returns.",
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
		svc, rpcErr := serviceArg(args, "err_audit")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allFindings []erraudit.Finding
		for _, f := range files {
			if f.Lang != "go" {
				continue
			}
			allFindings = append(allFindings, erraudit.Scan(f.Path, string(f.Content))...)
		}
		sort.Slice(allFindings, func(i, j int) bool {
			if allFindings[i].File != allFindings[j].File {
				return allFindings[i].File < allFindings[j].File
			}
			return allFindings[i].Line < allFindings[j].Line
		})
		result := make([]any, len(allFindings))
		for i, f := range allFindings {
			result[i] = map[string]any{
				"file":    f.File,
				"line":    f.Line,
				"message": f.Message,
			}
		}
		return map[string]any{"findings": result, "count": len(result)}, nil
	})
}

// ─── coding: sql_map ──────────────────────────────────────────────────────────

func registerSQLMap(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "sql_map",
		Description: "Extract SQL table references from source files (FROM/JOIN/INTO/UPDATE patterns).",
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
		svc, rpcErr := serviceArg(args, "sql_map")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allTables []sqlmap.Table
		for _, f := range files {
			allTables = append(allTables, sqlmap.Extract(f.Path, string(f.Content))...)
		}
		sort.Slice(allTables, func(i, j int) bool {
			if allTables[i].File != allTables[j].File {
				return allTables[i].File < allTables[j].File
			}
			if allTables[i].Line != allTables[j].Line {
				return allTables[i].Line < allTables[j].Line
			}
			return allTables[i].Name < allTables[j].Name
		})
		result := make([]any, len(allTables))
		for i, t := range allTables {
			result[i] = map[string]any{
				"name": t.Name,
				"file": t.File,
				"line": t.Line,
			}
		}
		return map[string]any{"tables": result, "count": len(result)}, nil
	})
}

// ─── coding: field_usage ──────────────────────────────────────────────────────

func registerFieldUsage(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "field_usage",
		Description: "Report which fields of a named Go struct are used vs unused within its source file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":     map[string]any{"type": "string", "description": "Service / repo identifier"},
				"struct_name": map[string]any{"type": "string", "description": "Go struct type name"},
			},
			"required": []string{"service", "struct_name"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service    string `json:"service"`
			StructName string `json:"struct_name"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "field_usage: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "field_usage: service required"}
		}
		if p.StructName == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "field_usage: struct_name required"}
		}
		files, rpcErr := fetchSources(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var allUsage []fieldusage.FieldUse
		for _, f := range files {
			if f.Lang != "go" {
				continue
			}
			usages := fieldusage.Scan(f.Path, string(f.Content), p.StructName)
			allUsage = append(allUsage, usages...)
		}
		result := make([]any, len(allUsage))
		for i, u := range allUsage {
			result[i] = map[string]any{
				"field": u.Field,
				"used":  u.Used,
			}
		}
		return map[string]any{"fields": result, "struct": p.StructName}, nil
	})
}

// ─── coding: clones ───────────────────────────────────────────────────────────

func registerClones(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "clones",
		Description: "Detect structurally duplicate code (clone groups) across source files using content-normalised hashing.",
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
		svc, rpcErr := serviceArg(args, "clones")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		fileMap := make(map[string]string, len(files))
		for _, f := range files {
			fileMap[f.Path] = string(f.Content)
		}
		groups := clone.Detect(fileMap)
		result := make([]any, len(groups))
		for i, g := range groups {
			result[i] = map[string]any{
				"hash":  g.Hash,
				"files": stringsToAny(g.Files),
			}
		}
		return map[string]any{"groups": result, "count": len(result)}, nil
	})
}

// ─── coding: migration_targets ────────────────────────────────────────────────

func registerMigrationTargets(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "migration_targets",
		Description: "Identify symbols that depend on deprecated packages (io/ioutil, golang.org/x/net/context, etc.) and should be migrated.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":         map[string]any{"type": "string", "description": "Service / repo identifier"},
				"deprecated_pkgs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Additional deprecated package paths (default list is always included)"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service        string   `json:"service"`
			DeprecatedPkgs []string `json:"deprecated_pkgs"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "migration_targets: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "migration_targets: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		targets := migrationtargets.Find(nodes, p.DeprecatedPkgs)
		result := make([]any, len(targets))
		for i, t := range targets {
			result[i] = map[string]any{
				"symbol": t.Symbol,
				"reason": t.Reason,
			}
		}
		return map[string]any{"targets": result, "count": len(result)}, nil
	})
}

// ─── coding: get_go_symbols ───────────────────────────────────────────────────

func registerGetGoSymbols(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "get_go_symbols",
		Description: "Return all Go-kind symbols (GoFunc, GoMethod, GoType, GoInterface, GoConst, GoVar) for the service, optionally filtered by kind.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"kinds":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional kind filter (e.g. [\"GoFunc\",\"GoMethod\"])"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string   `json:"service"`
			Kinds   []string `json:"kinds"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "get_go_symbols: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "get_go_symbols: service required"}
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		goKinds := map[core.SourceKind]bool{
			core.KindGoFunc:      true,
			core.KindGoMethod:    true,
			core.KindGoType:      true,
			core.KindGoInterface: true,
			core.KindGoConst:     true,
			core.KindGoVar:       true,
		}
		// Build kind filter if specified.
		wantKinds := make(map[core.SourceKind]bool, len(p.Kinds))
		for _, k := range p.Kinds {
			wantKinds[core.SourceKind(k)] = true
		}
		var filtered []core.SymbolNode
		for _, n := range nodes {
			if !goKinds[n.Kind] {
				continue
			}
			if len(wantKinds) > 0 && !wantKinds[n.Kind] {
				continue
			}
			filtered = append(filtered, n)
		}
		return map[string]any{"symbols": toNodeDTOs(filtered), "count": len(filtered)}, nil
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

// ─── coding: package_deps ─────────────────────────────────────────────────────

func registerPackageDeps(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "package_deps",
		Description: "Build a package-level dependency graph from symbol packages and their Calls edges.",
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
		svc, rpcErr := serviceArg(args, "package_deps")
		if rpcErr != nil {
			return nil, rpcErr
		}
		nodes, err := backend.ListAllSymbols(ctx, svc)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// Build package dep graph: fromPkg → set of toPkg.
		pkgDeps := make(map[string]map[string]bool)
		for _, n := range nodes {
			fromPkg := n.Package
			if fromPkg == "" {
				continue
			}
			if _, ok := pkgDeps[fromPkg]; !ok {
				pkgDeps[fromPkg] = make(map[string]bool)
			}
			for _, callee := range n.Calls {
				// callee format: "service::pkg.Name" or "pkg.Name" or "Name"
				toPkg := calleePackage(callee)
				if toPkg != "" && toPkg != fromPkg {
					pkgDeps[fromPkg][toPkg] = true
				}
			}
		}
		// Convert to JSON-serialisable map[string][]string.
		result := make(map[string]any, len(pkgDeps))
		for pkg, deps := range pkgDeps {
			targets := make([]string, 0, len(deps))
			for t := range deps {
				targets = append(targets, t)
			}
			sort.Strings(targets)
			result[pkg] = stringsToAny(targets)
		}
		return map[string]any{"package_deps": result}, nil
	})
}

// calleePackage extracts the package portion of a callee key.
// Formats: "service::pkg.Name", "pkg.Name", "Name".
func calleePackage(callee string) string {
	// Strip service prefix.
	if idx := strings.Index(callee, "::"); idx >= 0 {
		callee = callee[idx+2:]
	}
	// The package is everything before the last dot.
	if idx := strings.LastIndex(callee, "."); idx >= 0 {
		return callee[:idx]
	}
	return ""
}

// ─── coding: hotspots ─────────────────────────────────────────────────────────
//
// NOTE: complexity-based hotspots are derived from SymbolNode.Complexity.
// Git-churn-based hotspots require direct git repository access; that path is
// intentionally left as a TODO comment below.

func registerHotspots(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "hotspots",
		Description: "Return top-N complexity-based hotspots (symbols with highest cyclomatic complexity). Git-churn hotspots require repo git access and are not available here.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"top_n":   map[string]any{"type": "integer", "description": "Number of hotspots to return (default 20)"},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			TopN    int    `json:"top_n"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "hotspots: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "hotspots: service required"}
		}
		topN := p.TopN
		if topN <= 0 {
			topN = 20
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		// TODO: add git-churn hotspots when a git repo accessor is wired to the backend.
		hotspots := complexity.Hotspots(nodes, topN)
		hs := make([]any, len(hotspots))
		for i, h := range hotspots {
			hs[i] = map[string]any{
				"name":       h.Name,
				"file":       h.FilePath,
				"line":       h.Line,
				"complexity": h.Complexity,
			}
		}
		return map[string]any{"hotspots": hs}, nil
	})
}

// ─── shared: license ──────────────────────────────────────────────────────────

func registerLicense(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "license",
		Description: "Detect the SPDX license identifier from the service's LICENSE source file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		svc, rpcErr := serviceArg(args, "license")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		detected := "UNKNOWN"
		for _, f := range files {
			base := f.Path
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if base == "LICENSE" || base == "LICENSE.txt" || base == "LICENSE.md" || base == "LICENCE" {
				detected = license.Detect(f.Content)
				break
			}
		}
		return map[string]any{"spdx": detected}, nil
	})
}

// ─── shared: license_policy ───────────────────────────────────────────────────

func registerLicensePolicy(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "license_policy",
		Description: "Evaluate the detected license against an allowed list and report whether it is permitted.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"allowed": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed SPDX identifiers (e.g. [\"MIT\",\"Apache-2.0\"])"},
			},
			"required": []string{"service", "allowed"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string   `json:"service"`
			Allowed []string `json:"allowed"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "license_policy: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "license_policy: service required"}
		}
		files, rpcErr := fetchSources(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		detected := "UNKNOWN"
		for _, f := range files {
			base := f.Path
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if base == "LICENSE" || base == "LICENSE.txt" || base == "LICENSE.md" || base == "LICENCE" {
				detected = license.Detect(f.Content)
				break
			}
		}
		ok, reason := license.Policy(detected, p.Allowed)
		return map[string]any{
			"spdx":    detected,
			"ok":      ok,
			"reason":  reason,
			"allowed": stringsToAny(p.Allowed),
		}, nil
	})
}

// ─── shared: security_rollup ──────────────────────────────────────────────────

func registerSecurityRollup(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "security_rollup",
		Description: "Aggregate security posture: secret_scan + shell_out + http_client_calls findings plus detected license, returned as one posture report.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service":          map[string]any{"type": "string", "description": "Service / repo identifier"},
				"allowed_licenses": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed SPDX license identifiers"},
			},
			"required": []string{"service"},
		},
		group: GroupShared,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service         string   `json:"service"`
			AllowedLicenses []string `json:"allowed_licenses"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "security_rollup: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "security_rollup: service required"}
		}
		files, rpcErr := fetchSources(ctx, backend, p.Service)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var secrets, shellOuts, httpCalls []security.Finding
		detectedLicense := "UNKNOWN"
		for _, f := range files {
			secrets = append(secrets, security.SecretScan(f.Path, f.Content)...)
			shellOuts = append(shellOuts, security.ShellOut(f.Path, f.Content)...)
			httpCalls = append(httpCalls, security.HTTPClientCalls(f.Path, f.Content)...)
			// Check LICENSE files.
			base := f.Path
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if base == "LICENSE" || base == "LICENSE.txt" || base == "LICENSE.md" {
				detectedLicense = license.Detect(f.Content)
			}
		}
		licenseOK, licenseReason := license.Policy(detectedLicense, p.AllowedLicenses)
		return map[string]any{
			"secrets":         securityFindingsToAny(secrets),
			"shell_outs":      securityFindingsToAny(shellOuts),
			"http_calls":      securityFindingsToAny(httpCalls),
			"license":         detectedLicense,
			"license_ok":      licenseOK,
			"license_reason":  licenseReason,
			"secret_count":    len(secrets),
			"shell_out_count": len(shellOuts),
			"http_call_count": len(httpCalls),
		}, nil
	})
}

// ─── shared: ingest ───────────────────────────────────────────────────────────

func registerIngest(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "ingest",
		Description: "Re-ingest the named service to refresh the symbol store; pass path (absolute repo root) to onboard a NEW service into the index.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"path":    map[string]any{"type": "string", "description": "Absolute corpus root — required only when first onboarding a new service"},
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

		if p.Path != "" {
			ia, ok := backend.(ingestAtBackend)
			if !ok {
				return nil, &rpcError{Code: ErrCodeInvalid, Message: "ingest: this backend does not support onboarding by path"}
			}
			if err := ia.IngestServiceAt(ctx, svc, p.Path); err != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: "ingest: " + err.Error()}
			}
		} else if err := backend.IngestService(ctx, svc); err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "ingest: " + err.Error()}
		}
		// Critic finding F2 (BLOCKING, phase-P fix round): bust the
		// CLI's service-identity cache (cmd/fathom/identcache.go) on
		// every successful ingest via this MCP tool too — a hook
		// re-resolving identity through a daemon this ingest just ran
		// against must never see a cached pre-ingest answer for up to
		// the cache's TTL (the O15 class).
		hookcache.BustIdent()

		result := map[string]any{"service": svc, "ok": true}
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
		return map[string]any{"hits": out, "count": len(out)}, nil
	})
}

// ─── context: repo_map ────────────────────────────────────────────────────────

func registerRepoMap(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "repo_map",
		Description: "Build a budget-bounded symbol map ranked by outbound call-edge count (most-connected symbols first), suitable for LLM context assembly.",
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
			budget = 4000
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		mapStr := packer.RepoMap(nodes, budget)
		return map[string]any{"map": mapStr, "budget": budget}, nil
	})
}

// ─── context: pack_context ────────────────────────────────────────────────────

func registerPackContext(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "pack_context",
		Description: "Pack ranked evidence items (from QuerySymbols) into a token budget, returning the highest-scoring items that fit.",
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
			budget = 4000
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
				ID:    n.Service + "::" + n.Package + "." + n.Name,
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
		return map[string]any{
			"included": included,
			"tokens":   packed.Tokens,
			"dropped":  packed.Dropped,
			"budget":   budget,
		}, nil
	})
}

// ─── ops/meta: estimate_cost ──────────────────────────────────────────────────

func registerEstimateCost(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "estimate_cost",
		Description: "Estimate the token count and USD cost of a text string (at a given per-thousand-token rate).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text":          map[string]any{"type": "string", "description": "Text to estimate"},
				"rate_per_ktok": map[string]any{"type": "number", "description": "USD per thousand tokens (default 0.002)"},
			},
			"required": []string{"text"},
		},
		group: GroupShared,
	}, func(_ context.Context, args json.RawMessage) (any, *rpcError) {
		var p struct {
			Text        string  `json:"text"`
			RatePerKTok float64 `json:"rate_per_ktok"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "estimate_cost: invalid args"}
		}
		if p.Text == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "estimate_cost: text required"}
		}
		rate := p.RatePerKTok
		if rate <= 0 {
			rate = 0.002
		}
		tokens := cost.Estimate(p.Text)
		usd := cost.PriceUSD(tokens, rate)
		costTracker.Record("estimate_cost", tokens, usd)
		return map[string]any{
			"tokens": tokens,
			"usd":    usd,
			"rate":   rate,
		}, nil
	})
}

// ─── ops/meta: cost_report ────────────────────────────────────────────────────

func registerCostReport(s *Server) {
	s.registerTool(toolDescriptor{
		Name:        "cost_report",
		Description: "Return accumulated token counts and USD costs recorded by estimate_cost and other cost-tracking tools.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		group: GroupShared,
	}, func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		report := costTracker.Report()
		totalTok, totalUSD := costTracker.Totals()
		rows := make([]any, len(report))
		for i, r := range report {
			rows[i] = map[string]any{
				"tool":   r.Tool,
				"tokens": r.Tokens,
				"usd":    r.USD,
			}
		}
		return map[string]any{
			"tools":        rows,
			"total_tokens": totalTok,
			"total_usd":    totalUSD,
		}, nil
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
		logSub := func(name, group string, argsRaw json.RawMessage, offSurface bool, claimsAttached int, verdictOutcome map[string]int, latency time.Duration, rpcErr *rpcError) {
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
			s.callLogger.log(name, sa.Service, group, s.effectiveSurface(ctx), "batch", originFrom(ctx), offSurface, claimsAttached, verdictOutcome, latency, subErr)
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
				logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, 0, gateErr)
				results[i] = batchResult{Name: call.Name, Error: gateErr.Message}
				continue
			}
			subOff := false
			if sc.enforced() && !s.toolOnSurface(sc.surface, te.desc) {
				if s.strictFor(sc.surface) {
					gateErr := &rpcError{Code: ErrCodeNotFound, Message: "tool " + call.Name + " is not on the " + sc.surface + " surface; POST /rpc serves the full surface"}
					logSub(call.Name, string(te.desc.group), argsRaw, false, 0, nil, 0, gateErr)
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
				logSub(call.Name, string(te.desc.group), argsRaw, subOff, 0, nil, 0, valErr)
				results[i] = batchResult{Name: call.Name, Error: valErr.Message}
				continue
			}
			start := time.Now()
			result, rpcErr := te.handler(ctx, argsRaw)
			logSub(call.Name, string(te.desc.group), argsRaw, subOff, claimsAttachedFrom(result), verdictOutcomeFrom(result), time.Since(start), rpcErr)
			if rpcErr != nil {
				results[i] = batchResult{Name: call.Name, Error: rpcErr.Message}
				continue
			}
			results[i] = batchResult{Name: call.Name, Result: result}
		}
		return map[string]any{"results": results, "count": len(results)}, nil
	})
}

// ─── proto: proto_contract ────────────────────────────────────────────────────

func registerProtoContract(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "proto_contract",
		Description: "Parse .proto source files and return the extracted services, RPCs, messages, and fields.",
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
		svc, rpcErr := serviceArg(args, "proto_contract")
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, rpcErr := fetchSources(ctx, backend, svc)
		if rpcErr != nil {
			return nil, rpcErr
		}
		var contracts []any
		for _, f := range files {
			if !strings.HasSuffix(f.Path, ".proto") {
				continue
			}
			c := proto.Parse(string(f.Content))
			svcs := make([]any, len(c.Services))
			for i, ps := range c.Services {
				rpcs := make([]any, len(ps.RPCs))
				for j, rpc := range ps.RPCs {
					rpcs[j] = map[string]any{
						"name":   rpc.Name,
						"input":  rpc.Input,
						"output": rpc.Output,
					}
				}
				svcs[i] = map[string]any{"name": ps.Name, "rpcs": rpcs}
			}
			msgs := make([]any, len(c.Messages))
			for i, m := range c.Messages {
				fields := make([]any, len(m.Fields))
				for j, fld := range m.Fields {
					fields[j] = map[string]any{
						"name":   fld.Name,
						"type":   fld.Type,
						"number": fld.Number,
					}
				}
				msgs[i] = map[string]any{"name": m.Name, "fields": fields}
			}
			contracts = append(contracts, map[string]any{
				"file":     f.Path,
				"package":  c.Package,
				"services": svcs,
				"messages": msgs,
			})
		}
		return map[string]any{"contracts": contracts, "count": len(contracts)}, nil
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

// ─── schema_export ────────────────────────────────────────────────────────────

func registerSchemaExport(s *Server, backend QueryInterface) {
	s.registerTool(toolDescriptor{
		Name:        "schema_export",
		Description: "Export the symbol graph for a service as JSON or Graphviz DOT format.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string", "description": "Service / repo identifier"},
				"format":  map[string]any{"type": "string", "description": "Output format: \"json\" (default) or \"dot\"", "enum": []string{"json", "dot"}},
			},
			"required": []string{"service"},
		},
		group: GroupCoding,
	}, func(ctx context.Context, args json.RawMessage) (any, *rpcError) {
		if backend == nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: "no query backend configured"}
		}
		var p struct {
			Service string `json:"service"`
			Format  string `json:"format"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, &rpcError{Code: ErrCodeParams, Message: "schema_export: invalid args"}
		}
		if p.Service == "" {
			return nil, &rpcError{Code: ErrCodeParams, Message: "schema_export: service required"}
		}
		format := p.Format
		if format == "" {
			format = "json"
		}
		nodes, err := backend.ListAllSymbols(ctx, p.Service)
		if err != nil {
			return nil, &rpcError{Code: ErrCodeInternal, Message: err.Error()}
		}
		switch format {
		case "dot":
			// Build edges from Calls.
			var edges []core.Edge
			for _, n := range nodes {
				for _, callee := range n.Calls {
					edges = append(edges, core.Edge{
						FromService: n.Service,
						FromFile:    n.FilePath,
						FromSymbol:  n.Package + "." + n.Name,
						ToSymbol:    callee,
					})
				}
			}
			dot := schemaexport.ExportDOT(edges)
			return map[string]any{"format": "dot", "output": dot}, nil
		default:
			jsonStr, exportErr := schemaexport.ExportJSON(nodes)
			if exportErr != nil {
				return nil, &rpcError{Code: ErrCodeInternal, Message: exportErr.Error()}
			}
			return map[string]any{"format": "json", "output": jsonStr}, nil
		}
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
