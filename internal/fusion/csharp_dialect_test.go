package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// csharpDialectCorpus is a small C# repository: a namespaced class with a
// property (the dialect's CSharpProperty, which the CGO csharpextract.go
// never emitted) and a method whose calls exercise the dialect's call rules
// (LB-F189..LB-F191): a generic method called with explicit type arguments,
// nameof, and a member call on a receiver.
func csharpDialectCorpus() map[string]string {
	return map[string]string{
		"src/App/Worker.cs": "using System;\n\nnamespace App.Work\n{\n    public class Worker\n    {\n        public string Name { get; set; }\n\n" +
			"        public int Run(Registry reg)\n        {\n            var n = Make<int>(2);\n            Console.WriteLine(nameof(reg));\n" +
			"            reg.Add(n);\n            return n;\n        }\n\n        private static T Make<T>(T v) => v;\n    }\n}\n",
		"src/App/Registry.cs": "namespace App.Work\n{\n    public class Registry\n    {\n        public void Add(int n) { }\n    }\n}\n",
	}
}

// TestCSharpDialectIsWhatFusionDispatches (D1, LB-F232): lang-parity reads
// the C# tags dialect at least as good as the CGO csharpextract.go on Polly
// and Newtonsoft.Json once the Roslyn dump dates a definition by its name
// (MERGE, docs/bench/readings/lang/parity/csharp.json, O457), so fusion.New
// serves .cs from the dialect, which emits a property.
func TestCSharpDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := csharpDialectCorpus()
	rel := "src/App/Worker.cs"
	e, _, ok, unclassified := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
	if !ok || unclassified {
		t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
	}
	if _, isTS := e.(*extractor.TSExtractor); !isTS {
		t.Fatalf("%s dispatches to %T, want the C# tags dialect", rel, e)
	}
	nodes, err := e.Extract("svc", rel, []byte(corpus[rel]))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	var got []string
	for _, n := range nodes {
		k := string(n.Kind) + ":" + n.Name + " in " + n.Package
		have[k] = true
		got = append(got, k)
	}
	sort.Strings(got)
	for _, want := range []string{"CSharpClass:Worker in App.Work", "CSharpProperty:Worker.Name in App.Work", "CSharpMethod:Worker.Run in App.Work"} {
		if !have[want] {
			t.Errorf("%s: %s missing from %q (the C# dialect's kinds)", rel, want, got)
		}
	}
}

// TestIngest_CSharpDialect (the product-reach proof, LB-F189..LB-F191,
// LB-F233): an ingest stores the dialect's symbols and calls for a .cs file:
// the property the CGO walker never emitted, a generic callee without its
// type arguments (`Make`, never `Make<int>`), no nameof, and a member call
// qualified by its receiver (`reg.Add`).
func TestIngest_CSharpDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, csharpDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "csx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "csx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"src/App/Worker.cs|CSharpProperty|Worker.Name", "src/App/Worker.cs|CSharpMethod|Worker.Run", "src/App/Registry.cs|CSharpMethod|Registry.Add"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	run := got["src/App/Worker.cs|CSharpMethod|Worker.Run"]
	calls := append([]string(nil), run.Calls...)
	sort.Strings(calls)
	if strings.Join(calls, ",") != "Console.WriteLine,Make,reg.Add" {
		t.Errorf("Worker.Run calls = %q, want Console.WriteLine,Make,reg.Add (no type arguments, no nameof, the receiver kept)", calls)
	}
}

// TestIngest_ExtractorEpoch_CSharpDialectDispatchForcesReExtraction pins
// epoch 51 (LB-F232, LB-F233; wave lh's one bump, ADR-048 D2: renumbered at
// its merge to origin/main's 50 + 1): a .cs file fingerprinted by the CGO
// csharpextract.go holds no CSharpProperty and its calls keep their type
// arguments, so the bump "50"→"51" must re-read it through the dialect. The
// seeded fingerprints are salted with "50", the true prior epoch, so the test
// fails if the bump reverts (the XT-5 rule in epoch_test.go).
func TestIngest_ExtractorEpoch_CSharpDialectDispatchForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	corpus := csharpDialectCorpus()
	writeCorpus(t, dir, corpus)
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch51-cs", Path: dir}
	ctx := context.Background()
	for name := range corpus {
		old := sha256.Sum256(append([]byte(corpus[name]), []byte("\x00fathom-extractor-epoch:50")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(old[:])); err != nil {
			t.Fatalf("SetFingerprint %s: %v", name, err)
		}
	}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-51 fingerprint was honored): ExtractorEpoch was not bumped for the C# dispatch switch", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, o := range obs {
		saw = saw || (o.SourceRef == "src/App/Worker.cs" && o.SourceKind == "CSharpProperty" && o.Symbol == "Worker.Name")
	}
	if !saw {
		t.Error("re-extraction did not surface the dialect's CSharpProperty Worker.Name in src/App/Worker.cs")
	}
}
