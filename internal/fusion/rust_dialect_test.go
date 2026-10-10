package fusion

import (
	"context"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// rustDialectCorpus is a small crate: a root declaring a file module, a const and a
// static (kinds only the tags dialect emits; the CGO rustextract.go had none) and a
// function calling into the module through its path.
func rustDialectCorpus() map[string]string {
	return map[string]string{
		"Cargo.toml": "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
		"src/lib.rs": "mod util;\n\npub const MAX: u32 = 3;\npub static NAME: &str = \"app\";\n\n" +
			"pub fn top() -> u32 {\n    util::helper();\n    MAX\n}\n",
		"src/util.rs": "pub fn helper() {}\n",
	}
}

// TestRustDialectIsWhatFusionDispatches (D1, K25, LB-F388): lang-parity reads the Rust
// tags dialect at least as good as the CGO rustextract.go on every gated metric on
// ripgrep and clap once the harness resolves a symbol defined in more than one
// document to every one (MERGE, docs/bench/readings/lang/parity/rust.json, O539), so
// fusion.New serves .rs from the dialect, which emits a RustConst and a RustStatic.
func TestRustDialectIsWhatFusionDispatches(t *testing.T) {
	if got := DispatchTable()[".rs"]; got != "tags dialect `rust`" {
		t.Errorf("DispatchTable[.rs] = %q, want the rust tags dialect", got)
	}
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := rustDialectCorpus()
	rel := "src/lib.rs"
	e, _, ok, unclassified := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
	}
	if _, isTS := e.(*extractor.TSExtractor); !isTS {
		t.Fatalf("%s dispatches to %T, want the Rust tags dialect", rel, e)
	}
}

// TestIngest_RustDialect (the product-reach proof, LB-F388, LB-F389): an ingest of a
// small crate stores the dialect's symbols for a .rs file (a RustConst and a RustStatic
// among them) and the file-resolved edges its imports make: the call through `util::`
// and the file edge `mod util;` draws to src/util.rs (K25 ruling 4).
func TestIngest_RustDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, rustDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "rsx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "rsx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"src/lib.rs|RustConst|MAX", "src/lib.rs|RustStatic|NAME", "src/lib.rs|RustFn|top", "src/lib.rs|RustMod|util", "src/util.rs|RustFn|helper"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	edges, err := st.TypedEdges(ctx, "rsx")
	if err != nil {
		t.Fatal(err)
	}
	pairs := map[string]bool{}
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			pairs[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
	}
	helper := core.SymbolKey("rsx", "src/util", "helper")
	for _, want := range []string{
		core.SymbolKey("rsx", "src/lib", "top") + " -> " + helper,  // util::helper()
		core.SymbolKey("rsx", "src/lib", "util") + " -> " + helper, // mod util; (a file edge to util.rs's anchor)
	} {
		if !pairs[want] {
			t.Errorf("file-resolved edge %s not stored; have %v", want, pairs)
		}
	}
}
