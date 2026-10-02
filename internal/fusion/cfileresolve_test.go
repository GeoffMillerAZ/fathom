package fusion_test

// LB-Q-c (design §13.2 step 13): the c dialect's imports.scm resolves
// across two fixture files via its manifest's include-path strategy —
// mirroring TestResolveFileEdges_TypeScriptImports/_PythonImports'
// shape for the C-specific "whole/wildcard" include semantics
// (a #include brings every top-level name of its target into scope,
// design §6) instead of duplicating LB-F11's own strategy-level
// TestResolveStrategies coverage.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_CIncludes(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "c", []string{"util.h", "main.c"})

	edges, report := fusion.ResolveFileEdges(context.Background(), "c", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	if report.FilesScanned != 2 {
		t.Errorf("FilesScanned = %d, want 2", report.FilesScanned)
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
		fkey("main", "use"): fkey("util", "helper"), // #include "util.h" brings helper into scope (include-path is wholeIsWildcard)
	}
	got := fileResolveEdgeSet(edges)
	if len(got) != len(want) {
		t.Fatalf("got %d edges %v, want %d %v", len(got), got, len(want), want)
	}
	for from, to := range want {
		if got[from] != to {
			t.Errorf("edge from %s: got %s, want %s", from, got[from], to)
		}
	}
}

// LB-Q-c-rest (O281): a call whose name no definition in the caller's file or
// its transitive includes binds, while an included header declares it, binds
// the one .c file that defines it with external linkage. Two definers bind
// nothing, a static definition is no definer, and a name no included header
// declares is left alone. Each outcome is counted.
func TestResolveFileEdges_CPrototype(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cprotofixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	paths := []string{"api.h", "outer.h", "open.c", "twice_a.c", "twice_b.c", "dual_static.c", "dual_real.c", "stray.c", "caller.c"}
	files, symbols := loadFileResolveFixture(t, dir, "c", paths)
	edges, report := fusion.ResolveFileEdges(context.Background(), "c", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("unexpectedly skipped: %s", report.SkipReason)
	}
	got := fileResolveEdgeSetAll(edges)
	from := fkey("caller", "run")
	want := map[string]bool{
		from + " -> " + fkey("open", "api_open"):      true, // a prototype in api.h, reached through outer.h
		from + " -> " + fkey("open", "api_wrapped"):   true, // extern "C" before the prototype
		from + " -> " + fkey("dual_real", "api_dual"): true, // the static definition is no definer
		from + " -> " + fkey("api", "api_inline"):     true, // defined in a transitively included header
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing edge %s", k)
		}
	}
	for k := range got {
		if !want[k] && strings.HasPrefix(k, from) {
			t.Errorf("unexpected edge %s (api_twice has two definers, api_missing none, stray is declared nowhere)", k)
		}
	}
	if report.ProtoBound != 3 {
		t.Errorf("ProtoBound = %d, want 3", report.ProtoBound)
	}
	if report.ProtoAmbiguous != 1 {
		t.Errorf("ProtoAmbiguous = %d, want 1 (api_twice)", report.ProtoAmbiguous)
	}
	if report.ProtoNoDefiner != 1 {
		t.Errorf("ProtoNoDefiner = %d, want 1 (api_missing)", report.ProtoNoDefiner)
	}
	if report.AncestorIncludes != 0 {
		t.Errorf("AncestorIncludes = %d, want 0", report.AncestorIncludes)
	}
}

// TestResolveFileEdges_CPrototypeBuildFilter: two platform trees define one
// function; the compile database compiles one, so only that one is a definer.
func TestResolveFileEdges_CPrototypeBuildFilter(t *testing.T) {
	write := func(root, rel, s string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{"hdr.h", "main.c", "unix/impl.c", "win/impl.c"}
	setup := func(withDB bool) string {
		root := t.TempDir()
		write(root, "hdr.h", "int plat_init(void);\n")
		write(root, "main.c", "#include \"hdr.h\"\nint run(void) { return plat_init(); }\n")
		write(root, "unix/impl.c", "int plat_init(void) { return 1; }\n")
		write(root, "win/impl.c", "int plat_init(void) { return 2; }\n")
		if withDB {
			db := `[{"directory":"` + root + `/build","command":"cc -c ../main.c","file":"../main.c"},` +
				`{"directory":"` + root + `/build","command":"cc -c ../unix/impl.c","file":"../unix/impl.c"}]`
			write(root, "build/compile_commands.json", db)
		}
		return root
	}
	t.Run("with-database", func(t *testing.T) {
		root := setup(true)
		files, symbols := loadFileResolveFixture(t, root, "c", paths)
		edges, report := fusion.ResolveFileEdges(context.Background(), "c", root, files, symbols)
		got := fileResolveEdgeSetAll(edges)
		if want := fkey("main", "run") + " -> " + fkey("unix/impl", "plat_init"); len(got) != 1 || !got[want] {
			t.Errorf("edges = %v, want only %s", got, want)
		}
		if report.ProtoBound != 1 || report.ProtoBuildFiltered != 1 || report.CompileDBs != 1 {
			t.Errorf("ProtoBound/ProtoBuildFiltered/CompileDBs = %d/%d/%d, want 1/1/1", report.ProtoBound, report.ProtoBuildFiltered, report.CompileDBs)
		}
		for _, e := range edges {
			if e.ToSymbol == fkey("unix/impl", "plat_init") {
				if f := symbolFile(symbols, e.ToSymbol); f != "unix/impl.c" {
					t.Errorf("bound %s, want unix/impl.c", f)
				}
			}
		}
	})
	t.Run("no-database", func(t *testing.T) {
		root := setup(false)
		files, symbols := loadFileResolveFixture(t, root, "c", paths)
		edges, report := fusion.ResolveFileEdges(context.Background(), "c", root, files, symbols)
		if len(edges) != 0 {
			t.Errorf("edges = %v, want none (two definers, no build to choose)", fileResolveEdgeSetAll(edges))
		}
		if report.ProtoAmbiguous != 1 || report.ProtoBound != 0 {
			t.Errorf("ProtoAmbiguous/ProtoBound = %d/%d, want 1/0", report.ProtoAmbiguous, report.ProtoBound)
		}
	})
}

func symbolFile(symbols []core.SymbolNode, key string) string {
	for _, s := range symbols {
		if s.Key() == key {
			return s.FilePath
		}
	}
	return ""
}

// TestResolveFileEdges_CAncestorInclude: a quoted include rooted at an ancestor
// directory resolves with no compile database and is counted.
func TestResolveFileEdges_CAncestorInclude(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lib/curlx/b64.h", "int b64_encode(int v) { return v; }\n")
	write("lib/curlx/b64.c", "#include \"curlx/b64.h\"\nint use(void) { return b64_encode(1); }\n")
	files, symbols := loadFileResolveFixture(t, root, "c", []string{"lib/curlx/b64.h", "lib/curlx/b64.c"})
	edges, report := fusion.ResolveFileEdges(context.Background(), "c", root, files, symbols)
	if want := fkey("lib/curlx/b64", "use") + " -> " + fkey("lib/curlx/b64", "b64_encode"); !fileResolveEdgeSetAll(edges)[want] {
		t.Errorf("edges = %v, want %s", fileResolveEdgeSetAll(edges), want)
	}
	if report.AncestorIncludes != 1 {
		t.Errorf("AncestorIncludes = %d, want 1", report.AncestorIncludes)
	}
}
