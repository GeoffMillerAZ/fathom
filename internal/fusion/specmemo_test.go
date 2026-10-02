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

// oneParseIngest ingests the fixture once and returns the tags-engine parses
// the ingest ran, the sorted file-resolved edges and the sorted file_imports
// rows.
func oneParseIngest(t *testing.T, dir string, files []string) (int64, string, string, Report) {
	t.Helper()
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "op", Path: dir}
	before := extractor.ParseTotal()
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	parses := extractor.ParseTotal() - before
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

// TestOneParsePerDispatchedFile (EW-52, EW-53, O361): an ingest parses each
// file of a file-resolve language once in wasm: a tags-dispatched file (C,
// Python) no longer parses once to extract and again for the file-resolved
// pass, and a CGO incumbent's (C#, Java, Rust, PHP, C++) file specs come from
// its own native tree, so it is never parsed in wasm at all. The file-resolved
// edges and file_imports rows are those the two-parse shape writes.
func TestOneParsePerDispatchedFile(t *testing.T) {
	dir := t.TempDir()
	corpus := map[string]string{
		"c/util.h":       "int helper(int x);\n",
		"c/util.c":       "#include \"util.h\"\nint helper(int x) { return x + 1; }\n",
		"c/main.c":       "#include <stdio.h>\n#include \"util.h\"\nint run(void) { return helper(2); }\n",
		"py/lib.py":      "def helper():\n    return 1\n",
		"py/app.py":      "from py.lib import helper\n\ndef main():\n    return helper()\n",
		"py/__init__.py": "",
		"cs/Lib.cs":      "namespace App.Lib { public class Helper { public static int Run() { return 1; } } }\n",
		"cs/Main.cs":     "using App.Lib;\nnamespace App { public class Main { int Go() { return Helper.Run(); } } }\n",
		"jv/Lib.java":    "package jv;\npublic class Lib { public static int run() { return 1; } }\n",
		"jv/Main.java":   "package jv;\nimport jv.Lib;\npublic class Main { int go() { return Lib.run(); } }\n",
		"rs/lib.rs":      "pub fn helper() -> i32 { 1 }\n",
		"rs/main.rs":     "mod lib;\nuse crate::lib::helper;\nfn run() -> i32 { helper() }\n",
		"ph/Lib.php":     "<?php\nnamespace App;\nclass Lib { public static function run() { return 1; } }\n",
		"ph/Main.php":    "<?php\nnamespace App;\nuse App\\Lib;\nclass Main { function go() { return Lib::run(); } }\n",
		"cx/lib.hpp":     "#pragma once\nint helper(int x);\n",
		"cx/lib.cpp":     "#include \"lib.hpp\"\nint helper(int x) { return x; }\n",
		"cx/main.cpp":    "#include \"lib.hpp\"\nint run() { return helper(1); }\n",
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
	// Wasm parses: C (3 files) and Python (3) once each; the incumbents none;
	// plus c/util.h, which the C++ pass reads as a header the router sent to C
	// (read as foreign, it keeps its own parse).
	if oneParses != 7 {
		t.Errorf("one-parse ingest ran %d wasm parses, want 7 (one per tags-dispatched file, none for an incumbent, one foreign header)", oneParses)
	}
	// Two-parse shape: those 6 twice, every incumbent file once in wasm for
	// its file specs (11), and the foreign header: 6*2 + 11 + 1.
	if twoParses != 24 {
		t.Errorf("two-parse ingest ran %d wasm parses, want 24", twoParses)
	}
	if rep.FileResolvedEdges == 0 {
		t.Error("report carries no file-resolved edges")
	}
}
