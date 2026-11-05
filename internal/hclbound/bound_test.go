package hclbound

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// extractorBounds are the extractors' bounds: the parser may nest 10,000 levels where the stand-alone reads allow 128.
var extractorBounds = Bounds{MaxDepth: 10_000}

const over = 12_000 // past extractorBounds.MaxDepth

func repeat(unit string) string { return strings.Repeat(unit, over) }

func refusedBy(t *testing.T, src string, b Bounds, bound string) *Refusal {
	t.Helper()
	r := Check([]byte(src), "a.tf", b)
	if r == nil || r.Bound != bound {
		t.Fatalf("want a refusal by %q, got %v", bound, r)
	}
	return r
}

func reads(t *testing.T, name, src string, b Bounds) {
	t.Helper()
	if r := Check([]byte(src), "a.tf", b); r != nil {
		t.Errorf("%s is refused: %v", name, r)
	}
}

func within(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s: still running after %v", name, d)
	}
}

func TestCountsWhatTheParserRecursesOn(t *testing.T) {
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
				r := refusedBy(t, open+repeat(op), extractorBounds, BoundDepth)
				if !strings.Contains(r.Reason, "nesting deeper than 10000") {
					t.Fatalf("the reason does not state the bound: %q", r.Reason)
				}
			})
		}
	}
}

func TestCountsOpenDirectives(t *testing.T) {
	for name, src := range map[string]string{
		"if":       "a = <<E\n" + repeat("%{if a}\n"),
		"for":      "a = <<E\n" + repeat("%{for a in b}\n"),
		"quoted":   "a = \"" + repeat("%{if a}") + "\"\n",
		"if tilde": "a = <<E\n" + repeat("%{~ if a ~}\n"),
		"mixed":    "a = <<E\n" + strings.Repeat("%{if a}%{for a in b}\n", 10_000),
	} {
		t.Run(name, func(t *testing.T) { refusedBy(t, src, extractorBounds, BoundDepth) })
	}
	// A directive that is closed is not a level: as many as you like, in a row, and nested a few deep.
	for name, src := range map[string]string{
		"closed in a row":       "a = <<E\n" + strings.Repeat("%{if a}x%{endif}\n", 3_000) + "E\n",
		"closed in a row, else": "a = <<E\n" + strings.Repeat("%{if a}x%{else}y%{endif}\n", 3_000) + "E\n",
		"closed nested":         "a = <<E\n" + strings.Repeat("%{for a in b}%{if a}x%{endif}%{endfor}\n", 3_000) + "E\n",
		"quoted":                "a = \"" + strings.Repeat("%{if a}x%{endif}", 3_000) + "\"\n",
		// A directive its template leaves open is closed by the template's end:
		// each string is parsed on its own, so many of them nest nothing.
		"unclosed in many strings":  strings.Repeat("a = \"%{if a}x\"\n", 30_000),
		"unclosed in many heredocs": strings.Repeat("a = <<E\n%{for a in b}\nE\n", 30_000),
	} {
		t.Run("flat "+name, func(t *testing.T) { reads(t, name, src, extractorBounds) })
	}
}

// A file that is long but flat reads whole: no run of operators, brackets or directives is deeper than a statement.
func TestFlatFileIsNotRefused(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30_000; i++ {
		fmt.Fprintf(&b, "a%d = !b\nc%d = -1\nd%d = x ? y : z\n", i, i, i)
	}
	b.WriteString("block \"x\" {\n")
	for i := 0; i < 30_000; i++ {
		fmt.Fprintf(&b, "  v%d = -%d\n  w%d = !true\n", i, i, i)
	}
	b.WriteString("}\nlocals {\n  m = {\n")
	for i := 0; i < 30_000; i++ {
		fmt.Fprintf(&b, "    k%d = -%d\n", i, i)
	}
	b.WriteString("  }\n  e = {for k, v in x :\n k => -v\n}\n")
	for i := 0; i < 30_000; i++ {
		fmt.Fprintf(&b, "  f%d = {for k, v in x : k => -v if !v}\n  g%d = [for v in x : !v]\n", i, i)
	}
	b.WriteString("  l = [\n")
	for i := 0; i < 30_000; i++ {
		fmt.Fprintf(&b, "    -%d,\n", i)
	}
	b.WriteString("  ]\n}\n")
	reads(t, "a flat file", b.String(), extractorBounds)
}

