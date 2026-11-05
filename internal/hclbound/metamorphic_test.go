package hclbound

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Check must refuse what the parser would recurse on, however the tokens are spaced: the parser's
// peeker skips comments, and skips newlines inside any bracket but a brace, so `[/**/*]` is a splat and so is
// `[\n*]` inside parentheses. M3 reopened three times because each check matched the raw token stream.
//
// The metamorphic property: for every shape, a comment between each token pair, and a newline or a line comment
// between each token pair where the parser ignores newlines, refuse exactly when the plain shape refuses. It runs in a
// subprocess, since a missed shape is a fatal stack overflow that ends the process, not a failed test.

const metaChildEnv = "HCLBOUND_METAMORPHIC_CHILD"

type boundShape struct {
	name, expr string
	refused    bool
}

func boundShapes() []boundShape {
	max := readBounds.MaxDepth
	n := 4 * max
	return []boundShape{
		// Operator runs.
		{"bang", strings.Repeat("!", n) + "true", true},
		{"minus", strings.Repeat("-", n) + "1", true},
		{"mixed ops", strings.Repeat("!-", n/2) + "1", true},
		{"ops at cap", strings.Repeat("!", max+1) + "true", true},
		{"ops under cap", strings.Repeat("!", max-8) + "true", false},
		// Splat chains.
		{"splat", "a" + strings.Repeat("[*]", n), true},
		{"splat+attr", "a" + strings.Repeat("[*].b", n), true},
		{"splat+index", "a" + strings.Repeat("[*][0]", n), true},
		{"nested index", "a[" + strings.Repeat("b[*]", n) + "]", true},
		{"splat at cap", "a" + strings.Repeat("[*]", max+1), true},
		{"splat under cap", "a" + strings.Repeat("[*]", max-8), false},
		{"separate chains", "[a[*], " + strings.Repeat("a[*], ", 5*max) + "b[*]]", false},
		// Ternaries and directives.
		{"ternary", "a" + strings.Repeat("?a", n), true},
		{"ternary in ()", "(a" + strings.Repeat("?a", n) + ")", true},
		{"ternary at cap", "a" + strings.Repeat("?a", max+1), true},
		{"if directive", `"` + strings.Repeat("%{if a}", n) + `"`, true},
		{"closed for", `"` + strings.Repeat("%{for a in b}", n) + strings.Repeat("%{endfor}", n) + `"`, true},
		{"heredoc if", "<<EOT\n" + strings.Repeat("%{if a}", n) + "\nEOT", true},
		{"mixed", "a" + strings.Repeat("?a", n/2) + `"` + strings.Repeat("%{if a}", n/2) + `"`, true},
		{"colon-closed chain", "a" + strings.Repeat("?b:a", 5*max) + "?b:c", false},
		{"closed directives", `"` + strings.Repeat("%{if a}x%{endif}", 5*max) + `"`, false},
		{"separate strings", `["` + strings.Repeat(`%{if a}x", "%{if a}`, 5*max) + `x"]`, false},
		{"object colons", "{" + strings.Repeat("a: b,", 5*max) + "}", false},
		{"for colon", "[for x in y : x]", false},
	}
}

