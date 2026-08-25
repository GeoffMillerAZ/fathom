package fusion_test

// TestClosureCallsReachFromEnclosingTest is VS-7 (docs/plans/
// verify-spine.md): a call inside a function literal (t.Run body, defer
// func, goroutine, callback) is attributed to the enclosing named function
// — proven directly against the typed pass rather than assumed. Both
// classifyCall's ast.Inspect (typedcalls.go) and collectCalls' ast.Inspect
// (goextract.go) already descend into FuncLit bodies with no special
// exclusion, so this is a PIN test: it establishes the "call it at the
// test's top level" folk rule was already false, with evidence, rather
// than fixing anything.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestClosureCallsReachFromEnclosingTest(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "reachfix"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()

	if _, err := fusion.EnrichTypedEdges(ctx, st, "reachfix", dir, nil); err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	edges, err := st.TypedEdges(ctx, "reachfix")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}

	key := func(pkg, name string) string { return core.SymbolKey("reachfix", pkg, name) }
	from, to := key("reachfix", "TestClosureOnly"), key("reachfix", "onlyInClosure")

	for _, e := range edges {
		if e.FromSymbol == from && e.ToSymbol == to && e.Kind == core.EdgeDirect {
			return // found: the call inside the t.Run closure was attributed
			// to the enclosing TestClosureOnly, not lost.
		}
	}
	t.Errorf("no EdgeDirect TestClosureOnly -> onlyInClosure found — a call made only inside a t.Run closure did not reach its enclosing named function")
}
