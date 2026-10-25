package extractor

import (
	"strings"
	"testing"
)

// hlslSameShape fails t unless every text keeps src's byte length and line
// breaks: the step's rewrites never move a line, column or offset.
func hlslSameShape(t *testing.T, src string, texts [][]byte) {
	t.Helper()
	for _, text := range texts {
		if len(text) != len(src) {
			t.Fatalf("length %d, want %d (equal byte length keeps every offset)", len(text), len(src))
		}
		for i := range src {
			if (src[i] == '\n') != (text[i] == '\n') {
				t.Fatalf("a line break moved at byte %d", i)
			}
		}
	}
}

// TestHLSLMacroTextsRegisterClause pins rewrite (a): a cbuffer or tbuffer
// header's register clause, space-qualified or not, is blanked between the
// name and the `{`; a register on a resource declaration is the grammar's
// own and stays; a file with no such header yields nothing to try.
func TestHLSLMacroTextsRegisterClause(t *testing.T) {
	src := "cbuffer Frame : register(b0)\n{\n    float4 eye;\n};\n" +
		"tbuffer Lights : register( t3 , space2 ) {\n    float4 color;\n}\n" +
		"cbuffer Plain\n{\n    float x;\n};\n" +
		"Texture2D albedo : register(t0, space1);\n"
	texts := hlslMacroTexts([]byte(src))
	if len(texts) != 1 {
		t.Fatalf("got %d texts, want 1", len(texts))
	}
	hlslSameShape(t, src, texts)
	want := src
	for _, clause := range []string{": register(b0)", ": register( t3 , space2 )"} {
		want = strings.Replace(want, clause, strings.Repeat(" ", len(clause)), 1)
	}
	if got := string(texts[0]); got != want {
		t.Errorf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if hlslMacroTexts([]byte("cbuffer Plain { float x; };\nTexture2D t : register(t0);\n")) != nil {
		t.Error("a file with no register-qualified header has a rewrite")
	}
}

// TestHLSLMacroTextsDeclarationMacro pins rewrite (b): a function-like macro
// invocation (an all-capitals word with an underscore and its argument list)
// standing where a declaration starts and alone on its line with no `;` is
// blanked; one inside braces, one a `;` closes, one followed on its line by
// more code and a lower-case call all stay.
func TestHLSLMacroTextsDeclarationMacro(t *testing.T) {
	src := "#include \"a.hlsli\"\nMAKE_BINDINGS(1, 2)\nMAKE_MORE_BINDINGS(3)\n" +
		"float g;\nSET_UP_RESOURCES((4), 5)\n" +
		"void f() {\n    LOG_VALUE(g)\n}\nWITH_SLOT(6)\n" +
		"KEEP_ME(7);\nLEAD_IN(8) float h;\nkeep_me_too(9)\n"
	texts := hlslMacroTexts([]byte(src))
	if len(texts) != 1 {
		t.Fatalf("got %d texts, want 1", len(texts))
	}
	hlslSameShape(t, src, texts)
	want := src
	for _, call := range []string{"MAKE_BINDINGS(1, 2)", "MAKE_MORE_BINDINGS(3)", "SET_UP_RESOURCES((4), 5)", "WITH_SLOT(6)"} {
		want = strings.Replace(want, call, strings.Repeat(" ", len(call)), 1)
	}
	if got := string(texts[0]); got != want {
		t.Errorf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if hlslMacroTexts([]byte("KEEP_ME(7);\nvoid f() {\n    LOG_VALUE(g)\n}\n")) != nil {
		t.Error("a file with no declaration-position macro has a rewrite")
	}
}

// TestHLSLMacroTextsConditionals pins rewrite (c): a second candidate, tried
// only after the rewrites (a)+(b) text, also blanks the conditional directive
// lines with every branch kept; a file whose only change is its conditionals
// yields that one text; a file with neither yields nothing.
func TestHLSLMacroTextsConditionals(t *testing.T) {
	src := "cbuffer Frame : register(b0) { float4 eye; };\n" +
		"float4 shade(float4 c) {\n#if USE_FOG\n    return fog(c,\n#else\n    return plain(c,\n#endif\n        eye);\n}\n" +
		"#define KEEP 1\n"
	texts := hlslMacroTexts([]byte(src))
	if len(texts) != 2 {
		t.Fatalf("got %d texts, want 2 (rewrites, then rewrites with conditionals blanked)", len(texts))
	}
	hlslSameShape(t, src, texts)
	clause := ": register(b0)"
	first := strings.Replace(src, clause, strings.Repeat(" ", len(clause)), 1)
	if got := string(texts[0]); got != first {
		t.Errorf("first text:\n%s\nwant:\n%s", got, first)
	}
	second := first
	for _, dir := range []string{"#if USE_FOG", "#else", "#endif"} {
		second = strings.Replace(second, dir, strings.Repeat(" ", len(dir)), 1)
	}
	if got := string(texts[1]); got != second {
		t.Errorf("second text:\n%s\nwant:\n%s", got, second)
	}
	only := "void f() {\n#ifdef A\n    g(1,\n#else\n    g(2,\n#endif\n    3);\n}\n"
	texts = hlslMacroTexts([]byte(only))
	if len(texts) != 1 {
		t.Fatalf("conditionals only: got %d texts, want 1", len(texts))
	}
	hlslSameShape(t, only, texts)
	if strings.Contains(string(texts[0]), "#") {
		t.Errorf("conditionals only: a directive survived:\n%s", texts[0])
	}
	if hlslMacroTexts([]byte("#define A 1\nfloat f() { return A; }\n")) != nil {
		t.Error("a file with no rewrite and no conditional has a text")
	}
}
