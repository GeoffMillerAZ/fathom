// reckoning_prune_test.go — SR-35 (surface reckoning, design §2): the
// registry pin surface_prune_test.go set a precedent for. reckoningTools
// is an APPEND-ONLY list of tool names the surface reckoning plan
// (docs/plans/surface-reckoning.md) has actually removed from the
// registry as of the landed commit — one name added per removal row's
// own commit, mirroring how docs/upgrades/0.9.0-surface-reckoning.md's
// migration table and this plan's `Verified:` lines grow. It is
// never grown speculatively: a name lands here only in the commit that
// unregisters it, since listing a still-registered tool would make this
// test provably false, which law "done means proven" forbids.
package mcp_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// reckoningTools is the reckoning's own removed-tool list, grown one
// name per landed removal row's commit (never speculatively):
//   - cross_repo_blast_radius, dep_drift: SR-10 (F-17/F-18)
//   - find_callees: SR-21 (F-05)
//   - diff_impact: SR-25 (F-16)
//   - api_diff: SR-26 (F-20)
//   - brief (the MCP tool, not the CLI verb): SR-15 (F-53 refined)
//   - stale_docs: SR-28 (F-56) — folded into staleness_report's doc_drift
//   - discoveries: SR-29 (F-47) — the SubagentStart hook's push-form
//     render already covers the need
//   - arch_conformance, spec_coverage: SR-27 (F-35) — review_diff's engine
//     already ran both in-process (runArch, runSpecCoverage)
//   - spec_refs: SR-27 (F-35) — the edit hook's one real consumer reads
//     obligations' additive spec_refs section instead
//   - http_client_calls, fleet_drift, reachable, traverse, history: SR-12
//     (F-22) — 0 lifetime calls each; verify's `reachable_from` concept
//     never called the `reachable` tool
//   - complexity, god_object, community, dead_code, cloc, secret_scan,
//     shell_out, proto_conformance, traceability, evidence_summary,
//     contradiction_scan, batch: SR-19 (F-77) — the twelve legacy
//     analyzer tools; the complexity metric stays as a stored column
//     (internal/store/conformance.go) for U13
//   - memory_search: SR-13 (F-45) — anchored memory is removed outright
//     (0 memories in any repo; continuity threads carry the hand-off)
//   - list_modules, module_obligations, module_query, module_staleness:
//     SR-17 (F-68) — the module MCP tools, the /rpc/modules surface and
//     hosted-modules mode go; the module system (manifest, modules.cue,
//     internal/modspec, internal/modulehost, `fathom module`) stays
var reckoningTools = []string{
	"api_diff",
	"arch_conformance",
	"batch",
	"brief",
	"cloc",
	"community",
	"complexity",
	"contradiction_scan",
	"cross_repo_blast_radius",
	"dead_code",
	"dep_drift",
	"diff_impact",
	"discoveries",
	"evidence_summary",
	"find_callees",
	"fleet_drift",
	"god_object",
	"history",
	"http_client_calls",
	"list_modules",
	"memory_search",
	"module_obligations",
	"module_query",
	"module_staleness",
	"proto_conformance",
	"reachable",
	"secret_scan",
	"shell_out",
	"spec_coverage",
	"spec_refs",
	"stale_docs",
	"traceability",
	"traverse",
}

// TestRegistryCarriesNoReckonedTools is the SR-35 acceptance test: none of
// the tools the surface reckoning has actually removed so far remain
// registered anywhere in the full tool registry (bare /rpc, which the
// stdio surface also resolves to — always the complete set).
func TestRegistryCarriesNoReckonedTools(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	})
	registered := map[string]bool{}
	for _, n := range toolNames(t, resp) {
		registered[n] = true
	}
	for _, name := range reckoningTools {
		if registered[name] {
			t.Errorf("reckoning-removed tool %q is still registered", name)
		}
	}
}

// TestModuleToolsGoneEvenWithAModuleHost is SR-17's own pin: the four module
// tools were registered only when a module backend was wired in (serve-http
// --module), so NewServer(nil) above never saw them. A server built WITH a
// module backend — the one shape that used to register them, still used by
// obligations' module bindings — carries none of them, and "modules" is no
// longer a valid /rpc/{surface}.
func TestModuleToolsGoneEvenWithAModuleHost(t *testing.T) {
	srv, _ := newModuleTestServer()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	})
	for _, n := range toolNames(t, resp) {
		switch n {
		case "list_modules", "module_obligations", "module_query", "module_staleness":
			t.Errorf("module tool %q is still registered on a server with a module backend", n)
		}
	}
	if mcp.ValidSurface("modules") {
		t.Error(`"modules" is still a valid surface: /rpc/modules must 404`)
	}
}
