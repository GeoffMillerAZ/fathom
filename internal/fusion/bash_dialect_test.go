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

// bashDialectCorpus is a small shell repository: a .sh script whose function
// calls another and exports a variable (a BashVariable, which only the dialect
// emits), and a .bash library both spellings of a function definition.
func bashDialectCorpus() map[string]string {
	return map[string]string{
		"scripts/build.sh": "#!/usr/bin/env bash\nset -e\nexport BUILD_DIR=out\n\nbuild() {\n  log \"building\"\n  mkdir -p \"$BUILD_DIR\"\n}\n\nbuild\n",
		"lib/log.bash":     "function log {\n  echo \"$1\"\n}\n\nwarn() {\n  log \"warn: $1\"\n}\n",
	}
}

// TestBashDialectIsWhatFusionDispatches (D1, LB-F391): lang-parity reads the
// bash tags dialect at least as good as the CGO bashextract.go on bats-core
// and nvm at its demonstrated level, L2 (MERGE,
// docs/bench/readings/lang/parity/bash.json), so fusion.New serves .sh and
// .bash from the dialect, the manifest loop claiming both keys.
func TestBashDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	corpus := bashDialectCorpus()
	for _, rel := range []string{"scripts/build.sh", "lib/log.bash"} {
		e, _, ok, unclassified := p.resolveExtractor(rel, []byte(corpus[rel]), buildSiblingIndex(nil))
		if !ok || unclassified {
			t.Fatalf("%s: ok=%v unclassified=%v", rel, ok, unclassified)
		}
		ts, isTS := e.(*extractor.TSExtractor)
		if !isTS || ts.Name() != "bash" {
			t.Fatalf("%s dispatches to %T, want the bash tags dialect", rel, e)
		}
	}
	tab := DispatchTable()
	for _, k := range []string{".sh", ".bash"} {
		if got := tab[k]; got != "tags dialect `bash`" {
			t.Errorf("DispatchTable[%q] = %q, want tags dialect `bash`", k, got)
		}
	}
	// A script with no extension is classified by its shebang: the manifest's
	// shebangs are the classifier's, not a dispatch key, and since D4 (O756,
	// shebangdispatch.go) the content-sniff path sends it to the extractor
	// registered for .sh.
	run := []byte("#!/bin/bash\necho hi\n")
	if v := connector.ClassifyDialect(dialectRules(), "bin/run", run); v.Dialect != "bash" || v.By != connector.ClassifiedByShebang {
		t.Errorf("bin/run with a bash shebang classified %+v, want bash by shebang", v)
	}
	if e, _, ok, _ := p.resolveExtractor("bin/run", run, buildSiblingIndex(nil)); !ok || e != p.exts[".sh"] {
		t.Errorf("bin/run dispatches to %T (ok %v), want the extractor registered for .sh", e, ok)
	}
}

// TestIngest_BashDialect (the product-reach proof, LB-F391): an ingest stores
// the dialect's functions, its exported variable and the calls a function
// makes, for a .sh and a .bash file.
func TestIngest_BashDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, bashDialectCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "shx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "shx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"scripts/build.sh|BashFunction|build", "scripts/build.sh|BashVariable|BUILD_DIR", "lib/log.bash|BashFunction|log", "lib/log.bash|BashFunction|warn"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	build := got["scripts/build.sh|BashFunction|build"]
	calls := append([]string(nil), build.Calls...)
	sort.Strings(calls)
	if strings.Join(calls, ",") != "log,mkdir" {
		t.Errorf("build calls = %q, want [log mkdir]", calls)
	}
}
