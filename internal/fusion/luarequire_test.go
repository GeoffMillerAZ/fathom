package fusion

// luarequire_test.go — LB-Q-lua L3 (wave lang1): a literal require("a.b") resolves to a.b's file by the
// manifest's file-symbol strategy (dots are directory separators, as package.path reads them; the repository
// root, then lua/, lib/ and src/; a/b.lua, then a/b/init.lua), and a call into the required file binds its
// function. A computed module name resolves nothing.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_LuaRequireResolves(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"app/util.lua":      "function trim(s)\n  return s\nend\n",
		"app/sub/init.lua":  "function sub_entry()\nend\n",
		"lua/plug/core.lua": "function plug_core()\nend\n",
		"src/lib/extra.lua": "function extra_fn()\nend\n",
		"app/ignored.lua":   "function ignored_fn()\nend\n",
		"main.lua":          "local util = require(\"app.util\")\nlocal sub = require \"app.sub\"\nlocal ok, plug = pcall(require, \"plug.core\")\nrequire('lib.extra')\nlocal n = \"ignored\"\nlocal dyn = require(\"app.\" .. n)\nfunction main()\n  trim(\"x\")\n  sub_entry()\n  plug_core()\n  extra_fn()\n  ignored_fn()\nend\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "lu", Path: dir}); err != nil {
		t.Fatal(err)
	}
	all, err := st.TypedEdges(ctx, "lu")
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
		"lu::main.main -> lu::app/util.trim",           // require("app.util"): app/util.lua
		"lu::main.main -> lu::app/sub/init.sub_entry",  // require "app.sub": app/sub/init.lua
		"lu::main.main -> lu::lua/plug/core.plug_core", // pcall(require, "plug.core"): lua/ root
		"lu::main.main -> lu::src/lib/extra.extra_fn",  // require('lib.extra'): src/ root
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	for k := range got {
		if k == "lu::main.main -> lu::app/ignored.ignored_fn" || k == "lu::main.<module> -> lu::app/ignored.<module>" {
			t.Errorf("a computed require resolved: %s", k)
		}
	}
}
