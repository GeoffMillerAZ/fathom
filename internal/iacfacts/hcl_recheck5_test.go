package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// IAC29: the bound popped its bracket stack on any closer, but the parser's recover(`}`) swallows `]}` as one close
// while its frame stays, so the count fell as the parser's depth grew and a 390 KB file ended the process. A closer
// now pops only the level it matches, so the parser's frames are a subset of the counted ones. 2,000 repeats stay
// under the stack limit here, and the family test runs the same shapes far past it in a child with a small stack.
func TestHCLBoundCountsMismatchedClosers(t *testing.T) {
	const n = 2000
	shapes := map[string]string{
		"brace, bracket-brace pairs":     "x = {a" + strings.Repeat("]}[{", n),
		"brace, pairs with directives":   "x = {a" + strings.Repeat("]}[{%{if a}", n),
		"paren, mismatched brace":        "x = (a" + strings.Repeat(")}[{", n),
		"bracket, closers between opens": "x = [a" + strings.Repeat("}]({", n),
	}
	for name, src := range shapes {
		if _, err := ReadTerraformFile("a.tf", []byte(src+"\n")); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
		if body, _ := (&bindingReader{}).parse([]byte(src+"\n"), "z.tf"); body != nil {
			t.Errorf("%s: binding must skip the file", name)
		}
	}
	// Balanced nesting of every kind still reads, and so does a template interpolation holding a brace.
	ok := "resource \"t\" \"n\" {\n  x = {a = [f({b = \"${ {c = 1}.c }\"})]}\n  y = {for k, v in z : k => [v]}\n}\n"
	if set, err := ReadTerraformFile("a.tf", []byte(ok)); err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 0 {
		t.Fatalf("balanced nesting must read: %v %+v", err, set)
	}
}

// IAC15: hclsyntax melds the literal pieces of a template pairwise and moves the template's whole tail on every meld,
// so the cost is melds times parts, which a sum of squared run lengths does not price. A valid heredoc of 255 lines
// and an interpolation, repeated 800 times, took 7 s; 4,000 times, 2m36s. A 2 MB first line and 16,000 short lines
// copied 30.8 GB.
func TestHCLBoundPricesMeldsTimesPartsAndBytes(t *testing.T) {
	within(t, 20*time.Second, "melds", func() {
		rep := strings.Repeat("a\n", 255) + "${b}\n"
		src := "x = <<A\n" + strings.Repeat(rep, 800) + "A\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("800 runs of 255 lines and an interpolation: want ErrLimit, got %v", err)
		}
		src = "x = <<A\n" + strings.Repeat("a", 2<<20) + "\n" + strings.Repeat("b\n", 16_000) + "A\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("a 2 MB first line and 16,000 lines: want ErrLimit, got %v", err)
		}
		// The budget is per template, so many templates of a few lines each are not one run.
		src = "x = [" + strings.Repeat("<<A\na\nb\nA\n,", 20_000) + "]\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); err != nil {
			t.Errorf("20,000 small heredocs must read: %v", err)
		}
	})
	within(t, 20*time.Second, "valid", func() {
		valid := map[string]string{
			"20,000 line heredoc":                       "x = <<A\n" + strings.Repeat("a line\n", 20_000) + "A\n",
			"20 runs of 255 lines and an interpolation": "x = <<A\n" + strings.Repeat(strings.Repeat("a\n", 255)+"${b}\n", 20) + "A\n",
			"11 heredocs of 5,000 lines": func() string {
				var b strings.Builder
				for i := 0; i < 11; i++ {
					fmt.Fprintf(&b, "x%d = <<A\n%sA\n", i, strings.Repeat("a line\n", 5000))
				}
				return b.String()
			}(),
		}
		for name, body := range valid {
			set, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n"+body+"}\n"))
			if err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 0 {
				t.Errorf("%s must read: %v", name, err)
			}
		}
	})
}
