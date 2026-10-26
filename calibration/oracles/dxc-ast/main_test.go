package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// dump is a hand-written DXC text AST in the shape `dxc -ast-dump` prints:
// a header's declarations first, then the unit's own, with clang's location
// compression (a file or line written only where it changes).
const dump = `TranslationUnitDecl 0x1 <<invalid sloc>> <invalid sloc>
|-TypedefDecl 0x2 <<invalid sloc>> <invalid sloc> implicit float4 'vector<float, 4>'
|-VarDecl 0x3 <./inc/common.hlsli:3:1, col:14> col:14 linearSampler 'SamplerState'
| ` + "`" + `-RegisterAssignment 0x4 <col:31> register(s0)
|-FunctionDecl 0x5 <line:5:1, line:8:1> line:5:8 used helper 'float (float)'
| |-ParmVarDecl 0x6 <col:15, col:21> col:21 used x 'float'
| ` + "`" + `-CompoundStmt 0x7 <line:6:1, line:8:1>
|   ` + "`" + `-ReturnStmt 0x8 <line:7:5, col:22>
|     ` + "`" + `-CallExpr 0x9 <col:12, col:22> 'float'
|       |-ImplicitCastExpr 0xa <col:12> 'float (*)(float)' <FunctionToPointerDecay>
|       | ` + "`" + `-DeclRefExpr 0xb <col:12> 'float (float)' lvalue Function 0xc 'saturate' 'float (float)'
|       ` + "`" + `-DeclRefExpr 0xd <col:21> 'float' lvalue ParmVar 0x6 'x' 'float'
|-HLSLBufferDecl 0xe <./Shaders/a b/main.hlsl:4:1, line:7:1> line:4:9 cbuffer Frame
| |-RegisterAssignment 0xf <col:23> register(b0)
| ` + "`" + `-VarDecl 0x10 parent cbuffer 0xe <line:6:5, col:12> col:12 used tint 'const float4'
|-EmptyDecl 0x11 <line:7:2> col:2
|-CXXRecordDecl 0x12 <line:9:1, line:13:1> line:9:8 referenced struct VSOut definition
| |-CXXRecordDecl 0x13 <col:1, col:8> col:8 implicit struct VSOut
| ` + "`" + `-FieldDecl 0x14 <line:11:5, col:12> col:12 position 'float4'
|-TypedefDecl 0x15 <line:15:1, col:15> col:15 Index 'uint'
|-FunctionTemplateDecl 0x16 <line:17:1, line:20:1> line:18:3 pick
| |-TemplateTypeParmDecl 0x17 <line:17:10, col:19> col:19 typename T
| ` + "`" + `-FunctionDecl 0x18 <line:18:1, line:20:1> line:18:3 pick 'T (T)'
|   ` + "`" + `-CompoundStmt 0x19 <col:12, line:20:1>
|-FunctionDecl 0x1a <line:22:1, line:27:1> line:22:8 main 'float4 (VSOut)'
| |-ParmVarDecl 0x1b <col:13, col:19> col:19 used v 'VSOut'
| ` + "`" + `-CompoundStmt 0x1c <line:23:1, line:27:1>
|   |-CallExpr 0x1d <line:24:5, col:16> 'float'
|   | ` + "`" + `-ImplicitCastExpr 0x1e <col:5> 'float (*)(float)' <FunctionToPointerDecay>
|   |   ` + "`" + `-DeclRefExpr 0x1f <col:5> 'float (float)' lvalue Function 0x5 'helper' 'float (float)'
|   |-CXXMemberCallExpr 0x20 <line:25:5, col:30> 'float4'
|   | ` + "`" + `-MemberExpr 0x21 <col:5, col:9> '<bound member function type>' .Sample 0x22
|   ` + "`" + `-CallExpr 0x23 <line:2:10, col:20> 'float'
|     ` + "`" + `-ImplicitCastExpr 0x24 <col:10> 'float (*)(float)' <FunctionToPointerDecay>
|       ` + "`" + `-DeclRefExpr 0x25 <col:10> 'float (float)' lvalue Function 0x26 'frac' 'float (float)'
|-FunctionDecl 0x27 <line:29:1, col:20> col:6 proto 'void ()'
` + "`" + `-FunctionDecl 0x28 prev 0x27 <line:30:1, line:31:1> line:30:6 proto 'void ()'
  ` + "`" + `-CompoundStmt 0x29 <col:14, line:31:1>
`

