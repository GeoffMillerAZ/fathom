package iacfacts

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

// Splat chains: the parser recurses once per `[*]` step and keeps recursing for the rest of the chain, so a closed
// `]` must not lower the count. 160,000 repeats killed the process with a fatal stack overflow, past the guard.
func TestReadTerraformSplatChainsAreRefused(t *testing.T) {
	const n = 200_000
	shapes := map[string]string{
		"splat":        "a" + strings.Repeat("[*]", n),
		"splat+attr":   "a" + strings.Repeat("[*].b", n),
		"splat+index":  "a" + strings.Repeat("[*][0]", n),
		"nested index": "a[" + strings.Repeat("b[*]", n) + "]",
	}
	for name, e := range shapes {
		if _, err := ReadTerraformFile("a.tf", tfWith(e)); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
		if body := (&bindingReader{}).parse([]byte("x = "+e+"\n"), "z.tf"); body != nil {
			t.Errorf("%s: binding must skip the file", name)
		}
	}
	// Cap + 1, and under the cap a chain still reads.
	if _, err := ReadTerraformFile("a.tf", tfWith("a"+strings.Repeat("[*]", MaxDepth+1))); !errors.Is(err, ErrLimit) {
		t.Fatalf("cap+1: %v", err)
	}
	if set, err := ReadTerraformFile("a.tf", tfWith("a"+strings.Repeat("[*]", MaxDepth-1))); err != nil || len(set.Resources) != 1 {
		t.Fatalf("a chain under the bound must read: %v", err)
	}
	// Many short chains in one file and one block body are separate chains, not one long one.
	many := strings.Repeat("  y = a[*].b[*].c\n", 5*MaxDepth)
	if set, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n"+many+"}\n")); err != nil || len(set.Resources) != 1 {
		t.Fatalf("separate chains must not add up: %v", err)
	}
	if set, err := ReadTerraformFile("a.tf", tfWith("[a[*], "+strings.Repeat("a[*], ", 5*MaxDepth)+"b[*]]")); err != nil || len(set.Resources) != 1 {
		t.Fatalf("chains split by commas must not add up: %v", err)
	}
}

func allocated(fn func()) uint64 {
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	fn()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// forBomb is a for-expression whose value has 10^levels elements from a few hundred bytes.
func forBomb(levels int) string {
	const l = "[0,1,2,3,4,5,6,7,8,9]"
	e := l
	for i := 0; i < levels; i++ {
		e = "[for i in " + e + " : " + l + "]"
	}
	return e
}

// Binding evaluated repository expressions to read a string: a 265-byte file allocated 835 MB, and `1+1+...` at
// 500,000 terms overflowed the stack. Only literal syntax names a source, a version or a constraint.
func TestBindingNeverEvaluatesRepositoryExpressions(t *testing.T) {
	bomb := forBomb(6)
	plus := strings.Repeat("1+", 500_000) + "1"
	bad := map[string]string{
		"for in required_providers": "terraform {\n  required_providers {\n    aws = " + bomb + "\n  }\n}\n",
		"for in source":             "terraform {\n  required_providers {\n    aws = { source = " + bomb + ", version = " + bomb + " }\n  }\n}\n",
		"for in module source":      "module \"m\" {\n  source = " + bomb + "\n}\n",
	}
	for name, tf := range bad {
		fsys := fstest.MapFS{"main.tf": file(tf + "resource \"aws_lb_listener\" \"main\" {}\n")}
		got := allocated(func() { bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}) })
		if got > 64<<20 {
			t.Errorf("%s: binding allocated %d MB", name, got>>20)
		}
	}
	// A sum is cheap to parse and fatal to evaluate (BinaryOpExpr.Value recurses once per term): reaching the end
	// of the call is the assertion, since a stack overflow ends the process.
	for name, tf := range map[string]string{
		"sum in source":        "terraform {\n  required_providers {\n    aws = { source = " + plus + " }\n  }\n}\n",
		"sum in module source": "module \"m\" {\n  source = " + plus + "\n}\n",
	} {
		fsys := fstest.MapFS{"main.tf": file(tf + "resource \"aws_lb_listener\" \"main\" {}\n")}
		if b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}); b.Status == "" {
			t.Errorf("%s: no status", name)
		}
	}
	lockSrc := "provider \"registry.terraform.io/hashicorp/aws\" {\n  version = " + bomb + "\n}\n"
	fsys := fstest.MapFS{"main.tf": file(reusableModule), LockfileName: file(lockSrc)}
	if got := allocated(func() { bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"}) }); got > 64<<20 {
		t.Errorf("lockfile version: binding allocated %d MB", got>>20)
	}
}

// A quoted literal, a heredoc-free escape and a number-free string still read; an interpolation or any other
// expression is not a literal.
func TestBindingLiteralStringIsSyntaxOnly(t *testing.T) {
	body := (&bindingReader{}).parse([]byte("a = \"x\\ty\"\nb = \"p${v}\"\nc = 1\nd = true\ne = (\"x\")\nf = \"a\" + \"b\"\ng = null\n"), "m.tf")
	want := map[string]struct {
		s  string
		ok bool
	}{"a": {"x\ty", true}, "b": {"", false}, "c": {"", false}, "d": {"", false}, "e": {"", false}, "f": {"", false}, "g": {"", false}}
	for k, w := range want {
		s, ok := literalString(body.Attributes[k].Expr)
		if s != w.s || ok != w.ok {
			t.Errorf("%s: got %q %v, want %q %v", k, s, ok, w.s, w.ok)
		}
	}
}
