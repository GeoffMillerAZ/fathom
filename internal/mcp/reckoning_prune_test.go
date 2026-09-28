// reckoning_prune_test.go — SR-35 (surface reckoning, design §2): the
// registry pin surface_prune_test.go set a precedent for. reckoningTools
// is an APPEND-ONLY list of tool names the surface reckoning plan
// (docs/plans/surface-reckoning.md) has actually removed from the
// registry as of the landed commit — one name added per removal row's
// own commit, mirroring how docs/upgrades/0.9.0-surface-reckoning.md's
// migration table and this plan's `Verified:` lines grow. It is
// deliberately NOT the plan's full eventual removal set: SR-13, SR-17 and
// SR-19 are still `[ ]` in the plan, and their tools are still
// registered — listing them here before their own row lands would make
// this test provably false, which law "done means proven" forbids.
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
var reckoningTools = []string{
	"api_diff",
	"arch_conformance",
	"brief",
	"cross_repo_blast_radius",
	"dep_drift",
	"diff_impact",
	"discoveries",
	"find_callees",
	"fleet_drift",
	"history",
	"http_client_calls",
	"reachable",
	"spec_coverage",
	"spec_refs",
	"stale_docs",
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