func TestParseDump(t *testing.T) {
	files := map[string]bool{"inc/common.hlsli": true, "Shaders/a b/main.hlsl": true}
	macro := func(path string, line int) bool { return path == "Shaders/a b/main.hlsl" && line == 2 }
	u := parseDump(strings.NewReader(dump), files, macro)
	want := []Def{
		{"Shaders/a b/main.hlsl", "Frame", "cbuffer", 4, 7},
		{"Shaders/a b/main.hlsl", "Index", "typedef", 15, 15},
		{"Shaders/a b/main.hlsl", "VSOut", "struct", 9, 13},
		{"Shaders/a b/main.hlsl", "main", "function", 22, 27},
		{"Shaders/a b/main.hlsl", "pick", "function", 18, 20},
		{"Shaders/a b/main.hlsl", "proto", "function", 30, 31},
		{"inc/common.hlsli", "helper", "function", 5, 8},
		{"inc/common.hlsli", "linearSampler", "variable", 3, 3},
	}
	sortDefs(u.Defs)
	if !reflect.DeepEqual(u.Defs, want) {
		t.Errorf("defs:\n%v\nwant:\n%v", u.Defs, want)
	}
	wantCalls := []Pair{
		{"Shaders/a b/main.hlsl", "main", "Sample"},
		{"Shaders/a b/main.hlsl", "main", "helper"},
		{"inc/common.hlsli", "helper", "saturate"},
	}
	sortPairs(u.Calls)
	if !reflect.DeepEqual(u.Calls, wantCalls) {
		t.Errorf("calls:\n%v\nwant:\n%v", u.Calls, wantCalls)
	}
	if u.MacroCalls != 1 {
		t.Errorf("macro calls %d, want 1 (frac, spelled on a #define line)", u.MacroCalls)
	}
}

func TestDefineLines(t *testing.T) {
	src := "float a;\n#define F(x) \\\n    frac(x)\n  #  define G 1\nfloat b;\n"
	got := defineLines([]byte(src))
	want := map[int]bool{2: true, 3: true, 4: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("define lines %v, want %v", got, want)
	}
}