// IAC29: the bound popped its bracket stack on any closer, but the parser's recover(`}`) swallows `]}` as one close
// while its frame stays, so the count fell as the parser's depth grew and a 390 KB file ended the process. A closer
// now pops only the level it matches, so the parser's frames are a subset of the counted ones.
func TestCountsMismatchedClosers(t *testing.T) {
	const n = 2000
	for name, src := range map[string]string{
		"brace, bracket-brace pairs":     "x = {a" + strings.Repeat("]}[{", n),
		"brace, pairs with directives":   "x = {a" + strings.Repeat("]}[{%{if a}", n),
		"paren, mismatched brace":        "x = (a" + strings.Repeat(")}[{", n),
		"bracket, closers between opens": "x = [a" + strings.Repeat("}]({", n),
	} {
		t.Run(name, func(t *testing.T) { refusedBy(t, src+"\n", readBounds, BoundDepth) })
	}
	// Balanced nesting of every kind still reads, and so does a template interpolation holding a brace.
	reads(t, "balanced nesting", "resource \"t\" \"n\" {\n  x = {a = [f({b = \"${ {c = 1}.c }\"})]}\n  y = {for k, v in z : k => [v]}\n}\n", readBounds)
}

// IAC13, IAC14: a `{for` brace drops newlines as a bracket does, and `[*` takes any token as its close once the
// parser is recovering. The bound modelled neither, and either let a file under 1 MB overflow the stack.
func TestCountsForBracesAndSplatsWhateverCloses(t *testing.T) {
	const n = 2000
	for name, src := range map[string]string{
		"for brace, newline splat":    "x = {for k in y : k => a" + strings.Repeat("\n[*]", n),
		"for brace, newline question": "x = {for k in y : k => a" + strings.Repeat("\n?a", n),
		"splat closed by paren":       "x = a[*)" + strings.Repeat("[*)", n),
		"splat closed by star paren":  "x = a[" + strings.Repeat("*)[", n),
		"splat closed by star brace":  "x = a[" + strings.Repeat("*}[", n),
		"splat closed by brace":       "x = a" + strings.Repeat("}[*", n),
	} {
		t.Run(name, func(t *testing.T) { refusedBy(t, src+"\n", readBounds, BoundDepth) })
	}
	// A for expression that spans lines, and a splat chain under the bound, still read.
	reads(t, "a multi-line for expression", "resource \"t\" \"n\" {\nx = {for k, v in y :\n k => a[*].b\n}\ny = a[*].b[*].c\n}\n", readBounds)
}

// IAC15: hclsyntax melds the literal pieces of one template pairwise, so a heredoc of 500,000 lines took 2m23s and
// 241 GB on valid syntax. IAC16: it concatenates a namespaced function name one segment at a time. The reason names
// the bound that fired, and a 20,000 line heredoc still reads.
func TestRefusesQuadraticTemplateAndNamespaceRuns(t *testing.T) {
	within(t, 20*time.Second, "heredoc", func() {
		r := refusedBy(t, "x = <<A\n"+strings.Repeat("a\n", 30_000)+"A\n", readBounds, BoundTemplate)
		if !strings.Contains(r.Reason, "melds 29999 literal pieces among 30000 parts") || !strings.Contains(r.Reason, "melding budget of 536870912 moves") {
			t.Errorf("the reason does not state the bound that fired: %q", r.Reason)
		}
		// A literal `$${` splits a quoted string into pieces the same way.
		refusedBy(t, "x = \""+strings.Repeat("a$${", 20_000)+"\"\n", readBounds, BoundTemplate)
		// The `$` and `%` lines that the lexer ends a piece at, and a shell script in a heredoc.
		refusedBy(t, "a = <<E\n"+strings.Repeat("$$$$$$$$$$\n", 4_000)+"E\n", extractorBounds, BoundTemplate)
		refusedBy(t, "a = <<E\n"+strings.Repeat("%%%%%%%%%%\n", 4_000)+"E\n", extractorBounds, BoundTemplate)
		refusedBy(t, "a = <<E\n"+strings.Repeat("echo $HOME $PWD\n", 10_000)+"E\n", extractorBounds, BoundTemplate)
		// An interpolation between pieces does not end the meld of the pieces beside it (M15).
		refusedBy(t, "a = <<E\n"+strings.Repeat("$a${b}\n", 100_000)+"E\n", extractorBounds, BoundTemplate)
	})
	within(t, 20*time.Second, "namespace", func() {
		r := refusedBy(t, "x = a"+strings.Repeat("::a", 5000)+"()\n", readBounds, BoundNamespace)
		if !strings.Contains(r.Reason, "more than 128 `::` segments") {
			t.Errorf("the reason does not state the bound: %q", r.Reason)
		}
	})
	// Real heredocs, a real script, a string of many interpolations and namespaced calls read.
	reads(t, "a 5,000 line heredoc", "resource \"t\" \"n\" {\n  x = <<A\n"+strings.Repeat("a line\n", 5000)+"A\n  y = provider::ns::fn(1)\n}\n", readBounds)
	reads(t, "a 20,000 line heredoc", "x = <<A\n"+strings.Repeat("a line\n", 20_000)+"A\n", readBounds)
	reads(t, "a script", "a = <<E\n"+strings.Repeat("echo $HOME $PWD\n", 2_000)+"E\n", extractorBounds)
	reads(t, "interpolations", "a = \""+strings.Repeat("x${v}$", 5_000)+"\"\n", extractorBounds)
}

