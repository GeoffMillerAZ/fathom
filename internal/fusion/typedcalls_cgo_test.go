package fusion_test

import (
	"context"
	"go/build"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestEnrichTypedEdges_CgoFileResolvesFromItsSource (EW-15): a file that
// imports "C" compiles from a generated copy in the build cache; the typed
// pass reads it under its source file's name, so its calls and the types it
// instantiates are known (before, every such file was skipped), and cgo's
// generated _Cfunc_ wrappers are neither edges nor discovery gaps.
func TestEnrichTypedEdges_CgoFileResolvesFromItsSource(t *testing.T) {
	if !build.Default.CgoEnabled {
		t.Skip("cgo is not enabled in this toolchain")
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "cgofixture"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	allowed := map[string]bool{"p/c.go": true, "p/plain.go": true}
	rep, err := fusion.EnrichTypedEdges(ctx, st, "cgofixture", dir, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if rep.PackagesFailed != 0 || rep.DiscoveryGaps != 0 {
		t.Fatalf("report %+v: want no failure and no discovery gap", rep)
	}
	edges, err := st.TypedEdges(ctx, "cgofixture")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"cgofixture::p/H.H.Do -> cgofixture::p/p.helper direct": false,
		"cgofixture::p/p.New -> cgofixture::p/p.H instantiates": false,
	}
	for _, e := range edges {
		k := e.FromSymbol + " -> " + e.ToSymbol + " " + string(e.Kind)
		if _, ok := want[k]; ok {
			want[k] = true
			if e.FromFile != "p/c.go" {
				t.Errorf("%s: FromFile %q, want the source file p/c.go", k, e.FromFile)
			}
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing typed edge %s; got %v", k, edges)
		}
	}
}
