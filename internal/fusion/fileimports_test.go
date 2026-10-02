package fusion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func edgeKeys(es []core.Edge) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
	}
	sort.Strings(out)
	return out
}

// TestFileResolve_ReusesImportsAndDisclosesFaults is RT-108 (design §7.2):
// the file-resolve pass reuses a file's recorded imports while its
// fingerprint is unchanged (not read, not parsed again), keeps binary and
// oversize files out of the parse, records a parse fault against the bytes
// so they are not re-parsed yet stay counted and named on every run, takes
// its symbols from the rows the ingest stored (the same edges a fresh
// extraction resolves), drops a deleted file's row, and does not run at all
// when no TS/JS/Python file changed.
func TestFileResolve_ReusesImportsAndDisclosesFaults(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"web/util.js":    "export function helper() { return 1 }\nexport function other() { return 2 }\n",
		"web/main.js":    "import { helper } from './util.js'\nexport function run() { return helper() }\n",
		"web/side.js":    "import { other } from './util.js'\nexport function side() { return other() }\n",
		"web/bad.js":     "import { helper } from './util.js'\nexport function bad() { return helper() }\n",
		"web/blob.js":    "export function x() {}\x00\x00\x00binary",
		"py/lib.py":      "def helper():\n    return 1\n",
		"py/app.py":      "from py.lib import helper\n\ndef main():\n    return helper()\n",
		"py/__init__.py": "",
		"README.md":      "# r\n",
	})
	parsed := map[string]int{}
	prev := extractImports
	extractImports = func(ext extractor.FileSpecExtractor, path string, src []byte) (extractor.FileSpecs, error) {
		parsed[path]++
		if path == "web/bad.js" {
			return extractor.FileSpecs{}, errors.New("tsextract: parse: injected fault")
		}
		return prev(ext, path, src)
	}
	t.Cleanup(func() { extractImports = prev })

	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "fi", Path: dir}
	ingest := func() Report {
		t.Helper()
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}

	rep := ingest()
	if rep.FileResolveFaults != 1 || len(rep.FileResolveFaultFiles) != 1 || rep.FileResolveFaultFiles[0] != "web/bad.js" {
		t.Fatalf("first ingest faults %d %v; want web/bad.js named", rep.FileResolveFaults, rep.FileResolveFaultFiles)
	}
	if parsed["web/blob.js"] != 0 {
		t.Fatal("a binary file reached the import parse")
	}
	if rep.FileResolvedEdges == 0 {
		t.Fatal("no file-resolved edges")
	}
	// The pass's symbols came from the stored rows: the same edges a fresh
	// extraction of the same (parseable) files resolves.
	var files []core.SourceFile
	var symbols []core.SymbolNode
	jsx, _ := extractor.NewTSExtractor("javascript")
	defer jsx.Close()
	for _, rel := range []string{"web/main.js", "web/side.js", "web/util.js"} {
		b, _ := os.ReadFile(filepath.Join(dir, rel))
		files = append(files, core.SourceFile{Path: rel, Content: b, Lang: "javascript"})
		nodes, err := jsx.Extract("fi", rel, b)
		if err != nil {
			t.Fatal(err)
		}
		symbols = append(symbols, nodes...)
	}
	want, _ := ResolveFileEdges(ctx, "javascript", dir, files, symbols)
	all, _ := st.TypedEdges(ctx, "fi")
	var got []core.Edge
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && strings.HasPrefix(e.FromFile, "web/") {
			got = append(got, e)
		}
	}
	if len(want) == 0 {
		t.Fatal("the fixture's JS files resolved no edges; the comparison would prove nothing")
	}
	if g, w := strings.Join(edgeKeys(got), "\n"), strings.Join(edgeKeys(want), "\n"); g != w {
		t.Fatalf("pipeline edges from stored symbols differ from a fresh extraction's:\n got %s\nwant %s", g, w)
	}

	// One JS file changes: only it is parsed again; the others reuse their
	// rows; the fault is not re-parsed but is still disclosed.
	for k := range parsed {
		delete(parsed, k)
	}
	writeCorpus(t, dir, map[string]string{"web/main.js": "import { helper } from './util.js'\nexport function run() { return helper() + 1 }\n"})
	rep = ingest()
	if parsed["web/main.js"] != 1 || len(parsed) != 1 {
		t.Fatalf("second ingest parsed %v; want only web/main.js", parsed)
	}
	if rep.FileResolveReused < 4 {
		t.Fatalf("second ingest reused %d file(s); want the unchanged ones (≥ 4)", rep.FileResolveReused)
	}
	if rep.FileResolveFaults != 1 {
		t.Fatalf("a recorded fault must stay disclosed: faults %d", rep.FileResolveFaults)
	}

	// No TS/JS/Python change: the pass does not run.
	for k := range parsed {
		delete(parsed, k)
	}
	writeCorpus(t, dir, map[string]string{"README.md": "# r2\n"})
	rep = ingest()
	if rep.FileResolveSkipped == "" || len(parsed) != 0 {
		t.Fatalf("README-only change: skipped %q, parsed %v; want skipped, nothing parsed", rep.FileResolveSkipped, parsed)
	}

	// A deleted file's recorded imports go with it.
	if err := os.Remove(filepath.Join(dir, "web/side.js")); err != nil {
		t.Fatal(err)
	}
	ingest()
	if _, ok, _ := st.GetFileImports(ctx, "fi", "web/side.js"); ok {
		t.Fatal("a deleted file's file_imports row survived the ingest")
	}
	if _, ok, _ := st.GetFileImports(ctx, "fi", "web/util.js"); !ok {
		t.Fatal("an unchanged file's file_imports row is missing")
	}
}
