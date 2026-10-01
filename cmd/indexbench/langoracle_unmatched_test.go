package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	src := "class Foo:\n    def bar(self):\n        return 1\n\n    def baz(self):\n        return 2\n\n\ndef run():\n    f = Foo()\n    h = f.baz\n    return f.bar()\n"
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
			def(pre+"Foo#", []int32{0, 0, 5, 16}, 0, 6, 9),
			def(pre+"Foo#bar().", []int32{1, 4, 2, 16}, 1, 8, 11),
			def(pre+"Foo#baz().", []int32{4, 4, 5, 16}, 4, 8, 11),
			def(pre+"run().", []int32{8, 0, 11, 18}, 8, 4, 7),
			ref(pre+"Foo#", 9, 8, 11),
			ref(pre+"Foo#baz().", 10, 10, 13),
			ref(pre+"Foo#bar().", 11, 13, 16),
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
		// run() writes Foo(): the index names the class (Foo#), not a
		// callable, so fathom's constructor call is fathom-only; `h = f.baz`
		// references the method without calling it, which the index counts
		// and fathom does not; f.bar() is on both sides.
		want := unmatchedCalls{Oracle: "scip-python",
			FathomOnly: []unmatchedPair{{"a.py", "run", "Foo"}},
			OracleOnly: []unmatchedPair{{"a.py", "run", "baz"}}}
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
		// Main.java's `import com.example.Util;` then Util.helper(3): measured,
		// though the resolver binds no call through a single-type import yet
		// (TestResolveFileEdges_JavaImports), so nothing agrees.
		{"java", "javafixture", `{"path": "com/example/Main.java", "line": 3, "to_path": "com/example/Util.java"}`, 1, 0},
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
