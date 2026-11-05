package iacfacts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// parseHCLBounded must refuse what the parser would recurse on, however the tokens are spaced: the parser's
// peeker skips comments, and skips newlines inside any bracket but a brace, so `[/**/*]` is a splat and so is
// `[\n*]` inside parentheses. M3 reopened three times because each check matched the raw token stream.
//
// The metamorphic property: for every shape, a comment between each token pair, and a newline or a line comment
// between each token pair where the parser ignores newlines, refuse exactly when the plain shape refuses. It runs in a
// subprocess, since a missed shape is a fatal stack overflow that ends the process, not a failed test.

const metaChildEnv = "IACFACTS_METAMORPHIC_CHILD"

type boundShape struct {
	name, expr string
	refused    bool
}

func boundShapes() []boundShape {
	n := 4 * MaxDepth
	return []boundShape{
		// Operator runs.
		{"bang", strings.Repeat("!", n) + "true", true},
		{"minus", strings.Repeat("-", n) + "1", true},
		{"mixed ops", strings.Repeat("!-", n/2) + "1", true},
		{"ops at cap", strings.Repeat("!", MaxDepth+1) + "true", true},
		{"ops under cap", strings.Repeat("!", MaxDepth-8) + "true", false},
		// Splat chains.
		{"splat", "a" + strings.Repeat("[*]", n), true},
		{"splat+attr", "a" + strings.Repeat("[*].b", n), true},
		{"splat+index", "a" + strings.Repeat("[*][0]", n), true},
		{"nested index", "a[" + strings.Repeat("b[*]", n) + "]", true},
		{"splat at cap", "a" + strings.Repeat("[*]", MaxDepth+1), true},
		{"splat under cap", "a" + strings.Repeat("[*]", MaxDepth-8), false},
		{"separate chains", "[a[*], " + strings.Repeat("a[*], ", 5*MaxDepth) + "b[*]]", false},
		// Ternaries and directives.
		{"ternary", "a" + strings.Repeat("?a", n), true},
		{"ternary in ()", "(a" + strings.Repeat("?a", n) + ")", true},
		{"ternary at cap", "a" + strings.Repeat("?a", MaxDepth+1), true},
		{"if directive", `"` + strings.Repeat("%{if a}", n) + `"`, true},
		{"closed for", `"` + strings.Repeat("%{for a in b}", n) + strings.Repeat("%{endfor}", n) + `"`, true},
		{"heredoc if", "<<EOT\n" + strings.Repeat("%{if a}", n) + "\nEOT", true},
		{"mixed", "a" + strings.Repeat("?a", n/2) + `"` + strings.Repeat("%{if a}", n/2) + `"`, true},
		{"colon-closed chain", "a" + strings.Repeat("?b:a", 5*MaxDepth) + "?b:c", false},
		{"closed directives", `"` + strings.Repeat("%{if a}x%{endif}", 5*MaxDepth) + `"`, false},
		{"separate strings", `["` + strings.Repeat(`%{if a}x", "%{if a}`, 5*MaxDepth) + `x"]`, false},
		{"object colons", "{" + strings.Repeat("a: b,", 5*MaxDepth) + "}", false},
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

func isLimit(err error) bool { return errors.Is(err, ErrLimit) }

func checkMetamorphic(t *testing.T) {
	for _, sh := range boundShapes() {
		vars := metaVariants(sh.expr)
		for _, vn := range []string{"plain", "comments", "newlines", "line comment", "wrapped"} {
			expr := vars[vn]
			_, err := ReadTerraformFile("a.tf", tfWith(expr))
			if isLimit(err) != sh.refused {
				t.Errorf("%s/%s: reader refused=%v, want %v (%v)", sh.name, vn, isLimit(err), sh.refused, err)
			}
			body, _ := (&bindingReader{}).parse([]byte("x = "+expr+"\n"), "z.tf")
			if (body == nil) != sh.refused {
				t.Errorf("%s/%s: binding skipped=%v, want %v", sh.name, vn, body == nil, sh.refused)
			}
		}
	}
	// Binding through ResolveIaCBinding: a spaced shape in a sibling file is skipped and disclosed, and the module
	// still answers.
	for _, sh := range boundShapes() {
		if !sh.refused {
			continue
		}
		for vn, expr := range metaVariants(sh.expr) {
			fsys := fstest.MapFS{"main.tf": file(reusableModule), "z.tf": file("x = " + expr + "\n")}
			b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
			if !strings.Contains(strings.Join(b.Disclosures, "|"), "past the read bound") {
				t.Errorf("%s/%s: binding did not disclose the skipped file: %v", sh.name, vn, b.Disclosures)
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
		if _, err := ReadTerraformFile("a.tf", tfWith(expr)); !isLimit(err) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
	}
	for name, expr := range map[string]string{
		"splat comment": "a" + strings.Repeat("[/**/*]", 140_000),
		"splat newline": "(a" + strings.Repeat("[\n*]", 200_000) + ")",
	} {
		fsys := fstest.MapFS{"main.tf": file(reusableModule), "z.tf": file("x = " + expr + "\n")}
		b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "past the read bound") {
			t.Errorf("%s: binding did not disclose the skipped file", name)
		}
	}
}

func TestHCLBoundMetamorphic(t *testing.T) {
	if os.Getenv(metaChildEnv) != "" {
		checkMetamorphic(t)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHCLBoundMetamorphic$", "-test.count=1")
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

// Real files are never refused: every .tf of the corpus named by IACFACTS_TF_CORPUS reads without ErrLimit and
// binds without a skipped file. Without the variable the test has nothing to read.
func TestHCLBoundDoesNotRefuseRealFiles(t *testing.T) {
	root := os.Getenv("IACFACTS_TF_CORPUS")
	if root == "" {
		t.Skip("IACFACTS_TF_CORPUS names a directory of real Terraform modules")
	}
	files := 0
	walkTF(t, root, func(path string, src []byte) {
		files++
		if _, err := ReadTerraformFile(path, src); isLimit(err) {
			t.Errorf("%s: refused: %v", path, err)
		}
		if body, _ := (&bindingReader{}).parse(src, path); body == nil {
			t.Errorf("%s: binding skipped the file", path)
		}
	})
	if files == 0 {
		t.Fatalf("no .tf file under the corpus")
	}
	t.Logf("%d files read", files)
}

// walkTF calls fn for every .tf and .tofu file under root that fits the read bound.
func walkTF(t *testing.T, root string, fn func(path string, src []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(p); ext == ".tf" || ext == ".tofu" {
			if src, err := os.ReadFile(p); err == nil && len(src) <= MaxFileBytes {
				fn(p, src)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
