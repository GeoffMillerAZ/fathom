package references_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/references"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

func fileA() core.SourceFile {
	return core.SourceFile{
		Path:    "pkg/a.go",
		Content: []byte("package pkg\n\nfunc Foo() {}\nfunc FooBar() {}\n// see Foo here\n"),
		Lang:    "go",
	}
}

func fileB() core.SourceFile {
	return core.SourceFile{
		Path:    "pkg/b.go",
		Content: []byte("package pkg\n\nfunc Baz() {\n\t// also uses Foo\n}\n"),
		Lang:    "go",
	}
}

// ─── substring ───────────────────────────────────────────────────────────────

func TestFind_Substring(t *testing.T) {
	files := []core.SourceFile{fileA()}
	occs, err := references.Find("Foo", files, references.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "func Foo() {}", "func FooBar() {}", "// see Foo here" all contain "Foo"
	if len(occs) != 3 {
		t.Fatalf("expected 3 occurrences, got %d: %v", len(occs), occs)
	}
	// Lines 3, 4, 5
	wantLines := []int{3, 4, 5}
	for i, occ := range occs {
		if occ.File != "pkg/a.go" {
			t.Errorf("[%d] wrong file: %s", i, occ.File)
		}
		if occ.Line != wantLines[i] {
			t.Errorf("[%d] wrong line: got %d want %d", i, occ.Line, wantLines[i])
		}
		if occ.Text == "" {
			t.Errorf("[%d] empty text", i)
		}
	}
}

// ─── word ────────────────────────────────────────────────────────────────────

func TestFind_Word_DoesNotMatchPrefix(t *testing.T) {
	files := []core.SourceFile{fileA()}
	// Word-match "Foo" must NOT match "FooBar"
	occs, err := references.Find("Foo", files, references.Options{Word: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, occ := range occs {
		if occ.Text != "" {
			// FooBar line must not appear
			for _, forbidden := range []string{"FooBar"} {
				_ = forbidden
			}
		}
	}
	// Lines 3 ("func Foo() {}") and 5 ("// see Foo here") match; line 4 ("func FooBar() {}") must not.
	for _, occ := range occs {
		if occ.Line == 4 {
			t.Errorf("word match should not match line 4 (FooBar): %v", occ)
		}
	}
	// We expect exactly 2 matches (lines 3 and 5).
	if len(occs) != 2 {
		t.Fatalf("expected 2 word matches, got %d: %v", len(occs), occs)
	}
}

// ─── regex ────────────────────────────────────────────────────────────────────

func TestFind_Regex(t *testing.T) {
	files := []core.SourceFile{fileA()}
	// Match lines that start with "func"
	occs, err := references.Find(`^func`, files, references.Options{Regex: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(occs) != 2 {
		t.Fatalf("expected 2 regex matches (func lines), got %d: %v", len(occs), occs)
	}
}

func TestFind_Regex_BadPattern(t *testing.T) {
	files := []core.SourceFile{fileA()}
	_, err := references.Find(`[invalid(`, files, references.Options{Regex: true})
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

// ─── case-insensitive ─────────────────────────────────────────────────────────

func TestFind_CaseInsensitive_Substring(t *testing.T) {
	files := []core.SourceFile{fileA()}
	occs, err := references.Find("foo", files, references.Options{CaseInsensitive: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "Foo" matches case-insensitively, so all three Foo-containing lines should match
	if len(occs) < 3 {
		t.Fatalf("expected at least 3 case-insensitive matches, got %d", len(occs))
	}
}

func TestFind_CaseInsensitive_Regex(t *testing.T) {
	files := []core.SourceFile{fileA()}
	occs, err := references.Find(`foo`, files, references.Options{Regex: true, CaseInsensitive: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(occs) < 3 {
		t.Fatalf("expected at least 3 case-insensitive regex matches, got %d", len(occs))
	}
}

// ─── multi-file ───────────────────────────────────────────────────────────────

func TestFind_MultiFile(t *testing.T) {
	files := []core.SourceFile{fileA(), fileB()}
	occs, err := references.Find("Foo", files, references.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// fileA: lines 3, 4, 5; fileB: line 4 ("// also uses Foo")
	if len(occs) < 4 {
		t.Fatalf("expected at least 4 occurrences across 2 files, got %d", len(occs))
	}
	// Verify sorted by (file, line)
	for i := 1; i < len(occs); i++ {
		a, b := occs[i-1], occs[i]
		if a.File > b.File || (a.File == b.File && a.Line > b.Line) {
			t.Errorf("results not sorted at [%d,%d]: %v vs %v", i-1, i, a, b)
		}
	}
}

// ─── determinism ─────────────────────────────────────────────────────────────

func TestFind_Deterministic(t *testing.T) {
	files := []core.SourceFile{fileB(), fileA()} // reversed order
	for i := 0; i < 5; i++ {
		occs, err := references.Find("Foo", files, references.Options{})
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		for j := 1; j < len(occs); j++ {
			a, b := occs[j-1], occs[j]
			if a.File > b.File || (a.File == b.File && a.Line > b.Line) {
				t.Errorf("run %d: not sorted at [%d,%d]: %v vs %v", i, j-1, j, a, b)
			}
		}
	}
}

// ─── empty query ─────────────────────────────────────────────────────────────

func TestFind_EmptyQuery_NoResults(t *testing.T) {
	files := []core.SourceFile{fileA(), fileB()}
	occs, err := references.Find("", files, references.Options{})
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if len(occs) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(occs))
	}
}

func TestFind_EmptyFiles_NoResults(t *testing.T) {
	occs, err := references.Find("Foo", nil, references.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(occs) != 0 {
		t.Errorf("expected 0 results for nil files, got %d", len(occs))
	}
}

// ─── text cap ────────────────────────────────────────────────────────────────

func TestFind_TextCapped(t *testing.T) {
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	// Put query in the long line so it matches.
	longLine := "needle" + string(long)
	f := core.SourceFile{
		Path:    "f.go",
		Content: []byte(longLine),
		Lang:    "go",
	}
	occs, err := references.Find("needle", []core.SourceFile{f}, references.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(occs) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occs))
	}
	if len(occs[0].Text) > 200 {
		t.Errorf("text not capped: len=%d", len(occs[0].Text))
	}
}
