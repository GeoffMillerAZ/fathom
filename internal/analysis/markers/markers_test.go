package markers_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/markers"
)

const goSrc = `package p

// TODO: implement validation
func Validate() {}

// FIXME [AUTH-42]: broken auth
func Auth() {}

// HACK #99 workaround
func Workaround() {}

// plain comment
func Fine() {}
`

const multiKindSrc = `package p

// XXX this is bad
// BUG overflow possible
// DEPRECATED use NewFoo instead
`

func TestScan_GoComments(t *testing.T) {
	ms := markers.Scan("file.go", goSrc)
	if len(ms) != 3 {
		t.Fatalf("expected 3 markers, got %d: %v", len(ms), ms)
	}
}

func TestScan_Kinds(t *testing.T) {
	ms := markers.Scan("f.go", multiKindSrc)
	kindSet := make(map[markers.Kind]bool)
	for _, m := range ms {
		kindSet[m.Kind] = true
	}
	for _, want := range []markers.Kind{markers.KindXXX, markers.KindBUG, markers.KindDEPRECATED} {
		if !kindSet[want] {
			t.Errorf("missing kind %s", want)
		}
	}
}

func TestScan_JiraTicket(t *testing.T) {
	ms := markers.Scan("f.go", goSrc)
	var fixme *markers.Marker
	for i := range ms {
		if ms[i].Kind == markers.KindFIXME {
			fixme = &ms[i]
			break
		}
	}
	if fixme == nil {
		t.Fatal("FIXME not found")
	}
	if fixme.Ticket != "AUTH-42" {
		t.Errorf("want ticket AUTH-42, got %q", fixme.Ticket)
	}
}

func TestScan_GitHubTicket(t *testing.T) {
	ms := markers.Scan("f.go", goSrc)
	var hack *markers.Marker
	for i := range ms {
		if ms[i].Kind == markers.KindHACK {
			hack = &ms[i]
			break
		}
	}
	if hack == nil {
		t.Fatal("HACK not found")
	}
	if hack.Ticket != "#99" {
		t.Errorf("want ticket #99, got %q", hack.Ticket)
	}
}

func TestScan_NoTicket(t *testing.T) {
	ms := markers.Scan("f.go", goSrc)
	var todo *markers.Marker
	for i := range ms {
		if ms[i].Kind == markers.KindTODO {
			todo = &ms[i]
			break
		}
	}
	if todo == nil {
		t.Fatal("TODO not found")
	}
	if todo.Ticket != "" {
		t.Errorf("expected no ticket, got %q", todo.Ticket)
	}
}

func TestScan_LineNumbers(t *testing.T) {
	ms := markers.Scan("f.go", goSrc)
	for _, m := range ms {
		if m.Line <= 0 {
			t.Errorf("invalid line number %d for marker %s", m.Line, m.Kind)
		}
	}
}

func TestScan_CaseInsensitive(t *testing.T) {
	src := "// todo: something\n// fixme: something else\n"
	ms := markers.Scan("f.go", src)
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers (case-insensitive), got %d", len(ms))
	}
}

func TestScan_Empty(t *testing.T) {
	ms := markers.Scan("f.go", "")
	if len(ms) != 0 {
		t.Fatalf("expected empty results, got %v", ms)
	}
}

func TestScan_Sorted(t *testing.T) {
	ms := markers.Scan("f.go", goSrc)
	for i := 1; i < len(ms); i++ {
		if ms[i].Line < ms[i-1].Line {
			t.Errorf("not sorted: line %d before line %d", ms[i-1].Line, ms[i].Line)
		}
	}
}

func TestScan_Determinism(t *testing.T) {
	var prev []markers.Marker
	for i := 0; i < 5; i++ {
		ms := markers.Scan("f.go", goSrc)
		if prev != nil {
			if len(ms) != len(prev) {
				t.Fatalf("non-deterministic length at iteration %d", i)
			}
			for j := range ms {
				if ms[j] != prev[j] {
					t.Fatalf("non-deterministic output at iteration %d", i)
				}
			}
		}
		prev = ms
	}
}

func TestScan_FileField(t *testing.T) {
	ms := markers.Scan("myfile.py", "# TODO fix this\n")
	if len(ms) != 1 {
		t.Fatal("expected 1 marker")
	}
	if ms[0].File != "myfile.py" {
		t.Errorf("expected file=myfile.py, got %q", ms[0].File)
	}
}

func TestScan_NonGoSources(t *testing.T) {
	// Python-style
	pySrc := "# TODO [PY-99]: add types\n# FIXME this breaks\n"
	ms := markers.Scan("script.py", pySrc)
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers in Python source, got %d", len(ms))
	}
	if ms[0].Ticket != "PY-99" {
		t.Errorf("expected ticket PY-99, got %q", ms[0].Ticket)
	}
}
