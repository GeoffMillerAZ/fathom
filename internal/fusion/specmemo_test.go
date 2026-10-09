package fusion

import (
	"context"
	"errors"
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
// Python, PHP since LB-F183, C# since LB-F232, C++ since LB-F117) no longer
// parses once to extract and again for the file-resolved pass, and a CGO
// incumbent's (Java, Rust) file
// specs come from its own native tree, so it is never parsed in wasm at all. The file-resolved
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
	// Wasm parses: C (3 files), Python (3), PHP (2, LB-F183), C# (2,
	// LB-F232) and C++ (3, LB-F117) once each; the incumbents none. c/util.h, a header the router sent to C, is read
	// by the C++ pass only when a C++ include reaches it
	// (foreignreach_test.go): none here does.
	if oneParses != 13 {
		t.Errorf("one-parse ingest ran %d wasm parses, want 13 (one per tags-dispatched file, none for an incumbent, no unreached foreign header)", oneParses)
	}
	// Two-parse shape: those 13 twice and every incumbent file once in wasm for
	// its file specs (4): 13*2 + 4.
	if twoParses != 30 {
		t.Errorf("two-parse ingest ran %d wasm parses, want 30", twoParses)
	}
	if rep.FileResolvedEdges == 0 {
		t.Error("report carries no file-resolved edges")
	}
}

// noFileImportsStore hides the optional FileImportsStore capability: its
// resolve pass reads no stored symbols and extracts each file it reads.
type noFileImportsStore struct{ store.Store }

// TestOneParseWithoutFileImportsStore: on a store without file_imports the
// resolve pass takes a file's symbols from its own extraction, so the
// extract stage's memo must not stand in for that read; the file-resolved
// edges are the two-parse shape's.
func TestOneParseWithoutFileImportsStore(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"c/util.h":       "int helper(int x);\n",
		"c/util.c":       "#include \"util.h\"\nint helper(int x) { return x + 1; }\n",
		"c/main.c":       "#include \"util.h\"\nint run(void) { return helper(2); }\n",
		"py/lib.py":      "def helper():\n    return 1\n",
		"py/app.py":      "from py.lib import helper\n\ndef main():\n    return helper()\n",
		"py/__init__.py": "",
	})
	ingest := func() string {
		t.Helper()
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), noFileImportsStore{st}, Config{})
		defer p.Close()
		ctx := context.Background()
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: "nofi", Path: dir}); err != nil {
			t.Fatal(err)
		}
		all, _ := st.TypedEdges(ctx, "nofi")
		var edges []string
		for _, e := range all {
			if e.Kind == core.EdgeFileResolved {
				edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
			}
		}
		sort.Strings(edges)
		return strings.Join(edges, "\n")
	}
	specMemoOff = true
	t.Cleanup(func() { specMemoOff = false })
	two := ingest()
	specMemoOff = false
	one := ingest()
	if two == "" {
		t.Fatal("the fixture resolved no file-resolved edges; the comparison would prove nothing")
	}
	if one != two {
		t.Errorf("file-resolved edges differ without file_imports\n one %s\n two %s", one, two)
	}
}

// failObservationsStore fails the extract stage's observation write: an
// ingest that stops after the extract stage put specs in the memo and before
// any resolve pass took them.
type failObservationsStore struct{ *store.MemoryStore }

func (failObservationsStore) PutObservations(context.Context, []core.Observation) error {
	return errors.New("observation write refused")
}

// TestSpecMemoEmptyAfterFailedIngest: the memo holds one ingest's specs and
// nothing past it, a failed ingest included, so a daemon that keeps the
// service's sibling index holds no file's specs between ingests.
func TestSpecMemoEmptyAfterFailedIngest(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"c/util.c":  "int helper(int x) { return x + 1; }\n",
		"py/lib.py": "def helper():\n    return 1\n",
	})
	p := New(connector.NewLocalConnector(), failObservationsStore{store.NewMemoryStore()}, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "halt", Path: dir}); err == nil {
		t.Fatal("ingest succeeded; the fixture must stop it at the observation write")
	}
	m := p.specMemoFor("halt")
	if m == nil {
		t.Fatal("no memo for the service: the ingest never built its sibling index")
	}
	m.mu.Lock()
	n := len(m.m)
	m.mu.Unlock()
	if n != 0 {
		t.Errorf("the memo holds %d files' specs after the ingest returned, want 0", n)
	}
}
