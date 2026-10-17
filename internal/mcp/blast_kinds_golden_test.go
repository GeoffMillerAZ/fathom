package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// blast_kinds_golden_test.go pins blast_radius kind:"change" and
// kind:"rename" to their bytes on the shared resolution fixture: the
// delete kind's type users (O1127) and test_impact's ranking (O1128) must
// not move either. Regenerate with FATHOM_UPDATE_GOLDEN=1 only for a change
// meant to alter those answers.
func TestBlastRadius_ChangeAndRenameKeepTheirBytes(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	got := map[string]any{
		"change": callTool(t, srv, "blast_radius", map[string]any{
			"service": "svc", "changed_files": []any{"pkg/c.go"}, "include_heuristic": true,
		}),
		"change_resolved": callTool(t, srv, "blast_radius", map[string]any{
			"service": "svc", "changed_files": []any{"pkg/c.go"},
		}),
		"rename": callTool(t, srv, "blast_radius", map[string]any{"kind": "rename", "service": "svc", "symbol": "B"}),
	}
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "blast_kinds.golden.json")
	if os.Getenv("FATHOM_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (FATHOM_UPDATE_GOLDEN=1 writes it): %v", err)
	}
	if string(want) != string(b)+"\n" {
		t.Errorf("blast_radius change/rename bytes moved:\n got %s\nwant %s", b, want)
	}
}
