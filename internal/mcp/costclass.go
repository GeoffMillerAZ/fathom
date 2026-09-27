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

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// toolCostClasses is §3.1's table by today's cost.
var toolCostClasses = map[string]tenancy.Class{
	// point: one row, one capped file, or the process's own counters.
	"capabilities": tenancy.ClassPoint, "server_stats": tenancy.ClassPoint, "staleness_report": tenancy.ClassPoint,
	"read_source": tenancy.ClassPoint, "memory_search": tenancy.ClassPoint, "history": tenancy.ClassPoint,
	"list_modules": tenancy.ClassPoint,
	// batch holds no ticket of its own: each sub-call is admitted in its
	// own class (admitBatchCall).
	"batch": tenancy.ClassPoint,

	// index: one service's symbols, edges or graph from the store.
	"find_symbol": tenancy.ClassIndex, "find_callers": tenancy.ClassIndex, "find_callees": tenancy.ClassIndex,
	"query_symbols": tenancy.ClassIndex, "pack_context": tenancy.ClassIndex, "repo_map": tenancy.ClassIndex,
	"verify_plan": tenancy.ClassIndex, "test_impact": tenancy.ClassIndex, "rename_impact": tenancy.ClassIndex,
	"reachable": tenancy.ClassIndex, "dead_code": tenancy.ClassIndex, "community": tenancy.ClassIndex,
	"localize": tenancy.ClassIndex, "outline": tenancy.ClassIndex, "entrypoints": tenancy.ClassIndex,
	"complexity": tenancy.ClassIndex, "god_object": tenancy.ClassIndex, "api_diff": tenancy.ClassIndex,
	"arch_conformance": tenancy.ClassIndex, "diff_impact": tenancy.ClassIndex, "evidence_summary": tenancy.ClassIndex,
	"contradiction_scan": tenancy.ClassIndex, "findings_near": tenancy.ClassIndex,
	"module_obligations": tenancy.ClassIndex, "module_query": tenancy.ClassIndex, "module_staleness": tenancy.ClassIndex,

	// tree: a whole-tree read or a whole-service graph build.
	"symbol_context": tenancy.ClassTree, "obligations": tenancy.ClassTree, "search": tenancy.ClassTree,
	"secret_scan": tenancy.ClassTree, "cloc": tenancy.ClassTree, "spec_coverage": tenancy.ClassTree,
	"traceability": tenancy.ClassTree, "safe_delete_check": tenancy.ClassTree, "list_sources": tenancy.ClassTree,
	"discoveries": tenancy.ClassTree, "references": tenancy.ClassTree, "blast_radius": tenancy.ClassTree,
	"brief": tenancy.ClassTree, "traverse": tenancy.ClassTree, "http_client_calls": tenancy.ClassTree,
	"shell_out": tenancy.ClassTree, "proto_conformance": tenancy.ClassTree, "spec_refs": tenancy.ClassTree,
	"stale_docs": tenancy.ClassTree,

	// fleet: every service.
	"list_services": tenancy.ClassFleet, "fleet_drift": tenancy.ClassFleet, "cross_repo_blast_radius": tenancy.ClassFleet,
	"cross_repo_callers": tenancy.ClassFleet, "review_diff": tenancy.ClassFleet, "dep_drift": tenancy.ClassFleet,

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
