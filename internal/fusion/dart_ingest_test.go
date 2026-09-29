package fusion_test

// dart_ingest_test.go — LB-Q-dart: a .dart file reaches the dart dialect through discovery and
// dispatch (it was "no extractor registered"), and what the dialect extracts reaches the store — a
// class, its constructor, field, accessor and method, an enum and its constant, a top-level function
// and variable, calls named by callee, and an import resolved through pubspec.yaml's package name to a
// file-tier edge.

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

func TestIngest_DartReachesTheDartDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"pubspec.yaml": "name: acme\nenvironment:\n  sdk: ^3.0.0\n",
		"lib/main.dart": `import 'package:acme/src/model.dart';

final registry = Model.build();

void run() {
  final m = Model('x');
  m.save();
  print(describe(m));
}
`,
		"lib/src/model.dart": `enum Kind { small, large }

class Model {
  Model(this.name);

  Model.build() : name = 'built';

  final String name;

  String get label => name;

  void save() {
    _log();
  }

  void _log() {}
}

String describe(Model m) => m.label;
`,
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if lang, ok := connector.UnsupportedLanguageForPath("lib/main.dart"); ok {
		t.Fatalf("a .dart file is still disclosed as unsupported (%s)", lang)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "dart-svc", Path: dir}
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
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.Symbol] = o
	}
	for symbol, kind := range map[string]core.SourceKind{
		"registry":    "DartVariable",
		"run":         "DartFunction",
		"Kind":        "DartEnum",
		"Kind.small":  "DartEnumConstant",
		"Model":       "DartClass",
		"Model.Model": "DartConstructor",
		"Model.build": "DartConstructor",
		"Model.name":  "DartField",
		"Model.label": "DartAccessor",
		"Model.save":  "DartMethod",
		"Model._log":  "DartMethod",
		"describe":    "DartFunction",
	} {
		o, ok := got[symbol]
		if !ok {
			t.Errorf("no observation for %q (got %v)", symbol, keysOf(got))
			continue
		}
		if o.SourceKind != kind {
			t.Errorf("%s: kind %q, want %q", symbol, o.SourceKind, kind)
		}
	}
	for symbol, want := range map[string]string{
		"run":        "Model,describe,print,save",
		"registry":   "build",
		"Model.save": "_log",
	} {
		if calls := strings.Join(got[symbol].Calls, ","); calls != want {
			t.Errorf("%s calls = %q, want %q", symbol, calls, want)
		}
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "lib/main.dart" && strings.HasSuffix(e.ToSymbol, "describe") {
			found = true
		}
	}
	if !found {
		t.Errorf("no file-resolved edge from lib/main.dart to describe (import 'package:acme/src/model.dart'); edges %+v", edges)
	}
}
