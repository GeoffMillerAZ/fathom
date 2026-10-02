package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestExtractLocationState: clang's JSON repeats a file or line only where it
// changes, so a definition's file and lines come from the whole tree's print
// order, a macro-expanded head from its expansion location, and a declaration
// outside the subject is left out.
func TestExtractLocationState(t *testing.T) {
	dump := `{"kind":"TranslationUnitDecl","inner":[
	 {"kind":"FunctionDecl","name":"sys","loc":{"file":"/sdk/stdio.h","line":9},"range":{"begin":{"line":9},"end":{"line":9}},"inner":[{"kind":"CompoundStmt"}]},
	 {"kind":"FunctionDecl","name":"helper","loc":{"file":"/s/a.c","line":7},"range":{"begin":{"col":1},"end":{"line":9}},"inner":[{"kind":"ParmVarDecl","loc":{"line":7}},{"kind":"CompoundStmt","range":{"begin":{"line":7},"end":{"line":9}}}]},
	 {"kind":"FunctionDecl","name":"wrapped","loc":{"spellingLoc":{"file":"/s/a.h","line":2},"expansionLoc":{"file":"/s/a.c","line":11}},"range":{"begin":{"spellingLoc":{"line":2},"expansionLoc":{"line":11}},"end":{"line":11}},"inner":[{"kind":"CompoundStmt"}]},
	 {"kind":"FunctionDecl","name":"proto","loc":{"line":13},"range":{"begin":{"line":13},"end":{"line":13}}},
	 {"kind":"RecordDecl","name":"Pt","tagUsed":"struct","completeDefinition":true,"loc":{"line":15},"range":{"begin":{"line":15},"end":{"line":18}}},
	 {"kind":"RecordDecl","name":"Fwd","tagUsed":"struct","loc":{"line":19},"range":{"begin":{"line":19},"end":{"line":19}}},
	 {"kind":"EnumDecl","name":"Color","loc":{"line":20},"range":{"begin":{"line":20},"end":{"line":22}},"inner":[{"kind":"EnumConstantDecl"}]},
	 {"kind":"TypedefDecl","name":"PtAlias","loc":{"line":24},"range":{"begin":{"line":24},"end":{"line":24}}}
	]}`
	got, err := Extract(strings.NewReader(dump), "/s")
	if err != nil {
		t.Fatal(err)
	}
	want := []Def{
		{"a.c", "helper", "function", 7, 9},
		{"a.c", "wrapped", "function", 11, 11},
		{"a.c", "Pt", "struct", 15, 18},
		{"a.c", "Color", "enum", 20, 22},
		{"a.c", "PtAlias", "type_alias", 24, 24},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Extract = %v\nwant %v", got, want)
	}
}

func TestClangArgs(t *testing.T) {
	got := clangArgs(entry{Command: `cc -DX=1 -I inc -MD -MT a.o -MF a.d -o a.o -c /s/a.c`})
	want := []string{"-DX=1", "-I", "inc", "/s/a.c", "-fsyntax-only", "-w", "-Xclang", "-ast-dump=json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clangArgs = %v, want %v", got, want)
	}
}

// TestClangEndToEnd runs the real compiler over a small file when one is
// installed.
func TestClangEndToEnd(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("no clang")
	}
	dir := t.TempDir()
	src := "typedef struct Pt { int x;\n int y; } Pt;\nenum Color { RED,\n GREEN };\nstatic int helper(int a) {\n  return a;\n}\n#define WRAP(x) int x(void)\nWRAP(wrapped) { return 1; }\nint main(void)\n{\n  return helper(1);\n}\n"
	file := filepath.Join(dir, "t.c")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	got, err := dump(clang, entry{Directory: dir, Command: "cc -c " + file, File: file}, real)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"Pt": {1, 2}, "Color": {3, 4}, "helper": {5, 7}, "wrapped": {9, 9}, "main": {10, 13}}
	seen := map[string][2]int{}
	for _, d := range got {
		if d.Kind != "type_alias" || d.Name != "Pt" { // Pt is a record and a typedef over one range
			seen[d.Name] = [2]int{d.Line, d.End}
		}
	}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("definitions = %v, want %v", seen, want)
	}
}
