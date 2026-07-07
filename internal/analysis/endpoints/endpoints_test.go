package endpoints_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/endpoints"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

// helper: build Route quickly
func r(method, path, handler, file string, line int) routes.Route {
	return routes.Route{Method: method, Path: path, Handler: handler, File: file, Line: line}
}

// -----------------------------------------------------------------
// Basic inventory
// -----------------------------------------------------------------

func TestInventoryBasic(t *testing.T) {
	rts := []routes.Route{
		r("GET", "/health", "HealthHandler", "main.go", 10),
		r("POST", "/users", "CreateUser", "users.go", 20),
		r("GET", "/users", "ListUsers", "users.go", 15),
	}
	inv := endpoints.Inventory(rts)
	if len(inv) != 3 {
		t.Fatalf("expected 3 endpoints, got %d: %v", len(inv), inv)
	}
	// sorted: DELETE < GET < POST; within GET: /health < /users
	byKey := make(map[string]endpoints.Endpoint)
	for _, e := range inv {
		byKey[e.Method+":"+e.Path] = e
	}

	e := byKey["GET:/health"]
	if len(e.Handlers) != 1 || e.Handlers[0] != "HealthHandler" {
		t.Errorf("unexpected handlers: %v", e.Handlers)
	}
	if len(e.Files) != 1 || e.Files[0] != "main.go" {
		t.Errorf("unexpected files: %v", e.Files)
	}
	if e.Lines[0] != 10 {
		t.Errorf("unexpected line: %d", e.Lines[0])
	}
}

// -----------------------------------------------------------------
// Deduplication: same method+path from multiple files
// -----------------------------------------------------------------

func TestInventoryDeduplicate(t *testing.T) {
	rts := []routes.Route{
		r("GET", "/items", "ListItems", "v1.go", 5),
		r("GET", "/items", "ListItems", "v1.go", 5),   // exact duplicate
		r("GET", "/items", "ListItemsV2", "v2.go", 8), // different handler, different file
	}
	inv := endpoints.Inventory(rts)
	if len(inv) != 1 {
		t.Fatalf("expected 1 endpoint, got %d: %v", len(inv), inv)
	}
	e := inv[0]
	if e.Method != "GET" || e.Path != "/items" {
		t.Errorf("unexpected method/path: %v", e)
	}
	// handlers: ListItems and ListItemsV2, sorted
	if len(e.Handlers) != 2 {
		t.Errorf("expected 2 handlers, got %v", e.Handlers)
	}
	if e.Handlers[0] != "ListItems" || e.Handlers[1] != "ListItemsV2" {
		t.Errorf("handlers not sorted: %v", e.Handlers)
	}
	// files: v1.go and v2.go, sorted
	if len(e.Files) != 2 {
		t.Errorf("expected 2 files, got %v", e.Files)
	}
	if e.Files[0] != "v1.go" || e.Files[1] != "v2.go" {
		t.Errorf("files not sorted: %v", e.Files)
	}
}

// -----------------------------------------------------------------
// First-occurrence line tracking
// -----------------------------------------------------------------

func TestInventoryFirstLinePerFile(t *testing.T) {
	rts := []routes.Route{
		r("GET", "/x", "H1", "a.go", 100),
		r("GET", "/x", "H2", "a.go", 200), // same file — should keep line 100
		r("GET", "/x", "H3", "b.go", 50),
	}
	inv := endpoints.Inventory(rts)
	if len(inv) != 1 {
		t.Fatalf("expected 1 endpoint")
	}
	e := inv[0]
	// files sorted: a.go, b.go
	if e.Files[0] != "a.go" || e.Lines[0] != 100 {
		t.Errorf("a.go line=%d, want 100", e.Lines[0])
	}
	if e.Files[1] != "b.go" || e.Lines[1] != 50 {
		t.Errorf("b.go line=%d, want 50", e.Lines[1])
	}
}

// -----------------------------------------------------------------
// Empty input
// -----------------------------------------------------------------

func TestInventoryEmpty(t *testing.T) {
	inv := endpoints.Inventory(nil)
	if len(inv) != 0 {
		t.Errorf("expected empty, got %v", inv)
	}
	inv2 := endpoints.Inventory([]routes.Route{})
	if len(inv2) != 0 {
		t.Errorf("expected empty, got %v", inv2)
	}
}

// -----------------------------------------------------------------
// Sorted output determinism
// -----------------------------------------------------------------

func TestInventoryDeterminism(t *testing.T) {
	rts := []routes.Route{
		r("POST", "/z", "ZH", "z.go", 1),
		r("GET", "/a", "AH", "a.go", 1),
		r("DELETE", "/m", "MH", "m.go", 1),
		r("GET", "/b", "BH", "b.go", 1),
	}
	a := endpoints.Inventory(rts)
	b := endpoints.Inventory(rts)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i].Method != b[i].Method || a[i].Path != b[i].Path {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
	// verify sort order: DELETE < GET < POST
	if a[0].Method != "DELETE" {
		t.Errorf("expected DELETE first, got %s", a[0].Method)
	}
	if a[1].Method != "GET" || a[1].Path != "/a" {
		t.Errorf("expected GET /a second, got %s %s", a[1].Method, a[1].Path)
	}
	if a[2].Method != "GET" || a[2].Path != "/b" {
		t.Errorf("expected GET /b third")
	}
	if a[3].Method != "POST" {
		t.Errorf("expected POST last, got %s", a[3].Method)
	}
}

// -----------------------------------------------------------------
// Round-trip: Extract -> Inventory
// -----------------------------------------------------------------

func TestInventoryRoundTrip(t *testing.T) {
	import_routes := []routes.Route{
		r("GET", "/health", "HealthHandler", "svc.go", 10),
		r("POST", "/orders", "CreateOrder", "orders.go", 25),
		r("GET", "/orders", "ListOrders", "orders.go", 30),
		r("GET", "/orders", "ListOrders", "orders_v2.go", 5),
	}
	inv := endpoints.Inventory(import_routes)
	if len(inv) != 3 {
		t.Fatalf("expected 3 unique endpoints, got %d", len(inv))
	}
	byKey := make(map[string]endpoints.Endpoint)
	for _, e := range inv {
		byKey[e.Method+":"+e.Path] = e
	}
	ge := byKey["GET:/orders"]
	if len(ge.Files) != 2 {
		t.Errorf("GET /orders should have 2 files, got %v", ge.Files)
	}
}
