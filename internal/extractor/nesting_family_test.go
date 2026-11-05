package extractor

import (
	"fmt"
	"strings"
	"testing"
)

// The guards in nesting.go decide from the parser's own tokens, and each one
// carries a belief about what the parser recurses on or spends its time on. A
// shape the belief missed ended the process (a for-expression turns newlines
// off, a directive nests past the sequence that opens it, a proto `-` and a
// comment recurse, an attribute scanner calls itself) or cost minutes (a template
// melds its pieces pairwise, proto builds a name by concatenation, cue keeps
// every lexical error in a list it searches). These tests run each guard over a
// family of shapes, one per kind of belief, at a size the guard must refuse and
// at the size of a flat file it must read. The size is above MaxNesting, not
// near the 2 MiB limit, because the guard decides before any parser runs.

func over(unit string) string { return strings.Repeat(unit, MaxNesting+2_000) }

func TestHCLRefusalCountsWhatTheParserRecursesOn(t *testing.T) {
	// Every opener below makes the parser ignore newlines (or never resets its
	// operand), so a prefix operator on each following line nests one level.
	openers := map[string]string{
		"for object":                 "a = {for k, v in x : k => \n",
		"for object, label first":    "a = {for k, v in x :\nk => \n",
		"for object after a comment": "a = {\n// c\nfor k, v in x : k => \n",
		"for tuple":                  "a = [for v in x : \n",
		"call":                       "a = f(\n",
		"bracket":                    "a = [\n",
		"interpolation":              "a = \"${\n",
		"directive expression":       "a = <<E\n%{if \n",
		"nested for in an object":    "a = {b = {for k, v in x : k => \n",
	}
	ops := map[string]string{"not": "!\n", "minus": "-\n", "conditional": "a ?\n"}
	for on, open := range openers {
		for opn, op := range ops {
			t.Run(on+"/"+opn, func(t *testing.T) {
				if why := hclRefusal([]byte(open + over(op))); !strings.Contains(why, "nesting") {
					t.Fatalf("a run of %d operators after %q reads %q", MaxNesting+2_000, open, why)
				}
			})
		}
	}
}

func TestHCLRefusalCountsOpenDirectives(t *testing.T) {
	for name, src := range map[string]string{
		"if":       "a = <<E\n" + over("%{if a}\n"),
		"for":      "a = <<E\n" + over("%{for a in b}\n"),
		"quoted":   "a = \"" + over("%{if a}") + "\"\n",
		"if tilde": "a = <<E\n" + over("%{~ if a ~}\n"),
		"mixed":    "a = <<E\n" + strings.Repeat("%{if a}%{for a in b}\n", MaxNesting),
	} {
		t.Run(name, func(t *testing.T) {
			if why := hclRefusal([]byte(src)); !strings.Contains(why, "nesting") {
				t.Fatalf("%d open directives read %q", MaxNesting+2_000, why)
			}
		})
	}
	// A directive that is closed is not a level: as many as you like, in a row,
	// and nested a few deep.
	for name, src := range map[string]string{
		"closed in a row":       "a = <<E\n" + strings.Repeat("%{if a}x%{endif}\n", 3*MaxNesting) + "E\n",
		"closed in a row, else": "a = <<E\n" + strings.Repeat("%{if a}x%{else}y%{endif}\n", 3*MaxNesting) + "E\n",
		"closed nested":         "a = <<E\n" + strings.Repeat("%{for a in b}%{if a}x%{endif}%{endfor}\n", 3*MaxNesting) + "E\n",
		"quoted":                "a = \"" + strings.Repeat("%{if a}x%{endif}", 3*MaxNesting) + "\"\n",
		// A directive its template leaves open is closed by the template's end:
		// each string is parsed on its own, so many of them nest nothing.
		"unclosed in many strings":  strings.Repeat("a = \"%{if a}x\"\n", 3*MaxNesting),
		"unclosed in many heredocs": strings.Repeat("a = <<E\n%{for a in b}\nE\n", 3*MaxNesting),
	} {
		t.Run("flat "+name, func(t *testing.T) {
			if why := hclRefusal([]byte(src)); why != "" {
				t.Fatalf("a flat template is refused: %q", why)
			}
		})
	}
}

