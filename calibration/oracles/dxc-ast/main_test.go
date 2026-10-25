package main

import (
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
