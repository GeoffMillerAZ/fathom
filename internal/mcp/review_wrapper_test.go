package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestReviewDiff_FindingsPersistedThroughAutoRefreshWrapper pins EW-80
// (O1072) through the wrapper every daemon serves with: review_diff's
// findings bridge reaches the claim writes (PutClaims, SetClaimStatus,
// SourceFingerprint, SetClaimValidTo, WriteSupersedes) through
// autoRefreshBackend, so a finding is persisted with its claim id instead
// of findings_persisted:false on every default daemon.
func TestReviewDiff_FindingsPersistedThroughAutoRefreshWrapper(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := "rules: [\n\t{from_pkg: \"api\", to_pkg: \"store\", allow: false},\n]\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: corpusRoot},
		sources:             map[string][]byte{"api/handler.go": reviewHandlerSource},
	}
	srv := mcp.NewServer(mcp.NewAutoRefreshForTest(t, fake))
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
		},
	})
	p := extToolPayload(t, resp)
	if p["findings_persisted"] != true {
		t.Fatalf("findings_persisted = %v through the auto-refresh wrapper, want true: the claim writes were stripped", p["findings_persisted"])
	}
	if len(fake.claims) == 0 {
		t.Fatal("no claim reached the wrapped backend's PutClaims")
	}
}