// IAC15: hclsyntax melds the literal pieces of a template pairwise and moves the template's whole tail on every meld,
// so the cost is melds times parts, which a sum of squared run lengths does not price. A valid heredoc of 255 lines
// and an interpolation, repeated 800 times, took 7 s; 4,000 times, 2m36s. A 2 MB first line and 16,000 short lines
// copied 30.8 GB.
func TestPricesMeldsTimesPartsAndBytes(t *testing.T) {
	within(t, 20*time.Second, "melds", func() {
		rep := strings.Repeat("a\n", 255) + "${b}\n"
		refusedBy(t, "x = <<A\n"+strings.Repeat(rep, 800)+"A\n", readBounds, BoundTemplate)
		refusedBy(t, "x = <<A\n"+strings.Repeat("a", 2_000_000)+"\n"+strings.Repeat("b\n", 16_000)+"A\n", readBounds, BoundTemplate)
		// The budget is per template, so many templates of a few lines each are not one run.
		reads(t, "20,000 small heredocs", "x = ["+strings.Repeat("<<A\na\nb\nA\n,", 20_000)+"]\n", readBounds)
		// ... but they are one file: the file budget adds the templates up.
		refusedBy(t, strings.Repeat("x = <<A\n"+strings.Repeat("a line\n", 22_000)+"A\n", 20), readBounds, BoundTemplates)
	})
	within(t, 20*time.Second, "valid", func() {
		reads(t, "20 runs of 255 lines and an interpolation", "x = <<A\n"+strings.Repeat(strings.Repeat("a\n", 255)+"${b}\n", 20)+"A\n", readBounds)
		var b strings.Builder
		for i := 0; i < 11; i++ {
			fmt.Fprintf(&b, "x%d = <<A\n%sA\n", i, strings.Repeat("a line\n", 5000))
		}
		reads(t, "11 heredocs of 5,000 lines", b.String(), readBounds)
	})
}

// IAC34: `[*` followed by a quote or a heredoc skips the opener (the parser reads it as the splat's close), but the
// lexer's template is open, and its close popped the enclosing template's frame, so every later piece of the
// enclosing template was in no frame: a 781 KB file took 1m30s and read.
func TestSplatThatSkipsATemplateOpenerKeepsTheEnclosingFrame(t *testing.T) {
	within(t, 20*time.Second, "iac34", func() {
		refusedBy(t, "x = <<A\n${a[*<<B\nB\n]}\n"+strings.Repeat("a\n", 400_000), readBounds, BoundTemplate)
		refusedBy(t, "x = \"${a[*\"\"]}"+strings.Repeat("a$${", 200_000)+"\"\n", readBounds, BoundTemplate)
		// The skipped template's own pieces count in its own frame.
		refusedBy(t, "x = <<A\n${a[*<<B\n"+strings.Repeat("a\n", 40_000)+"B\n]}\nA\n", readBounds, BoundTemplate)
		// A small one reads, so the frame it pushes is popped.
		reads(t, "a small skipped template", "x = <<A\n${a[*<<B\nB\n]}\n"+strings.Repeat("a\n", 100)+"A\n", readBounds)
	})
}

