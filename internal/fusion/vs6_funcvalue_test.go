package fusion_test

// TestFuncValueReferencesAreEdges is VS-6 (docs/plans/verify-spine.md): a
// function referenced as a VALUE — mux.HandleFunc(p, h), a route-table
// literal {p, h}, an http.HandlerFunc(h) conversion, middleware(h), a
// method value s.handleX — is an edge from the enclosing function, typed
// pass. Also pins the exclusions the pass must respect: a composite-literal
// KEY (the struct field name "h"/"path") and the left side of an assignment
// ("s") must never themselves become edge targets or sources of a phantom
// edge.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestFuncValueReferencesAreEdges(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "reachfix"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "reachfix", dir, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	if report.PackagesFailed != 0 {
		t.Errorf("PackagesFailed = %d, want 0 (fixture module must build clean)", report.PackagesFailed)
	}
	if report.EdgesFuncValue == 0 {
		t.Error("EdgesFuncValue = 0, want > 0")
	}

	edges, err := st.TypedEdges(ctx, "reachfix")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}

	key := func(pkg, name string) string { return core.SymbolKey("reachfix", pkg, name) }
	newServer := key("reachfix", "newServer")

	has := func(from, to string, kind core.EdgeKind) bool {
		for _, e := range edges {
			if e.FromSymbol == from && e.ToSymbol == to && e.Kind == kind {
				return true
			}
		}
		return false
	}

	// A bare selector argument (mux.HandleFunc("/a", handleA)).
	if !has(newServer, key("reachfix", "handleA"), core.EdgeFuncValue) {
		t.Error("newServer -> handleA: want an EdgeFuncValue edge (bare selector arg)")
	}
	// A keyed route-table composite literal ({path: "/b", h: handleB}).
	if !has(newServer, key("reachfix", "handleB"), core.EdgeFuncValue) {
		t.Error("newServer -> handleB: want an EdgeFuncValue edge (route-table literal value)")
	}
	// An http.HandlerFunc(handleC) type-conversion argument.
	if !has(newServer, key("reachfix", "handleC"), core.EdgeFuncValue) {
		t.Error("newServer -> handleC: want an EdgeFuncValue edge (conversion arg)")
	}
	// wrap(handleD): a real call to wrap (EdgeDirect) AND a value ref to handleD.
	if !has(newServer, key("reachfix", "wrap"), core.EdgeDirect) {
		t.Error("newServer -> wrap: want an EdgeDirect edge (a genuine call)")
	}
	if !has(newServer, key("reachfix", "handleD"), core.EdgeFuncValue) {
		t.Error("newServer -> handleD: want an EdgeFuncValue edge (middleware arg)")
	}
	// s.handleE: a method value.
	if !has(newServer, key("server", "server.handleE"), core.EdgeFuncValue) {
		t.Error("newServer -> server.handleE: want an EdgeFuncValue edge (method value)")
	}

	// Exclusions: no edge whose ToSymbol names the composite-literal KEY
	// idents ("path", "h") or the assignment LHS ident ("s") as if they
	// were referenced functions — those names don't correspond to any real
	// symbol in the fixture, so any such edge would be a fabrication.
	for _, bad := range []string{
		key("reachfix", "path"), key("reachfix", "h"), key("reachfix", "s"),
	} {
		for _, e := range edges {
			if e.ToSymbol == bad {
				t.Errorf("found a fabricated edge to %q (composite-literal key or assignment LHS leaked into the value-ref pass): %+v", bad, e)
			}
		}
	}
}
