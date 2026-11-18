package deps_test

import (
	"context"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func TestPythonDependencyReader(t *testing.T) {
	r := &deps.Reader{}
	repo := os.DirFS("testdata/python")
	files, err := r.ReadFiles(t.Context(), repo, []deps.Input{{Path: "app/main.py"}})
	if err != nil {
		t.Fatal(err)
	}
	got := files[0]
	if got.Dialect != "python" || got.Complete {
		t.Fatalf("expected incomplete inferred Python: %+v", got)
	}

	type binding struct {
		raw, to string
		line    int
	}
	var edges []binding
	for _, e := range got.Edges {
		if e.Resolution == deps.ResolvedFile {
			edges = append(edges, binding{e.Raw, e.To, e.Line})
		}
	}
	for _, want := range []binding{
		{"app.core", "app/__init__.py", 2},
		{"app.core", "app/core.py", 2},
		{"top", "top.py", 2},
		{".core", "app/core.py", 3},
		{".", "app/__init__.py", 7},
		{".", "app/child.py", 7},
		{"app.core", "app/core.py", 8},
		{"app.child", "app/child.py", 9},
	} {
		if !slices.Contains(edges, want) {
			t.Errorf("missing %+v in %+v", want, edges)
		}
	}
	for _, line := range []int{10, 11, 12} {
		if !slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool {
			return u.Code == deps.UnknownDynamicImport && u.Line == line
		}) {
			t.Errorf("missing dynamic import evidence at %d: %+v", line, got.Unknowns)
		}
	}
	if json := string(asJSON(t, got)); strings.Contains(json, "ignored") || strings.Contains(json, "stray") ||
		strings.Contains(json, "/private/") {
		t.Fatalf("non-construct or unsafe operand leaked: %s", json)
	}
}

func TestPythonDependencyGaps(t *testing.T) {
	r, err := deps.New(deps.TrustUnmeasured("python"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, from, src string
		repo            fstest.MapFS
		code            deps.UnknownCode
		target          string
	}{
		{"namespace", "entry.py", "import group.part", fstest.MapFS{"group/part.py": {}}, deps.UnknownMissingConfig, "group/part.py"},
		{"namespace ancestor", "ns/regular/main.py", "from .leaf import value", fstest.MapFS{
			"ns/regular/__init__.py": {},
			"ns/regular/leaf.py":     {},
		}, deps.UnknownMissingConfig, "ns/regular/leaf.py"},
		{"relative namespace", "group/main.py", "from . import part", fstest.MapFS{"group/part.py": {}}, deps.UnknownMissingConfig, "group/part.py"},
		{"beyond root", "entry.py", "from .. import side", fstest.MapFS{"side.py": {}}, deps.UnknownMissingConfig, ""},
		{"built-in collision", "entry.py", "import sys", fstest.MapFS{"sys.py": {}}, deps.UnknownMissingConfig, "sys.py"},
		{"src root", "entry.py", "import group.part", fstest.MapFS{"src/group/__init__.py": {}, "src/group/part.py": {}}, deps.UnknownMissingConfig, "src/group/part.py"},
		{"unavailable", "entry.py", "import from_environment", fstest.MapFS{}, deps.UnknownMissingConfig, ""},
		{"malformed", "entry.py", "from broken import (", fstest.MapFS{}, deps.UnknownParseError, ""},
		{"hook", "entry.py", "sys.meta_path.append(hook)", fstest.MapFS{}, deps.UnknownDynamicImport, ""},
		{"import inside interpolation", "entry.py", "f'{__import__(key)}'", fstest.MapFS{}, deps.UnknownDynamicImport, ""},
		{"star package", "entry.py", "from app import *", fstest.MapFS{"app/__init__.py": {}}, deps.UnknownDynamicImport, "app/__init__.py"},
		{"package attribute", "entry.py", "from app import child", fstest.MapFS{"app/__init__.py": {}, "app/child.py": {}}, deps.UnknownDynamicImport, "app/child.py"},
		{"opaque initializer", "entry.py", "import app.child", fstest.MapFS{
			"app/__init__.py": {Data: []byte("__path__ = destination")},
			"app/child.py":    {},
		}, deps.UnknownMissingConfig, "app/child.py"},
		{"unsupported native module", "entry.py", "import app", fstest.MapFS{"app.so": {}, "app.py": {}}, deps.UnknownMissingConfig, "app.py"},
		{"symlink precedence", "entry.py", "import app", fstest.MapFS{"app/__init__.py": {Mode: fs.ModeSymlink}, "app.py": {}}, deps.UnknownMissingConfig, "app.py"},
		{"conditional script root", "tools/main.py", "import sibling", fstest.MapFS{"tools/sibling.py": {}}, deps.UnknownMissingConfig, "tools/sibling.py"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.ReadFile(t.Context(), tc.repo, deps.Input{Path: tc.from}, []byte(tc.src))
			if err != nil || got.Complete || !slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == tc.code }) {
				t.Fatalf("expected %s gap: %+v, %v", tc.code, got, err)
			}
			if tc.target != "" && !slices.ContainsFunc(got.Edges, func(e deps.Edge) bool { return e.To == tc.target }) {
				t.Fatalf("lost detectable target %s: %+v", tc.target, got.Edges)
			}
		})
	}
}

