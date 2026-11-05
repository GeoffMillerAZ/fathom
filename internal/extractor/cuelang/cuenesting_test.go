package cuelang

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The cue scanner reads an attribute by recursion (`@a(` calls itself once per
// bracket inside the attribute and once per `@` that follows another), and the
// parser keeps every lexical error in a list it searches for a repeat. A guard
// that drives the same scanner cannot survive the first and cannot bound the
// second unless it counts, so cueRefusal scans a copy in which an `@` is an
// operator and counts the errors. The sizes are above extractor.MaxNesting, not
// near the 2 MiB limit: a refusal is decided before the parser runs.

func over(unit string) string { return strings.Repeat(unit, extractor.MaxNesting+2_000) }

func TestCUERefusalFollowsAttributes(t *testing.T) {
	for name, c := range map[string]struct{ src, want string }{
		"attribute per line":           {over("@a(\n"), "nesting"},
		"bare at per line":             {over("@\n"), "nesting"},
		"brackets in an attribute":     {"@a(" + strings.Repeat(strings.Repeat("(", 100)+"\n", 200), "nesting"},
		"mixed brackets":               {"@a(" + over("[{\n"), "nesting"},
		"attributes in an attribute":   {"x: 1 " + over("@a(@b("), "nesting"},
		"attribute with no paren":      {over("@a\n"), "lexical errors"},
		"attribute with no paren, tag": {"a: 1 " + over("@a b\n"), "lexical errors"},
		"closer instead of a paren":    {over("@a)\n"), "lexical errors"},
		"illegal tokens":               {"a: " + over("^\n"), "lexical errors"},
		"illegal bytes":                {"a: " + over("\x01\n"), "lexical errors"},
		"backticks":                    {"a: " + over("`\n"), "lexical errors"},
	} {
		t.Run(name, func(t *testing.T) {
			if why := cueRefusal([]byte(c.src)); !strings.Contains(why, c.want) {
				t.Fatalf("reads %q, want a refusal naming %q", why, c.want)
			}
		})
	}
}

// A file with many well-formed attributes, closed ones nested a few deep, an
// attribute whose text holds brackets in a string, and a few errors, is read.
func TestFlatCUEAttributesAreNotRefused(t *testing.T) {
	var b strings.Builder
	b.WriteString("package p\n\n")
	for i := 0; i < 3*extractor.MaxNesting; i++ {
		fmt.Fprintf(&b, "f%d: int @go(F%d,type=*int) @tag(a,b(c[d{e}]))\n", i, i)
		fmt.Fprintf(&b, "g%d: string @go(\"((((\") // @ (\n", i)
	}
	b.WriteString("email: \"a@b.c\" @doc(x)\n")
	if why := cueRefusal([]byte(b.String())); why != "" {
		t.Fatalf("a flat file is refused: %q", why)
	}
	// A handful of lexical errors is a broken file, not an unreadable one.
	if why := cueRefusal([]byte("a: ^^^\nb: 1\n")); why != "" {
		t.Fatalf("a few errors are refused: %q", why)
	}
}

// The file resolver reads a .cue file's imports through the parser too, in its
// imports-only mode, and the scanner recurses on an attribute before it knows the
// file has no import: 2 MiB of `@a(` ended the process (a fatal stack overflow).
// It asks the same guard Extract does.
func TestCUEFileSpecsAreGuarded(t *testing.T) {
	c := &CUEExtractor{}
	src := []byte(strings.Repeat("@a(", (2<<20)/3))
	if fs, err := c.ExtractFileSpecs("a.cue", src); err != nil || len(fs.Imports) != 0 || len(fs.Exports) != 0 {
		t.Errorf("ExtractFileSpecs: %+v, %v", fs, err)
	}
	if spans, err := c.ExtractImportSpans("a.cue", src); err != nil || len(spans) != 0 {
		t.Errorf("ExtractImportSpans: %v, %v", spans, err)
	}
	ok := []byte("package p\n\nimport \"strings\"\n\nx: strings.ToUpper(\"a\")\n")
	if fs, _ := c.ExtractFileSpecs("a.cue", ok); len(fs.Imports) != 1 || len(fs.Exports) != 1 {
		t.Errorf("a real file reads %+v", fs)
	}
}
