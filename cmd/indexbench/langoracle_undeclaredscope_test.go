package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// G-R7: a producer without declared scope retains its pre-scope contract.
// Native SQL supplies neutral calls; CUDA's compiler companions supply
// call_sites or active_lines and must not create a second neutral calls family.
func TestScoreToolOracleUndeclaredScopeNoOp(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "dialect.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := langbench.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Name: "caller", FilePath: "a.ts", Calls: []string{"callee"}},
		{Name: "ungraded", FilePath: "b.ts", Calls: []string{"extra"}},
	}
	for _, tc := range []struct {
		name, oracle string
		seeded       bool
	}{
		{"native SQL nonempty calls", `{"tool":"sql-native","tier":"A","calls":[{"path":"a.ts","from":"caller","to":"callee"}]}`, false},
		{"native SQL empty calls", `{"tool":"sql-native","tier":"A","calls":[]}`, true},
		{"CUDA compiler call sites", `{"tool":"clang-ast","tier":"B","call_sites":[{"path":"a.ts","line":1,"col":1,"name":"callee","class":"resolved"}]}`, true},
		{"CUDA preprocessor", `{"tool":"clang-preprocessor","tier":"A","active_lines":{"a.ts":[[1,1]]}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oracle := filepath.Join(t.TempDir(), "oracle.json")
			if err := os.WriteFile(oracle, []byte(tc.oracle), 0o644); err != nil {
				t.Fatal(err)
			}
			run := func(extraFile bool) (langbench.Subject, *callDiff) {
				t.Helper()
				sub := langbench.Subject{Counts: map[string]int{}, Families: map[string]string{}}
				if tc.seeded {
					sub.Counts[langbench.CntCallTruth] = 7
					sub.Counts[langbench.CntCallFathom] = 9
					sub.Families[langbench.FamilyCalls] = "B"
				}
				measured := map[string]bool{"a.ts": true}
				input := nodes[:1]
				if extraFile {
					measured["b.ts"] = true
					input = nodes
				}
				_, dropped, _, diff, err := scoreToolOracle(oracle, m, input, &sub, measured)
				if err != nil || dropped != 0 {
					t.Fatalf("unscoped producer: dropped=%d err=%v", dropped, err)
				}
				return sub, diff
			}
			before, beforeDiff := run(false)
			after, afterDiff := run(true)
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(beforeDiff, afterDiff) {
				t.Fatalf("undeclared scope expanded coverage: before=%+v after=%+v", before, after)
			}
			if tc.seeded {
				if afterDiff != nil || after.Counts[langbench.CntCallTruth] != 7 || after.Counts[langbench.CntCallFathom] != 9 || after.Families[langbench.FamilyCalls] != "B" {
					t.Fatalf("unscoped companion changed existing calls family: %+v diff=%+v", after, afterDiff)
				}
			} else if afterDiff == nil || after.Counts[langbench.CntCallTruth] != 1 || after.Counts[langbench.CntCallTruthFound] != 1 || after.Counts[langbench.CntCallFathom] != 1 || after.Counts[langbench.CntCallFathomInTrue] != 1 {
				t.Fatalf("legacy call coverage changed: %+v diff=%+v", after, afterDiff)
			}
		})
	}
}
