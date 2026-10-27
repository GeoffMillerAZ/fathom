package pysyntax

import (
	"reflect"
	"testing"
)

func TestLexStringPrefixesAndTripleQuotes(t *testing.T) {
	toks, issues := Lex([]byte("a = rb'x\\n'\nb = '''p\nq'''\nc = U\"u\"\nd = t\"hi\"\n"))
	if len(issues) != 0 {
		t.Fatalf("issues %+v", issues)
	}
	var strs [][]StrPart
	var lines []int
	for _, tk := range toks {
		if tk.Kind == String {
			if tk.Text != "" {
				t.Fatalf("string token carries text %q", tk.Text)
			}
			strs = append(strs, tk.Parts)
			lines = append(lines, tk.Line)
		}
	}
	want := [][]StrPart{{{Lit: "x\\n"}}, {{Lit: "p\nq"}}, {{Lit: "u"}}, {{Lit: "hi"}}}
	if !reflect.DeepEqual(strs, want) {
		t.Fatalf("strings %+v, want %+v", strs, want)
	}
	if !reflect.DeepEqual(lines, []int{1, 2, 4, 5}) {
		t.Fatalf("string lines %v", lines)
	}
	// t"..." is not a prefix this tokenizer knows: a name, then a string.
	for i, tk := range toks {
		if tk.Kind == String && tk.Line == 5 && (toks[i-1].Kind != Name || toks[i-1].Text != "t") {
			t.Fatalf("t-string prefix read as %+v", toks[i-1])
		}
	}
}

func TestLexFStringParts(t *testing.T) {
	toks, issues := Lex([]byte("y = f\"/api/{settings.V!r}/{{x}}{n:>3}\"\nz = f''\n"))
	if len(issues) != 0 {
		t.Fatalf("issues %+v", issues)
	}
	var got [][]StrPart
	for _, tk := range toks {
		if tk.Kind == String {
			got = append(got, tk.Parts)
		}
	}
	want := [][]StrPart{{{Lit: "/api/"}, {Dyn: true, Expr: "settings.V"}, {Lit: "/{x}"}, {Dyn: true, Expr: "n"}}, nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("f-string parts %+v, want %+v", got, want)
	}
}

func TestLexLineContinuationAndBrackets(t *testing.T) {
	toks, issues := Lex([]byte("x = 1 + \\\n  2\nf = (\n  1,\n)\nz = 1\n"))
	if len(issues) != 0 {
		t.Fatalf("issues %+v", issues)
	}
	var kinds []Kind
	for _, tk := range toks {
		if tk.Kind == Name && tk.Text == "z" && tk.Line != 6 {
			t.Fatalf("z on line %d, want 6", tk.Line)
		}
		if tk.Kind == Newline || tk.Kind == Indent || tk.Kind == Dedent {
			kinds = append(kinds, tk.Kind)
		}
	}
	// One logical line each: the continuation and the bracket join lines.
	if !reflect.DeepEqual(kinds, []Kind{Newline, Newline, Newline}) {
		t.Fatalf("layout tokens %v", kinds)
	}
	if last := toks[len(toks)-1]; last.Kind != EOF {
		t.Fatalf("last token %+v", last)
	}
}

func TestLexIncompleteSignals(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []Issue
	}{
		{"complete", "import os\nos.system('x')\n", nil},
		{"unterminated string", "a = 1\nb = 'open\nc = 2\n", []Issue{{Line: 2, Kind: UnterminatedString}}},
		{"unterminated triple string", "a = 1\n\nb = \"\"\"open\nmore\n", []Issue{{Line: 3, Kind: UnterminatedString}}},
		{"nested f-string reuses its quote", "a = 1\nb = f\"{x[\"k\"]}\"\n", []Issue{{Line: 2, Kind: NestedFStringQuote}}},
		{"bracket open at end of file", "a = [\n  1,\n  (2,\n", []Issue{{Line: 1, Kind: OpenBracket}}},
		{"nested f-string reuses its quote past a brace", "x = f\"{d['}'][\"a\"]}\"\n", []Issue{{Line: 1, Kind: NestedFStringQuote}}},
		{"brace inside a nested string of the other quote", "x = f\"{'{'}\"\n", nil},
		{"NUL byte", "a = 1\nb = 'x\x00'\n", []Issue{{Line: 2, Kind: NULByte}}},
		{"every NUL line", "a = 1\x00\x00\nb = 2\x00\n", []Issue{{Line: 1, Kind: NULByte}, {Line: 2, Kind: NULByte}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := Lex([]byte(tc.src))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("issues %+v, want %+v", got, tc.want)
			}
			for _, is := range got {
				if is.Kind.String() == "" {
					t.Fatalf("issue %+v has no cause", is)
				}
			}
		})
	}
}

func FuzzLex(f *testing.F) {
	for _, s := range []string{"", "x = 1\n", "f\"{a[\"b\"]}\"", "(\n", "'''", "a\x00b", "\\\r\n", "if x:\n\ty\n  z\n"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		toks, _ := Lex(src)
		if len(toks) == 0 || toks[len(toks)-1].Kind != EOF {
			t.Fatalf("no EOF token: %+v", toks)
		}
		for i := 1; i < len(toks); i++ {
			if toks[i].Line < toks[i-1].Line {
				t.Fatalf("line went back at %d: %+v", i, toks)
			}
		}
	})
}
