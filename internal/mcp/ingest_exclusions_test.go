package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// exclusionsFakeBackend reports the directories its last discovery of a
// service pruned by the default cache list (RT-145).
type exclusionsFakeBackend struct {
	extendedFakeBackend
	pruned []string
}

func (e *exclusionsFakeBackend) IngestService(context.Context, string) error { return nil }
func (e *exclusionsFakeBackend) DefaultIgnoredDirs(string) []string          { return e.pruned }

// RT-145 review / I38: the MCP ingest tool — the documented way an agent
// refreshes — names every directory the default cache list pruned, as the
// CLI summary and --json do.
func TestIngestTool_NamesDefaultIgnoredDirs(t *testing.T) {
	be := &exclusionsFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		pruned:              []string{".cache", "compute/target"},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc"}},
	})
	p := extToolPayload(t, resp)
	got, ok := p["default_ignored_dirs"].([]any)
	if !ok || len(got) != 2 || got[0] != ".cache" || got[1] != "compute/target" {
		t.Fatalf("default_ignored_dirs = %#v, want [.cache compute/target]", p["default_ignored_dirs"])
	}
}
