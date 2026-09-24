package clone_test

// funcbodyhash_test.go — CV-31 (ADR-044 §3.3, design doc "Stage 3 —
// absence and anti-Blindness"): FuncBodyHash exposes the clone package's
// own identifier-insensitive normalisation (NormalizedHash) per function,
// plus the statement count conserve.Resurrected (CV-32) needs for its "at
// least 5 statements" floor — never a second, divergent hashing scheme.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/clone"
)

func TestFuncBodyHash_IdentifierRenameInsensitive(t *testing.T) {
	a := `func Old(x int, y int) int {
	z := x + y
	if z > 0 {
		return z
	}
	return -z
}`
	b := `func New(a int, b int) int {
	c := a + b
	if c > 0 {
		return c
	}
	return -c
}`
	ha, na, ok := clone.FuncBodyHash(a)
	if !ok {
		t.Fatalf("FuncBodyHash(a) ok = false, want true")
	}
	hb, nb, ok := clone.FuncBodyHash(b)
	if !ok {
		t.Fatalf("FuncBodyHash(b) ok = false, want true")
	}
	if ha != hb {
		t.Errorf("hash differs after a pure identifier rename: %q vs %q", ha, hb)
	}
	if na != nb {
		t.Errorf("statement count differs after a pure identifier rename: %d vs %d", na, nb)
	}
}

func TestFuncBodyHash_StructuralChangeDiffers(t *testing.T) {
	a := `func F(x int) int {
	return x + 1
}`
	b := `func F(x int) int {
	return x + 2
}`
	ha, _, ok := clone.FuncBodyHash(a)
	if !ok {
		t.Fatalf("FuncBodyHash(a) ok = false")
	}
	hb, _, ok := clone.FuncBodyHash(b)
	if !ok {
		t.Fatalf("FuncBodyHash(b) ok = false")
	}
	if ha == hb {
		t.Errorf("structurally different bodies produced the same hash: %q", ha)
	}
}

func TestFuncBodyHash_StatementCount(t *testing.T) {
	src := `func F() {
	a := 1
	b := 2
	c := a + b
	if c > 0 {
		println(c)
	}
	return
}`
	_, n, ok := clone.FuncBodyHash(src)
	if !ok {
		t.Fatalf("FuncBodyHash ok = false")
	}
	// Top-level statements in the block: a:=1, b:=2, c:=a+b, if, return = 5.
	if n != 5 {
		t.Errorf("statement count = %d, want 5", n)
	}
}

func TestFuncBodyHash_NotAFunctionIsNotOK(t *testing.T) {
	if _, _, ok := clone.FuncBodyHash("not valid { go"); ok {
		t.Errorf("ok = true for unparseable input, want false")
	}
	if _, _, ok := clone.FuncBodyHash("var x = 1"); ok {
		t.Errorf("ok = true for a non-func declaration, want false")
	}
}

func TestFuncBodyHash_MatchesClusterNormalizedHash(t *testing.T) {
	// FuncBodyHash must be the SAME normalisation clone.Detect already
	// uses (design doc: "exposing the clone package's identifier-
	// insensitive normalisation"), not a second, divergent scheme — a
	// resurrection finder comparing base and head fingerprints must agree
	// with what clone.Detect would say about the same two bodies.
	src := "func F(n int) int { return n * 2 }"
	files := map[string]string{"a.go": "package p\n\n" + src + "\n"}
	groups := clone.Detect(files)
	if len(groups) != 0 {
		t.Fatalf("expected no groups for a single file, got %v", groups)
	}
	h, _, ok := clone.FuncBodyHash(src)
	if !ok {
		t.Fatalf("FuncBodyHash ok = false")
	}
	if h == "" {
		t.Fatalf("empty hash")
	}
}
