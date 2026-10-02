package fusion_test

// compilecommands_dialect_test.go — LB-Q-compile-commands (design §13): the compile-commands doc dialect is
// reached by ingest. compile_commands.json is claimed by basename (before the generic ".json" extractor, the
// package.json precedent), at any depth, and reads one CompileUnit per database entry whose references are its
// include directories, macros and output; a database that is not an array of entries reads nothing.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_CompileCommandsDialectIsDispatched(t *testing.T) {
	dir := t.TempDir()
	db := `[
  {"directory": "/w/build", "command": "cc -I/w/include -isystem /w/sys -DNDEBUG -o a.o -c /w/src/a.c", "file": "/w/src/a.c"},
  {"directory": "/w/build", "arguments": ["cc", "-Iinc", "-c", "b.c"], "file": "b.c", "output": "b.o"}
]
`
	for rel, src := range map[string]string{
		"compile_commands.json":       db,
		"tools/compile_commands.json": "{\"not\": \"a database\"}\n",
		"package.json":                "{\"name\": \"x\"}\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := connector.LanguageForPath("sub/compile_commands.json"); got != "compile-commands" {
		t.Errorf("LanguageForPath(sub/compile_commands.json) = %q, want compile-commands", got)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "compdb-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, defaultCfg())
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, o := range obs {
		if o.SourceKind == core.KindCompileUnit {
			if o.SourceRef != "compile_commands.json" {
				t.Errorf("CompileUnit %q from %s; the object-literal file must read nothing", o.Symbol, o.SourceRef)
			}
			got[o.Symbol] = o.Calls
		}
	}
	want := map[string][]string{
		"/w/src/a.c": {"-DNDEBUG", "-I/w/include", "-isystem/w/sys", "-oa.o"},
		"b.c":        {"-Iinc", "-ob.o"},
	}
	if len(got) != len(want) {
		t.Fatalf("units %v, want %v", got, want)
	}
	for name, calls := range want {
		if !slices.Equal(got[name], calls) {
			t.Errorf("%s: calls %v, want %v", name, got[name], calls)
		}
	}
}
