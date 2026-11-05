package iacfacts

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// IAC13 and IAC14 were the fourth round in which a shape of HCL killed the process: boundTokens models the parser's
// recursion, and each fix modelled the shape that was found. This test is the family. A stack overflow cannot be
// recovered, so the shapes run in a child process with a small stack: every prefix and unit below, the unit repeated
// far past MaxDepth, must come back (refused or parsed) inside a time bound. The child names the shape it was on in a
// progress file before each parse, so a crash names it.
//
// IACFACTS_HCL_FAMILY=full widens the sweep to units of up to three tokens from the whole alphabet, which is the
// 31,000 shape sweep that found IAC14.
const (
	hclFamilyChildEnv = "IACFACTS_HCL_FAMILY_CHILD"
	hclFamilyReps     = 6000
	hclFamilyStack    = 16 << 20
	hclFamilySlow     = 5 * time.Second
)

type hclShape struct{ prefix, unit string }

func hclFamilyShapes(full bool) []hclShape {
	prefixes := []string{"x = a", "x = (a", "x = {for k in y : k => a", "x = [a", "x = f(a", "x = {a", "x = a ? b"}
	toks := []string{"[", "]", "*", ")", "}", "\n", "a", "?", ":", "!", "::", "for", "\"", "/**/"}
	if full {
		toks = []string{"[", "]", "*", "(", ")", "{", "}", "\n", "a", ".", "?", ":", "!", "-", "for", "=>", "/**/", ",", "::", "\"", "<<A\n"}
	}
	var units []string
	for _, a := range toks {
		units = append(units, a)
		for _, b := range toks {
			units = append(units, a+b)
			if full {
				for _, c := range toks {
					units = append(units, a+b+c)
				}
			}
		}
	}
	// Units longer than the sweep reaches: the ones found by reading the parser, and the ones that cost it time.
	units = append(units, "\n[*]", "\n?a", "[*)", "*)[", "*}[", "}[*", "::a", "a\n", "\\n", "[\n*]", "%{if a}", "%{for a in b}", "${a", "\n!", ".*",
		// IAC29: the parser's recover(`}`) swallows the `]}` of a mismatched pair as one close while its frame stays.
		"]}[{", "]}[{%{if a}", "{a]}[{", ")}[{", "]}({", "]]}[{")
	var out []hclShape
	for _, p := range prefixes {
		for _, u := range units {
			out = append(out, hclShape{p, u})
		}
	}
	// IAC29 sat past every unit the sweep above reaches, which is at most three tokens. A fixed-seed random sweep
	// of units of four to seven tokens covers the shapes nobody has named yet.
	rnd := rand.New(rand.NewSource(29))
	rtoks := []string{"[", "]", "*", "(", ")", "{", "}", "\n", "a", ".", "?", ":", "!", "-", "for", "=>", "/**/", ",", "::", "\"", "<<A\n", "%{if a}", "%{endif}", "${", "1", "in", "if"}
	for i := 0; i < 250; i++ {
		var u strings.Builder
		for j, n := 0, 4+rnd.Intn(4); j < n; j++ {
			u.WriteString(rtoks[rnd.Intn(len(rtoks))])
		}
		for _, p := range prefixes[:3] {
			out = append(out, hclShape{p, u.String()})
		}
	}
	return out
}

func TestHCLBoundFamilyChild(t *testing.T) {
	progress := os.Getenv(hclFamilyChildEnv)
	if progress == "" {
		return // the parent runs this test in a child; on its own it has nothing to do
	}
	debug.SetMaxStack(hclFamilyStack)
	for _, sh := range hclFamilyShapes(os.Getenv("IACFACTS_HCL_FAMILY") == "full") {
		src := sh.prefix + strings.Repeat(sh.unit, hclFamilyReps) + "\n"
		_ = os.WriteFile(progress, []byte(fmt.Sprintf("%q x %d", sh.prefix+" + "+sh.unit, hclFamilyReps)), 0o600)
		start := time.Now()
		_, _, _ = parseHCLBounded("a.tf", []byte(src))
		if d := time.Since(start); d > hclFamilySlow {
			t.Errorf("%q repeated %d times took %v", sh.prefix+" + "+sh.unit, hclFamilyReps, d)
		}
	}
}

func TestHCLBoundFamilyNeverOverflowsOrCrawls(t *testing.T) {
	if os.Getenv(hclFamilyChildEnv) != "" {
		return
	}
	progress := t.TempDir() + "/progress"
	cmd := exec.Command(os.Args[0], "-test.run=^TestHCLBoundFamilyChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), hclFamilyChildEnv+"="+progress)
	out, err := cmd.CombinedOutput()
	if err != nil {
		last, _ := os.ReadFile(progress)
		tail := string(out)
		if len(tail) > 1500 {
			tail = tail[:1500]
		}
		t.Fatalf("child failed on %s: %v\n%s", last, err, tail)
	}
}

// IAC13, IAC14: a `{for` brace drops newlines as a bracket does, and `[*` takes any token as its close once the
// parser is recovering. The bound modelled neither, and either let a file under 1 MB overflow the stack.
func TestHCLBoundCountsForBracesAndSplatsWhateverCloses(t *testing.T) {
	const n = 2000
	shapes := map[string]string{
		"for brace, newline splat":    "x = {for k in y : k => a" + strings.Repeat("\n[*]", n),
		"for brace, newline question": "x = {for k in y : k => a" + strings.Repeat("\n?a", n),
		"splat closed by paren":       "x = a[*)" + strings.Repeat("[*)", n),
		"splat closed by star paren":  "x = a[" + strings.Repeat("*)[", n),
		"splat closed by star brace":  "x = a[" + strings.Repeat("*}[", n),
		"splat closed by brace":       "x = a" + strings.Repeat("}[*", n),
	}
	for name, src := range shapes {
		if _, err := ReadTerraformFile("a.tf", []byte(src+"\n")); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
	}
	// A for expression that spans lines, and a splat chain under the bound, still read.
	ok := "x = {for k in y :\n k => a[*].b\n}\ny = a[*].b[*].c\n"
	set, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n"+ok+"}\n"))
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("a multi-line for expression must read: %v", err)
	}
}

// IAC15: hclsyntax melds the literal pieces of one template pairwise, so a heredoc of 500,000 lines took 2m23s and
// 241 GB on valid syntax. IAC16: it concatenates a namespaced function name one segment at a time.
func TestHCLBoundRefusesQuadraticTemplateAndNamespaceRuns(t *testing.T) {
	within(t, 20*time.Second, "heredoc", func() {
		src := "x = <<A\n" + strings.Repeat("a\n", 30_000) + "A\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("a 30,000 line heredoc: want ErrLimit, got %v", err)
		}
		// A literal `$${` splits a quoted string into pieces the same way.
		src = "x = \"" + strings.Repeat("a$${", 20_000) + "\"\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("a quoted string of 40,000 pieces: want ErrLimit, got %v", err)
		}
	})
	within(t, 20*time.Second, "namespace", func() {
		src := "x = a" + strings.Repeat("::a", 5000) + "()\n"
		if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
			t.Errorf("a 5,000 segment function name: want ErrLimit, got %v", err)
		}
	})
	// Real heredocs and namespaced calls read.
	src := "resource \"t\" \"n\" {\n  x = <<A\n" + strings.Repeat("a line\n", 5000) + "A\n  y = provider::ns::fn(1)\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src))
	if err != nil || len(set.Resources) != 1 || set.SyntaxErrors != 0 {
		t.Fatalf("a 5,000 line heredoc must read: %v %+v", err, set)
	}
}
