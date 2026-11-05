package hclbound

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// IAC13 and IAC14 were the fourth round in which a shape of HCL killed the process: the guard models the parser's
// recursion, and each fix modelled the shape that was found. This test is the family. A stack overflow cannot be
// recovered, so the shapes run in a child process with a small stack: every prefix and unit below, the unit repeated
// far past the depth bound, must come back (refused or parsed) inside a time bound. The child names the shape it was
// on in a progress file before each parse, so a crash names it.
//
// HCLBOUND_FAMILY=full widens the sweep to units of up to three tokens from the whole alphabet, which is the
// 31,000 shape sweep that found IAC14.
const (
	familyChildEnv = "HCLBOUND_FAMILY_CHILD"
	familyReps     = 6000
	familyStack    = 16 << 20
	familySlow     = 5 * time.Second
)

// readBounds are the iacfacts reads' bounds: the smaller depth is the one a 16 MiB stack must survive.
var readBounds = Bounds{MaxDepth: 128, MaxTokens: 1 << 20}

type shape struct{ prefix, unit string }

func familyShapes(full bool) []shape {
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
		"]}[{", "]}[{%{if a}", "{a]}[{", ")}[{", "]}({", "]]}[{",
		// IAC34 and IAC35: a template the `[*` skips the opener of, and a directive the parser drops.
		"[*<<B\nB\n]", "[*\"\"]", "%{}a\n", "%{foo}$${",
		// a dropped directive that leaves a bracket open.
		"%{f(}a\n", "%{f[}a\n", "%{f(${a}}a\n",
		// N13: a `[*` right before the directive's close is two plain tokens to the parser, which stops at that close.
		"%{f [*}a\n", "%{[*}a\n", "%{f a[*}a\n", "%{f [\n*}a\n", "%{f [/**/*}a\n",
		// N14: a `%{for` whose header the parser drops.
		"a%{for}a%{endfor}", "a%{for x}a%{endfor}", "a%{for x,}a%{endfor}", "a%{for x y}a%{endfor}", "a%{for}a%{endif}", "%{endfor}a%{for}a\n")
	var out []shape
	for _, p := range prefixes {
		for _, u := range units {
			out = append(out, shape{p, u})
		}
	}
	// IAC29 sat past every unit the sweep above reaches, which is at most three tokens. A fixed-seed random sweep
	// of units of four to seven tokens covers the shapes nobody has named yet.
	rnd := rand.New(rand.NewSource(29))
	rtoks := []string{"[", "]", "*", "(", ")", "{", "}", "\n", "a", ".", "?", ":", "!", "-", "for", "=>", "/**/", ",", "::", "\"", "<<A\n", "%{if a}", "%{endif}", "%{", "${", "1", "in", "if"}
	for i := 0; i < 250; i++ {
		var u strings.Builder
		for j, n := 0, 4+rnd.Intn(4); j < n; j++ {
			u.WriteString(rtoks[rnd.Intn(len(rtoks))])
		}
		for _, p := range prefixes[:3] {
			out = append(out, shape{p, u.String()})
		}
	}
	return out
}

// parse is what a caller does: check, then parse what the guard let through.
func parse(src []byte) *Refusal {
	if r := Check(src, "a.tf", readBounds); r != nil {
		return r
	}
	_, _ = hclsyntax.ParseConfig(src, "a.tf", hcl.InitialPos)
	return nil
}

func TestFamilyChild(t *testing.T) {
	progress := os.Getenv(familyChildEnv)
	if progress == "" {
		return // the parent runs this test in a child; on its own it has nothing to do
	}
	debug.SetMaxStack(familyStack)
	for _, sh := range familyShapes(os.Getenv("HCLBOUND_FAMILY") == "full") {
		src := sh.prefix + strings.Repeat(sh.unit, familyReps) + "\n"
		_ = os.WriteFile(progress, []byte(fmt.Sprintf("%q x %d", sh.prefix+" + "+sh.unit, familyReps)), 0o600)
		start := time.Now()
		parse([]byte(src))
		if d := time.Since(start); d > familySlow {
			t.Errorf("%q repeated %d times took %v", sh.prefix+" + "+sh.unit, familyReps, d)
		}
	}
}

func TestFamilyNeverOverflowsOrCrawls(t *testing.T) {
	if os.Getenv(familyChildEnv) != "" {
		return
	}
	progress := t.TempDir() + "/progress"
	cmd := exec.Command(os.Args[0], "-test.run=^TestFamilyChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), familyChildEnv+"="+progress)
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