// fakeDXC writes a stand-in dxc: --version, an -ast-dump that fails for
// bad.hlsl and prints one function for good.hlsl, and a -M rule naming
// good.hlsl's header.
func fakeDXC(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dxc")
	script := `#!/bin/sh
for a in "$@"; do last="$a"; done
case "$*" in
*--version*) echo "fake dxc 1.0"; exit 0 ;;
*" -M "*) echo "good.cso: ./good.hlsl ./inc.hlsli"; exit 0 ;;
esac
case "$last" in
./bad.hlsl) echo "./bad.hlsl:1:1: error: unknown type name 'Foo'" >&2; exit 1 ;;
./old.hlsl) case "$*" in *"-HV 2018"*) printf '%s\n' 'TranslationUnitDecl 0x1 <<invalid sloc>> <invalid sloc>' ;;
  *) echo "./old.hlsl:2:5: error: operator overloading is not allowed" >&2; exit 1 ;; esac ;;
./good.hlsl) printf '%s\n' 'TranslationUnitDecl 0x1 <<invalid sloc>> <invalid sloc>' \
  '|-FunctionDecl 0x2 <./good.hlsl:1:1, line:3:1> line:1:7 main '"'"'float ()'"'"'' \
  '| ` + "`" + `-CompoundStmt 0x3 <line:2:1, line:3:1>' ;;
esac
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRunScope: a unit whose dump writes an error is not read; a passing
// unit and the header its -M rule names are the scope, written into the
// oracle file and to -scope.
func TestRunScope(t *testing.T) {
	subject := t.TempDir()
	for _, f := range []string{"bad.hlsl", "good.hlsl", "inc.hlsli"} {
		if err := os.WriteFile(filepath.Join(subject, f), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(t.TempDir(), "files")
	if err := os.WriteFile(list, []byte("bad.hlsl\ngood.hlsl\ninc.hlsli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scopeOut := filepath.Join(t.TempDir(), "scope")
	var stdout, stderr bytes.Buffer
	if err := run(subject, fakeDXC(t), "lib_6_3", "lib_6_9", list, scopeOut, "", nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var out oracleFile
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if want := []string{"good.hlsl", "inc.hlsli"}; !reflect.DeepEqual(out.Scope, want) {
		t.Errorf("scope %v, want %v", out.Scope, want)
	}
	if b, _ := os.ReadFile(scopeOut); string(b) != "good.hlsl\ninc.hlsli\n" {
		t.Errorf("-scope wrote %q", b)
	}
	if want := []Def{{"good.hlsl", "main", "function", 1, 3}}; !reflect.DeepEqual(out.Definitions, want) {
		t.Errorf("definitions %v, want %v", out.Definitions, want)
	}
}

func TestFirstErrorReadsFatal(t *testing.T) {
	got := firstError("./a.h:12:10: fatal error: 'b.h' file not found\n#include \"b.h\"\n")
	if want := "a.h:12: 'b.h' file not found"; got != want {
		t.Errorf("firstError = %q, want %q", got, want)
	}
	if c := errorClass(got); c != "'_' file not found" {
		t.Errorf("errorClass = %q", c)
	}
}

// TestRunRetries (H5-R5): a unit with an error under the run's setting is
// retried under -HV 2018 and the newest library profile, read under the first
// that parses it, and disclosed; one no setting parses is excluded with its
// first error, counted by class with examples.
func TestRunRetries(t *testing.T) {
	subject := t.TempDir()
	for _, f := range []string{"bad.hlsl", "old.hlsl"} {
		if err := os.WriteFile(filepath.Join(subject, f), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(t.TempDir(), "files")
	if err := os.WriteFile(list, []byte("bad.hlsl\nold.hlsl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run(subject, fakeDXC(t), "lib_6_3", "lib_6_9", list, "", "", nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var out oracleFile
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if want := []string{"old.hlsl"}; !reflect.DeepEqual(out.Scope, want) {
		t.Errorf("scope %v, want %v", out.Scope, want)
	}
	all := strings.Join(out.Disclose, "\n")
	for _, want := range []string{
		"2 units with an error under that setting, 1 of them not read under any",
		"-T lib_6_3 -HV 2018 admits 1 and -T lib_6_9 -HV 2021 admits 0",
		"1 admitted: old.hlsl (-T lib_6_3 -HV 2018)",
		`1 "unknown type name '_'" (e.g. bad.hlsl:1: unknown type name 'Foo')`,
		"of the 1 units excluded, 0 stop at an #include this run's -I roots do not resolve (a build passes its own) and 1 use a name",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("disclose lacks %q:\n%s", want, all)
		}
	}
}

// TestParseDumpFixture reads the pinned DXC's own dump of testdata/fixture.hlsl
// (testdata/README.md has the command line): every kind the oracle writes, a
// struct's methods with a body (one defined out of line) as kind method with
// their calls, and a call a macro body spells counted, not written.
func TestParseDumpFixture(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "fixture.dump.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	files := map[string]bool{"fixture.hlsl": true, "fixture.hlsli": true}
	src, err := os.ReadFile(filepath.Join("testdata", "fixture.hlsl"))
	if err != nil {
		t.Fatal(err)
	}
	defines := defineLines(src)
	u := parseDump(f, files, func(p string, line int) bool { return p == "fixture.hlsl" && defines[line] })
	wantDefs := []Def{
		{"fixture.hlsl", "Frame", "cbuffer", 8, 11},
		{"fixture.hlsl", "Index", "typedef", 13, 13},
		{"fixture.hlsl", "Ray", "struct", 15, 21},
		{"fixture.hlsl", "at", "method", 19, 19},
		{"fixture.hlsl", "create", "method", 18, 18},
		{"fixture.hlsl", "len", "method", 23, 23},
		{"fixture.hlsl", "main", "function", 25, 32},
		{"fixture.hlsli", "Material", "struct", 4, 7},
		{"fixture.hlsli", "helper", "function", 9, 12},
		{"fixture.hlsli", "linearSampler", "variable", 2, 2},
	}
	sortDefs(u.Defs)
	if !reflect.DeepEqual(u.Defs, wantDefs) {
		t.Errorf("defs:\n%v\nwant:\n%v", u.Defs, wantDefs)
	}
	wantCalls := []Pair{
		{"fixture.hlsl", "at", "saturate"},
		{"fixture.hlsl", "len", "length"},
		{"fixture.hlsl", "main", "at"},
		{"fixture.hlsl", "main", "create"},
		{"fixture.hlsl", "main", "helper"},
		{"fixture.hlsl", "main", "len"},
		{"fixture.hlsli", "helper", "saturate"},
	}
	sortPairs(u.Calls)
	if !reflect.DeepEqual(u.Calls, wantCalls) {
		t.Errorf("calls:\n%v\nwant:\n%v", u.Calls, wantCalls)
	}
	if u.MacroCalls != 1 {
		t.Errorf("macro calls %d, want 1 (frac, spelled on the #define line)", u.MacroCalls)
	}
	// H5-R7: main names the header's type Material (line 28), its field
	// rough and its function helper: one file pair, at its first reference.
	if want := []Ref{{Path: "fixture.hlsl", Line: 28, ToPath: "fixture.hlsli", Addr: true}}; !reflect.DeepEqual(u.Refs, want) {
		t.Errorf("refs %v, want %v", u.Refs, want)
	}
}

// typeDump (H6-R2): a typedef MyAlias of Material in a.hlsli, struct Material
// in b.hlsli, and a Light struct in each of two namespaces (c.hlsli, d.hlsli);
// f.hlsl declares `MyAlias m` (sugared 'MyAlias', canonical 'Material'), a
// `B::Light l`, and reads c.hlsli's variable gain through its address.
const typeDump = `TranslationUnitDecl 0x1 <<invalid sloc>> <invalid sloc>
|-TypedefDecl 0x2 <./a.hlsli:1:1, col:18> col:18 referenced MyAlias 'Material':'Material'
|-CXXRecordDecl 0x3 <./b.hlsli:1:1, line:4:1> line:1:8 referenced struct Material definition
| ` + "`" + `-FieldDecl 0x4 <line:3:5, col:11> col:11 rough 'float'
|-NamespaceDecl 0x5 <./c.hlsli:1:1, line:5:1> line:1:11 A
| |-CXXRecordDecl 0x6 <line:2:1, line:2:20> line:2:8 struct Light definition
| ` + "`" + `-VarDecl 0x7 <line:3:1, col:7> col:7 used gain 'float'
|-NamespaceDecl 0x8 <./d.hlsli:1:1, line:3:1> line:1:11 B
| ` + "`" + `-CXXRecordDecl 0x9 <line:2:1, line:2:20> line:2:8 referenced struct Light definition
` + "`" + `-FunctionDecl 0xa <./f.hlsl:3:1, line:8:1> line:3:7 main 'float ()'
  ` + "`" + `-CompoundStmt 0xb <line:4:1, line:8:1>
    |-DeclStmt 0xc <line:5:5, col:14>
    | ` + "`" + `-VarDecl 0xd <col:5, col:13> col:13 used m 'MyAlias':'Material'
    |-DeclStmt 0xe <line:6:5, col:15>
    | ` + "`" + `-VarDecl 0xf <col:5, col:14> col:14 l 'B::Light'
    ` + "`" + `-ReturnStmt 0x10 <line:7:5, col:19>
      ` + "`" + `-DeclRefExpr 0x11 <col:12, col:15> 'float' lvalue Var 0x7 'gain' 'float'
`

// TestParseDumpTypeRefs (H6-R2, as clang-objc's TypeRef): a quoted type's
// sugared half is the name written, so f.hlsl names a.hlsli's typedef and
// never b.hlsli's Material; a name two type declarations of the unit share
// (Light) is ambiguous, skipped and counted; each pair says how it resolved.
func TestParseDumpTypeRefs(t *testing.T) {
	files := map[string]bool{"a.hlsli": true, "b.hlsli": true, "c.hlsli": true, "d.hlsli": true, "f.hlsl": true}
	u := parseDump(strings.NewReader(typeDump), files, func(string, int) bool { return false })
	want := []Ref{
		{Path: "a.hlsli", Line: 1, ToPath: "b.hlsli"}, // the typedef names Material
		{Path: "f.hlsl", Line: 5, ToPath: "a.hlsli"},
		{Path: "f.hlsl", Line: 7, ToPath: "c.hlsli", Addr: true},
	}
	if !reflect.DeepEqual(u.Refs, want) {
		t.Errorf("refs %+v, want %+v", u.Refs, want)
	}
	if u.AmbiguousTypes != 1 || u.AmbiguousRefs != 1 {
		t.Errorf("ambiguous types %d refs %d, want 1 and 1 (Light, declared in c.hlsli and d.hlsli)", u.AmbiguousTypes, u.AmbiguousRefs)
	}
}
