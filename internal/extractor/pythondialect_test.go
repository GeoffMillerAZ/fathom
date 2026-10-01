package extractor

// pythondialect_test.go — LB-F65. The python dialect left TestTagsDialectParity's
// legacy projection (see migratedTagsDialects) because its call text changed:
// calls.qualify compacts it and drops a callee that ends in no name, and a
// decorator's or a module-level assignment's calls now belong to the definition
// they decorate or define. Its goldens are now the full ones TestDialectGoldens
// checks. This test keeps the guarantee that mattered: every symbol the
// pre-LB-F65 goldens listed (name, kind, first line) is still emitted. The table
// is those goldens' symbol lists as they were at fdd2bb59; a symbol may be added
// to the fixtures' goldens, never removed from here (the Kotlin and Ruby
// precedent).

import (
	"path/filepath"
	"testing"
)

type legacyPythonSymbol struct {
	name, kind string
	line       int
}

var legacyPythonSymbols = map[string][]legacyPythonSymbol{
	"shapes.py": {
		{"LIMIT", "PythonVar", 8},
		{"counter", "PythonVar", 10},
		{"top", "PythonFunc", 13},
		{"fetch", "PythonFunc", 20},
		{"decorated", "PythonFunc", 25},
		{"Base", "PythonClass", 32},
		{"Base.__init__", "PythonFunc", 35},
		{"Base.size", "PythonFunc", 40},
		{"Base.build", "PythonFunc", 44},
		{"Inner", "PythonClass", 47},
		{"Inner.deep", "PythonFunc", 48},
		{"local", "PythonFunc", 49},
		{"Child", "PythonClass", 54},
		{"Child.method", "PythonFunc", 55},
		{"outer", "PythonFunc", 60},
		{"Local", "PythonClass", 61},
		{"Local.m", "PythonFunc", 62},
	},
	"upgrade.py": {
		{"IntOrStr", "PythonTypeAlias", 4},
		{"Model", "PythonClass", 7},
		{"count", "PythonVar", 9},
		{"Model.label", "PythonFunc", 13},
		{"Model.label", "PythonFunc", 17},
		{"Model.handle", "PythonFunc", 24},
	},
	"broken.py": {
		{"good", "PythonFunc", 1},
		{"broken", "PythonFunc", 4},
		{"Survivor", "PythonClass", 7},
		{"Survivor.ok", "PythonFunc", 8},
	},
}

func TestPythonDialectKeepsLegacySymbols(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor(python): %v", err)
	}
	defer e.Close()
	for fixture, want := range legacyPythonSymbols {
		t.Run(fixture, func(t *testing.T) {
			got := extractGolden(t, e, filepath.Join("dialects", "python", "testdata", fixture))
			have := map[legacyPythonSymbol]bool{}
			for _, s := range got.Symbols {
				have[legacyPythonSymbol{s.Name, s.Kind, s.Line}] = true
			}
			for _, w := range want {
				if !have[w] {
					t.Errorf("%s: legacy symbol %s %s at line %d is no longer emitted", fixture, w.kind, w.name, w.line)
				}
			}
		})
	}
}
