package mcp_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestIngestTool_TypedEdgesNameSkippedPackages (EW-83, O1506): the MCP
// ingest tool's typed_edges block carries the run's skips, by reason with
// their first names, and the directories and modules no load covered; a
// status replayed from the store carries none of them (they are never
// persisted) and never reads 0.
func TestIngestTool_TypedEdgesNameSkippedPackages(t *testing.T) {
	call := func(t *testing.T, st fusion.TypedEdgeStatus, id int) map[string]any {
		t.Helper()
		srv := mcp.NewServer(&typedIngestBackend{status: st, known: true})
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: id, Method: "tools/call",
			Params: map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc"}},
		})
		p := extToolPayload(t, resp)
		te, ok := p["typed_edges"].(map[string]any)
		if !ok {
			t.Fatalf("no typed_edges block: %v", p)
		}
		return te
	}

	te := call(t, fusion.TypedEdgeStatus{
		PackagesSeen: 7, Packages: 4, EdgesTotal: 2, PackagesSkipped: 3,
		Skips: []fusion.TypedSkip{
			{Reason: "build constraints", Count: 1, Names: []string{"example.com/x/tagged"}},
			{Reason: "test main", Count: 2, Names: []string{"example.com/x/a.test", "example.com/x/b.test"}},
		},
		NotLoaded: []fusion.TypedSkip{
			{Reason: "module path not the repository's", Count: 1, Names: []string{"other/go.mod"}},
			{Reason: "outside ./...", Count: 1, Names: []string{"_wip"}},
		},
	}, 1360)
	if got, _ := te["packages_skipped"].(float64); int(got) != 3 {
		t.Errorf("packages_skipped = %v, want 3", te["packages_skipped"])
	}
	wantSkipped := []any{
		map[string]any{"reason": "build constraints", "count": float64(1), "packages": []any{"example.com/x/tagged"}},
		map[string]any{"reason": "test main", "count": float64(2), "packages": []any{"example.com/x/a.test", "example.com/x/b.test"}},
	}
	if !reflect.DeepEqual(te["skipped"], wantSkipped) {
		t.Errorf("skipped = %#v\nwant %#v", te["skipped"], wantSkipped)
	}
	wantNotLoaded := []any{
		map[string]any{"reason": "module path not the repository's", "count": float64(1), "paths": []any{"other/go.mod"}},
		map[string]any{"reason": "outside ./...", "count": float64(1), "paths": []any{"_wip"}},
	}
	if !reflect.DeepEqual(te["not_loaded"], wantNotLoaded) {
		t.Errorf("not_loaded = %#v\nwant %#v", te["not_loaded"], wantNotLoaded)
	}

	// Nothing skipped: the count reads 0 and the lists are omitted.
	clean := call(t, fusion.TypedEdgeStatus{PackagesSeen: 2, Packages: 2}, 1361)
	if got, ok := clean["packages_skipped"].(float64); !ok || got != 0 {
		t.Errorf("a run that skipped nothing reads packages_skipped 0, got %v", clean["packages_skipped"])
	}
	for _, k := range []string{"skipped", "not_loaded"} {
		if _, ok := clean[k]; ok {
			t.Errorf("%s present for a run that skipped nothing: %v", k, clean[k])
		}
	}

	// Replayed from the store: the skip fields are not persisted, so they
	// are omitted rather than read as 0.
	replayed := call(t, fusion.TypedEdgeStatus{PackagesSeen: 7, Packages: 4, Persisted: true, RecordedAt: time.Unix(0, 0)}, 1362)
	for _, k := range []string{"packages_skipped", "skipped", "not_loaded"} {
		if _, ok := replayed[k]; ok {
			t.Errorf("a persisted status carries %s = %v; it must omit it", k, replayed[k])
		}
	}
}
