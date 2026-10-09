package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestCmdLangOracleCSharpCompilerSites (LB-F287): lang-oracle reads the C#
// compiler's call sites and kept lines from a -tool-oracle file
// (calibration/oracles/roslyn calls mode) beside a scip-dotnet index: a
// generic method called with explicit type arguments, where scip-dotnet
// writes no occurrence (O271), and a delegate-typed parameter invoked are in
// the call truth; a call only an `#if` branch no compiled configuration keeps
// writes is left out of both counts; each count is disclosed and the compiler
// is named among the reading's oracles at tier A.
func TestCmdLangOracleCSharpCompilerSites(t *testing.T) {
	subject := t.TempDir()
	src := "class C {\n" +
		"  void Run(System.Action<int> cb) {\n" +
		"    Policy.Handle<int>();\n" +
		"    cb(2);\n" +
		"#if NEVER\n" +
		"    Gone();\n" +
		"#endif\n" +
		"  }\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(subject, "C.cs"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	const pre = "scip-dotnet nuget . . App/"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
			{Symbol: pre + "C#", Range: []int32{0, 6, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: pre + "C#Run().", Range: []int32{1, 7, 10}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: pre + "Policy#", Range: []int32{2, 4, 10}},
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "cs.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	sites := filepath.Join(dir, "semcalls.json")
	if err := os.WriteFile(sites, []byte(`{"tool": "roslyn-semantic", "version": "test roslyn", "tier": "A", "query_clauses": 3, "call_sites": [
 {"path": "C.cs", "line": 3, "col": 12, "name": "Handle", "class": "resolved"},
 {"path": "C.cs", "line": 4, "col": 5, "name": "cb", "class": "object"}],
 "active_lines": {"C.cs": [[1, 5], [7, 9]]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "csharp.json")
	args := []string{"-dialect", "csharp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "csharp", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs", "-tool-oracle", sites,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, out)
	if r.Counts[langbench.CntCallFathom] != 2 || r.Counts[langbench.CntCallFathomInTrue] != 2 || r.Counts[langbench.CntCSharpCallsAdded] != 2 || r.Counts[langbench.CntCallsInactive] != 1 {
		t.Errorf("call_fathom %d in truth %d added %d inactive %d, want 2, 2, 2, 1\n%s", r.Counts[langbench.CntCallFathom], r.Counts[langbench.CntCallFathomInTrue],
			r.Counts[langbench.CntCSharpCallsAdded], r.Counts[langbench.CntCallsInactive], rawOut)
	}
	notes := strings.Join(r.Notes, "\n")
	for _, note := range []string{noteCSharpCallsAdded, noteCSharpCallsInactive, noteCSharpQueryClauses} {
		if !strings.Contains(notes, note) {
			t.Errorf("not disclosed: %q\n%s", note, rawOut)
		}
	}
	named := false
	for _, o := range r.Oracles {
		named = named || (o.Tool == "roslyn-semantic" && o.Tier == "A" && o.Version == "test roslyn")
	}
	if !named {
		t.Errorf("the compiler oracle is not named among the reading's oracles: %+v", r.Oracles)
	}
	if strings.Contains(notes, noteCallsInactive) {
		t.Error("a C# reading carries a clang note")
	}
}

// TestCmdLangOracleCSharpCompilerFileEdges (LB-F287, O592): lang-oracle reads
// the C# compiler's cross-file edges from the calls-mode file beside a
// scip-dotnet index. An edge the compiler binds and the index writes no
// occurrence for (`Run<Item>()`: a generic method's type argument, O271)
// joins the file-edge truth, and fathom's resolver agreeing with it counts;
// an edge from a file the index holds nothing of stays out (§1.4); the count
// is disclosed.
func TestCmdLangOracleCSharpCompilerFileEdges(t *testing.T) {
	subject := t.TempDir()
	for name, src := range map[string]string{
		"A.cs":      "namespace App {\n  class A {\n    void Go() { Util.Run<Item>(); }\n  }\n}\n",
		"Item.cs":   "namespace App {\n  class Item { }\n}\n",
		"Util.cs":   "namespace App {\n  static class Util {\n    public static void Run<T>() { }\n  }\n}\n",
		"Linked.cs": "namespace App {\n  class Linked { Item i; }\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(subject, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const pre = "scip-dotnet nuget . . App/"
	def := func(sym string, line int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: pre + sym, Range: []int32{line, 4, 8}, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{
			{RelativePath: "A.cs", Occurrences: []*scippb.Occurrence{def("A#", 1), def("A#Go().", 2),
				{Symbol: pre + "Util#", Range: []int32{2, 16, 20}}, {Symbol: pre + "Util#Run().", Range: []int32{2, 21, 24}}}},
			{RelativePath: "Item.cs", Occurrences: []*scippb.Occurrence{def("Item#", 1)}},
			{RelativePath: "Util.cs", Occurrences: []*scippb.Occurrence{def("Util#", 1), def("Util#Run().", 2)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "cs.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	sites := filepath.Join(dir, "semcalls.json")
	if err := os.WriteFile(sites, []byte(`{"tool": "roslyn-semantic", "version": "test roslyn", "tier": "A", "call_sites": [
 {"path": "A.cs", "line": 3, "col": 22, "name": "Run", "class": "resolved"}],
 "file_edges": [["A.cs", "Item.cs"], ["A.cs", "Util.cs"], ["Linked.cs", "Item.cs"]], "file_edges_multi_declared": 0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "csharp.json")
	args := []string{"-dialect", "csharp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "csharp", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs", "-tool-oracle", sites,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, out)
	if got := r.Counts[langbench.CntCSharpFileEdgesAdded]; got != 1 {
		t.Errorf("%s = %d, want 1 (A.cs -> Item.cs; Linked.cs is no document of the index)\n%s", langbench.CntCSharpFileEdgesAdded, got, rawOut)
	}
	if got := r.Counts[langbench.CntFileEdgesTruth]; got != 2 {
		t.Errorf("file_edges_truth = %d, want 2 (A.cs -> Util.cs, A.cs -> Item.cs)\n%s", got, rawOut)
	}
	if got := r.Counts[langbench.CntFileEdgesAgree]; got != 2 {
		t.Errorf("file_edges_agree = %d, want 2: fathom's resolver binds Util.Run and the type argument Item\n%s", got, rawOut)
	}
	if notes := strings.Join(r.Notes, "\n"); !strings.Contains(notes, noteCSharpFileEdgesAdded) {
		t.Errorf("not disclosed: %q\n%s", noteCSharpFileEdgesAdded, rawOut)
	}
}