// IAC35: a `%{` that is not before if, else, endif, for or endfor is an error the parser reports and drops, so it
// makes no part and the literals on each side still meld. 976 KB took 19.9 s, and 1.9 MB took 1m08s.
func TestInvalidDirectiveDoesNotSeparateLiterals(t *testing.T) {
	within(t, 20*time.Second, "iac35", func() {
		refusedBy(t, "x = <<A\n"+strings.Repeat("%{}a\n", 200_000)+"A\n", readBounds, BoundTemplate)
		refusedBy(t, "x = \""+strings.Repeat("%{foo}$${", 200_000)+"\"\n", readBounds, BoundTemplate)
		refusedBy(t, "x = <<A\n"+strings.Repeat("}#c\n%{", 100_000)+"\nA\n", readBounds, BoundTemplate)
		// A directive that is one does separate them, and a template of many of them reads.
		reads(t, "valid directives between lines", "x = <<A\n"+strings.Repeat("%{if a}b%{endif}\n", 4_000)+"A\n", readBounds)
		reads(t, "else directives", "x = <<A\n"+strings.Repeat("%{if a}b%{else}c%{endif}\n", 4_000)+"A\n", readBounds)
	})
}

func TestTokenBound(t *testing.T) {
	src := "x = " + strings.Repeat("1,", 10_000) + "\n"
	r := refusedBy(t, src, Bounds{MaxDepth: 128, MaxTokens: 1_000}, BoundTokens)
	if !strings.Contains(r.Reason, "over the bound of 1000") {
		t.Errorf("the reason does not state the bound: %q", r.Reason)
	}
	reads(t, "no token bound", src, Bounds{MaxDepth: 128})
}

// The extractors' guard counted two things the iacfacts guard did not, and the union keeps both: a bracket pair that is
// an operand of what follows it (an index or a call's result applied again and again) nests the tree one level for
// each, and the operators pending before a bracket are on the parser's stack as long as the bracket is open.
func TestCountsPostfixStepsAndOperatorsPendingBeforeABracket(t *testing.T) {
	refusedBy(t, "a = [\nf(x)\n"+strings.Repeat("[0]", over)+"\n", extractorBounds, BoundDepth)
	refusedBy(t, "a = f(x)"+strings.Repeat("[0].b", 200)+"\n", readBounds, BoundDepth)
	refusedBy(t, "a = f(x)"+strings.Repeat("(1)", 200)+"\n", readBounds, BoundDepth)
	// 6,000 brackets, each with one operator pending before it: neither count alone passes 10,000, the sum does.
	refusedBy(t, "a = "+strings.Repeat("!(", 6_000), extractorBounds, BoundDepth)
	// The pending operators go when their bracket closes, and a statement or an item ends a postfix run.
	reads(t, "closed operator runs", strings.Repeat("a = !(b)\n", 100_000), extractorBounds)
	reads(t, "closed operator runs, stand-alone", strings.Repeat("a = !(b)\n", 100_000), readBounds)
	reads(t, "postfix steps across statements", strings.Repeat("a = f(x)[0].b[1]\n", 50_000), readBounds)
	reads(t, "postfix steps across items", "a = ["+strings.Repeat("f(x)[0], ", 50_000)+"]\n", readBounds)
	reads(t, "a few postfix steps", "a = f(x)[0].b[1](2)[3]\n", readBounds)
}

// A closer that matches no bracket pops nothing from the filtered stream either: boundTokens drops a newline inside a
// for expression's brace, and a stray `)` there must not end that. With the stray closer popping the brace's level, the
// newline would split a run of operators the parser recurses through whole into two runs under the bound.
func TestStrayCloserKeepsTheNewlineHiddenInAForBrace(t *testing.T) {
	src := "x = {for a in b : a => ) " + strings.Repeat("!", 100) + "\n" + strings.Repeat("!", 100) + "a}\n"
	refusedBy(t, src, readBounds, BoundDepth)
	reads(t, "half the run", "x = {for a in b : a => ) "+strings.Repeat("!", 100)+"a}\n", readBounds)
}

