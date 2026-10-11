package fusion

// scriptcalls_test.go — owner ruling S1's lever (wave lf7, item shell,
// LB-F520): a bash script's top-level call (a call its <module> symbol makes)
// is stored only when its callee is a definition of the repository — in the
// script, in a file it sources, or anywhere else the service defines it;
// echo, grep and every other external command carry no navigation value and
// are not stored. A call inside a function, and every other dialect's module
// calls, are unchanged.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_BashModuleCallsBind(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"bin/tool":       "#!/usr/bin/env bash\n. ../lib/helpers.sh\nlocal_fn() {\n  echo in\n  helper_fn\n}\nhelper_fn\nlocal_fn\nlater_fn\necho done\ngrep -q x y\n",
		"lib/helpers.sh": "helper_fn() {\n  echo hi\n}\nhelper_two() {\n  :\n}\n",
		"z/later.sh":     "later_fn() {\n  :\n}\nprintf '%s' x\n",
		"app/main.py":    "print('x')\n",
	})
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	ingest := func() map[string]core.Observation {
		t.Helper()
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "sh", Path: dir}); err != nil {
			t.Fatal(err)
		}
		obs, err := st.QueryObservations(ctx, "sh", "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]core.Observation{}
		for _, o := range obs {
			out[o.SourceRef+"#"+o.Symbol] = o
		}
		return out
	}
	calls := func(o core.Observation) []string {
		c := slices.Clone(o.Calls)
		slices.Sort(c)
		return c
	}

	got := ingest()
	for key, want := range map[string][]string{
		"bin/tool#<module>":    {"helper_fn", "later_fn", "local_fn"}, // sourced, defined elsewhere in the repository, in the script; echo, grep and `.` gone
		"bin/tool#local_fn":    {"echo", "helper_fn"},                 // a function's calls stay as extracted
		"z/later.sh#<module>":  nil,                                   // printf: an external command
		"app/main.py#<module>": {"print"},                             // python's module calls are not in the lever's scope
	} {
		o, ok := got[key]
		if !ok {
			t.Fatalf("no observation %s; have %d", key, len(got))
		}
		if c := calls(o); !slices.Equal(c, want) {
			t.Errorf("%s calls = %q, want %q", key, c, want)
		}
	}
	// The stored call index holds the same rows: no `echo` row from a module symbol.
	callers, indexed, err := st.ObservationsCalling(ctx, "sh", "echo")
	if err != nil || !indexed {
		t.Fatalf("ObservationsCalling: indexed %v, err %v", indexed, err)
	}
	for _, o := range callers {
		if o.Symbol == core.ModuleSymbolName {
			t.Errorf("a module symbol's echo row was stored: %s", o.SourceRef)
		}
	}
	// A held script is fingerprinted after its rows are written.
	if _, found, err := st.Fingerprint(ctx, "sh", "bin/tool"); err != nil || !found {
		t.Errorf("bin/tool fingerprint found %v, err %v", found, err)
	}
	// The top-level call into the sourced file still binds (file-resolved edge).
	edges, err := st.TypedEdges(ctx, "sh")
	if err != nil {
		t.Fatal(err)
	}
	var bound bool
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromSymbol == "sh::bin/tool.<module>" && e.ToSymbol == "sh::lib/helpers.helper_fn" {
			bound = true
		}
	}
	if !bound {
		t.Error("missing file-resolved edge sh::bin/tool.<module> -> sh::lib/helpers.helper_fn")
	}

	// An incremental ingest reads the definitions of the files it does not
	// re-read from the store: lib/helpers.sh is unchanged, its helper_two
	// still keeps the edited script's new top-level call.
	if err := os.WriteFile(filepath.Join(dir, "bin/tool"), []byte("#!/usr/bin/env bash\n. ../lib/helpers.sh\nhelper_two\nls -la\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = ingest()
	if c := calls(got["bin/tool#<module>"]); !slices.Equal(c, []string{"helper_two"}) {
		t.Errorf("after the edit, bin/tool <module> calls = %q, want [helper_two]", c)
	}
}
