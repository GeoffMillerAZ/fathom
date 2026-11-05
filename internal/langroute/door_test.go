package langroute

import (
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestDialectOfAndExtractFileDoc(t *testing.T) {
	r, err := NewRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cases := []struct{ path, src, dialect string }{
		{"main.tf", "variable \"x\" {}\n", "terraform"},
		{"a.go", "package a\n", "go"},
		{"a.py", "def f():\n    pass\n", "python"},
		{"Dockerfile", "FROM alpine\n", "dockerfile"},
		{"d.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n", "kubernetes"},
		{".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  a:\n    runs-on: x\n    steps:\n    - run: echo\n", "github-actions"},
		{"docker-compose.yml", "services:\n  a:\n    image: x\n", "docker-compose"},
		{"app.ts", "export function f() {}\n", "typescript"},
	}
	for _, c := range cases {
		ex, err := r.ExtractFileDoc("_", nil, c.path, []byte(c.src))
		if err != nil || ex.Skipped != "" {
			t.Fatalf("%s: skipped %q err %v", c.path, ex.Skipped, err)
		}
		if got := DialectOf(ex.Extractor, ex.ToolKind, c.path, ex.Nodes); got != c.dialect {
			t.Errorf("%s: dialect %q, want %q", c.path, got, c.dialect)
		}
	}
	if got := DialectOf(nil, "", "unknown.zzz", nil); got != "" {
		t.Errorf("no signal: %q", got)
	}
}

// ExtractFileDoc's nodes are ExtractFile's, byte for byte: the door adds the
// document result and changes nothing the readings were taken through.
func TestExtractFileDocNodesEqualExtractFile(t *testing.T) {
	r, err := NewRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	files := map[string]string{
		"main.tf":    "resource \"a\" \"b\" {\n  x = var.y\n}\nvariable \"y\" {}\n",
		"rules.yml":  "groups:\n- name: g\n  rules:\n  - alert: A\n    expr: up == 0\n    labels:\n      s: p\n",
		"go.mod":     "module x\n\ngo 1.22\n",
		"README.md":  "# T\n\nsee [a](a.md)\n",
		"a.go":       "package a\nfunc F() {}\n",
		"nginx.conf": "http {\n  server {\n    listen 80;\n  }\n}\n",
	}
	for p, src := range files {
		a, errA := r.ExtractFile(nil, "_", nil, p, []byte(src))
		b, errB := r.ExtractFileDoc("_", nil, p, []byte(src))
		if (errA == nil) != (errB == nil) || a.Skipped != b.Skipped || a.ToolKind != b.ToolKind || len(a.Nodes) != len(b.Nodes) {
			t.Fatalf("%s: %+v / %+v", p, a, b)
		}
		for i := range a.Nodes {
			if a.Nodes[i].Name != b.Nodes[i].Name || a.Nodes[i].Kind != b.Nodes[i].Kind || a.Nodes[i].Line != b.Nodes[i].Line ||
				a.Nodes[i].Signature != b.Nodes[i].Signature || len(a.Nodes[i].Calls) != len(b.Nodes[i].Calls) {
				t.Errorf("%s node %d differs: %+v / %+v", p, i, a.Nodes[i], b.Nodes[i])
			}
		}
	}
}

// A generic reader names itself, and a basename a dialect owns does not label a
// file that dialect's content check refused: the generic reader read it.
func TestDialectOfNamesTheReaderThatReadTheFile(t *testing.T) {
	r, err := NewRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, c := range []struct{ path, src, dialect string }{
		{"data.json", `{"a": 1}`, GenericJSON},
		{"data.toml", "a = 1\n", GenericTOML},
		{"envoy.json", `{"not": "an envoy configuration"}`, GenericJSON},
		{"kong.json", `{"not": "a kong configuration"}`, GenericJSON},
		{"package.json", `{"name": "web", "version": "1.0.0"}`, "package-json"},
	} {
		ex, err := r.ExtractFileDoc("_", nil, c.path, []byte(c.src))
		if err != nil || ex.Skipped != "" {
			t.Fatalf("%s: skipped %q err %v", c.path, ex.Skipped, err)
		}
		if got := DialectOf(ex.Extractor, ex.ToolKind, c.path, ex.Nodes); got != c.dialect {
			t.Errorf("%s: dialect %q, want %q", c.path, got, c.dialect)
		}
	}
}

type spyExtractor struct{ calls int }

func (s *spyExtractor) Extract(string, string, []byte) ([]core.SymbolNode, error) {
	s.calls++
	return nil, nil
}

// ExtractFileDocSteps calls its step after dispatch settles on an extractor and
// before it runs, and again after it returns. A step that fails at either point
// stops the file with that error.
func TestExtractFileDocStepsChecksBetweenStages(t *testing.T) {
	r, err := NewRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	spy := &spyExtractor{}
	r.RegisterExtractor(".spy", spy)
	stop := errors.New("stop")
	for _, c := range []struct {
		name      string
		failAt    int
		wantCalls int
		wantSteps int
	}{
		{"never", 0, 1, 2},
		{"before the extractor", 1, 0, 1},
		{"after the extractor", 2, 1, 2},
	} {
		spy.calls = 0
		steps := 0
		step := func() error {
			steps++
			if steps == c.failAt {
				return stop
			}
			return nil
		}
		ex, err := r.ExtractFileDocSteps(step, "_", nil, "a.spy", []byte("x"))
		if c.failAt == 0 && (err != nil || ex.Skipped != "") {
			t.Errorf("%s: err %v skipped %q", c.name, err, ex.Skipped)
		}
		if c.failAt != 0 && !errors.Is(err, stop) {
			t.Errorf("%s: err %v, want the step's", c.name, err)
		}
		if spy.calls != c.wantCalls || steps != c.wantSteps {
			t.Errorf("%s: extractor ran %d times and step %d, want %d and %d", c.name, spy.calls, steps, c.wantCalls, c.wantSteps)
		}
	}
	// A skipped file reaches no stage boundary.
	steps := 0
	ex, _ := r.ExtractFileDocSteps(func() error { steps++; return stop }, "_", nil, "a.bin", []byte{0, 1, 2, 0})
	if ex.Skipped == "" || steps != 0 {
		t.Errorf("a binary file: skipped %q after %d steps", ex.Skipped, steps)
	}
}

// A header that emits nothing is still read by the dialect its sniff names.
func TestDialectOfContentNamesTheHeaderDialect(t *testing.T) {
	r, err := NewRouter()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, c := range []struct{ src, dialect string }{
		{"#define X 1\n", "c"},
		{"namespace a {}\n", "cpp"},
		{"#import <Foundation/Foundation.h>\n", "objective-c"},
	} {
		ex, err := r.ExtractFileDoc("_", nil, "x/h.h", []byte(c.src))
		if err != nil || ex.Skipped != "" {
			t.Fatalf("%q: skipped %q err %v", c.src, ex.Skipped, err)
		}
		if got := DialectOfContent(ex.Extractor, ex.ToolKind, "x/h.h", []byte(c.src), ex.Nodes); got != c.dialect {
			t.Errorf("%q: dialect %q, want %q", c.src, got, c.dialect)
		}
	}
}
