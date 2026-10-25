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
