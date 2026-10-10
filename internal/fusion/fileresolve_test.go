package fusion_test

// SU-11 (docs/plans/substrate.md §B, ADR-041 decision 5): the per-file
// import/module-scope resolver for TypeScript/JavaScript and Python.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// loadFileResolveFixture reads every file directly under dir (non-
// recursive is fine — both fixtures are flat/one-subdir), extracts symbols
// with the same tree-sitter extractor ResolveFileEdges itself uses, and
// returns both the raw sources and the symbol set — exactly the two inputs
// a real ingest would hand the resolver.
func loadFileResolveFixture(t *testing.T, dir, lang string, relPaths []string) ([]core.SourceFile, []core.SymbolNode) {
	t.Helper()
	ext, err := extractor.NewTSExtractor(lang)
	if err != nil {
		t.Fatalf("NewTSExtractor(%s): %v", lang, err)
	}
	var files []core.SourceFile
	var symbols []core.SymbolNode
	for _, rel := range relPaths {
		content, rerr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if rerr != nil {
			t.Fatalf("read %s: %v", rel, rerr)
		}
		files = append(files, core.SourceFile{Path: rel, Content: content, Lang: lang})
		nodes, eerr := ext.Extract("fixture", rel, content)
		if eerr != nil {
			t.Fatalf("extract %s: %v", rel, eerr)
		}
		for i := range nodes {
			nodes[i].Service = "fixture"
		}
		symbols = append(symbols, nodes...)
	}
	return files, symbols
}

func fileResolveEdgeSet(edges []core.Edge) map[string]string {
	m := map[string]string{}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			continue
		}
		m[e.FromSymbol] = e.ToSymbol
	}
	return m
}

func fkey(pkg, name string) string { return core.SymbolKey("fixture", pkg, name) }

func TestResolveFileEdges_TypeScriptImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "tsfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "javascript", []string{"util.ts", "index.ts", "wrapper.ts"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "javascript", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", report.FilesScanned)
	}
	if report.EdgesResolved != len(edges) {
		t.Errorf("report.EdgesResolved = %d, want %d (len(edges))", report.EdgesResolved, len(edges))
	}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved {
			t.Errorf("edge %+v: Kind = %q, want file-resolved", e, e.Kind)
		}
		if core.EdgeConfidenceOf(e) != core.EdgeConfidenceHeuristic {
			t.Errorf("edge %+v: EdgeConfidenceOf = %q, want heuristic", e, core.EdgeConfidenceOf(e))
		}
	}

	want := map[string]string{
		fkey("index", "run"):    fkey("util", "helper"), // named cross-file import
		fkey("wrapper", "wrap"): fkey("util", "other"),  // namespace member access
	}
	got := fileResolveEdgeSet(edges)
	// "run" also calls the same-file "local" — both edges share the From
	// key, so check that one separately via the full edge list.
	if got[fkey("index", "run")] != want[fkey("index", "run")] {
		t.Errorf("run -> %q, want %q", got[fkey("index", "run")], want[fkey("index", "run")])
	}
	if got[fkey("wrapper", "wrap")] != want[fkey("wrapper", "wrap")] {
		t.Errorf("wrap -> %q, want %q", got[fkey("wrapper", "wrap")], want[fkey("wrapper", "wrap")])
	}
	var sawRunToLocal bool
	for _, e := range edges {
		if e.FromSymbol == fkey("index", "run") && e.ToSymbol == fkey("index", "local") {
			sawRunToLocal = true
		}
	}
	if !sawRunToLocal {
		t.Errorf("missing same-file module-scope edge run -> local; edges: %+v", edges)
	}
	if len(edges) != 3 {
		t.Errorf("edge count = %d, want 3 (run->helper, run->local, wrap->other); got %+v", len(edges), edges)
	}

	// Every resolved edge's From/To must be an actually-extracted symbol —
	// this resolver never invents a key.
	known := map[string]bool{}
	for _, s := range symbols {
		known[s.Key()] = true
	}
	for _, e := range edges {
		if !known[e.FromSymbol] {
			t.Errorf("edge From %s is not an extracted symbol", e.FromSymbol)
		}
		if !known[e.ToSymbol] {
			t.Errorf("edge To %s is not an extracted symbol", e.ToSymbol)
		}
	}
}

