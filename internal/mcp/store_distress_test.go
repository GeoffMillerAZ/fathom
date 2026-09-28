package mcp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// countErrFleet is a fleet whose symbol-count read fails.
type countErrFleet struct {
	*fakeFleet
	err error
}

func (f countErrFleet) SymbolAggregatesErr(string) (int, []core.SourceKind, error) {
	return 0, nil, f.err
}

// TestListServicesCountErrorConformsToSchema (RT-41, I38): a service whose
// symbol count cannot be read carries symbol_count_error and no
// symbol_count — and that result still conforms to list_services' own
// advertised outputSchema (a server MUST conform to it).
func TestListServicesCountErrorConformsToSchema(t *testing.T) {
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	fleet := countErrFleet{fakeFleet: &fakeFleet{svcs: map[string][]core.SymbolNode{"svc": nil}}, err: errors.New("database or disk is full (13)")}
	srv := mcp.NewServerWithFleet(&extendedFakeBackend{services: []string{"svc"}}, fleet)
	// list_services lives on the extended surface (SR-33 moved it off
	// /rpc/agent's now-eight-tool default); either scoped surface serves
	// its outputSchema identically, this test's actual concern.
	ctx := mcp.WithContracts(mcp.WithRequestScope(context.Background(), mcp.SurfaceExtended, true), true)
	listResp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{}})
	var schema map[string]any
	for _, raw := range listResp["result"].(map[string]any)["tools"].([]any) {
		if e := raw.(map[string]any); e["name"] == "list_services" {
			schema, _ = e["outputSchema"].(map[string]any)
		}
	}
	if schema == nil {
		t.Fatal("list_services advertises no outputSchema")
	}
	resp := scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: map[string]any{"name": "list_services", "arguments": map[string]any{}}})
	sc, _ := resp["result"].(map[string]any)["structuredContent"].(map[string]any)
	svcs, _ := sc["services"].([]any)
	if len(svcs) != 1 {
		t.Fatalf("services = %v", sc)
	}
	e := svcs[0].(map[string]any)
	if _, has := e["symbol_count"]; has {
		t.Fatalf("an unreadable count rendered symbol_count: %v", e)
	}
	if e["symbol_count_error"] == nil {
		t.Fatalf("no symbol_count_error: %v", e)
	}
	if msg := mcp.ValidateAgainstSchemaForTest(schema, sc); msg != "" {
		t.Fatalf("list_services with a count error breaks its outputSchema: %s", msg)
	}
}

// fullDiskBackend's ingest fails with SQLITE_FULL.
type fullDiskBackend struct{ extendedFakeBackend }

func (f *fullDiskBackend) IngestService(context.Context, string) error {
	return errors.New("fusion: store symbols: database or disk is full (13)")
}

// TestIngestToolWriteErrorIsStoreDistress (RT-41, §2.1): an ingest that
// fails writing the store with SQLITE_FULL sets store distress, so the
// next ingest and refresh are refused instead of admitted to fail again.
func TestIngestToolWriteErrorIsStoreDistress(t *testing.T) {
	tenancy.ResetLifecycleStateForTest()
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	srv := mcp.NewServerWithFleet(&fullDiskBackend{extendedFakeBackend{services: []string{"svc"}}}, &fakeFleet{svcs: map[string][]core.SymbolNode{"svc": nil}})
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	_ = scopedRoundtrip(t, srv, ctx, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc"}}})
	if why, on := tenancy.StoreDistressNow(time.Now()); !on {
		t.Fatalf("an ingest's SQLITE_FULL did not set store distress (%q)", why)
	}
}
