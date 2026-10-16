// Package mcp_test — VB-30 wiring: blast_radius and review_diff each
// report service_ref hops (kind "service_ref", tier "heuristic") using the
// fauxtown checkout-service/catalog-service fixture pair
// (internal/analysis/serviceref's own golden fixture). cross_repo_blast_radius
// carried this same wiring until SR-10 (surface reckoning F-17) removed it
// for 0 lifetime calls.
package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// serviceRefFakeBackend embeds extendedFakeBackend (satisfies the rest of
// mcp.QueryInterface) and overrides Sources with per-service, Lang-tagged
// source content — serviceref.BuildFromStore's routesLang/
// ExtractClientCalls both gate on core.SourceFile.Lang, which
// extendedFakeBackend's own stub Sources() never sets.
type serviceRefFakeBackend struct {
	extendedFakeBackend
	bySvc map[string][]core.SourceFile
}

func (b *serviceRefFakeBackend) Sources(_ context.Context, svc string) ([]core.SourceFile, error) {
	return b.bySvc[svc], nil
}

// ServiceRefsFor answers with the rows an ingest of bySvc would have
// written (RT-89): the tools join stored rows, never source.
func (b *serviceRefFakeBackend) ServiceRefsFor(_ context.Context, svc string) (store.ServiceRefs, bool, error) {
	var out store.ServiceRefs
	for _, f := range b.bySvc[svc] {
		r := serviceref.ExtractFile(f.Path, f.Content)
		out.Routes = append(out.Routes, r.Routes...)
		out.Calls = append(out.Calls, r.Calls...)
	}
	return out, true, nil
}

// fauxtownRoot resolves the fixture fleet root the same way
// internal/analysis/serviceref's own tests do.
func fauxtownRootForMCP(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "connector", "testdata", "fauxtown")
}

func readServiceRefFixture(t *testing.T, root, svc, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, svc, name))
	if err != nil {
		t.Fatalf("read fixture %s/%s: %v", svc, name, err)
	}
	return data
}

// newServiceRefServer builds a two-service fleet (catalog-service,
// checkout-service) from the real fauxtown fixture files, wired for
// blast_radius/review_diff to walk.
func newServiceRefServer(t *testing.T) (*mcp.Server, string, string) {
	t.Helper()
	root := fauxtownRootForMCP(t)
	catalogSrc := readServiceRefFixture(t, root, "catalog-service", "catalog.go")
	checkoutSrc := readServiceRefFixture(t, root, "checkout-service", "checkout.go")

	backend := &serviceRefFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes: []core.SymbolNode{
				{Service: "catalog-service", Package: "catalog", Name: "getItemHandler", Kind: core.KindGoFunc, FilePath: "catalog-service/catalog.go", Line: 34},
			},
			services: []string{"catalog-service", "checkout-service"},
		},
		bySvc: map[string][]core.SourceFile{
			// unrelated.go is indexed and registers no route: the file
			// TestBlastRadius_ServiceRefHops_EmptyWhenNoRouteChanged changes.
			"catalog-service":  {{Path: "catalog-service/catalog.go", Content: catalogSrc, Lang: "go"}, {Path: "catalog-service/unrelated.go", Content: []byte("package catalog\n"), Lang: "go"}},
			"checkout-service": {{Path: "checkout-service/checkout.go", Content: checkoutSrc, Lang: "go"}},
		},
	}
	return mcp.NewServer(backend), "catalog-service/catalog.go", "checkout-service/checkout.go"
}

func firstServiceRefHop(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	raw, ok := payload["service_ref_hops"].([]any)
	if !ok {
		t.Fatalf("service_ref_hops missing or wrong type: %#v", payload["service_ref_hops"])
	}
	if len(raw) != 1 {
		t.Fatalf("service_ref_hops: got %d entries, want 1: %+v", len(raw), raw)
	}
	hop, ok := raw[0].(map[string]any)
	if !ok {
		t.Fatalf("service_ref_hops[0] is not an object: %#v", raw[0])
	}
	return hop
}

func assertServiceRefHopShape(t *testing.T, hop map[string]any) {
	t.Helper()
	if hop["kind"] != "service_ref" {
		t.Errorf("hop[kind] = %v, want service_ref", hop["kind"])
	}
	if hop["tier"] != "heuristic" {
		t.Errorf("hop[tier] = %v, want heuristic", hop["tier"])
	}
	if hop["method"] != "GET" {
		t.Errorf("hop[method] = %v, want GET", hop["method"])
	}
	if hop["template"] != "/items/{}" {
		t.Errorf("hop[template] = %v, want /items/{}", hop["template"])
	}
	if hop["from_service"] != "checkout-service" {
		t.Errorf("hop[from_service] = %v, want checkout-service", hop["from_service"])
	}
	if hop["to_service"] != "catalog-service" {
		t.Errorf("hop[to_service] = %v, want catalog-service", hop["to_service"])
	}
	// Exactly one service (catalog-service) registers GET /items/:id in
	// this fixture, so the hop must NOT be ambiguous.
	if hop["ambiguous"] != false {
		t.Errorf("hop[ambiguous] = %v, want false (only catalog-service registers this route)", hop["ambiguous"])
	}
	if want := []any{"catalog-service"}; !reflect.DeepEqual(hop["candidates"], want) {
		t.Errorf("hop[candidates] = %v, want %v", hop["candidates"], want)
	}
}

func TestBlastRadius_ServiceRefHops_CrossesProcessBoundary(t *testing.T) {
	srv, catalogFile, _ := newServiceRefServer(t)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"service": "catalog-service", "changed_files": []string{catalogFile}},
		},
	})
	payload := xtraPayload(t, resp)
	assertServiceRefHopShape(t, firstServiceRefHop(t, payload))
}

func TestBlastRadius_ServiceRefHops_EmptyWhenNoRouteChanged(t *testing.T) {
	srv, catalogFile, _ := newServiceRefServer(t)
	_ = catalogFile
	// Changing an indexed file that registers no route must report zero
	// hops, not error (a file the index does not hold is an error, O1071).
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "blast_radius",
			"arguments": map[string]any{"service": "catalog-service", "changed_files": []string{"catalog-service/unrelated.go"}},
		},
	})
	payload := xtraPayload(t, resp)
	hops, _ := payload["service_ref_hops"].([]any)
	if len(hops) != 0 {
		t.Errorf("service_ref_hops = %+v, want empty for an unrelated changed file", hops)
	}
}

func TestReviewDiff_ServiceRefHops(t *testing.T) {
	srv, catalogFile, _ := newServiceRefServer(t)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "catalog-service", "changed_files": []string{catalogFile}},
		},
	})
	payload := xtraPayload(t, resp)
	assertServiceRefHopShape(t, firstServiceRefHop(t, payload))
}

func (b *serviceRefFakeBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := b.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}
