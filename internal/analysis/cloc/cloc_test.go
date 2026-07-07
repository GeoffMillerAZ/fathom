package cloc_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/cloc"
)

// ─── Go / C-style ────────────────────────────────────────────────────────────

const goSrc = `package foo

// Package comment.

import "fmt"

// Greet says hello.
func Greet(name string) {
	// print it
	fmt.Println("Hello, " + name) // inline comment
}

/* block comment
   spanning two lines */
func noop() {}
`

func TestCount_Go_Blank(t *testing.T) {
	s := cloc.Count("go", goSrc)
	// Lines 2, 4, 6, 12 are blank → 4 blank lines.
	if s.Blank != 4 {
		t.Errorf("Blank = %d, want 4", s.Blank)
	}
}

func TestCount_Go_Comments(t *testing.T) {
	s := cloc.Count("go", goSrc)
	// Lines 3 (//Package), 7 (//Greet), 9(//print), 13(/*block), 14(spanning) → 5 comment-only lines.
	if s.Comment != 5 {
		t.Errorf("Comment = %d, want 5", s.Comment)
	}
}

func TestCount_Go_Code(t *testing.T) {
	s := cloc.Count("go", goSrc)
	// import, func Greet, fmt.Println (has inline comment but is code), }, closing /* */, func noop → 6 code lines.
	if s.Code != 6 {
		t.Errorf("Code = %d, want 6", s.Code)
	}
}

func TestCount_Go_Total(t *testing.T) {
	s := cloc.Count("go", goSrc)
	wantLines := 4 + 5 + 6
	if s.Total() != wantLines {
		t.Errorf("Total = %d, want %d (blank=%d comment=%d code=%d)",
			s.Total(), wantLines, s.Blank, s.Comment, s.Code)
	}
}

func TestCount_Go_LineCommentOnly(t *testing.T) {
	src := "// just a comment\n"
	s := cloc.Count("go", src)
	if s.Comment != 1 || s.Code != 0 || s.Blank != 0 {
		t.Errorf("unexpected: %+v", s)
	}
}

func TestCount_Go_InlineCommentIsCode(t *testing.T) {
	src := "x := 1 // assign\n"
	s := cloc.Count("go", src)
	if s.Code != 1 || s.Comment != 0 {
		t.Errorf("inline comment line should be code: %+v", s)
	}
}

func TestCount_Go_BlockCommentMultiLine(t *testing.T) {
	src := "/* line1\nline2\nline3 */\n"
	s := cloc.Count("go", src)
	if s.Comment != 3 {
		t.Errorf("Comment = %d, want 3", s.Comment)
	}
}

func TestCount_Go_EmptySource(t *testing.T) {
	s := cloc.Count("go", "")
	if s.Blank != 0 || s.Comment != 0 || s.Code != 0 {
		t.Errorf("expected all zeros for empty source, got %+v", s)
	}
}

func TestCount_Go_Determinism(t *testing.T) {
	a := cloc.Count("go", goSrc)
	b := cloc.Count("go", goSrc)
	if a != b {
		t.Errorf("non-deterministic: %+v vs %+v", a, b)
	}
}

// ─── JavaScript / TypeScript ─────────────────────────────────────────────────

const jsSrc = `// top comment

function hello(name) {
  // say hi
  console.log("Hello " + name); // inline
}

/* block
   comment */
const x = 42;
`

func TestCount_JS_Counts(t *testing.T) {
	s := cloc.Count("js", jsSrc)
	if s.Blank != 2 {
		t.Errorf("JS Blank = %d, want 2", s.Blank)
	}
	if s.Comment != 4 {
		t.Errorf("JS Comment = %d, want 4", s.Comment)
	}
	if s.Code != 4 {
		t.Errorf("JS Code = %d, want 4", s.Code)
	}
}

func TestCount_TS_SameAsJS(t *testing.T) {
	a := cloc.Count("js", jsSrc)
	b := cloc.Count("ts", jsSrc)
	if a != b {
		t.Errorf("JS and TS should produce same counts, got %+v vs %+v", a, b)
	}
}

// ─── Python ──────────────────────────────────────────────────────────────────

const pySrc = `# module comment

def greet(name):
    """Say hello.

    Extended docstring.
    """
    # inline comment
    print("Hello, " + name)  # trailing

class Foo:
    pass
`

func TestCount_Python_Blank(t *testing.T) {
	s := cloc.Count("py", pySrc)
	// Lines 2 and 10 are blank outside any docstring.
	// Line 5 (blank inside triple-quoted docstring) is counted as comment.
	if s.Blank != 2 {
		t.Errorf("Py Blank = %d, want 2", s.Blank)
	}
}

func TestCount_Python_Comments(t *testing.T) {
	s := cloc.Count("py", pySrc)
	// Line 1 (#), line 4 ("""), line 5 (blank inside docstring — counted as comment), line 6 (Extended), line 7 ("""), line 8 (#)
	// = 6 comment lines
	if s.Comment != 6 {
		t.Errorf("Py Comment = %d, want 6", s.Comment)
	}
}

func TestCount_Python_Code(t *testing.T) {
	s := cloc.Count("py", pySrc)
	// def greet, print, class Foo, pass → 4 code lines
	if s.Code != 4 {
		t.Errorf("Py Code = %d, want 4", s.Code)
	}
}

func TestCount_Python_HashComment(t *testing.T) {
	src := "# comment\nx = 1\n"
	s := cloc.Count("python", src)
	if s.Comment != 1 || s.Code != 1 {
		t.Errorf("unexpected: %+v", s)
	}
}

func TestCount_Python_Determinism(t *testing.T) {
	a := cloc.Count("py", pySrc)
	b := cloc.Count("py", pySrc)
	if a != b {
		t.Errorf("non-deterministic: %+v vs %+v", a, b)
	}
}

// ─── Unknown language (generic) ──────────────────────────────────────────────

func TestCount_Unknown_NoComments(t *testing.T) {
	src := "\nfoo bar\nbaz\n"
	s := cloc.Count("ruby", src)
	if s.Blank != 1 || s.Code != 2 || s.Comment != 0 {
		t.Errorf("generic: %+v", s)
	}
}

// ─── Aggregate ───────────────────────────────────────────────────────────────

func TestAggregate_Basic(t *testing.T) {
	m := map[string][]cloc.Stats{
		"go": {
			{Blank: 1, Comment: 2, Code: 3},
			{Blank: 4, Comment: 5, Code: 6},
		},
		"py": {
			{Blank: 10, Comment: 0, Code: 5},
		},
	}
	agg := cloc.Aggregate(m)
	goT := agg["go"]
	if goT.Blank != 5 || goT.Comment != 7 || goT.Code != 9 {
		t.Errorf("go aggregate wrong: %+v", goT)
	}
	pyT := agg["py"]
	if pyT.Blank != 10 || pyT.Comment != 0 || pyT.Code != 5 {
		t.Errorf("py aggregate wrong: %+v", pyT)
	}
}

func TestAggregate_Empty(t *testing.T) {
	agg := cloc.Aggregate(nil)
	if len(agg) != 0 {
		t.Errorf("expected empty map, got %v", agg)
	}
}

func TestAggregate_Determinism(t *testing.T) {
	m := map[string][]cloc.Stats{
		"go": {{Blank: 1, Comment: 2, Code: 3}},
	}
	a := cloc.Aggregate(m)
	b := cloc.Aggregate(m)
	if a["go"] != b["go"] {
		t.Errorf("non-deterministic")
	}
}
