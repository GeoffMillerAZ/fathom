package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangOracleToolOracleScope (LB-Q-hlsl, H5-R4): a tool oracle that
// reads only part of the measured files (a compiler front end that refuses
// some units) names them in `scope`. Its definitions and calls are graded on
// those files alone, so a file outside is neither a recall nor a precision
// miss, while the run itself (tenancy, file edges) still covers every file,
// and the reading discloses how many files the oracle's scope held: an
// imports oracle's reference pair from b.hlsl, outside the scope, is file-edge
// truth (H6-R3).
func TestCmdLangOracleToolOracleScope(t *testing.T) {
	subject := t.TempDir()
	for name, src := range map[string]string{
		"a.hlsl": "float Helper(float x) { return x; }\nfloat4 Main(float4 c) { return Helper(c.x); }\n",
		"b.hlsl": "float Other(float x) { return Unknown(x); }\n",
	} {
		if err := os.WriteFile(filepath.Join(subject, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dump := filepath.Join(t.TempDir(), "dxc.json")
	if err := os.WriteFile(dump, []byte(`{"tool": "dxc-ast", "version": "test", "tier": "B", "scope": ["a.hlsl"],
 "definitions": [{"path": "a.hlsl", "name": "Helper", "kind": "function", "line": 1, "end": 1}, {"path": "a.hlsl", "name": "Main", "kind": "function", "line": 2, "end": 2}],
 "calls": [{"path": "a.hlsl", "from": "Main", "to": "Helper"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := filepath.Join(t.TempDir(), "refs.json")
	if err := os.WriteFile(refs, []byte(`{"tool": "dxc-ast", "version": "test", "tier": "B",
 "imports": [{"path": "b.hlsl", "line": 1, "to_path": "a.hlsl", "same_package": true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "hlsl.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle([]string{"-dialect", "hlsl", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "none"),
		"-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "hlsl", "dialect.json"),
		"-tool-oracle", dump, "-tool-oracle", refs, "-out", out, "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, out)
	if len(r.Subjects) != 1 || r.Subjects[0].Files != 2 {
		t.Fatalf("subjects = %+v, want one measuring both files", r.Subjects)
	}
	for k, want := range map[string]int{langbench.CntOracleDefs: 2, langbench.CntFathomDefs: 2, langbench.CntMatched: 2, langbench.CntCallTruth: 1, langbench.CntCallFathom: 1} {
		if got := r.Counts[k]; got != want {
			t.Errorf("%s = %d, want %d (b.hlsl is outside the oracle's scope)\n%s", k, got, want, raw)
		}
	}
	if got := r.Counts[langbench.CntFileEdgesTruth]; got != 1 {
		t.Errorf("file_edges_truth = %d, want 1 (b.hlsl -> a.hlsl: the run's file edges cover every file, the scope aside)\n%s", got, raw)
	}
	if !bytes.Contains(raw, []byte("1 of 2 measured files")) {
		t.Errorf("the oracle's scope is not disclosed:\n%s", raw)
	}
}
