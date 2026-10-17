package callgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestProductReach_HardeningCorpus is EW-15: the wired check's
// over-approximations, each read off a real ingest (the Go extractor and the
// typed pass) of testdata/hardening, a module built to fool it. The product
// walk must not count
//
//   - a blank-identifier reference (`var _ = f`, `_ = f`) as a call;
//   - a `package main` under a build constraint the default build excludes
//     (`//go:build ignore`) as an entry point;
//   - a receiver-variable call the syntax pass matched to the one
//     first-party method of that name (`r.Len()` on a strings.Reader, never
//     lib.Stack's Len) once the typed pass has resolved the caller's file;
//   - an interface call's implementation whose type no product code
//     instantiates (Fake, built only by a test): interface dispatch is
//     cross-checked against the types the reached code instantiates (RTA);
//
// and a package linked only through a method (main calls reg.T's Run) still
// runs its init.
func TestProductReach_HardeningCorpus(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("testdata", "hardening"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "go.mod")); err != nil {
		t.Fatalf("corpus missing: %v", err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "fixmod", Path: src}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, "fixmod", "")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, "fixmod")
	if err != nil {
		t.Fatal(err)
	}
	if len(typed) == 0 {
		t.Fatal("the typed pass wrote no edges: the corpus did not type-check")
	}
	r := callgraph.ProductEntryReach(callgraph.BuildBoth(nodes, typed))

	entries := map[string]bool{}
	for _, e := range r.Entries() {
		entries[e] = true
	}
	if !entries["fixmod::cmd/app/main.main"] || entries["fixmod::tools/gen/main.main"] {
		t.Errorf("entries = %v: want cmd/app's main, never the //go:build ignore generator's", r.Entries())
	}
	for e := range entries {
		if e == "fixmod::reg/reg.seam" || e == "fixmod::reg/reg.table" {
			t.Errorf("entries = %v: a package variable runs with its package's initializers, it is no entry point of its own", r.Entries())
		}
	}

	for _, c := range []struct {
		symbol string
		wired  bool
		why    string
	}{
		{"fixmod::lib/lib.Wired", true, "main calls it"},
		{"fixmod::other/other.Render", true, "main calls it"},
		{"fixmod::iface/Real.Real.Do", true, "main converts a Real to Doer and calls Do"},
		{"fixmod::reg/T.T.Run", true, "main calls it on a declared reg.T"},
		{"fixmod::reg/reg.InitOnly", true, "reg's init calls it, and main links reg through T.Run"},
		{"fixmod::reg/reg.SeamOnly", true, "reg's var seam calls it at start-up's package initialization (O1124)"},
		{"fixmod::reg/reg.TableOnly", true, "reg's var table registers it (O1124)"},
		{"fixmod::lib/lib.DeadViaBlank", false, "only `var _ = DeadViaBlank` names it"},
		{"fixmod::lib/lib.DeadViaUncalledMainHelper", false, "only a helper main takes with `_ = unusedHelper` calls it"},
		{"fixmod::lib/lib.GenOnly", false, "only a //go:build ignore generator calls it"},
		{"fixmod::lib/Stack.Stack.Len", false, "main's r.Len() is strings.Reader's"},
		{"fixmod::lib/lib.StackOnly", false, "only Stack.Len calls it"},
		{"fixmod::iface/Fake.Fake.Do", false, "no product code instantiates Fake"},
		{"fixmod::iface/iface.FakeOnly", false, "only Fake.Do and a test call it"},
		{"fixmod::lib/lib.Render", false, "main calls other.Render, not this one"},
		{"fixmod::lib/lib.TestOnly", false, "control: only a test calls it"},
	} {
		entry, ok := r.ReachedBy(c.symbol)
		if ok != c.wired {
			t.Errorf("%s: product-reached = %v (entry %q), want %v — %s", c.symbol, ok, entry, c.wired, c.why)
		}
	}

	// Determinism: a second build over the same ingest agrees.
	r2 := callgraph.ProductEntryReach(callgraph.BuildBoth(nodes, typed))
	if !reflect.DeepEqual(r.Entries(), r2.Entries()) || r.Len() != r2.Len() {
		t.Errorf("two walks differ: %v/%d vs %v/%d", r.Entries(), r.Len(), r2.Entries(), r2.Len())
	}
}
