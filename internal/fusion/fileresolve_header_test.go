package fusion

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestFileResolve_HeadersRouteBySniff is LB-Q-c-rest step 4 (product-reached):
// ingest's file resolver reads a .h as the header router sends it, so a C
// header's own calls draw file edges (a .h was never in the resolver's
// universe), and a header the router sent to C++ is still read by C for the
// includes and prototypes a C file's compiler passes through.
func TestFileResolve_HeadersRouteBySniff(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"util.h":    "int helper(int v);\nstatic inline int twice(int v) { return helper(v) * 2; }\n",
		"helper.c":  "int helper(int v) { return v; }\n",
		"main.c":    "#include \"util.h\"\nint run(void) { return twice(1); }\n",
		"cxx.h":     "#pragma once\nclass Widget { public: int size(); };\nint widget_make(int v);\n",
		"widget.c":  "int widget_make(int v) { return v; }\n",
		"use_cxx.c": "#include \"cxx.h\"\nint use(void) { return widget_make(2); }\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "hdr", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edges, _ := st.TypedEdges(ctx, "hdr")
	fileOf := map[string]string{}
	obs, _ := st.SourceObservations(ctx, "hdr", []string{"util.h", "helper.c", "main.c", "cxx.h", "widget.c", "use_cxx.c"})
	for _, o := range obs {
		s := o.ToSymbolNode()
		fileOf[s.Key()] = s.FilePath
	}
	var got []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+" -> "+fileOf[e.ToSymbol])
		}
	}
	has := func(want string) {
		t.Helper()
		for _, g := range got {
			if g == want {
				return
			}
		}
		t.Errorf("no file edge %q; got %s", want, strings.Join(got, "; "))
	}
	has("main.c -> util.h")      // an include binds a header's inline function (the header is in C's universe)
	has("util.h -> helper.c")    // the header's own call, through its own prototype
	has("use_cxx.c -> widget.c") // the prototype is in a header the router sent to C++
}
