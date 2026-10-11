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

// hostedMixFiles is a component tree's mix (withastro/astro's examples/,
// LB-F550): Astro, Vue and Svelte components whose frontmatter, scripts and
// styles are hosted TypeScript and CSS (F2), beside the .ts, .tsx, .mjs and
// .css files the base already read. Each name sorts so the kinds interleave
// in directory order, the pool's worst case.
var hostedMixFiles = map[string]string{
	"f%03d.astro": "---\nimport Card from \"./g%03d.astro\";\nconst title: string = \"t\";\nfunction pick() { return title }\n---\n<Card title={pick()} />\n<style>.a { color: red }</style>\n<script>function s() { return 1 }</script>\n",
	"g%03d.astro": "---\nconst { title } = Astro.props;\n---\n<h1>{title}</h1>\n",
	"f%03d.ts":    "export function f(n: number) { return n }\nexport let counter = f(1)\n",
	"f%03d.tsx":   "export function View() { return <div>{f(1)}</div> }\nfunction f(n: number) { return n }\n",
	"f%03d.mjs":   "export function g() { return h() }\nfunction h() { return 1 }\n",
	"f%03d.css":   ".a { color: red }\n#b { color: blue }\n",
}

// TestHostedRangesShareTheirDialectsParsers is LB-F550's proof (K26, O982):
// a component host's hosted TypeScript and CSS ranges are read on the parser
// the TypeScript and CSS files themselves lease, so a tree mixing components
// with the files the base read holds one module per grammar variant, under
// MaxLiveModules, and never re-instantiates one per file. Before the fix the
// host leased its ranges under its own keys, seven variants churned the
// six-module pool, and the instantiations grew with the file count.
func TestHostedRangesShareTheirDialectsParsers(t *testing.T) {
	count := func(perKind int) int64 {
		dir := t.TempDir()
		for i := 0; i < perKind; i++ {
			for name, body := range hostedMixFiles {
				if strings.Count(body, "%03d") > 0 {
					body = fmt.Sprintf(body, i)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf(name, i)), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		tsengine.CloseIdleParsers()
		p := fusion.New(connector.NewLocalConnector(), store.NewMemoryStore(), fusion.Config{Concurrency: 2})
		defer p.Close()
		before := tsengine.Instantiations()
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "hosted", Path: dir}); err != nil {
			t.Fatal(err)
		}
		return tsengine.Instantiations() - before
	}
	small, large := count(2), count(24)
	t.Logf("instantiations: 2 files per kind %d, 24 per kind %d", small, large)
	// astro, the TypeScript, TSX and JavaScript variants, CSS: five
	// variants, each instantiated once.
	if large > int64(tsengine.MaxLiveModules) {
		t.Errorf("a mixed component tree instantiated %d grammar modules, want at most %d (one per variant, under the pool's cap)", large, tsengine.MaxLiveModules)
	}
	if large > small {
		t.Errorf("instantiations grew with the file count: %d for 2 files per kind, %d for 24", small, large)
	}
}
