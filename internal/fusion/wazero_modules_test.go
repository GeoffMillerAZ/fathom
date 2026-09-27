package fusion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func writeCorpus(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestWazeroModulesStableAcrossIngests is RT-107 (design §7.2): the
// file-resolve pass reuses the pipeline's own tree-sitter extractors instead
// of instantiating two new ones per language per ingest, so the number of
// live wazero modules after three ingests of a JS+Python corpus equals the
// number after the first; TSExtractor.Close releases its modules, and the
// runtime caps each module's memory.
func TestWazeroModulesStableAcrossIngests(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"web/util.js":    "export function helper() { return 1 }\n",
		"web/main.js":    "import { helper } from './util.js'\nexport function run() { return helper() }\n",
		"web/view.ts":    "import { run } from './main'\nexport function view(): number { return run() }\n",
		"py/lib.py":      "def helper():\n    return 1\n",
		"py/app.py":      "from py.lib import helper\n\ndef main():\n    return helper()\n",
		"py/__init__.py": "",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	ctx := context.Background()
	svc := core.ServiceRef{ID: "mods", Path: dir}
	baseline := tsengine.LiveModules() // other tests' pipelines hold theirs

	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FileResolvedEdges == 0 {
		t.Fatal("the fixture resolved no file edges; the pass under test did not run")
	}
	after1 := tsengine.LiveModules()
	if after1 == baseline {
		t.Fatal("no live wazero module counted after an ingest that extracted JS and Python")
	}
	for i := 0; i < 2; i++ {
		// Change every file so extraction and the file-resolve pass both
		// run in full again.
		writeCorpus(t, dir, map[string]string{
			"web/util.js": "export function helper() { return " + string(rune('2'+i)) + " }\n",
			"web/main.js": "import { helper } from './util.js'\nexport function run() { return helper() + " + string(rune('2'+i)) + " }\n",
			"py/app.py":   "from py.lib import helper\n\ndef main():\n    return helper() + " + string(rune('2'+i)) + "\n",
		})
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
	}
	if after3 := tsengine.LiveModules(); after3 != after1 {
		t.Fatalf("live wazero modules %d after the first ingest, %d after the third; the file-resolve pass must reuse the pipeline's extractors", after1, after3)
	}

	// ResolveFileEdges called directly closes the extractor it made.
	before := tsengine.LiveModules()
	files := []core.SourceFile{{Path: "web/util.js", Content: []byte("export function helper() { return 1 }\n"), Lang: "javascript"}}
	ResolveFileEdges(ctx, "javascript", dir, files, nil)
	if got := tsengine.LiveModules(); got != before {
		t.Fatalf("ResolveFileEdges left %d module(s) open", got-before)
	}

	// Close releases an extractor's modules.
	ext, err := extractor.NewTSExtractor("python")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ext.Extract("s", "a.py", []byte("def f():\n    return 1\n")); err != nil {
		t.Fatal(err)
	}
	if tsengine.LiveModules() != before+1 {
		t.Fatalf("an extractor's first parse should hold one module: %d -> %d", before, tsengine.LiveModules())
	}
	ext.Close()
	if got := tsengine.LiveModules(); got != before {
		t.Fatalf("TSExtractor.Close left %d module(s) open", got-before)
	}
	// Pipeline.Close releases the pipeline's.
	p.Close()
	if got, want := tsengine.LiveModules(), before-(after1-baseline); got != want {
		t.Fatalf("after Pipeline.Close %d module(s) live, want %d", got, want)
	}
	if tsengine.MemoryLimitPages != 1024 {
		t.Fatalf("module memory cap %d pages, want 1024 (64 MiB)", tsengine.MemoryLimitPages)
	}
}