// spaceTokens returns src with sep after every token in [lo, hi) whose gap to the next token the parser lets sep
// stand in. A gap inside a template's literal text is not one: a comment there is text. With needBracket a gap is
// also refused unless the innermost bracket ignores newlines.
func spaceTokens(src string, lo, hi int, sep string, needBracket bool) string {
	toks, _ := hclsyntax.LexConfig([]byte(src), "m.tf", hcl.InitialPos)
	var tmpl []bool // true: template literal text; false: code inside ${ } or %{ }
	var brk []hclsyntax.TokenType
	var sb strings.Builder
	last := 0
	for i, t := range toks {
		if t.Type == hclsyntax.TokenEOF {
			break
		}
		switch t.Type {
		case hclsyntax.TokenOQuote, hclsyntax.TokenOHeredoc:
			tmpl = append(tmpl, true)
		case hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			tmpl = append(tmpl, false)
			brk = append(brk, t.Type)
		case hclsyntax.TokenCQuote, hclsyntax.TokenCHeredoc:
			tmpl = tmpl[:len(tmpl)-1]
		case hclsyntax.TokenTemplateSeqEnd:
			tmpl = tmpl[:len(tmpl)-1]
			brk = brk[:len(brk)-1]
		case hclsyntax.TokenOBrace, hclsyntax.TokenOParen, hclsyntax.TokenOBrack:
			brk = append(brk, t.Type)
		case hclsyntax.TokenCBrace, hclsyntax.TokenCParen, hclsyntax.TokenCBrack:
			brk = brk[:len(brk)-1]
		}
		if i+1 >= len(toks) || t.Range.Start.Byte < lo || toks[i+1].Range.End.Byte > hi {
			continue
		}
		if len(tmpl) > 0 && tmpl[len(tmpl)-1] {
			continue
		}
		if needBracket && (len(brk) == 0 || brk[len(brk)-1] == hclsyntax.TokenOBrace) {
			continue
		}
		end := t.Range.End.Byte
		sb.WriteString(src[last:end])
		sb.WriteString(sep)
		last = end
	}
	sb.WriteString(src[last:])
	return sb.String()
}

// metaVariants are the spaced forms of one expression, each as the expression to put after `x = `.
func metaVariants(expr string) map[string]string {
	plain := "x = " + expr + "\n"
	wrapped := "x = (" + expr + "\n)\n"
	cut := func(s string) string { return strings.TrimSuffix(strings.TrimPrefix(s, "x = "), "\n") }
	return map[string]string{
		"plain":        expr,
		"comments":     cut(spaceTokens(plain, 4, len(plain)-1, "/**/", false)),
		"newlines":     cut(spaceTokens(wrapped, 5, len(wrapped)-2, "\n", true)),
		"line comment": cut(spaceTokens(wrapped, 5, len(wrapped)-2, "# c\n", true)),
		"wrapped":      cut(wrapped),
	}
}

func checkMetamorphic(t *testing.T) {
	for _, sh := range boundShapes() {
		vars := metaVariants(sh.expr)
		for _, vn := range []string{"plain", "comments", "newlines", "line comment", "wrapped"} {
			r := parse([]byte("x = " + vars[vn] + "\n"))
			if (r != nil) != sh.refused {
				t.Errorf("%s/%s: refused=%v, want %v (%v)", sh.name, vn, r != nil, sh.refused, r)
			}
		}
	}
	// The sizes that ended the process: a repeat of a splat or a directive hidden by a comment or a newline, at
	// the counts where the parser's stack or heap gave out.
	const big = 200_000
	huge := map[string]string{
		"splat comment":      "a" + strings.Repeat("[/**/*]", big),
		"splat star comment": "a" + strings.Repeat("[*/**/]", big),
		"splat newline":      "(a" + strings.Repeat("[\n*]", big) + ")",
		"splat both":         "(a" + strings.Repeat("[/**/\n*]", big/2) + ")",
		"directive comment":  `"` + strings.Repeat("%{/**/if a}", 40_000) + `"`,
		"directive newline":  `("` + strings.Repeat("%{\nif a}", 40_000) + `")`,
	}
	for name, expr := range huge {
		if r := parse([]byte("x = " + expr + "\n")); r == nil {
			t.Errorf("%s: want a refusal", name)
		}
	}
}

func TestMetamorphic(t *testing.T) {
	if os.Getenv(metaChildEnv) != "" {
		checkMetamorphic(t)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMetamorphic$", "-test.count=1")
	cmd.Env = append(os.Environ(), metaChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 4000 {
			tail = tail[len(tail)-4000:]
		}
		t.Fatalf("the metamorphic run failed (a fatal error ends the child): %v\n%s", err, tail)
	}
}