func TestResolveFileEdges_PythonImports(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "pyfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "python", []string{"pkg/util.py", "main.py", "pkg/wrapper.py"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "python", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", report.FilesScanned)
	}

	got := fileResolveEdgeSet(edges)
	if want, have := fkey("pkg/util", "helper"), got[fkey("main", "run")]; have != want {
		t.Errorf("run -> %q, want %q", have, want)
	}
	if want, have := fkey("pkg/util", "other"), got[fkey("pkg/wrapper", "wrap")]; have != want {
		t.Errorf("wrap -> %q, want %q", have, want)
	}
	var sawRunToLocal bool
	for _, e := range edges {
		if e.FromSymbol == fkey("main", "run") && e.ToSymbol == fkey("main", "local") {
			sawRunToLocal = true
		}
	}
	if !sawRunToLocal {
		t.Errorf("missing same-file module-scope edge run -> local; edges: %+v", edges)
	}
	// K18: each file's import of pkg.util targets pkg/util.py's module
	// symbol, made from the importing file's module symbol.
	for _, from := range []string{"main", "pkg/wrapper"} {
		if want, have := fkey("pkg/util", "<module>"), got[fkey(from, "<module>")]; have != want {
			t.Errorf("%s's module -> %q, want %q", from, have, want)
		}
	}
	if len(edges) != 5 {
		t.Errorf("edge count = %d, want 5 (run->helper, run->local, wrap->other and the two module imports); got %+v", len(edges), edges)
	}

	known := map[string]bool{}
	for _, s := range symbols {
		known[s.Key()] = true
	}
	for _, e := range edges {
		if !known[e.FromSymbol] {
			t.Errorf("edge From %s is not an extracted symbol", e.FromSymbol)
		}
		if !known[e.ToSymbol] {
			t.Errorf("edge To %s is not an extracted symbol", e.ToSymbol)
		}
	}
}

// TestResolveFileEdges_SkippedWhenScipIndexPresent proves the resolver
// defers to a real SCIP import (SU-9, another wave) rather than silently
// downgrading its typed_edges rows — an index.scip at the repo root is
// enough to skip, regardless of language or corpus.
func TestResolveFileEdges_SkippedWhenScipIndexPresent(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "tsfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	scratch := t.TempDir()
	for _, rel := range []string{"util.ts", "index.ts", "wrapper.ts"} {
		content, rerr := os.ReadFile(filepath.Join(dir, rel))
		if rerr != nil {
			t.Fatalf("read %s: %v", rel, rerr)
		}
		if werr := os.WriteFile(filepath.Join(scratch, rel), content, 0o644); werr != nil {
			t.Fatalf("write %s: %v", rel, werr)
		}
	}
	if err := os.WriteFile(filepath.Join(scratch, "index.scip"), []byte("not a real scip index, presence is all that matters"), 0o644); err != nil {
		t.Fatalf("write index.scip: %v", err)
	}

	files, symbols := loadFileResolveFixture(t, scratch, "javascript", []string{"util.ts", "index.ts", "wrapper.ts"})
	edges, report := fusion.ResolveFileEdges(context.Background(), "javascript", scratch, files, symbols)
	if !report.Skipped {
		t.Fatal("expected Skipped=true with an index.scip present")
	}
	if report.SkipReason == "" {
		t.Error("expected a non-empty SkipReason")
	}
	if len(edges) != 0 {
		t.Errorf("expected zero edges when skipped, got %d", len(edges))
	}
}

// TestResolveFileEdges_TSWorkspacePackages (LB-F225, LB-F227): through the
// public resolver, a pnpm workspace package imported by name binds the
// definitions of its source (its main/types outputs named back through its
// tsconfig's outDir and rootDir), a subpath binds the file it names, a
// package importing its own name binds itself, and a package a negated glob
// leaves out binds nothing.
func TestResolveFileEdges_TSWorkspacePackages(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "tspnpm"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "javascript", []string{
		"libs/alpha/src/index.ts", "libs/alpha/src/util.ts", "libs/beta/index.ts", "libs/beta/other.ts",
		"libs/ignored/index.ts", "tools/script.ts",
	})
	edges, report := fusion.ResolveFileEdges(context.Background(), "javascript", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	where := map[string]string{}
	for _, s := range symbols {
		where[s.Key()] = s.FilePath + "|" + s.Name
	}
	got := map[string]bool{}
	for _, e := range edges {
		got[where[e.FromSymbol]+" -> "+where[e.ToSymbol]] = true
	}
	for _, want := range []string{
		"libs/beta/index.ts|beta -> libs/alpha/src/index.ts|alpha",
		"libs/beta/index.ts|beta -> libs/alpha/src/util.ts|util",
		"libs/beta/index.ts|beta -> libs/beta/other.ts|other",
		"tools/script.ts|script -> libs/alpha/src/index.ts|alpha",
	} {
		if !got[want] {
			t.Errorf("missing edge %s; got %v", want, got)
		}
	}
	if got["tools/script.ts|script -> libs/ignored/index.ts|ignored"] {
		t.Errorf("a package the workspace's negated glob leaves out resolved: %v", got)
	}
}
