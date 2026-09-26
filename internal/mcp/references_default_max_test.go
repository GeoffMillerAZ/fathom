// Package mcp_test — BP-41 (docs/plans/bench-pyramid.md): references'
// default cap is 25 occurrences, and the cut names how to get more.
// TestReferencesOccurrenceUnder100Bytes pins the per-row wire size and
// TestReferences_RefusesNonSymbolQuery the three accepted query forms;
// this pins the default the row names, which no test counted before.
package mcp_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestReferencesDefaultMaxIs25NamesHowToGetMore(t *testing.T) {
	var content strings.Builder
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&content, "x := FindMe(%d)\n", i)
	}
	be := &referencesFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		sourcesByService: map[string][]core.SourceFile{
			"svc": {{Path: "pkg/many.go", Content: []byte(content.String()), Lang: "go"}},
		},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2030, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"query": "FindMe"},
		},
	})
	p := xtraPayload(t, resp)

	occs, _ := p["occurrences"].([]any)
	if len(occs) != 25 {
		t.Fatalf("occurrences = %d with no max given, want the default 25", len(occs))
	}
	elided, ok := p["elided"].(map[string]any)
	if !ok {
		t.Fatalf("elided block missing although 55 of 80 occurrences were cut: %v", p)
	}
	if of, _ := elided["of"].(float64); of != 80 {
		t.Errorf("elided.of = %v, want 80 (every occurrence found)", elided["of"])
	}
	if n, _ := elided["count"].(float64); n != 55 {
		t.Errorf("elided.count = %v, want 55", elided["count"])
	}
	how, _ := elided["how_to_get_more"].(string)
	if !strings.Contains(how, "max") {
		t.Errorf("how_to_get_more = %q, want it to name the max argument", how)
	}
}
