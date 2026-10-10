package main

// selectts_module_test.go — LB-F412 (wave lf4, owner ruling D1, O748): with
// the module-to-module edge a binding import duplicates left out (LB-F411),
// the graph `fathom tests select` walks (callgraph.BuildBoth over the stored
// symbols and typed edges) still reaches a TypeScript test file's module
// symbol, through the calls its describe/it callbacks write, from the
// function they call. tests select itself names a test by a Test* symbol
// only, so it selects no JS/TS test file (O786); this pins the reach a
// selection of one would walk.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestTestsSelectGraphReachesTSTestFileThroughModuleCalls(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", "{\"name\": \"lever\", \"version\": \"1.0.0\"}\n")
	write("src/price.ts", "export function total(n: number) {\n  return n * 2;\n}\n")
	write("src/other.ts", "export function unrelated() {\n  return 0;\n}\n")
	write("src/price.test.ts", "import { total } from \"./price\";\n\ndescribe(\"total\", () => {\n  it(\"doubles\", () => {\n    expect(total(2)).toBe(4);\n  });\n});\n")
	write("src/other.spec.ts", "import { unrelated } from \"./other\";\n\ndescribe(\"unrelated\", () => {\n  it(\"is zero\", () => {\n    expect(unrelated()).toBe(0);\n  });\n});\n")

	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "lever-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()
	nodes, err := c.ListAllSymbols(ctx, "lever-svc")
	if err != nil {
		t.Fatal(err)
	}
	typed, err := c.TypedEdges(ctx, "lever-svc")
	if err != nil {
		t.Fatal(err)
	}
	key := func(pkg, name string) string { return core.SymbolKey("lever-svc", pkg, name) }
	for _, e := range typed {
		if e.FromSymbol == key("src/price.test", core.ModuleSymbolName) && e.ToSymbol == key("src/price", core.ModuleSymbolName) {
			t.Errorf("the test file's binding import of src/price kept its module edge (LB-F411)")
		}
	}
	g := callgraph.BuildBoth(nodes, typed)
	if !callgraph.Reachable(g, key("src/price.test", core.ModuleSymbolName), key("src/price", "total")) {
		var calls []string
		for _, n := range nodes {
			if n.FilePath == "src/price.test.ts" {
				calls = append(calls, n.Name+" -> "+strings.Join(n.Calls, ","))
			}
		}
		t.Errorf("tests select's graph does not reach src/price.test.ts's module from total; its rows: %v", calls)
	}
	if callgraph.Reachable(g, key("src/other.spec", core.ModuleSymbolName), key("src/price", "total")) {
		t.Errorf("a test file calling only another module reaches total")
	}
}
