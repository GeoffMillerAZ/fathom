package fusion

// bashsource_test.go — LB-Q-bash L3 (wave lf6, item shell): a bash script's
// `source FILE` and `. FILE` with a literal path resolve, by the manifest's
// file-symbol strategy, against the script's directory and then the
// repository root; the script's top level is its module symbol (K18), so a
// resolved source is a module edge and a call into the sourced file binds its
// function. An extensionless script the shebang rule dispatches takes part on
// both sides (O793). A dynamic path ($VAR, $(dirname "$0")/x) resolves nothing.

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_BashSourceResolves(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"bin/tool":               "#!/usr/bin/env bash\n. ../lib/helpers.sh\nsource \"$DIR/ignored.sh\"\n\nmain() {\n  helper_fn\n}\nhelper_fn\n",
		"lib/helpers.sh":         "helper_fn() {\n  echo hi\n}\n",
		"lib/ignored.sh":         "ignored_fn() {\n  :\n}\n",
		"scripts/run.sh":         "#!/bin/sh\nsource libexec/runner\nrun_it\n",
		"libexec/runner":         "#!/bin/bash\nrun_it() {\n  :\n}\n",
		"scripts/nested.sh":      "\\. ./sub/inner.bash\n",
		"scripts/sub/inner.bash": "inner_fn() {\n  :\n}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "sh", Path: dir}); err != nil {
		t.Fatal(err)
	}
	all, err := st.TypedEdges(ctx, "sh")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			got[e.FromSymbol+" -> "+e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"sh::bin/tool.<module> -> sh::lib/helpers.<module>",       // the source statement, from the script's top level
		"sh::bin/tool.main -> sh::lib/helpers.helper_fn",          // a call in a function
		"sh::bin/tool.<module> -> sh::lib/helpers.helper_fn",      // a top-level call
		"sh::scripts/run.<module> -> sh::libexec/runner.<module>", // the repository root, an extensionless target
		"sh::scripts/run.<module> -> sh::libexec/runner.run_it",
		"sh::scripts/nested.<module> -> sh::scripts/sub/inner.<module>", // `\.` against the script's directory
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	for k := range got {
		if strings.Contains(k, "lib/ignored") {
			t.Errorf("a dynamic source path resolved: %s", k)
		}
	}
}
