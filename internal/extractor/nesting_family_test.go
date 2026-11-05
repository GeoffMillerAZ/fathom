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

// The HCL guard and its family tests are internal/hclbound's. These hold the wiring: the extractors' bound is
// MaxNesting, the reason names the bound that fired, and a flat file reads.
func TestHCLRefusalIsTheSharedGuard(t *testing.T) {
	for name, c := range map[string]struct{ src, want string }{
		"nesting":  {"a = [for v in x : \n" + over("!\n"), "hcl parse: nesting deeper than 10000"},
		"template": {"a = <<E\n" + strings.Repeat("echo $HOME $PWD\n", 10_000) + "E\n", "melding budget"},
		// The parser melds the pieces on either side of an interpolation, so a string of them is not flat (M15).
		"interpolations": {"a = <<E\n" + strings.Repeat("$a${b}\n", 100_000) + "E\n", "melding budget"},
	} {
		if why := hclRefusal([]byte(c.src)); !strings.Contains(why, c.want) {
			t.Errorf("%s: reads %q, want a refusal naming %q", name, why, c.want)
		}
	}
	for name, src := range map[string]string{
		"a script":             "a = <<E\n" + strings.Repeat("echo $HOME $PWD\n", 2_000) + "E\n",
		"a long flat file":     strings.Repeat("a = !b\nc = -1\nd = x ? y : z\n", 3*MaxNesting),
		"a 20,000 line script": "a = <<E\n" + strings.Repeat("a line\n", 20_000) + "E\n",
	} {
		if why := hclRefusal([]byte(src)); why != "" {
			t.Errorf("%s is refused: %q", name, why)
		}
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
