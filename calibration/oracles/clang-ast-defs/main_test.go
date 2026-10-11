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

// TestExtractInternalLinkage (owner ruling M3, O863): the file-scope functions and
// objects a translation unit defines with internal linkage, by C11 6.2.2: a file-scope
// declaration with `static` gives its identifier internal linkage (p3), and a later
// declaration of it with no storage class or `extern` keeps the linkage the prior one
// gave (p4, p5), so a definition written without `static` after a `static` prototype is
// internal. Each is placed at its name (a macro-written one at its expansion), in the
// subject only; an external definition, an `extern` declaration and a block-scope static
// (no linkage, p6) are none.
func TestExtractInternalLinkage(t *testing.T) {
	dump := `{"kind":"TranslationUnitDecl","inner":[
	 {"kind":"FunctionDecl","name":"sys_cb","storageClass":"static","loc":{"file":"/sdk/x.h","line":4},"range":{"begin":{"line":4},"end":{"line":4}},"inner":[{"kind":"CompoundStmt"}]},
	 {"kind":"FunctionDecl","name":"h_inline","storageClass":"static","inline":true,"loc":{"file":"/s/h.h","line":2},"range":{"begin":{"line":2},"end":{"line":2}},"inner":[{"kind":"CompoundStmt"}]},
	 {"kind":"FunctionDecl","name":"close_cb","storageClass":"static","loc":{"file":"/s/a.c","line":3},"range":{"begin":{"line":3},"end":{"line":3}}},
	 {"kind":"VarDecl","name":"called","storageClass":"static","loc":{"line":5},"range":{"begin":{"line":5},"end":{"line":5}}},
	 {"kind":"VarDecl","name":"counter","loc":{"line":6},"range":{"begin":{"line":6},"end":{"line":6}}},
	 {"kind":"VarDecl","name":"ext","storageClass":"extern","loc":{"line":7},"range":{"begin":{"line":7},"end":{"line":7}}},
	 {"kind":"FunctionDecl","name":"close_cb","loc":{"line":10},"range":{"begin":{"line":9},"end":{"line":12}},"inner":[{"kind":"CompoundStmt","inner":[
	   {"kind":"DeclStmt","inner":[{"kind":"VarDecl","name":"seen","storageClass":"static","loc":{"line":11},"range":{"begin":{"line":11},"end":{"line":11}}}]}]}]},
	 {"kind":"FunctionDecl","name":"run_test_x","storageClass":"static","loc":{"spellingLoc":{"file":"/s/t.h","line":8},"expansionLoc":{"file":"/s/a.c","line":14}},"range":{"begin":{"line":14},"end":{"line":16}},"inner":[{"kind":"CompoundStmt"}]},
	 {"kind":"FunctionDecl","name":"main","loc":{"line":18},"range":{"begin":{"line":18},"end":{"line":20}},"inner":[{"kind":"CompoundStmt"}]}
	]}`
	u, err := ExtractUnit(strings.NewReader(dump), "/s")
	if err != nil {
		t.Fatal(err)
	}
	want := []Linked{
		{"a.c", "called", 5},
		{"a.c", "close_cb", 10},
		{"a.c", "run_test_x", 14},
		{"h.h", "h_inline", 2},
	}
	if !reflect.DeepEqual(u.Internal, want) {
		t.Errorf("internal = %v\nwant %v", u.Internal, want)
	}
	if defs, err := Extract(strings.NewReader(dump), "/s"); err != nil || len(defs) != len(u.Defs) {
		t.Errorf("Extract = %v, %v; want the unit's %d definitions", defs, err, len(u.Defs))
	}
}

// TestClangEndToEndInternalLinkage runs the real compiler: the AST, never the written
// text, says which definitions are internal (a prototype's `static` reaches a definition
// written without it; a `static` a macro writes is the compiler's too).
func TestClangEndToEndInternalLinkage(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("no clang")
	}
	dir := t.TempDir()
	hdr := "static inline int h_inline(void) { return 1; }\n"
	src := "#include \"h.h\"\n#define PRIVATE static\nstatic void close_cb(int);\nvoid close_cb(int x) { (void)x; }\nPRIVATE int called;\nint counter;\nextern int ext;\nint main(void)\n{\n  static int seen;\n  close_cb(called + counter + seen);\n  return h_inline();\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "h.h"), []byte(hdr), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "t.c")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(dir)
	u, err := dumpUnit(clang, entry{Directory: dir, Command: "cc -c " + file, File: file}, real)
	if err != nil {
		t.Fatal(err)
	}
	want := []Linked{{"h.h", "h_inline", 1}, {"t.c", "close_cb", 4}, {"t.c", "called", 5}}
	if !reflect.DeepEqual(u.Internal, want) {
		t.Errorf("internal = %v, want %v", u.Internal, want)
	}
}
