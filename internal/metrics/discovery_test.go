package metrics

import "testing"

func TestIsDiscoveryIndexTool_KnownTool_True(t *testing.T) {
	for _, tool := range []string{"find_symbol", "references", "symbol_context", "outline", "repo_map", "pack_context", "read_source", "query_symbols", "find_callers"} {
		if !IsDiscoveryIndexTool(tool) {
			t.Errorf("IsDiscoveryIndexTool(%q) = false, want true", tool)
		}
	}
}

// TestIsDiscoveryIndexTool_PlumbingTool_False pins the design §4 exclusion
// list directly.
func TestIsDiscoveryIndexTool_PlumbingTool_False(t *testing.T) {
	for _, tool := range []string{"status", "ingest", "list_services", "obligations", "staleness_report", "verify_plan"} {
		if IsDiscoveryIndexTool(tool) {
			t.Errorf("IsDiscoveryIndexTool(%q) = true, want false (plumbing)", tool)
		}
		if !IsPlumbingTool(tool) {
			t.Errorf("IsPlumbingTool(%q) = false, want true", tool)
		}
	}
}

func TestIsDiscoveryIndexTool_UnknownTool_False(t *testing.T) {
	if IsDiscoveryIndexTool("some_unrelated_tool") {
		t.Error("IsDiscoveryIndexTool(unknown) = true, want false")
	}
	if IsPlumbingTool("some_unrelated_tool") {
		t.Error("IsPlumbingTool(unknown) = true, want false")
	}
}
