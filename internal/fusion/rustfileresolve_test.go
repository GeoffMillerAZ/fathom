package fusion_test

// LB-Q-rust (design §13.2 step 13): the rust dialect's imports.scm resolves
// across fixture files through its manifest's crate-mod strategy — a
// `mod x;` declaration, a `crate::` path and a path rooted at a child module,
// with the calls each binds. `net::connect` reaches src/net/mod.rs from
// main.rs (a child module directory), `crate::util::helper` names an item of
// src/util.rs, and `util::other()` is a call through the whole-module binding
// `mod util;` makes (the scope operator is spelled `util.other` by the
// manifest's scope_sep). A path with a generic segment or a receiver
// (`Widget::new()`) is not resolved: a type-associated call has no
// module-level top-level symbol to reach.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_RustUse(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "rustfixture"))
	if err != nil {
		t.Fatal(err)
	}
	// Cargo.toml is read from disk by the resolver (crateSrcDir), not extracted.
	rels := []string{"src/main.rs", "src/util.rs", "src/net/mod.rs"}
	files, symbols := loadFileResolveFixture(t, dir, "rust", rels)

	edges, report := fusion.ResolveFileEdges(context.Background(), "rust", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", report.FilesScanned)
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: Kind = %q, want file-resolved", e, e.Kind)
		}
		if core.EdgeConfidenceOf(e) != core.EdgeConfidenceHeuristic {
			t.Errorf("edge %+v: EdgeConfidenceOf = %q, want heuristic", e, core.EdgeConfidenceOf(e))
		}
	}
	got := fileResolveEdgeSetAll(edges)
	from := fkey("src/main", "main")
	want := []string{
		from + " -> " + fkey("src/util", "helper"),     // use crate::util::helper; helper()
		from + " -> " + fkey("src/util", "other"),      // mod util; util::other()
		from + " -> " + fkey("src/net/mod", "connect"), // use net::connect; connect() (src/net/mod.rs)
		// K25 ruling 4 (LB-F389): each module reference is a file edge between the files'
		// anchors, from the `mod x;` declaration or, at the top level, the file's first
		// definition (`mod net;` on line 1).
		fkey("src/main", "net") + " -> " + fkey("src/net/mod", "connect"), // mod net; use net::connect;
		fkey("src/main", "util") + " -> " + fkey("src/util", "helper"),    // mod util;
		fkey("src/main", "net") + " -> " + fkey("src/util", "helper"),     // use crate::util::helper; (top level)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
}

// TestResolveFileEdges_RustModuleFileEdges (K25 ruling 4, O541, K18; LB-F389): a
// reference to a module path is a FILE edge to the module's defining file (x.rs or
// x/mod.rs), never a symbol of its own: `mod x;`, each module a `use` path names
// (`crate`, `self`, `super`, a child module, a workspace crate), from the definition
// around the line (the `mod x;` declaration itself, a function holding a `use`) or, at a
// file's top level, from the file's first definition, to the module file's first
// definition (the file's anchor: Rust mints no module symbol, K18). An inline `mod x { }`
// stays in its file, a path into the referring file draws nothing and an external
// crate (`std`) nothing either.
func TestResolveFileEdges_RustModuleFileEdges(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "rustmodfixture"))
	if err != nil {
		t.Fatal(err)
	}
	rels := []string{"src/lib.rs", "src/util.rs", "src/net/mod.rs", "src/net/tcp.rs"}
	files, symbols := loadFileResolveFixture(t, dir, "rust", rels)
	edges, report := fusion.ResolveFileEdges(context.Background(), "rust", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	fileOf := map[string]string{}
	for _, s := range symbols {
		fileOf[s.Key()] = s.FilePath
	}
	pairs := map[string]bool{}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: Kind = %q, want file-resolved", e, e.Kind)
		}
		to, ok := fileOf[e.ToSymbol]
		if !ok {
			t.Errorf("edge %+v: target is no symbol of the universe (a minted module symbol?)", e)
			continue
		}
		if fileOf[e.FromSymbol] != e.FromFile {
			t.Errorf("edge %+v: from symbol is not in its from file", e)
		}
		pairs[e.FromFile+" -> "+to] = true
	}
	want := []string{
		"src/lib.rs -> src/util.rs",        // mod util; use crate::util::Helper;
		"src/lib.rs -> src/net/mod.rs",     // mod net;
		"src/util.rs -> src/lib.rs",        // use super::top; (super is the crate root)
		"src/net/mod.rs -> src/net/tcp.rs", // mod tcp; use self::tcp::Conn;
		"src/net/tcp.rs -> src/lib.rs",     // use crate::inline::inside; (an inline mod is its file's)
		"src/net/tcp.rs -> src/util.rs",    // use crate::util::util_fn; inside fn dial
	}
	for _, w := range want {
		if !pairs[w] {
			t.Errorf("missing file edge %s; got %v", w, pairs)
		}
	}
	if len(pairs) != len(want) {
		t.Errorf("got %d file pairs %v, want %d", len(pairs), pairs, len(want))
	}
	got := fileResolveEdgeSetAll(edges)
	for _, w := range []string{
		fkey("src/lib", "util") + " -> " + fkey("src/util", "Helper"),     // from the `mod util;` declaration to util.rs's anchor
		fkey("src/net/tcp", "dial") + " -> " + fkey("src/util", "Helper"), // a use inside a function is that function's
		fkey("src/util", "Helper") + " -> " + fkey("src/lib", "util"),     // a top-level use is the file's (its first definition)
	} {
		if !got[w] {
			t.Errorf("missing edge %s; got %v", w, got)
		}
	}
}
