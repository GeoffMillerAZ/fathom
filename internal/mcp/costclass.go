package mcp

// costclass.go — every tool's cost class (design §3.1; RT-55, RT-152).
// The admission gate admits a tools/call in its class before the handler
// runs, so every registered tool declares one: registerTool panics without
// it. A class reflects the tool's CURRENT cost (the audit of 4200482a,
// committed as testdata/toolcost.json): a tool moves to a cheaper class
// only in the commit that rewrites it, which rewrites its cost row and
// carries its cost assertion (TestCostClassMatchesMeasuredCost).
// docs/mcp-tools.md's "Cost classes" table mirrors this one
// (TestEveryToolDeclaresCostClass).

import (
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// toolCostClasses is §3.1's table by today's cost.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var toolCostClasses = map[string]tenancy.Class{
	// point: one row, one capped file, or the process's own counters.
	// history (SR-12) was unregistered: 0 lifetime calls.
	"read_source":  tenancy.ClassPoint,
	"list_modules": tenancy.ClassPoint,
	// batch holds no ticket of its own: each sub-call is admitted in its
	// own class (admitBatchCall).
	"batch": tenancy.ClassPoint,

	// index: one service's symbols, edges or graph from the store.
	"find_symbol": tenancy.ClassIndex, "find_callers": tenancy.ClassIndex,
	"query_symbols": tenancy.ClassIndex, "pack_context": tenancy.ClassIndex, "repo_map": tenancy.ClassIndex,
	"verify_plan": tenancy.ClassIndex, "test_impact": tenancy.ClassIndex,
	// reachable (SR-12) was unregistered: 0 lifetime calls; verify's
	// reachable_from concept never called this tool.
	"dead_code": tenancy.ClassIndex, "community": tenancy.ClassIndex,
	"localize": tenancy.ClassIndex, "outline": tenancy.ClassIndex,
	"complexity": tenancy.ClassIndex, "god_object": tenancy.ClassIndex,
	"evidence_summary":   tenancy.ClassIndex,
	"contradiction_scan": tenancy.ClassIndex, "findings_near": tenancy.ClassIndex,
	"module_obligations": tenancy.ClassIndex, "module_query": tenancy.ClassIndex, "module_staleness": tenancy.ClassIndex,

	// tree: a whole-tree read or a whole-service graph build.
	// staleness_report (SR-28): moved here from "point" — its doc_drift
	// section (the retired stale_docs tool's own scan, folded in) now runs
	// a ScanSources pass on every call.
	// status (SR-23): declared at its heaviest section's cost — the
	// "sources" section walks a whole service's symbols exactly like the
	// retired list_sources did; "tools"/"daemon" are cheaper (point) but
	// the tool declares one class for all three.
	// blast_radius (SR-24): stays "tree" — its kind:"change" default
	// already was tree; kind:"rename"/"delete" (the retired
	// rename_impact/safe_delete_check tools' own answers, declared index/
	// tree before the merge) are no cheaper than that default, and one
	// tool declares one class for all three kinds.
	// obligations (SR-27): stays "tree" — its additive spec_refs section
	// (obligationsSpecRefs) uses search's own bounded, streaming grep
	// (searchRun/resolveCorpusRoot), never a second aggregated whole-tree
	// Sources fetch, so folding the retired spec_refs tool's function in
	// added no new whole-tree contender and no class bump.
	"symbol_context": tenancy.ClassTree, "obligations": tenancy.ClassTree, "search": tenancy.ClassTree,
	"secret_scan": tenancy.ClassTree, "cloc": tenancy.ClassTree,
	"traceability": tenancy.ClassTree, "status": tenancy.ClassTree,
	"references": tenancy.ClassTree, "blast_radius": tenancy.ClassTree,
	"shell_out": tenancy.ClassTree, "proto_conformance": tenancy.ClassTree,
	"staleness_report": tenancy.ClassTree,
	// stale_docs (SR-28) was unregistered; its own scan runs inside
	// staleness_report now (the entry above), never as a second call.
	// discoveries (SR-29) was unregistered; the SubagentStart hook's own
	// push-form render (a separate, unbudgeted CLI-side query) covers it.
	// arch_conformance, spec_coverage (SR-27) were unregistered; review_diff's
	// engine already ran both checks in-process (runArch, runSpecCoverage,
	// internal/analysis/review/engine.go).
	// spec_refs (SR-27) was unregistered; its one real consumer (the edit
	// hook's spec-line RPC) now reads obligations' additive spec_refs
	// section instead of a second call.
	// traverse, http_client_calls (SR-12) were unregistered: 0 lifetime
	// calls each.

	// fleet: every service.
	"list_services": tenancy.ClassFleet,
	// fleet_drift (SR-12) was unregistered: 0 lifetime calls.
	"cross_repo_callers": tenancy.ClassFleet, "review_diff": tenancy.ClassFleet,

	// ingest: writes the store; one machine-wide.
	"ingest": tenancy.ClassIngest, "compile_layer": tenancy.ClassIngest,
	"remove_service": tenancy.ClassIngest, "rename_service": tenancy.ClassIngest,
}

// costDecl is a descriptor's explicit class (tests' synthetic tools); the
// zero value defers to toolCostClasses.
type costDecl struct {
	class tenancy.Class
	set   bool
}

// declareCost declares a descriptor's class explicitly.
func declareCost(c tenancy.Class) costDecl { return costDecl{class: c, set: true} }

// resolveCostClass is a descriptor's declared class; ok is false when it
// declares none.
func resolveCostClass(desc toolDescriptor) (tenancy.Class, bool) {
	if desc.cost.set {
		return desc.cost.class, true
	}
	c, ok := toolCostClasses[desc.Name]
	return c, ok
}

// costClassOf is a call's class: the tool's declared class, raised by its
// arguments where §3.1 says so (`references` over services ["*"], or over
// an explicit list naming more than one service, is fleet: the same
// per-service work, so the same slot).
func costClassOf(te toolEntry, args json.RawMessage) tenancy.Class {
	if te.desc.Name == "references" && len(args) > 0 {
		var a struct {
			Services []string `json:"services"`
		}
		if json.Unmarshal(args, &a) == nil {
			distinct := map[string]bool{}
			for _, s := range a.Services {
				if s == "*" {
					return tenancy.ClassFleet
				}
				distinct[s] = true
			}
			if len(distinct) > 1 {
				return tenancy.ClassFleet
			}
		}
	}
	return te.class
}

// ToolCostClass is a registered tool's class for args (cmd/fathom's
// in-flight registration and console wrappers); ok is false for an
// unknown tool.
func (s *Server) ToolCostClass(name string, args json.RawMessage) (tenancy.Class, bool) {
	te, ok := s.findTool(name)
	if !ok {
		return tenancy.ClassIndex, false
	}
	return costClassOf(te, args), true
}

// DeclaredCostClass is a tool's declared class for args without a server
// (cmd/fathom's governor and console); an unknown tool is index.
func DeclaredCostClass(name string, args json.RawMessage) tenancy.Class {
	c, ok := toolCostClasses[name]
	if !ok {
		c = tenancy.ClassIndex
	}
	return costClassOf(toolEntry{desc: toolDescriptor{Name: name}, class: c}, args)
}

// RegisteredTools lists every registered tool's name, sorted — the cost
// harness (internal/mcp/costtest, RT-93) holds a row for each.
func (s *Server) RegisteredTools() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.tools))
	for _, e := range s.tools {
		out = append(out, e.desc.Name)
	}
	sort.Strings(out)
	return out
}
