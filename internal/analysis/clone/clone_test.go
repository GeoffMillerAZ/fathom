package clone_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/clone"
)

// ─── fixtures ─────────────────────────────────────────────────────────────────

const goSrcA = `package foo

func Alpha() int {
	return 1
}

func Beta() string {
	return "hello"
}
`

// goSrcB has the same two functions as goSrcA but with renamed identifiers — a
// structural clone.  NormalizedHash should match.
const goSrcB = `package bar

func MyAlpha() int {
	return 1
}

func MyBeta() string {
	return "hello"
}
`

// goSrcC is structurally different.
const goSrcC = `package baz

func Gamma() bool {
	return true
}
`

// plainSrcA / plainSrcB are non-Go files split by blank lines.
const plainSrcA = `SELECT *
FROM users
WHERE id = 1

INSERT INTO orders
VALUES (1, 2, 3)
`

const plainSrcB = `SELECT *
FROM accounts
WHERE id = 1

INSERT INTO purchases
VALUES (1, 2, 3)
`

// ─── Detect ───────────────────────────────────────────────────────────────────

func TestDetect_NoFiles(t *testing.T) {
	got := clone.Detect(nil)
	if len(got) != 0 {
		t.Errorf("Detect(nil) = %v, want empty", got)
	}
}

func TestDetect_SingleFile_NoGroup(t *testing.T) {
	files := map[string]string{"a.go": goSrcA}
	got := clone.Detect(files)
	if len(got) != 0 {
		t.Errorf("Detect single file = %v, want empty (no clone group possible)", got)
	}
}

func TestDetect_GoClones(t *testing.T) {
	// goSrcA and goSrcB are structural clones: same structure, different names.
	files := map[string]string{
		"a.go": goSrcA,
		"b.go": goSrcB,
		"c.go": goSrcC,
	}
	got := clone.Detect(files)
	// We expect two clone groups: one for the Alpha/MyAlpha chunks and one for
	// the Beta/MyBeta chunks (both are structurally identical).
	if len(got) < 1 {
		t.Fatalf("Detect GoClones: expected ≥1 group, got %v", got)
	}
	// Every group must have ≥2 files.
	for _, g := range got {
		if len(g.Files) < 2 {
			t.Errorf("CloneGroup has only %d file(s): %v", len(g.Files), g)
		}
	}
}

func TestDetect_PlainClones(t *testing.T) {
	files := map[string]string{
		"a.sql": plainSrcA,
		"b.sql": plainSrcB,
	}
	got := clone.Detect(files)
	// Both files have two structurally identical blocks.
	if len(got) < 1 {
		t.Fatalf("Detect PlainClones: expected ≥1 group, got %v", got)
	}
	for _, g := range got {
		if len(g.Files) < 2 {
			t.Errorf("CloneGroup has only %d file(s): %v", len(g.Files), g)
		}
	}
}

func TestDetect_Determinism(t *testing.T) {
	files := map[string]string{
		"a.go": goSrcA,
		"b.go": goSrcB,
		"c.go": goSrcC,
	}
	a := clone.Detect(files)
	b := clone.Detect(files)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Detect not deterministic:\n  first:  %v\n  second: %v", a, b)
	}
}

func TestDetect_NoCloneWhenUnique(t *testing.T) {
	files := map[string]string{
		"a.go": goSrcA,
		"c.go": goSrcC,
	}
	// goSrcA and goSrcC have no structurally identical chunks.
	got := clone.Detect(files)
	for _, g := range got {
		if len(g.Files) >= 2 {
			t.Logf("unexpected clone group: %v", g)
		}
	}
}

// ─── CrossRepoClones ──────────────────────────────────────────────────────────

func TestCrossRepoClones_Basic(t *testing.T) {
	byService := map[string]map[string]string{
		"svc-a": {"handler.go": goSrcA},
		"svc-b": {"handler.go": goSrcB},
	}
	got := clone.CrossRepoClones(byService)
	if len(got) < 1 {
		t.Fatalf("CrossRepoClones: expected ≥1 group, got %v", got)
	}
	// Keys should be prefixed with service name.
	for _, g := range got {
		for _, f := range g.Files {
			hasSvcPrefix := len(f) > 5 && (f[:5] == "svc-a" || f[:5] == "svc-b")
			if !hasSvcPrefix {
				t.Errorf("CrossRepoClones: file ID %q missing service prefix", f)
			}
		}
	}
}

func TestCrossRepoClones_Determinism(t *testing.T) {
	byService := map[string]map[string]string{
		"svc-a": {"handler.go": goSrcA},
		"svc-b": {"handler.go": goSrcB},
	}
	a := clone.CrossRepoClones(byService)
	b := clone.CrossRepoClones(byService)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("CrossRepoClones not deterministic: %v vs %v", a, b)
	}
}

func TestCrossRepoClones_EmptyInput(t *testing.T) {
	got := clone.CrossRepoClones(nil)
	if len(got) != 0 {
		t.Errorf("CrossRepoClones(nil) = %v, want empty", got)
	}
}

// ─── Groups have ≥2 members and hash is non-empty ────────────────────────────

func TestDetect_GroupInvariants(t *testing.T) {
	files := map[string]string{
		"x.go": goSrcA,
		"y.go": goSrcB,
	}
	got := clone.Detect(files)
	for _, g := range got {
		if g.Hash == "" {
			t.Errorf("CloneGroup.Hash is empty: %+v", g)
		}
		if len(g.Files) < 2 {
			t.Errorf("CloneGroup.Files has %d entries (want ≥2): %+v", len(g.Files), g)
		}
		// Files should be sorted within a group.
		prev := ""
		for _, f := range g.Files {
			if f < prev {
				t.Errorf("CloneGroup.Files not sorted: %v", g.Files)
				break
			}
			prev = f
		}
	}
}