type pythonResolver struct{}

func (pythonResolver) Resolve(ctx context.Context, req deps.ResolveRequest) deps.ResolveResult {
	_, err := fs.Stat(req.Repo, "canonical.py")
	if err == nil && ctx.Err() == nil && req.From == "input.text" && req.Raw == "custom.source" &&
		req.Kind == deps.KindImport && req.Line == 1 {
		return deps.ResolveResult{Edges: []deps.Edge{{Kind: deps.KindImport, To: "canonical.py", Resolution: deps.ResolvedFile}}}
	}
	return deps.ResolveResult{Unknown: &deps.Unknown{Code: deps.UnknownMissingConfig}}
}

func TestPythonDependencyResolver(t *testing.T) {
	r, err := deps.New(deps.WithResolver("python", pythonResolver{}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), fstest.MapFS{"canonical.py": {}}, deps.Input{Path: "input.text", Dialect: "python"}, []byte("from custom.source import name as alias"))
	if err != nil || len(got.Edges) != 1 || got.Edges[0].To != "canonical.py" ||
		got.Edges[0].Provenance != deps.ProvenanceExtension || got.Edges[0].Raw != "custom.source" {
		t.Fatalf("Python resolver did not receive the detected module: %+v, %v", got, err)
	}
}

func TestPythonDependencyPackagePrecedence(t *testing.T) {
	repo := fstest.MapFS{
		"group.py":                      {},
		"group/__init__.py":             {},
		"group/part.py":                 {},
		"group/nonpackage/unrelated.py": {},
		"nonpackage.py":                 {},
		"nonpackage/unrelated.py":       {},
	}
	r := &deps.Reader{}
	got, err := r.ReadFile(t.Context(), repo, deps.Input{Path: "main.py"}, []byte("import group.part; import nonpackage"))
	if err != nil {
		t.Fatal(err)
	}
	var targets []string
	for _, e := range got.Edges {
		if e.Resolution == deps.ResolvedFile {
			targets = append(targets, e.To)
		}
	}
	if want := []string{"group/__init__.py", "group/part.py", "nonpackage.py"}; !slices.Equal(targets, want) {
		t.Fatalf("package and module precedence: %v, want %v", targets, want)
	}
}

func TestPythonDependencyAdmission(t *testing.T) {
	r := &deps.Reader{}
	repo := fstest.MapFS{"helper.py": {}}
	in := deps.Input{Path: "main.py"}
	got, err := r.ReadFile(t.Context(), repo, in, []byte("import helper"))
	if err != nil || !got.Measured || got.Trusted || !got.Complete || len(got.Edges) != 1 {
		t.Fatalf("default reader must use the passing built-in admission: %+v, %v", got, err)
	}
	got, err = r.ReadFile(t.Context(), repo, in, []byte("__import__('helper')"))
	if err != nil || !got.Measured || got.Trusted || got.Complete ||
		!slices.ContainsFunc(got.Unknowns, func(u deps.Unknown) bool { return u.Code == deps.UnknownDynamicImport }) {
		t.Fatalf("dynamic import must remain incomplete after admission: %+v, %v", got, err)
	}
}
