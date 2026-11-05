package langroute

import "testing"

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
