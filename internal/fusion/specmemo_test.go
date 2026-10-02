package fusion

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// oneParseIngest ingests the fixture once and returns the wasm parses each
// file-resolve language's extractor ran, the sorted file-resolved edges and
// the sorted file_imports rows.
func oneParseIngest(t *testing.T, dir string, files []string) (map[string]int64, string, string, Report) {
	t.Helper()
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "op", Path: dir}
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	parses := map[string]int64{}
	for _, lang := range []string{"c", "python", "javascript"} {
		ext, done, err := p.fileResolveExtractor(lang)
		if err != nil {
			t.Fatal(err)
		}
		parses[lang] = ext.Parses()
		done()
	}
	all, _ := st.TypedEdges(ctx, "op")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol+"|"+string(e.Kind))
		}
	}
	sort.Strings(edges)
	var rows []string
	var fis store.FileImportsStore = st
	for _, f := range files {
		row, ok, _ := fis.GetFileImports(ctx, "op", f)
		if ok {
			rows = append(rows, f+"|"+row.ContentHash+"|"+row.ImportsJSON+"|"+row.Fault)
		}
	}
	return parses, strings.Join(edges, "\n"), strings.Join(rows, "\n"), rep
}

// TestOneParsePerDispatchedFile (EW-52, O361): an ingest parses each file of
// a tags-dispatched file-resolve language once, not once to extract and once
// for the file-resolved pass, and the file-resolved edges and file_imports
// rows are those the two-parse shape writes.
func TestOneParsePerDispatchedFile(t *testing.T) {
	dir := t.TempDir()
	corpus := map[string]string{
		"c/util.h":       "int helper(int x);\n",
		"c/util.c":       "#include \"util.h\"\nint helper(int x) { return x + 1; }\n",
		"c/main.c":       "#include <stdio.h>\n#include \"util.h\"\nint run(void) { return helper(2); }\n",
		"py/lib.py":      "def helper():\n    return 1\n",
		"py/app.py":      "from py.lib import helper\n\ndef main():\n    return helper()\n",
		"py/__init__.py": "",
	}
	writeCorpus(t, dir, corpus)
	files := make([]string, 0, len(corpus))
	for f := range corpus {
		files = append(files, f)
	}
	sort.Strings(files)

	specMemoOff = true
	t.Cleanup(func() { specMemoOff = false })
	twoParses, twoEdges, twoRows, _ := oneParseIngest(t, dir, files)
	specMemoOff = false
	oneParses, oneEdges, oneRows, rep := oneParseIngest(t, dir, files)

	if twoEdges == "" {
		t.Fatal("the fixture resolved no file-resolved edges; the comparison would prove nothing")
	}
	if oneEdges != twoEdges {
		t.Errorf("file-resolved edges differ\n one %s\n two %s", oneEdges, twoEdges)
	}
	if oneRows != twoRows {
		t.Errorf("file_imports rows differ\n one %s\n two %s", oneRows, twoRows)
	}
	// c: util.h, util.c, main.c; python: lib.py, app.py, __init__.py (extracted
	// and resolved once each).
	if oneParses["c"] != 3 || twoParses["c"] != 6 {
		t.Errorf("c parses: one-parse %d (want 3), two-parse %d (want 6)", oneParses["c"], twoParses["c"])
	}
	if oneParses["python"] != 3 || twoParses["python"] != 6 {
		t.Errorf("python parses: one-parse %d (want 3), two-parse %d (want 6)", oneParses["python"], twoParses["python"])
	}
	if rep.FileResolvedEdges == 0 {
		t.Error("report carries no file-resolved edges")
	}
}
