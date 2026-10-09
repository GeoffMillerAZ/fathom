package fusion

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cppDialectCorpus is a small C++ repository: a namespaced class behind an
// export macro (`class GEO_API Shape`, which the CGO cppextract.go misparses),
// a macro and an alias only the dialect emits, a .h header the CT-3 router
// sniffs as C++, and a caller whose calls exercise the dialect's call rules
// (LB-F117, O523): a member call keeps its receiver, a cast and sizeof are no
// call.
func cppDialectCorpus() map[string]string {
	return map[string]string{
		"src/geo/shape.hpp": "#pragma once\n#define GEO_API\n#define GEO_LIMIT 8\nnamespace geo {\nusing size_type = unsigned long;\n" +
			"class GEO_API Shape {\n public:\n  int area() const { return 1; }\n};\nint scale(int v);\n}\n",
		"src/geo/shape.cpp": "#include \"shape.hpp\"\nnamespace geo {\nint scale(int v) { return v * 2; }\n}\n",
		"src/geo/util.h":    "#pragma once\nnamespace geo {\nclass Grid {\n public:\n  int cells() const { return 4; }\n};\n}\n",
		"src/main.cc": "#include \"geo/shape.hpp\"\n#include \"geo/util.h\"\n\nint run(const geo::Shape& s, geo::Grid* g) {\n" +
			"  long n = static_cast<long>(sizeof(s));\n  return geo::scale(s.area()) + g->cells() + int(n);\n}\n",
	}
}

// TestCppDialectIsWhatFusionDispatches (D1, LB-F117): lang-parity reads the
// C++ tags dialect at least as good as the CGO cppextract.go on every metric
// up to its demonstrated level, L3, on fmt, json and spdlog once the
// cpp-macros pre-pass reads decorator macros (MERGE,
// docs/bench/readings/lang/parity/cpp.json, O455), so fusion.New serves
// .cc/.cpp/.cxx/.hpp/.hh/.hxx from the dialect, and the CT-3 header router
// sends a C++ header to it.
func TestCppDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := cppDialectCorpus()
	for _, rel := range []string{"x.cc", "x.cpp", "x.cxx", "x.hpp", "x.hh", "x.hxx"} {
		e, _, ok, unclassified := p.resolveExtractor(rel, []byte("int f() { return 0; }\n"), buildSiblingIndex(nil))
		if !ok || unclassified {
			t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
		}
		if ts, isTS := e.(*extractor.TSExtractor); !isTS || ts.Name() != "cpp" {
			t.Errorf("%s dispatches to %T, want the C++ tags dialect", rel, e)
		}
	}
	h, _, ok, _ := p.resolveExtractor("src/geo/util.h", []byte(corpus["src/geo/util.h"]), buildSiblingIndex(nil))
	if !ok {
		t.Fatal("src/geo/util.h: no extractor")
	}
	router, isRouter := h.(*extractor.CHeaderExtractor)
	if !isRouter {
		t.Fatalf("src/geo/util.h dispatches to %T, want the CT-3 header router", h)
	}
	if ts, isTS := router.Route([]byte(corpus["src/geo/util.h"])).(*extractor.TSExtractor); !isTS || ts.Name() != "cpp" {
		t.Errorf("a C++ .h routes to %T, want the C++ tags dialect", router.Route([]byte(corpus["src/geo/util.h"])))
	}
	if ts, isTS := router.Route([]byte("int f(void);\n")).(*extractor.TSExtractor); !isTS || ts.Name() != "c" {
		t.Errorf("a C .h routes to %T, want the C tags dialect", router.Route([]byte("int f(void);\n")))
	}
}

// TestIngest_CppDialect (the product-reach proof, LB-F117, O455, O523): an
// ingest stores the dialect's symbols and calls: the class behind an export
// macro under its namespace, the macro and the alias the CGO walker never
// emitted, a C++ .h header's class, and run's calls with the receivers kept
// and no call for the cast or sizeof.
func TestIngest_CppDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cppDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cppx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cppx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{
		"src/geo/shape.hpp|CppClass|geo.Shape", "src/geo/shape.hpp|CppFunc|geo.Shape.area",
		"src/geo/shape.hpp|CppMacro|GEO_LIMIT", "src/geo/shape.hpp|CppTypedef|geo.size_type",
		"src/geo/shape.cpp|CppFunc|geo.scale", "src/geo/util.h|CppClass|geo.Grid", "src/main.cc|CppFunc|run",
	} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	calls := append([]string(nil), got["src/main.cc|CppFunc|run"].Calls...)
	sort.Strings(calls)
	if strings.Join(calls, ",") != "g.cells,geo.scale,s.area" {
		t.Errorf("run calls = %q, want g.cells,geo.scale,s.area (receivers kept, no cast, no sizeof)", calls)
	}
}
