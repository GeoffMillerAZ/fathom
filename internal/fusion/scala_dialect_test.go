package fusion

// scala_dialect_test.go — LB-Q-scala: .scala and .sc reach the scala tags dialect through the
// registry the dialect manifests build, once the CGO scalaextract.go is retired (lang-parity MERGE,
// docs/bench/readings/lang/parity/scala.json). What only the dialect does (a member val, a field
// class parameter, a named given, an abstract def, a signature, a callee named by its last
// identifier) proves the dispatch is the dialect's, not the walker's.

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func scalaDialectCorpus() map[string]string {
	return map[string]string{
		"core/src/main/scala/app/Repo.scala": "package app\n\ntrait Repo {\n  def find(id: Int): Option[User]\n  val size: Int\n}\n\n" +
			"case class User(id: Int, name: String)\n\nobject Users extends Repo {\n  val size = 1\n\n" +
			"  def find(id: Int): Option[User] =\n    store.lookup(id).map(User(_, \"x\"))\n}\n",
		"scripts/build.sc": "val root = os.pwd\n\ndef list(dir: os.Path) = os.list(dir)\n",
	}
}

// TestScalaDialectIsWhatFusionDispatches: fusion.New serves .scala and .sc from the scala tags
// dialect, and DispatchTable (the generated truth docs/languages.md reads) says so.
func TestScalaDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for rel, src := range scalaDialectCorpus() {
		e, _, ok, unclassified := p.resolveExtractor(rel, []byte(src), buildSiblingIndex(nil))
		if !ok || unclassified {
			t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
		}
		x, isTS := e.(*extractor.TSExtractor)
		if !isTS || x.Name() != "scala" {
			t.Fatalf("%s dispatches to %T, want the scala tags dialect", rel, e)
		}
	}
	tab := DispatchTable()
	for _, key := range []string{".scala", ".sc"} {
		if got, want := tab[key], "tags dialect `scala`"; got != want {
			t.Errorf("DispatchTable[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestIngest_ScalaDialect (the product-reach proof): an ingest stores the dialect's symbols and calls
// for a .scala and a .sc file.
func TestIngest_ScalaDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, scalaDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "scx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "scx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	const repo = "core/src/main/scala/app/Repo.scala"
	for _, want := range []string{
		repo + "|ScalaTrait|Repo",
		repo + "|ScalaDef|Repo.find",
		repo + "|ScalaVal|size",
		repo + "|ScalaClass|User",
		repo + "|ScalaVal|name",
		repo + "|ScalaObject|Users",
		repo + "|ScalaDef|Users.find",
		"scripts/build.sc|ScalaVal|root",
		"scripts/build.sc|ScalaDef|list",
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
	find := got[repo+"|ScalaDef|Users.find"]
	calls := append([]string(nil), find.Calls...)
	sort.Strings(calls)
	if !slices.Equal(calls, []string{"User", "lookup", "map"}) {
		t.Errorf("Users.find calls = %q, want [User lookup map] (each callee by its last identifier)", calls)
	}
}
