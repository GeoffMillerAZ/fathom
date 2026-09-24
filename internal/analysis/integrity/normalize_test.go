package integrity

import "testing"

// TestNormalizedBodyHash_RenameOnlyEqual is CV-8's own anchor: a pure
// rename of every local identifier — same structure, same literals, same
// operators, different spelling — must leave the normalized hash
// unchanged.
func TestNormalizedBodyHash_RenameOnlyEqual(t *testing.T) {
	original := []byte(`package sample

import "testing"

func TestOriginal(t *testing.T) {
	x := 1
	y := 2
	if x != y {
		t.Fatalf("got %d want %d", x, y)
	}
}
`)
	renamed := []byte(`package sample

import "testing"

func TestRenamed(tt *testing.T) {
	a := 1
	b := 2
	if a != b {
		tt.Fatalf("got %d want %d", a, b)
	}
}
`)

	h1, ok1 := NormalizedBodyHash(original, "TestOriginal")
	if !ok1 {
		t.Fatalf("NormalizedBodyHash(original) not found")
	}
	h2, ok2 := NormalizedBodyHash(renamed, "TestRenamed")
	if !ok2 {
		t.Fatalf("NormalizedBodyHash(renamed) not found")
	}
	if h1 != h2 {
		t.Errorf("a pure rename changed the hash: %s != %s", h1, h2)
	}
	if h1 == "" {
		t.Errorf("hash is empty")
	}
}

// TestNormalizedBodyHash_OperatorChangeDiffers proves the hash is NOT
// insensitive to everything — inverting a comparison operator (the i28
// shape: a renamed test whose assertion was also inverted) must change
// the hash, so Check correctly fails to match it as a "renamed" pair.
func TestNormalizedBodyHash_OperatorChangeDiffers(t *testing.T) {
	original := []byte(`package sample

import "testing"

func TestA(t *testing.T) {
	err := doThing()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
`)
	inverted := []byte(`package sample

import "testing"

func TestB(t *testing.T) {
	err := doThing()
	if err == nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
`)
	h1, _ := NormalizedBodyHash(original, "TestA")
	h2, _ := NormalizedBodyHash(inverted, "TestB")
	if h1 == h2 {
		t.Errorf("an inverted operator produced the SAME hash: %s", h1)
	}
}

// TestNormalizedBodyHash_LiteralChangeDiffers: literals are kept verbatim
// (design §1.3), so changing one must change the hash.
func TestNormalizedBodyHash_LiteralChangeDiffers(t *testing.T) {
	a := []byte(`package sample

func TestA(t *testing.T) {
	x := 1
	_ = x
}
`)
	b := []byte(`package sample

func TestA(t *testing.T) {
	x := 2
	_ = x
}
`)
	h1, _ := NormalizedBodyHash(a, "TestA")
	h2, _ := NormalizedBodyHash(b, "TestA")
	if h1 == h2 {
		t.Errorf("changing a literal produced the SAME hash: %s", h1)
	}
}

// TestNormalizedBodyHash_NotFound: a symbol that is not a top-level Test*
// func with a body is reported not found, never a fabricated hash.
func TestNormalizedBodyHash_NotFound(t *testing.T) {
	src := []byte(`package sample

func TestA(t *testing.T) {}
`)
	if _, ok := NormalizedBodyHash(src, "TestMissing"); ok {
		t.Errorf("NormalizedBodyHash found a symbol that does not exist")
	}
	if _, ok := NormalizedBodyHash([]byte("not valid go{{{"), "TestA"); ok {
		t.Errorf("NormalizedBodyHash succeeded on unparseable source")
	}
}
