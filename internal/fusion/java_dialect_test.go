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

// javaDialectCorpus is a small Java repository: a class whose method makes a
// constructor call (`new Registry()`, LB-F230, O268: the CGO javaextract.go
// recorded none) and a member call on a receiver, and a class that
// declares a JavaAnnotation (a kind only the dialect emits).
func javaDialectCorpus() map[string]string {
	return map[string]string{
		"src/com/app/Worker.java": "package com.app;\n\nimport java.util.Objects;\n\npublic class Worker {\n    int count;\n\n" +
			"    public int run(Registry reg) {\n        Registry made = new Registry();\n        reg.add(count);\n        made.add(1);\n        return count;\n    }\n}\n",
		"src/com/app/Registry.java": "package com.app;\n\npublic class Registry {\n    public void add(int n) {\n    }\n}\n",
		"src/com/app/Marker.java":   "package com.app;\n\npublic @interface Marker {\n}\n",
	}
}

// TestJavaDialectIsWhatFusionDispatches (D1, LB-F301): lang-parity reads the
// Java tags dialect at least as good as the CGO javaextract.go on guava and
// retrofit once the file resolver lets a single-type import and a file's own
// type shadow its package scope (MERGE,
// docs/bench/readings/lang/parity/java.json, O513), so fusion.New serves
// .java from the dialect, which emits a JavaAnnotation.
func TestJavaDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := javaDialectCorpus()
	rel := "src/com/app/Marker.java"
	e, _, ok, unclassified := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
	}
	if _, isTS := e.(*extractor.TSExtractor); !isTS {
		t.Fatalf("%s dispatches to %T, want the Java tags dialect", rel, e)
	}
	nodes, err := e.Extract("svc", rel, []byte(corpus[rel]))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	have := false
	for _, n := range nodes {
		got = append(got, string(n.Kind)+":"+n.Name)
		have = have || (n.Kind == "JavaAnnotation" && n.Name == "Marker")
	}
	if !have {
		sort.Strings(got)
		t.Errorf("%s: JavaAnnotation:Marker missing from %q (the Java dialect's kinds)", rel, got)
	}
}

// TestIngest_JavaDialect (the product-reach proof, LB-F230, LB-F302): an
// ingest stores the dialect's symbols and calls for a .java file, the
// constructor call (`Registry`) and the member calls among them.
func TestIngest_JavaDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, javaDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "jvx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "jvx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"src/com/app/Worker.java|JavaMethod|Worker.run", "src/com/app/Registry.java|JavaMethod|Registry.add", "src/com/app/Marker.java|JavaAnnotation|Marker"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	run := got["src/com/app/Worker.java|JavaMethod|Worker.run"]
	calls := append([]string(nil), run.Calls...)
	sort.Strings(calls)
	if !strings.Contains(","+strings.Join(calls, ",")+",", ",Registry,") {
		t.Errorf("Worker.run calls = %q, want the constructor call Registry among them", calls)
	}
}
