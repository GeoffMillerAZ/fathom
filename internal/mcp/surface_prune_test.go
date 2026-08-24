// surface_prune_test.go — SP-B1/SP-B2 (docs/plans/surface-prune.md): pins
// the 34-tool deletion (59 frozen zero-call names minus 25 exemptions) and
// keeps the registry ↔ docs/mcp-tools.md three-place contract from
// silently drifting.
package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// prunedZeroCallTools is the frozen evidence list's 59 names minus the 25
// that passed the two-part exemption check (docs/plans/surface-prune.md
// "## Exemptions") — the 34 actually deleted from the registry.
var prunedZeroCallTools = []string{
	"antipatterns", "api_surface", "call_hierarchy", "call_path", "clones",
	"cognitive", "cost_report", "dependencies", "deprecation", "doc_coverage",
	"endpoint_inventory", "env_keys", "err_audit", "estimate_cost",
	"field_usage", "get_go_symbols", "hotspots", "import_cycles", "license",
	"license_policy", "markers", "migration_targets", "naming",
	"package_deps", "propagate_concern", "proto_contract",
	"query_cross_service", "routes", "schema_export", "security_rollup",
	"spec_status", "sql_map", "struct_tags", "type_hierarchy",
}

// TestRegistryCarriesNoZeroCallTools asserts none of the 34 pruned names
// remain registered anywhere in the full tool registry (bare /rpc — the
// stdio surface, which always sees everything).
func TestRegistryCarriesNoZeroCallTools(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	})
	registered := map[string]bool{}
	for _, n := range toolNames(t, resp) {
		registered[n] = true
	}
	for _, name := range prunedZeroCallTools {
		if registered[name] {
			t.Errorf("pruned tool %q is still registered", name)
		}
	}
}

// toolTableSpan is the substring of docs/mcp-tools.md holding the tool
// reference tables (Shared/Coding/Investigation/Platform/Module host) —
// bounded so prompt and resource table rows (a disjoint namespace from
// tools/list) never get counted as tool names.
func toolTableSpan(t *testing.T, doc string) string {
	t.Helper()
	start := strings.Index(doc, "## Shared tools")
	end := strings.Index(doc, "## SymbolNode shape")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("docs/mcp-tools.md: could not locate the tool-table span (Shared tools .. SymbolNode shape)")
	}
	return doc[start:end]
}

// docToolRow matches a markdown table row whose first cell is a
// backtick-quoted bare identifier: `| \`tool_name\` | ... |`.
var docToolRow = regexp.MustCompile("(?m)^\\|\\s*`([a-zA-Z_][a-zA-Z0-9_]*)`\\s*\\|")

// repoRoot walks up from this test file to the module root (the directory
// containing go.mod), so the test works regardless of the package's
// working directory under `go test`.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod) above internal/mcp")
		}
		dir = parent
	}
}

// TestMcpToolsDocMatchesRegistry is the SP-B2 acceptance test: the set of
// tool names documented in docs/mcp-tools.md's reference tables must equal
// exactly the set of tools the full registry (bare /rpc) actually serves —
// the three-place membership contract (surface registration, this test,
// the doc) can no longer silently drift apart. A name on only one side
// fails loudly, naming which side and which name.
func TestMcpToolsDocMatchesRegistry(t *testing.T) {
	docPath := filepath.Join(repoRoot(t), "docs", "mcp-tools.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	span := toolTableSpan(t, string(raw))
	docNames := map[string]bool{}
	for _, m := range docToolRow.FindAllStringSubmatch(span, -1) {
		docNames[m[1]] = true
	}
	if len(docNames) == 0 {
		t.Fatalf("parsed zero tool names out of docs/mcp-tools.md — parser or doc structure broke")
	}

	// newModuleTestServer registers a genuine (non-nil) module backend so
	// GroupModules tools (list_modules, module_obligations, module_query,
	// module_staleness — documented in the "Module host tools" table) show
	// up here too; NewServer(nil) never registers them at all (server.go:
	// registerModuleTools only runs when a --module store was mounted), so
	// comparing against that would false-flag every module tool as a stale
	// doc row.
	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	})
	registryNames := map[string]bool{}
	for _, n := range toolNames(t, resp) {
		registryNames[n] = true
	}

	var undocumented, stale []string
	for n := range registryNames {
		if !docNames[n] {
			undocumented = append(undocumented, n)
		}
	}
	for n := range docNames {
		if !registryNames[n] {
			stale = append(stale, n)
		}
	}
	sort.Strings(undocumented)
	sort.Strings(stale)
	if len(undocumented) > 0 {
		t.Errorf("registered but undocumented in docs/mcp-tools.md: %v", undocumented)
	}
	if len(stale) > 0 {
		t.Errorf("documented in docs/mcp-tools.md but not registered (stale row): %v", stale)
	}
}
