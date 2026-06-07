package ineffassign_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/ineffassign"
)

const cleanSrc = `package p

func Add(a, b int) int {
	sum := a + b
	return sum
}
`

const ineffSrc = `package p

func Bad() int {
	x := 1
	x = 2  // x was written on line 4 but never read before this re-assign
	return x
}
`

const doubleIneffSrc = `package p

func Double() {
	x := 0
	x = 1
	x = 2
	_ = x
}
`

const compoundSrc = `package p

func Inc() int {
	x := 0
	x += 1 // not ineffectual — compound reads then writes
	return x
}
`

const incDecSrc = `package p

func IncDec() int {
	x := 0
	x++   // reads then writes — not flagged
	return x
}
`

const blankSrc = `package p

func Blank() {
	_ = 42  // blank identifier — skip
}
`

func TestAnalyze_Clean(t *testing.T) {
	findings, err := ineffassign.Analyze("clean.go", cleanSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestAnalyze_Ineffectual(t *testing.T) {
	findings, err := ineffassign.Analyze("bad.go", ineffSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Var != "x" {
		t.Errorf("expected var=x, got %q", f.Var)
	}
	if f.Line != 4 {
		t.Errorf("expected line=4, got %d", f.Line)
	}
}

func TestAnalyze_DoubleIneffectual(t *testing.T) {
	findings, err := ineffassign.Analyze("d.go", doubleIneffSrc)
	if err != nil {
		t.Fatal(err)
	}
	// x:=0 is overwritten by x=1, then x=1 is overwritten by x=2 — 2 findings
	if len(findings) < 1 {
		t.Fatalf("expected at least 1 finding, got %d", len(findings))
	}
}

func TestAnalyze_CompoundAssign(t *testing.T) {
	findings, err := ineffassign.Analyze("c.go", compoundSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("compound assign should not be flagged, got %v", findings)
	}
}

func TestAnalyze_IncDec(t *testing.T) {
	findings, err := ineffassign.Analyze("i.go", incDecSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("inc/dec should not be flagged, got %v", findings)
	}
}

func TestAnalyze_BlankIdentifier(t *testing.T) {
	findings, err := ineffassign.Analyze("b.go", blankSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("blank identifier should not be flagged, got %v", findings)
	}
}

func TestAnalyze_ParseError(t *testing.T) {
	_, err := ineffassign.Analyze("bad.go", "not valid go!!")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAnalyze_Determinism(t *testing.T) {
	var prev []ineffassign.Finding
	for i := 0; i < 5; i++ {
		r, err := ineffassign.Analyze("bad.go", ineffSrc)
		if err != nil {
			t.Fatal(err)
		}
		if prev != nil {
			if len(r) != len(prev) {
				t.Fatalf("non-deterministic length at iteration %d", i)
			}
			for j := range r {
				if r[j] != prev[j] {
					t.Fatalf("non-deterministic output at iteration %d", i)
				}
			}
		}
		prev = r
	}
}

func TestAnalyze_Sorted(t *testing.T) {
	src := `package p
func F() {
	a := 1
	a = 2
	b := 3
	b = 4
	_ = a + b
}
`
	findings, err := ineffassign.Analyze("s.go", src)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(findings); i++ {
		if findings[i].Line < findings[i-1].Line {
			t.Errorf("findings not sorted by line: %d before %d", findings[i-1].Line, findings[i].Line)
		}
	}
}
