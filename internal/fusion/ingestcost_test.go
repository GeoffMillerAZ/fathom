package fusion_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ingestCostFiles is one file per wasm-grammar dialect ingest dispatches,
// each written perFile times: the guard's multi-language fixture.
var ingestCostFiles = map[string]string{
	"%d.ts":          "export class C%d { m(): number { return f(1) } }\nexport function f(n: number) { return n }\n",
	"%d.js":          "function g%d() { return h() }\nfunction h() { return 1 }\n",
	"%d.py":          "class K%d:\n    def m(self):\n        return f()\n\ndef f():\n    return 1\n",
	"%d.html":        "<html><body><div id=\"a%d\"></div><script>function s() { return 1 }</script></body></html>\n",
	"%d.css":         ".a%d { color: red }\n#b { color: blue }\n",
	"d%d.dockerfile": "FROM alpine:3 AS base%d\nARG X=1\nFROM base%d\n",
}

// TestIngestCostGuard (coordinator, 2026-09-28, docs/bench/readings/lang/
// footprint.md "Ingest cost after wave 1b"): an ingest compiles and
// instantiates each grammar a bounded number of times — never once per
// file — however many files of its dialect the service holds. The bound is
// machine-independent (module instantiations, not time): the per-worker
// module pool may hold one instance per extraction worker, so the ceiling
// is grammars × concurrency, and it must not grow with the file count.
func TestIngestCostGuard(t *testing.T) {
	const concurrency = 2
	count := func(perFile int) int64 {
		dir := t.TempDir()
		for i := 0; i < perFile; i++ {
			for name, body := range ingestCostFiles {
				n := strings.Count(name, "%d")
				args := make([]any, n)
				for j := range args {
					args[j] = i
				}
				nb := strings.Count(body, "%d")
				bargs := make([]any, nb)
				for j := range bargs {
					bargs[j] = i
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf(name, args...)), []byte(fmt.Sprintf(body, bargs...)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		p := fusion.New(connector.NewLocalConnector(), store.NewMemoryStore(), fusion.Config{Concurrency: concurrency})
		defer p.Close()
		before := tsengine.Instantiations()
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "cost", Path: dir}); err != nil {
			t.Fatal(err)
		}
		return tsengine.Instantiations() - before
	}
	small, large := count(2), count(40)
	t.Logf("instantiations: 2 files/dialect %d, 40 files/dialect %d", small, large)
	grammars := int64(len(ingestCostFiles)) + 1 // html injects javascript; tsx/ts variants share one grammar
	if large > grammars*concurrency {
		t.Errorf("ingest of 40 files per dialect instantiated %d grammar modules, want at most %d (grammars × concurrency)", large, grammars*concurrency)
	}
	if large > small+concurrency {
		t.Errorf("instantiations grew with the file count: %d for 2 files per dialect, %d for 40", small, large)
	}
}
