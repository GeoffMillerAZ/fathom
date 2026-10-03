package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ingestFixture ingests a copy of testdata/<name> through the pipeline and
// returns the file-resolved edges as "file:symbol -> file:symbol".
func ingestFixtureRefEdges(t *testing.T, name string) []string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dir := t.TempDir()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "refs-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	label := map[string]string{}
	for _, o := range obs {
		file, _, _ := strings.Cut(o.SourceRef, ":")
		label[svc.ID+"::"+strings.TrimSuffix(file, filepath.Ext(file))+"."+o.Symbol] = file + ":" + o.Symbol
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			out = append(out, e.FromFile+":"+label[e.FromSymbol]+" -> "+label[e.ToSymbol])
		}
	}
	return out
}

// TestIngest_CReferencesReachTheStore (LB-F196): through ingest, a C file's
// type, typedef and macro references are file-resolved edges to the header
// its own include resolves to.
func TestIngest_CReferencesReachTheStore(t *testing.T) {
	got := map[string]bool{}
	for _, e := range ingestFixtureRefEdges(t, "crefsfixture") {
		got[e] = true
	}
	for _, w := range []string{
		"src/use_item.c:src/use_item.c:total -> inc/types.h:item_t",
		"src/use_item.c:src/use_item.c:total -> inc/types.h:ITEM_MAX",
		"src/use_other.c:src/use_other.c:other_total -> inc/other/types.h:other_t",
		"src/mix.c:src/mix.c:pick -> inc/types.h:shape_t",
	} {
		if !got[w] {
			t.Errorf("missing %s\ngot %v", w, got)
		}
	}
	for k := range got {
		if strings.HasPrefix(k, "src/shadow.c:src/shadow.c:shadow_total -> inc/types.h:item_t") || strings.HasPrefix(k, "src/use_other.c:src/use_other.c:other_total -> inc/types.h") {
			t.Errorf("a false binding: %s", k)
		}
	}
}

// TestIngest_CppReferencesReachTheStore (LB-F196, O283): through ingest, a
// C++ qualified type name binds the definition whose namespaces end with its
// qualifier in a header the file includes; a name the file defines itself, a
// qualifier no reached header holds (std::) and another namespace's same-named
// type bind nothing.
func TestIngest_CppReferencesReachTheStore(t *testing.T) {
	got := map[string]bool{}
	for _, e := range ingestFixtureRefEdges(t, "cpprefsfixture") {
		got[e] = true
	}
	for _, w := range []string{
		"src/draw.cpp:src/draw.cpp:Canvas.draw -> include/lib/shapes.hpp:Point",
		"src/draw.cpp:src/draw.cpp:Canvas.draw -> include/lib/shapes.hpp:Shape",
	} {
		if !got[w] {
			t.Errorf("missing %s\ngot %v", w, got)
		}
	}
	for k := range got {
		if strings.Contains(k, "include/lib/other.hpp") || strings.HasPrefix(k, "src/shadow.cpp") {
			t.Errorf("a false binding: %s", k)
		}
		// TEST(shape_suite, Shape): the test name is no type, though lib::Shape
		// is reached (LB-F237, fmt's TEST(xchar_test, ostream) bound fmt::ostream).
		if strings.HasPrefix(k, "src/gtest_names.cpp:src/gtest_names.cpp:TEST ") && strings.HasSuffix(k, ":Shape") {
			t.Errorf("a test name bound a class: %s", k)
		}
	}
	// TEST_F(Point, area): the first argument of a fixture macro is a type.
	if !got["src/gtest_names.cpp:src/gtest_names.cpp:TEST_F -> include/lib/shapes.hpp:Point"] {
		t.Errorf("a fixture macro's type argument lost its edge\ngot %v", got)
	}
}

// TestIngest_RustWorkspaceReachesTheStore (LB-F197): through ingest, a type
// named by `use a::T` binds the file that defines it, past the facade crate's
// `pub use inner::T;`; a type named through `use a::{S}` binds the crate root
// (a bare crate name is a path root, not a child module); `crate::` in a
// [[bin]] root and in an integration test names that root's own modules.
func TestIngest_RustWorkspaceReachesTheStore(t *testing.T) {
	got := map[string]bool{}
	for _, e := range ingestFixtureRefEdges(t, "rustwsfixture") {
		got[e] = true
	}
	for _, w := range []string{
		"crates/b/src/lib.rs:crates/b/src/lib.rs:build -> crates/a/src/inner.rs:T",
		"crates/b/src/lib.rs:crates/b/src/lib.rs:build -> crates/a/src/lib.rs:S",
		"crates/core/main.rs:crates/core/main.rs:run -> crates/core/flags.rs:Flag",
		"tests/tests.rs:tests/tests.rs:main -> tests/util.rs:helper",
	} {
		if !got[w] {
			t.Errorf("missing %s", w)
		}
	}
}

// TestIngest_HeaderKeywordInCommentStaysC (LB-F195, O365): through ingest, a .h
// whose only C++ keyword sits in a comment or a string is extracted as C, not
// sent to the C++ grammar.
func TestIngest_HeaderKeywordInCommentStaysC(t *testing.T) {
	dir := t.TempDir()
	src := "/* A handle is a class of object the loop owns; a template of the\n * namespace of ids. */\n#ifndef H_H\n#define H_H\ntypedef struct handle_s { int fd; } handle_t;\nstatic const char *why = \"public: virtual\";\n#endif\n"
	if err := os.WriteFile(filepath.Join(dir, "handle.h"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "hdr-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	if _, err := pl.Ingest(context.Background(), svc); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]core.SourceKind{}
	for _, o := range obs {
		kinds[o.Symbol] = o.SourceKind
	}
	if kinds["handle_t"] != core.KindCTypedef {
		t.Errorf("handle_t kind = %q, want CTypedef (kinds %v)", kinds["handle_t"], kinds)
	}
}
