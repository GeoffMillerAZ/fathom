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
