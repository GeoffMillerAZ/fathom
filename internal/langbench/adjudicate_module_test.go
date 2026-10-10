package langbench

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestAdjudicateModuleSymbolEnclosesNothing (K18, LB-F381): a file's module
// symbol spans every line of its file, but it is the file, not a definition
// around one: D3's module-scope check ("no definition fathom emitted encloses
// it") reads every other definition and never the module symbol, so a
// top-level definition of a dialect that mints one is admitted as before.
func TestAdjudicateModuleSymbolEnclosesNothing(t *testing.T) {
	m, err := ParseManifest([]byte(`{"id":"typescript","kind":"language",
		"kinds":{"function":{"source_kind":"JSFunc","canonical":"function","callable":true},
		         "module":{"source_kind":"JSModule","canonical":"module"}},
		"required_kinds":["function"],
		"oracle_kinds":{"tool":{"fn":"function"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	src := "import { g } from \"./g\";\n\nexport function run(): number {\n  return g();\n}\n"
	nodes := []core.SymbolNode{
		{Name: core.ModuleSymbolName, Kind: "JSModule", FilePath: "main.ts", Line: 1, EndLine: 5},
		{Name: "run", Kind: "JSFunc", FilePath: "main.ts", Line: 3, EndLine: 5, Signature: "function run(): number"},
	}
	l1 := ScoreL1(m, "tool", nodes, nil)
	adj := Adjudication{
		Classes:     map[string]AdjudicationClass{"exported-fn": {Why: "the dumper skips exported functions", Pattern: `function\s+{name}\b`, Scope: "module"}},
		Definitions: []AdjudicatedDef{{Path: "main.ts", Line: 3, Name: "run", Class: "exported-fn"}},
	}
	accepted, refused := Adjudicate(&l1, nodes, adj, func(string) []byte { return []byte(src) })
	if !slices.Equal(accepted, []string{"main.ts:3 run (exported-fn)"}) || len(refused) != 0 {
		t.Errorf("accepted %v, refused %v; want run admitted (its module symbol encloses no definition)", accepted, refused)
	}
}
