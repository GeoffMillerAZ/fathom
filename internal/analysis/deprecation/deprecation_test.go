package deprecation_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/deprecation"
)

// ─── Scan with DefaultDeprecated ─────────────────────────────────────────────

const goSrc = `package foo

import (
	"io/ioutil"
	"os"
)

func readAll(path string) ([]byte, error) {
	return ioutil.ReadAll(nil)
}

func readFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func seek() {
	_ = os.SEEK_SET
}
`

func TestScan_DefaultDeprecated_Flags(t *testing.T) {
	fs := deprecation.Scan("foo.go", goSrc, nil)
	if len(fs) == 0 {
		t.Fatal("expected findings, got none")
	}
}

func TestScan_DefaultDeprecated_Symbols(t *testing.T) {
	fs := deprecation.Scan("foo.go", goSrc, nil)
	symbols := make(map[string]bool)
	for _, f := range fs {
		symbols[f.Symbol] = true
	}
	for _, want := range []string{"ioutil.ReadAll", "ioutil.ReadFile", "os.SEEK_SET"} {
		if !symbols[want] {
			t.Errorf("expected symbol %q in findings", want)
		}
	}
}

func TestScan_CorrectLines(t *testing.T) {
	fs := deprecation.Scan("foo.go", goSrc, nil)
	lineOf := make(map[string]int)
	for _, f := range fs {
		lineOf[f.Symbol] = f.Line
	}
	// ioutil.ReadAll is on line 9.
	if lineOf["ioutil.ReadAll"] != 9 {
		t.Errorf("ioutil.ReadAll on line %d, want 9", lineOf["ioutil.ReadAll"])
	}
	// ioutil.ReadFile is on line 13.
	if lineOf["ioutil.ReadFile"] != 13 {
		t.Errorf("ioutil.ReadFile on line %d, want 13", lineOf["ioutil.ReadFile"])
	}
}

func TestScan_FilePath(t *testing.T) {
	fs := deprecation.Scan("service/foo.go", goSrc, nil)
	for _, f := range fs {
		if f.File != "service/foo.go" {
			t.Errorf("expected File=service/foo.go, got %q", f.File)
		}
	}
}

// ─── Custom deprecated list ───────────────────────────────────────────────────

const customSrc = `package bar

func main() {
	legacyFunc()
	newFunc()
}
`

func TestScan_CustomList_Hit(t *testing.T) {
	fs := deprecation.Scan("bar.go", customSrc, []string{"legacyFunc"})
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	if fs[0].Symbol != "legacyFunc" {
		t.Errorf("symbol = %q, want legacyFunc", fs[0].Symbol)
	}
	if fs[0].Line != 4 {
		t.Errorf("line = %d, want 4", fs[0].Line)
	}
}

func TestScan_CustomList_NoFalsePositive(t *testing.T) {
	fs := deprecation.Scan("bar.go", customSrc, []string{"legacyFunc"})
	for _, f := range fs {
		if f.Symbol == "newFunc" {
			t.Errorf("newFunc should not be flagged")
		}
	}
}

// ─── Multiple symbols same line ───────────────────────────────────────────────

func TestScan_MultipleSymbolsOneLine(t *testing.T) {
	src := "x := ioutil.ReadFile(p); y := ioutil.ReadAll(r)\n"
	fs := deprecation.Scan("x.go", src, []string{"ioutil.ReadFile", "ioutil.ReadAll"})
	if len(fs) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(fs), fs)
	}
	// Both on line 1.
	for _, f := range fs {
		if f.Line != 1 {
			t.Errorf("expected line 1, got %d", f.Line)
		}
	}
}

// ─── Sorted output ────────────────────────────────────────────────────────────

func TestScan_Sorted(t *testing.T) {
	src := "ioutil.ReadFile(a)\nioutil.ReadAll(b)\n"
	fs := deprecation.Scan("f.go", src, []string{"ioutil.ReadFile", "ioutil.ReadAll"})
	for i := 1; i < len(fs); i++ {
		prev, cur := fs[i-1], fs[i]
		if prev.File > cur.File {
			t.Errorf("not sorted by File at %d", i)
		}
		if prev.File == cur.File && prev.Line > cur.Line {
			t.Errorf("not sorted by Line at %d", i)
		}
		if prev.File == cur.File && prev.Line == cur.Line && prev.Symbol > cur.Symbol {
			t.Errorf("not sorted by Symbol at %d", i)
		}
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

func TestScan_EmptySource(t *testing.T) {
	fs := deprecation.Scan("e.go", "", nil)
	if len(fs) != 0 {
		t.Errorf("expected no findings for empty source, got %v", fs)
	}
}

func TestScan_NoMatches(t *testing.T) {
	fs := deprecation.Scan("clean.go", "package clean\nfunc f() {}\n", nil)
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %v", fs)
	}
}

// ─── Determinism ─────────────────────────────────────────────────────────────

func TestScan_Determinism(t *testing.T) {
	a := deprecation.Scan("foo.go", goSrc, nil)
	b := deprecation.Scan("foo.go", goSrc, nil)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}
