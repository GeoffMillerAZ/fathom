package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// readUnmatched decodes an -unmatched file strictly.
func readUnmatched(t *testing.T, path string) unmatchedFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var u unmatchedFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, b)
	}
	return u
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// pyCallsFixture is a one-file Python subject and a scip-python index of it
// whose call truth and fathom's pairs differ both ways.
func pyCallsFixture(t *testing.T) (subject, index string) {
	t.Helper()
	subject = t.TempDir()
	src := "class Foo:\n    x = make()\n\n    def bar(self):\n        return 1\n\n\ndef run():\n    f = Foo()\n    g.zap()\n    return f.bar()\n"
	if err := os.WriteFile(filepath.Join(subject, "a.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	const pre = "scip-python python fixture 0.1 a/"
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-python", Version: "0.6.6"}},
		Documents: []*scippb.Document{{RelativePath: "a.py", Occurrences: []*scippb.Occurrence{
			def(pre+"Foo#", []int32{0, 0, 4, 16}, 0, 6, 9),
			ref(pre+"make().", 1, 8, 12),
			def(pre+"Foo#bar().", []int32{3, 4, 4, 16}, 3, 8, 11),
			def(pre+"run().", []int32{7, 0, 10, 18}, 7, 4, 7),
			ref(pre+"Foo#", 8, 8, 11),
			ref(pre+"Foo#bar().", 10, 13, 16),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	index = filepath.Join(t.TempDir(), "python.scip")
	if err := os.WriteFile(index, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return subject, index
}

func hasPair(ps []unmatchedPair, path, caller, callee string) bool {
	for _, p := range ps {
		if p.Path == path && p.Caller == caller && p.Callee == callee {
			return true
		}
	}
	return false
}

// TestCmdLangOracleUnmatchedCallPairs (LB-F64): -unmatched writes, beside
// the L1 misses, the subject's L2 call pairs both ways — the pairs fathom
// found that the calls oracle does not prove (fathom_only) and the pairs the
// oracle proves that fathom missed (oracle_only) — for a SCIP index's call
// truth and for a neutral oracle file's calls alike. The file is diagnosis:
// the reading never carries it.
func TestCmdLangOracleUnmatchedCallPairs(t *testing.T) {
	queue := filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")
	noCtags := filepath.Join(t.TempDir(), "none")

	t.Run("scip", func(t *testing.T) {
		subject, scip := pyCallsFixture(t)
		out := filepath.Join(t.TempDir(), "python.json")
		um := filepath.Join(t.TempDir(), "unmatched.json")
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle([]string{"-dialect", "python", "-subject", subject, "-ctags", noCtags, "-scip", scip,
			"-unmatched", um, "-out", out, "-queue", queue}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		u := readUnmatched(t, um)
		if u.Calls == nil {
			t.Fatal("no calls section for a subject whose calls were scored against a SCIP index")
		}
		if u.Calls.Oracle != "scip-python" {
			t.Errorf("calls oracle = %q, want scip-python", u.Calls.Oracle)
		}
		// g.zap() has a receiver the index does not resolve: fathom-only.
		// Foo's class body calls make(), which the index gives to Foo and
		// fathom to no definition: oracle-only. Foo() and f.bar() are on
		// both sides.
		want := unmatchedCalls{Oracle: "scip-python",
			FathomOnly: []unmatchedPair{{"a.py", "run", "zap"}},
			OracleOnly: []unmatchedPair{{"a.py", "Foo", "make"}}}
		if got, _ := json.Marshal(u.Calls); string(got) != string(must(json.Marshal(want))) {
			t.Errorf("calls = %s, want %s", got, must(json.Marshal(want)))
		}
		// Consistent with the reading's own counts.
		r, raw := readValidReading(t, out)
		if got, want := len(u.Calls.FathomOnly), r.Counts[langbench.CntCallFathom]-r.Counts[langbench.CntCallFathomInTrue]; got != want {
			t.Errorf("fathom_only = %d pairs, the reading's counts say %d", got, want)
		}
		if got, want := len(u.Calls.OracleOnly), r.Counts[langbench.CntCallTruth]-r.Counts[langbench.CntCallTruthFound]; got != want {
			t.Errorf("oracle_only = %d pairs, the reading's counts say %d", got, want)
		}
		if bytes.Contains(raw, []byte("fathom_only")) {
			t.Error("the reading carries the diagnosis")
		}
		// LB-F65: Foo() is the class written as a call, counted and disclosed.
		if r.Counts[langbench.CntScipCalledNames] != 1 || !bytes.Contains(raw, []byte("are written as a call")) {
			t.Errorf("the class written as a call is not counted and disclosed: %d\n%s", r.Counts[langbench.CntScipCalledNames], raw)
		}
		// No L1 oracle ran (ctags absent): the L1 sections are absent.
		if u.DialectOnly != nil || u.OracleOnly != nil {
			t.Errorf("L1 sections written without an L1 oracle: %+v %+v", u.DialectOnly, u.OracleOnly)
		}
	})

	t.Run("tool-oracle", func(t *testing.T) {
		subject := tsFixtureSubject(t)
		oracle := filepath.Join(t.TempDir(), "calls.json")
		if err := os.WriteFile(oracle, []byte(`{"tool": "fake-calls", "version": "1", "tier": "B", "calls": [
 {"path": "greeter.ts", "from": "greet", "to": "add"},
 {"path": "math.ts", "from": "add", "to": "sum"}
]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "typescript.json")
		um := filepath.Join(t.TempDir(), "unmatched.json")
		var stdout, stderr bytes.Buffer
		if code := cmdLangOracle([]string{"-dialect", "typescript", "-subject", subject, "-ctags", noCtags, "-tool-oracle", oracle,
			"-unmatched", um, "-out", out, "-queue", queue}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		u := readUnmatched(t, um)
		if u.Calls == nil || u.Calls.Oracle != "fake-calls" {
			t.Fatalf("calls section %+v, want one from fake-calls", u.Calls)
		}
		if len(u.Calls.OracleOnly) != 1 || !hasPair(u.Calls.OracleOnly, "math.ts", "add", "sum") {
			t.Errorf("oracle_only = %+v, want exactly (math.ts, add, sum)", u.Calls.OracleOnly)
		}
		// greeter.ts's greet also calls console.log: fathom-only, scoped to
		// the files the oracle's calls name (main.ts is not one).
		if !hasPair(u.Calls.FathomOnly, "greeter.ts", "greet", "log") || hasPair(u.Calls.FathomOnly, "greeter.ts", "greet", "add") {
			t.Errorf("fathom_only = %+v, want (greeter.ts, greet, log) and not the found (greet, add)", u.Calls.FathomOnly)
		}
		for _, p := range u.Calls.FathomOnly {
			if p.Path == "main.ts" {
				t.Errorf("fathom_only names %s, a file the oracle's calls do not cover", p.Path)
			}
		}
	})
}

// TestFileResolveLangServesCSharpAndJava (LB-F64): the L3 harness measures
// C# and Java file edges — fusion.ResolveFileEdges serves both through their
// namespace-dir manifests — so a neutral import dump (no SCIP index) gives
// their file_edges truth like every other resolved dialect's.
func TestFileResolveLangServesCSharpAndJava(t *testing.T) {
	for _, tc := range []struct {
		dialect, fixture, dump string
		truth, agree           int
	}{
		// Program.cs's `using U = MyApp.Util;` then U.Helper.Assist().
		{"csharp", "csfixture", `{"path": "Program.cs", "line": 1, "to_path": "MyApp/Util.cs"}`, 1, 1},
		// Main.java's `import com.example.Util;` then Util.helper(3): since
		// LB-F74 the import binds Util and the reference Util.helper reaches
		// Util's member (TestResolveFileEdges_JavaImports), so the edge agrees.
		{"java", "javafixture", `{"path": "com/example/Main.java", "line": 3, "to_path": "com/example/Util.java"}`, 1, 1},
		// Main.kt's `import com.example.util.Helper` and its star import of the
		// package: the PSI dump (O474) writes one entry per file the star import
		// names, and Main.run's three calls (Helper(), trimAll(), helperTop())
		// reach Helper.kt and Strings.kt, the edges the dump's imports truth holds.
		{"kotlin", "ktfixture", `{"path": "com/example/app/Main.kt", "line": 3, "to_path": "com/example/util/Helper.kt"}, {"path": "com/example/app/Main.kt", "line": 4, "to_path": "com/example/util/Helper.kt"}, {"path": "com/example/app/Main.kt", "line": 4, "to_path": "com/example/util/Strings.kt"}`, 2, 2},
	} {
		t.Run(tc.dialect, func(t *testing.T) {
			if _, ok := fileResolveLang[tc.dialect]; !ok {
				t.Fatalf("fileResolveLang lacks %s", tc.dialect)
			}
			subject := copyFixture(t, tc.fixture)
			dump := filepath.Join(t.TempDir(), "imports.json")
			if err := os.WriteFile(dump, []byte(`{"tool": "fake-imports", "version": "1", "tier": "B", "imports": [`+tc.dump+`]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), tc.dialect+".json")
			var stdout, stderr bytes.Buffer
			if code := cmdLangOracle([]string{"-dialect", tc.dialect, "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
				"-tool-oracle", dump, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			r, raw := readValidReading(t, out)
			if r.Counts[langbench.CntFileEdgesTruth] != tc.truth || r.Counts[langbench.CntFileEdgesAgree] != tc.agree {
				t.Errorf("file edges truth %d agree %d, want %d and %d\n%s", r.Counts[langbench.CntFileEdgesTruth], r.Counts[langbench.CntFileEdgesAgree], tc.truth, tc.agree, raw)
			}
			if r.MetricTiers[langbench.MFileEdgeRecall] != "B" {
				t.Errorf("file_edge_recall tier %q, want B (the dump's)", r.MetricTiers[langbench.MFileEdgeRecall])
			}
		})
	}
}

// TestCmdLangOracleFileEdgesScopedToIndexedFiles (LB-F65): an edge from a
// file the SCIP index does not hold (scip-php skips a test file no autoload
// root maps) is out of the file-edge scope, disclosed, never scored as
// fathom's error.
func TestCmdLangOracleFileEdgesScopedToIndexedFiles(t *testing.T) {
	subject := copyFixture(t, "pyfixture")
	const pre = "scip-python python fixture 0.1 `pkg.util`/"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-python", Version: "0.6.6"}},
		Documents: []*scippb.Document{{RelativePath: "pkg/util.py", Occurrences: []*scippb.Occurrence{
			{Symbol: pre + "helper().", Range: []int32{3, 4, 10}, EnclosingRange: []int32{3, 0, 4, 12}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	scip := filepath.Join(t.TempDir(), "python.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "python.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "python", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw2 := readValidReading(t, out)
	// main.py's run -> pkg/util.py's helper and pkg/wrapper.py's wrap ->
	// other: neither main.py nor pkg/wrapper.py is in the index, so neither
	// edge is scored.
	if r.Counts[langbench.CntFileEdgesFathom] != 0 {
		t.Errorf("file_edges_fathom = %d, want 0 (the edges come from a file the index does not hold)\n%s", r.Counts[langbench.CntFileEdgesFathom], raw2)
	}
	if !bytes.Contains(raw2, []byte("out of the file-edge scope")) {
		t.Errorf("the out-of-scope edges are not disclosed:\n%s", raw2)
	}
}

// TestCmdLangOracleUnmatchedFileEdges (LB-F234): beside the L1 and call-pair
// misses, -unmatched writes the file edges fathom drew that the truth lacks
// and the truth edges fathom did not draw, both read from an imports dump
// (LB-F57): the diagnosis a file_edge_precision below its bar needs (O474's
// Kotlin first reading), one (from, to) pair per line of the JSON.
func TestCmdLangOracleUnmatchedFileEdges(t *testing.T) {
	subject := copyFixture(t, "ktfixture")
	dump := filepath.Join(t.TempDir(), "imports.json")
	// Main.run reaches Helper.kt (the named import) and Strings.kt (the star
	// import, left out of this truth); Strings.kt is said to import Helper.kt
	// though no reference of it reaches there.
	if err := os.WriteFile(dump, []byte(`{"tool": "fake-imports", "version": "1", "tier": "B", "imports": [`+
		`{"path": "com/example/app/Main.kt", "line": 3, "to_path": "com/example/util/Helper.kt"},`+
		`{"path": "com/example/util/Strings.kt", "line": 3, "to_path": "com/example/util/Helper.kt"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "kotlin.json")
	um := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "kotlin", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-tool-oracle", dump, "-unmatched", um, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	u := readUnmatched(t, um)
	if u.FileEdges == nil {
		t.Fatalf("no file_edges in -unmatched: %+v", u)
	}
	want := unmatchedFileEdges{
		FathomOnly: [][2]string{{"com/example/app/Main.kt", "com/example/util/Strings.kt"}},
		OracleOnly: [][2]string{{"com/example/util/Strings.kt", "com/example/util/Helper.kt"}},
	}
	if len(u.FileEdges.Via) != 1 || !strings.Contains(u.FileEdges.Via[0][0], "Main.run") || !strings.Contains(u.FileEdges.Via[0][1], "Strings") && !strings.Contains(u.FileEdges.Via[0][1], "trimAll") {
		t.Errorf("fathom_only_via = %v, want Main.run reaching Strings.kt's trimAll", u.FileEdges.Via)
	}
	if len(u.FileEdges.By) != 1 || u.FileEdges.By[0] != "call trimAll" {
		t.Errorf("fathom_only_by = %v, want [call trimAll]", u.FileEdges.By)
	}
	u.FileEdges.Via, u.FileEdges.By = nil, nil
	if !reflect.DeepEqual(*u.FileEdges, want) {
		t.Errorf("file_edges = %+v, want %+v", *u.FileEdges, want)
	}
}

// TestImportMisses (LB-F235): an oracle import is a miss when no fathom import
// statement spanning its line bound the imported file; the miss carries what
// the statements there did bind, and an entry outside the measured files or
// onto its own file is never one.
func TestImportMisses(t *testing.T) {
	sites := []langbench.ImportSite{
		{Path: "a.kt", StartLine: 3, EndLine: 3, Local: "Helper", Targets: []string{"u/Helper.kt"}},
		{Path: "a.kt", StartLine: 4, EndLine: 5, Local: "helperTop", Targets: nil},
	}
	scope := map[string]bool{"a.kt": true, "u/Helper.kt": true, "u/Strings.kt": true}
	entries := []importOracleEntry{
		{Path: "a.kt", Line: 3, ToPath: "u/Helper.kt"},                    // bound
		{Path: "a.kt", Line: 5, ToPath: "u/Strings.kt", Module: "u.trim"}, // inside a two-line statement that bound nothing
		{Path: "a.kt", Line: 9, ToPath: "u/Strings.kt"},                   // no statement of fathom's at that line
		{Path: "a.kt", Line: 3, ToPath: "elsewhere/X.kt"},                 // not a measured file
		{Path: "a.kt", Line: 3, ToPath: "a.kt"},                           // its own file
		{Path: "a.kt", Line: 3, ToPath: ""},                               // no file in the subject
	}
	got := importMisses(entries, sites, scope)
	want := []unmatchedImport{
		{Path: "a.kt", Line: 5, ToPath: "u/Strings.kt", Module: "u.trim", Targets: []string{}},
		{Path: "a.kt", Line: 9, ToPath: "u/Strings.kt", Targets: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("importMisses = %+v, want %+v", got, want)
	}
}
