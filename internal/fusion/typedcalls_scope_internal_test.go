package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestTypedLoadPatternsFollowAllowedFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		name := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/root\n\ngo 1.23\n")
	write("kits/a/go.mod", "module example.com/root/kits/a\n\ngo 1.23\n")
	allowed := map[string]bool{
		"z/z.go": true, "a/a.go": true, "a/a_test.go": true,
		"kits/a/b/b.go": true, "kits/a/c/c_test.go": true,
		"ignored/bad.go": false, "testdata/f.go": true, "_hidden/f.go": true,
	}
	mods, _ := typedModulesSkipping(root, allowed, nil)
	if len(mods) != 2 {
		t.Fatalf("modules: %+v", mods)
	}
	want := [][]string{{"./a", "./z"}, {"./b", "./c"}}
	old := typedPackagesLoad
	defer func() { typedPackagesLoad = old }()
	var calls [][]string
	typedPackagesLoad = func(_ *packages.Config, patterns ...string) ([]*packages.Package, error) {
		calls = append(calls, append([]string(nil), patterns...))
		return nil, nil
	}
	gov := governTypedPass(context.Background())
	defer gov.close()
	for _, m := range mods {
		_, _, _ = loadTypedModule(gov, root, m, TypedEdgeEnv())
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("patterns = %v, want %v", calls, want)
	}
}

func TestTypedScopeSkipsIgnoredBrokenPackageAndReportsSelectedFailure(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		name := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/scope\n\ngo 1.23\n")
	write("good/good.go", "package good\nfunc Target() {}\nfunc Call() { Target() }\n")
	write("good/good_test.go", "package good_test\nimport (\"testing\"; \"example.com/scope/good\")\nfunc TestCall(t *testing.T) { good.Call() }\n")
	write("ignored/bad.go", "package ignored\nfunc Broken( {\n")
	write("selected/bad.go", "package selected\nfunc Broken( {\n")
	st := store.NewMemoryStore()
	allowed := map[string]bool{"good/good.go": true, "good/good_test.go": true}
	rep, err := EnrichTypedEdges(context.Background(), st, "scope", root, allowed)
	if err != nil || rep.PackagesFailed != 0 || rep.PackagesLoaded == 0 || rep.EdgesDirect < 2 {
		t.Fatalf("ignored broken package affected selected load: report=%+v err=%v", rep, err)
	}
	second, err := EnrichTypedEdges(context.Background(), st, "scope", root, allowed)
	if err != nil || second.PackagesFailed != 0 || second.PackagesCached == 0 {
		t.Fatalf("selected package did not cache: report=%+v err=%v", second, err)
	}
	allowed["selected/bad.go"] = true
	failed, err := EnrichTypedEdges(context.Background(), st, "scope", root, allowed)
	if err != nil && failed.PackagesFailed == 0 {
		t.Fatalf("selected broken package invisible: report=%+v err=%v", failed, err)
	}
	if failed.PackagesFailed == 0 {
		t.Fatalf("selected broken package not reported: %+v", failed)
	}
}