// N14: a `%{for` whose header is not NAME [, NAME] in is dropped by the parser with no part, so the literals either
// side of it meld. Counting it as a kept part priced those melds at zero: 1.7 MB of `a%{for}a%{endfor}` read and cost
// the parser 4.3 s.
func TestDroppedForDirectiveDoesNotHideLiterals(t *testing.T) {
	within(t, 90*time.Second, "n14", func() {
		for _, unit := range []string{"a%{for}a%{endfor}", "a%{for x}a%{endfor}", "a%{for x,}a%{endfor}", "a%{for x y}a%{endfor}", "a%{for}a%{endif}"} {
			for _, bounds := range []Bounds{extractorBounds, readBounds} {
				refusedBy(t, "x = \""+strings.Repeat(unit, 100_000)+"\"\n", bounds, BoundTemplate)
			}
		}
		for _, bounds := range []Bounds{extractorBounds, readBounds} {
			refusedBy(t, "x = <<A\n"+strings.Repeat("%{endfor}a%{for}a\n", 100_000)+"A\n", bounds, BoundTemplate)
		}
		// A valid header is a kept part, and a short template of it reads.
		for _, valid := range []string{"%{for x in y}", "%{for k, v in y}", "%{for\nx in y}"} {
			reads(t, "valid for "+valid, "x = \"a"+valid+"a%{endfor}\"\n", readBounds)
		}
		reads(t, "valid for, many", "x = \""+strings.Repeat("a%{for x in y}a%{endfor}", 10_000)+"\"\n", readBounds)
	})
}

// the parser skips a directive it drops with recover(`}`), which counts the `${` and `%{` it meets against the
// `}` that ends them and no bracket. A bracket the directive leaves open (`%{f(}`) does not keep it open: the literals
// after it still meld, so they are priced. 781 KB of this shape took 1m31s of CPU.
func TestInvalidDirectiveWithAnOpenBracketDoesNotHideLiterals(t *testing.T) {
	within(t, 20*time.Second, "iac41", func() {
		for _, open := range []string{"(", "[", "((", "([", "(${a}", "(%{f(}"} {
			refusedBy(t, "x = <<A\n%{f"+open+"}\n"+strings.Repeat("a\n", 100_000)+"A\n", readBounds, BoundTemplate)
			refusedBy(t, "x = \"%{f"+open+"}"+strings.Repeat("$${", 200_000)+"\"\n", readBounds, BoundTemplate)
		}
		// N13: inside a dropped directive a `[*` is two plain tokens, so it hides no close.
		for _, splat := range []string{"%{f [*}", "%{[*}", "%{f a[*}", "%{f [\n*}", "%{f [/**/*}"} {
			refusedBy(t, "x = <<A\n"+splat+"\n"+strings.Repeat("a\n", 100_000)+"A\n", readBounds, BoundTemplate)
		}
		refusedBy(t, "x = \"%{f[*}"+strings.Repeat("$${", 200_000)+"\"\n", readBounds, BoundTemplate)
		// A nested opener claims its own close, so the directive ends at the next one.
		refusedBy(t, "x = <<A\n%{f ${(a} b}\n"+strings.Repeat("a\n", 100_000)+"A\n", readBounds, BoundTemplate)
		// The shapes that read stay read.
		reads(t, "a short template after the directive", "x = <<A\n%{f(}\n"+strings.Repeat("a\n", 1_000)+"A\n", readBounds)
		reads(t, "a bracket closed inside the directive", "x = <<A\n%{f(a)}\n"+strings.Repeat("a\n", 1_000)+"A\n", readBounds)
		reads(t, "code after the template", "x = <<A\n%{f(}\na\nA\ny = [1, 2]\nz = {a = 1}\n", readBounds)
		reads(t, "a splat in a valid directive", "x = <<A\n%{if a[*]}b%{endif}\n"+strings.Repeat("a\n", 1_000)+"A\n", readBounds)
	})
}