func TestHCLRefusalCountsLiteralPieces(t *testing.T) {
	// The lexer ends a literal at every `$` and `%` that opens no sequence.
	for name, src := range map[string]string{
		"dollar":  "a = <<E\n" + strings.Repeat("$$$$$$$$$$\n", maxTemplatePieces/5) + "E\n",
		"percent": "a = <<E\n" + strings.Repeat("%%%%%%%%%%\n", maxTemplatePieces/5) + "E\n",
		"shell":   "a = <<E\n" + strings.Repeat("echo $HOME $PWD\n", maxTemplatePieces/2) + "E\n",
	} {
		t.Run(name, func(t *testing.T) {
			if why := hclRefusal([]byte(src)); !strings.Contains(why, "tokens in a row") {
				t.Fatalf("a template of more than %d pieces reads %q", maxTemplatePieces, why)
			}
		})
	}
	// A real script in a heredoc, and a string with many interpolations, are not
	// refused: an interpolation ends a run of pieces.
	script := "a = <<E\n" + strings.Repeat("echo $HOME $PWD\n", 2_000) + "E\n"
	interp := "a = \"" + strings.Repeat("x${v}$", 30_000) + "\"\n"
	for name, src := range map[string]string{"script": script, "interpolations": interp} {
		if why := hclRefusal([]byte(src)); why != "" {
			t.Errorf("%s is refused: %q", name, why)
		}
	}
}

// A file that is long but flat reads whole: no run of operators, brackets or
// directives is deeper than a statement.
func TestFlatHCLIsNotRefused(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "a%d = !b\nc%d = -1\nd%d = x ? y : z\n", i, i, i)
	}
	b.WriteString("block \"x\" {\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "  v%d = -%d\n  w%d = !true\n", i, i, i)
	}
	b.WriteString("}\nlocals {\n  m = {\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "    k%d = -%d\n", i, i)
	}
	b.WriteString("  }\n  e = {for k, v in x :\n k => -v\n}\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "  f%d = {for k, v in x : k => -v if !v}\n  g%d = [for v in x : !v]\n", i, i)
	}
	b.WriteString("  l = [\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "    -%d,\n", i)
	}
	b.WriteString("  ]\n}\n")
	if why := hclRefusal([]byte(b.String())); why != "" {
		t.Fatalf("a flat file is refused: %q", why)
	}
}

func TestProtoRefusalCountsWhatTheParserRecursesOn(t *testing.T) {
	const head = "syntax = \"proto3\";\n"
	for name, c := range map[string]struct{ src, want string }{
		"minus after =":               {head + "option a = \n" + over("-\n"), ""},
		"comment after =":             {head + "option a = \n" + over("//\n"), "nesting"},
		"comment text after =":        {head + "option a = \n" + over("// a b c\n"), "nesting"},
		"block comment after =":       {head + "option a = \n" + over("/* c */\n"), "nesting"},
		"minus in an option":          {head + "enum E {\n A = 1 [b = \n" + over("-\n"), ""},
		"minus in an aggregate":       {head + "option a = {\n b: \n" + over("-\n"), ""},
		"comment in an aggregate":     {head + "option a = {\n b: \n" + over("//\n"), "nesting"},
		"comment after a comma":       {head + "option a = [1,\n" + over("//\n"), "nesting"},
		"minus and comment alternate": {head + "option a = \n" + over("-\n//\n"), ""},
		// Each `-` and each bracket is a level of the same recursion, so they add.
		"minus then brackets":    {head + "option a = \n" + strings.Repeat("-\n", MaxNesting/2+500) + strings.Repeat("[\n", MaxNesting/2+500), "nesting"},
		"plus in an option":      {head + "enum E {\n A = 1 [b = \n" + over("+\n"), "tokens in a row"},
		"dots in a message":      {head + "message m {\n" + over(".\n"), "tokens in a row"},
		"dots and names":         {head + "message m {\n" + over("a.\n"), "tokens in a row"},
		"strings in a row":       {head + "option a = \"a\"\n" + over("\"a\"\n"), "tokens in a row"},
		"single quoted in a row": {head + "option a = 'a'\n" + over("'a'\n"), "tokens in a row"},
		"alternating":            {head + "message m {\n" + over("+.\n"), "tokens in a row"},
	} {
		t.Run(name, func(t *testing.T) {
			why := protoRefusal([]byte(c.src))
			if why == "" || (c.want != "" && !strings.Contains(why, c.want)) {
				t.Fatalf("reads %q, want a refusal naming %q", why, c.want)
			}
		})
	}
	// "minus ..." cases above accept either reason: the run of `-` is also a run.
}

func TestFlatProtoIsNotRefused(t *testing.T) {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\npackage a.b.c.d;\n")
	b.WriteString(strings.Repeat("// a license line\n", 3*MaxNesting)) // between statements, a comment recurses nowhere
	b.WriteString("option (x) = {\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "  k%d: -%d // c\n  s%d: \"a\" \"b\"\n", i, i, i)
	}
	b.WriteString("};\nenum E {\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "  A%d = %d [(o) = \"v\", deprecated = true];\n", i, i)
	}
	b.WriteString("}\nmessage M {\n")
	for i := 0; i < 3*MaxNesting; i++ {
		fmt.Fprintf(&b, "  repeated a.b.C f%d = %d; // field\n", i, i+1)
	}
	b.WriteString("  option (y) = [1, 2, 3, 4];\n}\n")
	if why := protoRefusal([]byte(b.String())); why != "" {
		t.Fatalf("a flat file is refused: %q", why)
	}
}
