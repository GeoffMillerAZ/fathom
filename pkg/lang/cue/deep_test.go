package cue_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/cue"
)

// The CUE parser is recursive descent, and Go reports a stack overflow as a
// fatal error that recover cannot catch, so a file nested deep enough inside the
// 2 MiB limit ends the caller's process. Each case runs in a subprocess, so a
// fatal fails the case and not the whole run. The shapes are not only brackets:
// a run of prefix operators, of field labels (`a: a: a:`), and a chain of
// selectors, calls, indexes or binary operators nest the tree one level each,
// and a closing bracket in a comment or a string must not hide any of it.
const deepLimit = 2<<20 - 256

type deepCase struct {
	name string
	src  func() string
}

func rep(s string, n int) string { return strings.Repeat(s, n) }

// lines is head, then as many lines of unit repeated per line as fit in the limit.
func lines(head, unit string, per int) string {
	line := rep(unit, per) + "\n"
	return head + rep(line, (deepLimit-len(head))/len(line))
}

// oneLine is head, then unit repeated to fill the limit on a single line, after
// a head of short lines: the minified-file check reads only the first 64 KiB, so
// a file that opens with blank lines gets through it.
func oneLine(head, unit string) string {
	pad := rep("\n", 66_000)
	return pad + head + rep(unit, (deepLimit-len(pad)-len(head))/len(unit))
}

var deepCases = []deepCase{
	{"brackets", func() string { return "a: " + rep("[\n", deepLimit/2-2) }},
	{"braces", func() string { return "a: " + rep("{\n", deepLimit/2-2) }},
	{"parens", func() string { return "a: " + rep("(\n", deepLimit/2-2) }},
	{"not", func() string { return "a: " + rep("!\n", deepLimit/2-2) }},
	{"minus", func() string { return "a: " + rep("-\n", deepLimit/2-2) }},
	{"labels on one line", func() string { return oneLine("", "a: ") }},
	{"labels on lines", func() string { return rep("a:\n", deepLimit/3) }},
	{"selectors", func() string { return oneLine("a: b", ".a") }},
	{"selectors on lines", func() string { return "a: b.\n" + rep("a.\n", deepLimit/3-2) }},
	{"indexes", func() string { return oneLine("a: b", "[0]") }},
	{"calls", func() string { return oneLine("a: b", "()") }},
	{"binary operators", func() string { return "a: 1 +\n" + rep("1 +\n", deepLimit/4-2) }},
	{"binary operators on a line", func() string { return oneLine("a: 1", " + 1") }},
	{"disjunction", func() string { return "a: 1 |\n" + rep("1 |\n", deepLimit/4-2) }},
	{"interpolations", func() string { return oneLine("a: ", `"\(`) }},
	{"structs on a line", func() string { return oneLine("a: ", "{a: ") }},
	{"brackets on a line", func() string { return oneLine("a: ", "[") }},
	{"a closer in a comment", func() string { return rep("a: [\n// ]\n", deepLimit/10) }},
	{"a closer in a string", func() string { return rep("a: [\n\"]\"\n", deepLimit/9) }},
	{"a stray closer", func() string { return rep("a: [{ ] \n", deepLimit/9) }},
	{"an unmatched interpolation", func() string { return rep("a: \"\\([ )\"\n[\n", deepLimit/14) }},
}

const deepChildEnv = "FATHOM_DEEP_CUE_CASE"

func TestDeepNestingIsRefusedNotFatal(t *testing.T) {
	for _, tc := range deepCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDeepNestingChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), deepChildEnv+"="+tc.name)
			out, err := cmd.CombinedOutput()
			if err != nil {
				tail := string(out)
				if len(tail) > 1500 {
					tail = tail[:1500]
				}
				t.Fatalf("the reader did not refuse the nesting: %v\n%s", err, tail)
			}
		})
	}
}

// TestDeepNestingChild is one case of the test above, run alone in its own
// process; without the variable it does nothing.
func TestDeepNestingChild(t *testing.T) {
	name := os.Getenv(deepChildEnv)
	if name == "" {
		t.Skip("run by TestDeepNestingIsRefusedNotFatal")
	}
	for _, tc := range deepCases {
		if tc.name != name {
			continue
		}
		r := newReader(t, cue.Option())
		src := tc.src()
		if len(src) > 2<<20 {
			t.Fatalf("the input is %d bytes, over the 2 MiB limit", len(src))
		}
		ok := extract(t, r, "x.cue", cueSrc, "x.cue")
		if ok.Skipped != "" || ok.Incomplete || len(ok.Notes) != 0 {
			t.Fatalf("control: skipped %q incomplete=%v notes=%v", ok.Skipped, ok.Incomplete, ok.Notes)
		}
		got := extract(t, r, "x.cue", src, "x.cue")
		if got.Skipped != "" {
			t.Fatalf("skipped (%s), so no reader was reached", got.Skipped)
		}
		if !got.Incomplete || !strings.Contains(strings.Join(got.Notes, " "), "nesting") {
			t.Fatalf("incomplete=%v notes=%v", got.Incomplete, got.Notes)
		}
		return
	}
	t.Fatalf("no case %q", name)
}

// A file that is long but flat reads whole: a closer in a comment or a string,
// a long run of fields, interpolations and a wide disjunction are not nesting.
func TestFlatCUEIsNotRefused(t *testing.T) {
	r := newReader(t, cue.Option())
	var b strings.Builder
	b.WriteString("package p\n\n" + rep("// ]]]]]]]]]]\n", 12_000) + "s: \"x ((((\"\n")
	for i := 0; i < 20_000; i++ {
		b.WriteString("k")
		b.WriteString(strings.Repeat("a", i%7))
		b.WriteString(": [1, 2, {x: \"\\(1)-\\(2)\"}]\n")
	}
	b.WriteString("d: ")
	for i := 0; i < 5_000; i++ {
		b.WriteString("\"v\" |\n")
	}
	b.WriteString("\"w\"\n")
	got := extract(t, r, "p/x.cue", b.String(), "p/x.cue")
	if got.Skipped != "" || got.Incomplete || len(got.Symbols) == 0 {
		t.Fatalf("skipped %q incomplete=%v notes=%v symbols=%d", got.Skipped, got.Incomplete, got.Notes, len(got.Symbols))
	}
}
