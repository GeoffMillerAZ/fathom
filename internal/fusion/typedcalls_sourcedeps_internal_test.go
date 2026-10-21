package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// A dependency may use an import only in a function body. Its package must
// remain type-correct so that the selected caller's edges can be trusted.
func TestTypedSourceDependencyBodyOnlyImport(t *testing.T) {
	base := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		name := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("dep/go.mod", "module example.com/dep\n\ngo 1.23\n")
	write("dep/dep.go", "package dep\nimport \"fmt\"\ntype Worker struct{}\nfunc (Worker) Run() { fmt.Println(\"used\") }\n")
	write("root/go.mod", "module example.com/root\n\ngo 1.23\nrequire example.com/dep v0.0.0\nreplace example.com/dep => ../dep\n")
	write("root/call.go", "package root\nimport \"example.com/dep\"\nfunc helper() {}\nfunc Call() { helper(); (dep.Worker{}).Run() }\n")
	st := store.NewMemoryStore()
	rep, err := EnrichTypedEdges(context.Background(), st, "root", filepath.Join(base, "root"), map[string]bool{"call.go": true})
	if err != nil || rep.PackagesFailed != 0 || rep.PackagesLoaded == 0 || rep.EdgesDirect == 0 {
		t.Fatalf("source dependency failed: report=%+v err=%v", rep, err)
	}
	mods, _ := typedModulesSkipping(filepath.Join(base, "root"), map[string]bool{"call.go": true}, nil)
	gov := governTypedPass(context.Background())
	defer gov.close()
	pkgs, _, err := loadTypedModule(gov, filepath.Join(base, "root"), mods[0], TypedEdgeEnv())
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if dep := pkg.Imports["example.com/dep"]; dep != nil && len(dep.Errors) > 0 {
			t.Fatalf("dependency errors were hidden by selected-root report: %v", dep.Errors)
		}
	}
	write("dep/dep.go", "package dep\nimport \"fmt\"\ntype Worker struct{}\nfunc (Worker) Run() { fmt.Println(missingName) }\n")
	brokenCached, err := EnrichTypedEdges(context.Background(), st, "root", filepath.Join(base, "root"), map[string]bool{"call.go": true})
	if err != nil || brokenCached.PackagesFailed == 0 || brokenCached.PackagesCached != 0 || len(brokenCached.FailedPackages) == 0 ||
		!strings.Contains(brokenCached.FailedPackages[0].Err, "example.com/dep") || !strings.Contains(brokenCached.FailedPackages[0].Err, "missingName") {
		t.Fatalf("cached selected package hid dependency error: report=%+v err=%v", brokenCached, err)
	}
	edges, err := st.TypedEdges(context.Background(), "root")
	if err != nil || len(edges) == 0 {
		t.Fatalf("failed dependency discarded last good rows: edges=%v err=%v", edges, err)
	}
	broken, err := EnrichTypedEdges(context.Background(), store.NewMemoryStore(), "root", filepath.Join(base, "root"), map[string]bool{"call.go": true})
	if err != nil || broken.PackagesFailed == 0 || broken.PackagesCached != 0 {
		t.Fatalf("dependency body type error hidden by selected-root report: %+v", broken)
	}
	write("dep/dep.go", "package dep\nimport \"fmt\"\ntype Worker struct{}\nfunc (Worker) Run() { fmt.Println(\"fixed\") }\n")
	fixed, err := EnrichTypedEdges(context.Background(), st, "root", filepath.Join(base, "root"), map[string]bool{"call.go": true})
	if err != nil || fixed.PackagesFailed != 0 || fixed.PackagesCached != 1 {
		t.Fatalf("fixed dependency did not restore selected cache: report=%+v err=%v", fixed, err)
	}
}

func TestTypedPackageLoadErrorsBoundedDeterministicAndCycleSafe(t *testing.T) {
	root := &packages.Package{ID: "root", Imports: map[string]*packages.Package{}}
	alpha := &packages.Package{ID: "alpha", Errors: []packages.Error{{Msg: "alpha failure"}, {Msg: "alpha failure"}}, Imports: map[string]*packages.Package{}}
	other := &packages.Package{ID: "other", Imports: map[string]*packages.Package{}}
	for i := 0; i < 7; i++ {
		other.Errors = append(other.Errors, packages.Error{Msg: fmt.Sprintf("other failure %d", i)})
	}
	root.Imports["z"] = other
	root.Imports["a"] = alpha
	alpha.Imports["root"] = root
	other.Imports["alpha"] = alpha

	want := []string{
		"alpha: -: alpha failure",
		"other: -: other failure 0",
		"other: -: other failure 1",
		"other: -: other failure 2",
		"other: -: other failure 3",
		"... 3 additional package error(s) omitted",
	}
	for i := 0; i < 10; i++ {
		if got := typedPackageLoadErrors(root); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: errors = %q, want %q", i, got, want)
		}
	}
}

func TestTypedPackageLoadErrorsUTF8Bound(t *testing.T) {
	root := &packages.Package{ID: "root", Imports: map[string]*packages.Package{}}
	dep := &packages.Package{ID: "dep", Errors: []packages.Error{{Msg: strings.Repeat("界", 400)}}}
	root.Imports["dep"] = dep
	got := typedPackageLoadErrors(root)
	if len(got) != 1 {
		t.Fatalf("messages = %q, want one", got)
	}
	if len(got[0]) > maxTypedLoadErrorBytes || !utf8.ValidString(got[0]) || !strings.HasSuffix(got[0], typedLoadTruncatedMarker) || !strings.HasPrefix(got[0], "dep: ") {
		t.Fatalf("UTF-8 diagnostic bound violated: bytes=%d messages=%q", len(got[0]), got)
	}
}
