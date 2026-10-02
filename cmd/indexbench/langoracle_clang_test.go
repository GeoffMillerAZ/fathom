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

// TestCmdLangOracleClangCallSites (LB-F116): lang-oracle reads the build's
// compiler's call sites from a -tool-oracle file (calibration/oracles/
// clang-calls) beside a scip-clang index: fathom's `buf.size()` on a
// template-dependent receiver has no callee before instantiation and is left
// out of both L2 counts, the count is disclosed, the compiler is named among
// the reading's oracles at tier A, and -unmatched lists the pairs as scored.
func TestCmdLangOracleClangCallSites(t *testing.T) {
	subject := t.TempDir()
	src := "template <typename B> int run(B& buf) {\n" +
		"  int n = buf.size();\n" +
		"  return n + g(1);\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(subject, "m.cc"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "m.cc", Occurrences: []*scippb.Occurrence{
			{Symbol: "cxx . . $ run(0000000000000001).", Range: []int32{0, 26, 29}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: "cxx . . $ g(0000000000000002).", Range: []int32{2, 13, 14}},
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "cpp.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	sites := filepath.Join(dir, "clang-calls.json")
	if err := os.WriteFile(sites, []byte(`{"tool": "clang-ast", "version": "test clang", "tier": "A", "call_sites": [
 {"path": "m.cc", "line": 2, "col": 15, "name": "size", "class": "dependent"},
 {"path": "m.cc", "line": 3, "col": 14, "name": "g", "class": "resolved"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "cpp.json")
	unm := filepath.Join(t.TempDir(), "unmatched.json")
	args := []string{"-dialect", "cpp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "cpp", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-defs", "-tool-oracle", sites, "-unmatched", unm,
		"-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, rawOut := readValidReading(t, out)
	if r.Counts[langbench.CntCallFathom] != 1 || r.Counts[langbench.CntCallFathomInTrue] != 1 || r.Counts[langbench.CntClangCallsDependent] != 1 {
		t.Errorf("call_fathom %d in truth %d dependent %d, want 1, 1, 1\n%s", r.Counts[langbench.CntCallFathom], r.Counts[langbench.CntCallFathomInTrue], r.Counts[langbench.CntClangCallsDependent], rawOut)
	}
	if !bytes.Contains(rawOut, []byte(noteClangDependent)) {
		t.Errorf("the dependent class is not disclosed:\n%s", rawOut)
	}
	named := false
	for _, o := range r.Oracles {
		named = named || (o.Tool == "clang-ast" && o.Tier == "A" && o.Version == "test clang")
	}
	if !named {
		t.Errorf("the compiler oracle is not named among the reading's oracles: %+v", r.Oracles)
	}
	var u unmatchedFile
	b, err := os.ReadFile(unm)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &u); err != nil {
		t.Fatal(err)
	}
	if u.Calls == nil || len(u.Calls.FathomOnly) != 0 {
		t.Errorf("-unmatched fathom_only = %+v, want none (size() is out of both counts)", u.Calls)
	}
}
