package mcp

import "testing"

func TestDoctorTypedProjectionOutputContract(t *testing.T) {
	schema := outputSchemaFor("staleness_report")
	metadata := map[string]any{"service": "fixture", "section": "typed_edges", "typed_edges": map[string]any{"known": false}}
	if why := validateAgainstSchema("$", schema, metadata); why != "" {
		t.Fatalf("metadata does not match advertised contract: %s", why)
	}
	for _, bad := range []map[string]any{
		{"service": "fixture"},
		{"service": "fixture", "section": "other"},
		{"section": "typed_edges"},
	} {
		if why := validateAgainstSchema("$", schema, bad); why == "" {
			t.Fatalf("incomplete or wrong variant accepted: %v", bad)
		}
	}
}
