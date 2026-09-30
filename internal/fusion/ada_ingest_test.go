package fusion_test

// ada_ingest_test.go — LB-Q-ada: a .ads or .adb file reaches the ada dialect through discovery,
// classification and dispatch (both extensions were disclosed as unsupported and fingerprinted by the
// no-extractor path), and what the dialect extracts reaches the store — a package specification and its
// body, the subprograms of each with a signature, a task and a protected object, an entry, and the calls a
// body makes named by callee. What only the dialect does (a spec and a body kept apart, `is` ending a
// signature) proves the dispatch is the dialect's.

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

var adaIngestFiles = map[string]string{
	"src/counters.ads": `package Counters is
   procedure Bump (N : in out Natural);
   function Twice (N : Natural) return Natural;
   protected type Gate is
      entry Enter;
   end Gate;
end Counters;
`,
	"src/counters.adb": `package body Counters is

   function Twice (N : Natural) return Natural is
   begin
      return N * 2;
   end Twice;

   procedure Bump (N : in out Natural) is
   begin
      N := Twice (N);
   end Bump;

   protected body Gate is
      entry Enter when True is
      begin
         Bump (Count);
      end Enter;
   end Gate;

end Counters;
`,
	"src/notes.txt": "not Ada\n",
}

func TestIngest_AdaReachesTheAdaDialect(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range adaIngestFiles {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"src/counters.ads", "src/counters.adb"} {
		if lang, ok := connector.UnsupportedLanguageForPath(path); ok {
			t.Fatalf("%s is still disclosed as unsupported (%s)", path, lang)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "ada-svc", Path: dir}
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
		got[o.SourceRef+"#"+o.Symbol] = o
	}
	want := map[string]struct {
		kind  core.SourceKind
		sig   string
		calls string
	}{
		"src/counters.ads#Counters":            {"AdaPackageSpec", "", ""},
		"src/counters.ads#Counters.Bump":       {"AdaProcedureSpec", "procedure Bump (N : in out Natural)", ""},
		"src/counters.ads#Counters.Twice":      {"AdaFunctionSpec", "function Twice (N : Natural) return Natural", ""},
		"src/counters.ads#Counters.Gate":       {"AdaProtectedSpec", "", ""},
		"src/counters.ads#Counters.Gate.Enter": {"AdaEntrySpec", "entry Enter", ""},
		"src/counters.adb#Counters":            {"AdaPackage", "", ""},
		"src/counters.adb#Counters.Twice":      {"AdaFunction", "function Twice (N : Natural) return Natural", ""},
		"src/counters.adb#Counters.Bump":       {"AdaProcedure", "procedure Bump (N : in out Natural)", "Twice"},
		"src/counters.adb#Counters.Gate":       {"AdaProtected", "", ""},
		"src/counters.adb#Counters.Gate.Enter": {"AdaEntry", "entry Enter when True", "Bump"},
	}
	for key, w := range want {
		o, ok := got[key]
		if !ok {
			t.Errorf("no observation %q (got %v)", key, sortedKeys(got))
			continue
		}
		if o.SourceKind != w.kind || o.Claim != w.sig || o.EndLine < o.Line || strings.Join(o.Calls, ",") != w.calls {
			t.Errorf("%s: kind %q claim %q calls %q lines %d-%d, want %s %q %q", key, o.SourceKind, o.Claim, strings.Join(o.Calls, ","), o.Line, o.EndLine, w.kind, w.sig, w.calls)
		}
	}
	for key := range got {
		if strings.HasPrefix(key, "src/notes.txt") {
			t.Errorf("unexpected observation %q: a file that is not Ada", key)
		}
	}
}
