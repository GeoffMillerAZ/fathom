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
