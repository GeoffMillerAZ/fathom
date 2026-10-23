package fusion

import (
	"archive/zip"
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func writeTypedLoadFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		name := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTypedImmutableDependency(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	for _, tt := range []struct {
		name       string
		mod        *packages.Module
		moduleMode bool
		want       bool
	}{
		{"standard library", nil, true, true},
		{"module-less without module roots", nil, false, false},
		{"main", &packages.Module{Main: true, Dir: filepath.Join(base, "main")}, true, false},
		{"replaced", &packages.Module{Dir: filepath.Join(base, "dep"), Replace: &packages.Module{}}, true, false},
		{"repo itself", &packages.Module{Dir: repo}, true, false},
		{"inside repo", &packages.Module{Dir: filepath.Join(repo, "dep")}, true, false},
		{"dot dot name inside repo", &packages.Module{Dir: filepath.Join(repo, "..local")}, true, false},
		{"module cache", &packages.Module{Dir: filepath.Join(base, "cache", "dep@v1.0.0")}, true, true},
		{"sibling prefix", &packages.Module{Dir: filepath.Join(base, "repo-other")}, true, true},
		{"unknown directory", &packages.Module{}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := typedImmutableDependency(&packages.Package{Module: tt.mod}, repo, tt.moduleMode); got != tt.want {
				t.Errorf("immutable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypedLoadImportCycleDiagnostic(t *testing.T) {
	t.Setenv("GO111MODULE", "on")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	dir := t.TempDir()
	writeTypedLoadFixture(t, dir, map[string]string{
		"go.mod":     "module example.com/root\n\ngo 1.23\n",
		"root.go":    "package root\nimport \"example.com/root/dep\"\nfunc Call() { dep.Work() }\n",
		"dep/dep.go": "package dep\nimport \"example.com/root\"\nfunc Work() { root.Call() }\n",
	})
	report, _ := runTypedLoadParity(t, dir, map[string]bool{"root.go": true})
	if report.PackagesFailed != 1 || len(report.FailedPackages) != 1 {
		t.Fatalf("cycle did not fail the importing root: %+v", report)
	}
	diagnostic := report.FailedPackages[0].Err
	if !strings.Contains(diagnostic, "import cycle or missing package") || strings.Contains(diagnostic, "no metadata") {
		t.Errorf("missing import diagnostic does not name the cycle possibility: %s", diagnostic)
	}
	if !strings.Contains(diagnostic, "import cycle not allowed") {
		t.Errorf("go list cycle error was lost: %s", diagnostic)
	}
}

func TestTypedLoadGOPATHDependencyBodyError(t *testing.T) {
	gopath := t.TempDir()
	t.Setenv("GO111MODULE", "off")
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOPACKAGESDRIVER", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	dir := filepath.Join(gopath, "src", "example.com", "root")
	writeTypedLoadFixture(t, dir, map[string]string{
		"root.go":    "package root\nimport \"example.com/root/dep\"\nfunc Call() { dep.Work() }\n",
		"dep/dep.go": "package dep\nfunc Work() { missingName() }\n",
	})
	gov := governTypedPass(context.Background())
	defer gov.close()
	roots, _, err := loadTypedModule(gov, dir, goModule{scoped: true, patterns: []string{"."}}, TypedEdgeEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Module != nil {
		t.Fatalf("want one GOPATH root without module information, got %v", roots)
	}
	root := roots[0]
	dep := root.Imports["example.com/root/dep"]
	if dep == nil || dep.Module != nil {
		t.Fatalf("fixture did not load the module-less non-root dependency: %v", dep)
	}
	if !dep.IllTyped || !root.IllTyped {
		t.Errorf("dependency body error did not fail its importing root: dependency ill-typed=%v, root ill-typed=%v", dep.IllTyped, root.IllTyped)
	}
	if errs := strings.Join(typedPackageLoadErrors(root), "; "); !strings.Contains(errs, "missingName") {
		t.Errorf("dependency body diagnostic was lost: %s", errs)
	}
}

// Keep the pre-split loader as an oracle in the normal suite. In particular,
// it checks dependency bodies, including immutable dependencies' bodies.
func legacyTypedLoad(gov *typedPassGovernor, _ string, dir string, patterns []string, env []string) ([]*packages.Package, *token.FileSet, error) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Context: gov.ctx, Dir: dir, Fset: fset, Tests: true, Env: env,
		ParseFile: gov.parseFile,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
	}, patterns...)
	return pkgs, fset, err
}

func TestTypedLoadParity(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOWORK", "off")
	for _, name := range []string{"dispatch and variants", "cgo", "immutable body error", "immutable declaration error", "replace body error", "first party body error"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "repo")
			files := map[string]string{
				"go.mod":  "module example.com/root\n\ngo 1.23\n",
				"root.go": "package root\nfunc Target() {}\nfunc Call() { Target() }\n",
			}
			if name == "dispatch and variants" {
				delete(files, "root.go")
				files["go.mod"] += "require example.com/dep v0.0.0\nreplace example.com/dep => ../dep\n"
				writeTypedLoadFixture(t, base, map[string]string{
					"dep/go.mod": "module example.com/dep\n\ngo 1.23\n",
					"dep/dep.go": "package dep\nimport \"fmt\"\nfunc Work() { fmt.Println(\"body-only import\") }\n",
				})
				files["a/a.go"] = "package a\ntype Box[T any] struct { Value T }\nfunc Identity[T any](v T) T { return v }\nfunc Target() {}\n"
				files["a/internal_test.go"] = "package a\nimport \"testing\"\nfunc TestTarget(t *testing.T) { Target() }\n"
				files["a/external_test.go"] = "package a_test\nimport (\"testing\"; \"example.com/root/b\")\nfunc TestBack(t *testing.T) { b.Call() }\n"
				files["b/b.go"] = `package b
import ("io"; "example.com/root/a"; "example.com/dep")
type Failure struct{}
func (Failure) Error() string { return "failure" }
type Reader struct{}
func (Reader) Read(p []byte) (int, error) { return 0, nil }
func DispatchError(e error) string { return e.Error() }
func DispatchReader(r io.Reader) { _, _ = r.Read(nil) }
func Call() { a.Target(); _ = a.Identity(a.Box[int]{Value: 1}); dep.Work() }
`
				// A per-file version newer than go.mod's version is still honored.
				files["b/version.go"] = "//go:build go1.24\n\npackage b\nfunc Version() { Call() }\n"
			} else if name == "cgo" {
				if !build.Default.CgoEnabled {
					t.Skip("cgo is not enabled in this toolchain")
				}
				files["cgo.go"] = "package root\n/* static int value(void) { return 1; } */\nimport \"C\"\nfunc Cgo() { _ = C.value(); Target() }\n"
			} else if name == "first party body error" {
				files["dep/dep.go"] = "package dep\nfunc Work() { missingName() }\n"
				files["root.go"] = "package root\nimport \"example.com/root/dep\"\nfunc Call() { dep.Work() }\n"
			} else {
				files["go.mod"] += "require example.com/dep v1.0.0\n"
				files["root.go"] = "package root\nimport \"example.com/dep\"\nfunc Target() {}\nfunc Call() { Target(); dep.Work() }\n"
				body := "package dep\nfunc Work() { missingName() }\n"
				if name == "immutable declaration error" {
					body = "package dep\nvar Value MissingType\nfunc Work() {}\n"
				}
				if name == "replace body error" {
					files["go.mod"] += "replace example.com/dep => ../dep\n"
					writeTypedLoadFixture(t, base, map[string]string{"dep/go.mod": "module example.com/dep\n\ngo 1.23\n", "dep/dep.go": body})
				} else {
					cache := filepath.Join(base, "modcache")
					seedTypedModuleCache(t, cache, body)
					t.Setenv("GOMODCACHE", cache)
					t.Setenv("GOFLAGS", "-mod=mod -modcacherw")
				}
			}
			writeTypedLoadFixture(t, dir, files)
			loader := typedLoadPackages
			t.Cleanup(func() { typedLoadPackages = loader })
			variant := false
			typedLoadPackages = func(gov *typedPassGovernor, repo, dir string, patterns []string, env []string) ([]*packages.Package, *token.FileSet, error) {
				roots, fset, err := loader(gov, repo, dir, patterns, env)
				rootSet := make(map[*packages.Package]bool)
				for _, p := range roots {
					rootSet[p] = true
				}
				packages.Visit(roots, nil, func(p *packages.Package) {
					if !rootSet[p] && p.ID == "example.com/root/b [example.com/root/a.test]" {
						variant = true
						if p.Syntax != nil || p.TypesInfo != nil || p.Types == nil || !p.Types.Complete() || p.IllTyped {
							t.Errorf("non-root test variant lost complete types or retained syntax/info: %v", typedPackageLoadErrors(p))
						}
					}
				})
				return roots, fset, err
			}
			var allowed map[string]bool
			if name == "first party body error" {
				// The first-party dependency must be a non-root to exercise
				// body checking independently of roots' mandatory full check.
				allowed = map[string]bool{"root.go": true}
			}
			gotReport, gotEdges := runTypedLoadParity(t, dir, allowed)
			typedLoadPackages = legacyTypedLoad
			oldReport, oldEdges := runTypedLoadParity(t, dir, allowed)
			if name == "dispatch and variants" && !variant {
				t.Fatal("fixture missed the non-root import-back test variant")
			}
			if name == "dispatch and variants" {
				for _, edge := range []struct {
					from, to string
					kind     core.EdgeKind
				}{
					{".DispatchError", ".Failure.Error", core.EdgeIfaceDispatch},
					{".DispatchReader", ".Reader.Read", core.EdgeIfaceDispatch},
					{".Call", ".Identity", core.EdgeDirect},
					{".Version", ".Call", core.EdgeDirect},
				} {
					found := false
					for _, got := range gotEdges {
						found = found || (strings.HasSuffix(got.FromSymbol, edge.from) && strings.HasSuffix(got.ToSymbol, edge.to) && got.Kind == edge.kind)
					}
					if !found {
						t.Errorf("missing %s -> %s %s", edge.from, edge.to, edge.kind)
					}
				}
			}
			if name == "immutable body error" {
				if gotReport.PackagesTotal != 1 || gotReport.PackagesLoaded != 1 || gotReport.PackagesFailed != 0 || len(gotReport.FailedPackages) != 0 || gotReport.EdgesDirect != 1 || len(gotEdges) != 1 {
					t.Fatalf("new loader did not load root with body-only dependency error: %+v, edges=%v", gotReport, gotEdges)
				}
				if oldReport.PackagesFailed != 1 || oldReport.PackagesLoaded != 0 || len(oldReport.FailedPackages) != 1 || !strings.Contains(oldReport.FailedPackages[0].Err, "missingName") || len(oldEdges) != 0 {
					t.Fatalf("old loader did not fail root on dependency body: %+v, edges=%v", oldReport, oldEdges)
				}
				// Remove exactly the stated semantic difference. All remaining
				// report fields must match, not just the failure counter.
				oldReport.PackagesFailed, oldReport.PackagesLoaded = 0, 1
				oldReport.FailedPackages, oldReport.EdgesDirect = nil, 1
			} else {
				if !reflect.DeepEqual(gotEdges, oldEdges) {
					t.Errorf("edges differ:\nnew: %+v\nold: %+v", gotEdges, oldEdges)
				}
				if strings.Contains(name, "error") {
					if gotReport.PackagesFailed == 0 {
						t.Fatal("declaration or mutable dependency body error was hidden")
					}
				} else if gotReport.PackagesFailed != 0 || len(gotEdges) == 0 {
					t.Fatalf("clean fixture failed or had no edges: %+v", gotReport)
				}
			}
			if !reflect.DeepEqual(gotReport, oldReport) {
				t.Errorf("reports differ:\nnew: %+v\nold: %+v", gotReport, oldReport)
			}
		})
	}
}

func runTypedLoadParity(t *testing.T, dir string, allowed map[string]bool) (TypedEdgeReport, []core.Edge) {
	t.Helper()
	st := store.NewMemoryStore()
	report, err := EnrichTypedEdges(context.Background(), st, "fixture", dir, allowed)
	if err != nil {
		t.Fatal(err)
	}
	edges, err := st.TypedEdges(context.Background(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(edges, func(i, j int) bool { return fmt.Sprint(edges[i]) < fmt.Sprint(edges[j]) })
	// Compare counters and diagnostics, never wall time.
	report.Duration = 0
	return report, edges
}

func TestTypedLoadRuntimeVersionDiagnostic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX go wrapper")
	}
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	writeTypedLoadFixture(t, bin, map[string]string{"go": "#!/bin/sh\n" +
		"if [ \"$1\" = list ] && [ \"$2\" = -e ] && [ \"$3\" = -f ] && [ \"$4\" = '{{context.ReleaseTags}}' ]; then\n" +
		"  echo '[go1.1 go1.99]'\n  exit 0\nfi\nexec " + shellQuote(realGo) + " \"$@\"\n"})
	if err := os.Chmod(filepath.Join(bin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOWORK", "off")
	dir := filepath.Join(base, "repo")
	writeTypedLoadFixture(t, dir, map[string]string{
		"go.mod":  "module example.com/root\n\ngo 1.23\n",
		"root.go": "package root\nvar X MissingType\n",
	})
	loader := typedLoadPackages
	t.Cleanup(func() { typedLoadPackages = loader })
	got, _ := runTypedLoadParity(t, dir, nil)
	typedLoadPackages = legacyTypedLoad
	want, _ := runTypedLoadParity(t, dir, nil)
	if len(got.FailedPackages) != 1 || !strings.Contains(got.FailedPackages[0].Err, "This application uses version go1.") || !strings.Contains(got.FailedPackages[0].Err, "runs version go1.99 of 'go list'") {
		t.Fatalf("runtime-version diagnostic missing: %+v", got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime-version diagnostic differs:\nnew: %+v\nold: %+v", got, want)
	}
}

// Populate only a private on-disk download cache. The go command unpacks it
// with GOPROXY=off; the dependency is outside the repo and has no replacement.
func seedTypedModuleCache(t *testing.T, cache, body string) {
	t.Helper()
	mod := "module example.com/dep\n\ngo 1.23\n"
	download := filepath.Join(cache, "cache", "download", "example.com", "dep", "@v")
	writeTypedLoadFixture(t, download, map[string]string{
		"v1.0.0.mod":  mod,
		"v1.0.0.info": "{\"Version\":\"v1.0.0\",\"Time\":\"2020-01-01T00:00:00Z\"}",
	})
	f, err := os.Create(filepath.Join(download, "v1.0.0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, file := range []struct{ name, body string }{{"go.mod", mod}, {"dep.go", body}} {
		w, err := z.Create("example.com/dep@v1.0.0/" + file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTypedLoadRetainsOnlyRootSyntaxAndInfo(t *testing.T) {
	dir := t.TempDir()
	writeTypedLoadFixture(t, dir, map[string]string{
		"go.mod":           "module example.com/root\n\ngo 1.23\n",
		"root.go":          "package root\nimport \"net/http\"\nfunc Client() *http.Client { return http.DefaultClient }\n",
		"root_test.go":     "package root\nimport \"testing\"\nfunc TestClient(t *testing.T) { _ = Client() }\n",
		"external_test.go": "package root_test\nimport (\"testing\"; \"example.com/root\")\nfunc TestExternal(t *testing.T) { _ = root.Client() }\n",
	})
	gov := governTypedPass(context.Background())
	defer gov.close()
	roots, fset, err := loadTypedModule(gov, dir, goModule{}, TypedEdgeEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 4 {
		t.Fatalf("root variants = %d, want plain, internal, external and test main", len(roots))
	}
	rootSet := make(map[*packages.Package]bool)
	rootFiles := make(map[string]*ast.File)
	for _, p := range roots {
		rootSet[p] = true
		if p.Syntax == nil || p.TypesInfo == nil || p.Types == nil || !p.Types.Complete() || p.IllTyped {
			t.Fatalf("root %s lost syntax/types or failed: %v", p.ID, typedPackageLoadErrors(p))
		}
		info := p.TypesInfo
		if info.Types == nil || info.Defs == nil || info.Uses == nil || info.Implicits == nil || info.Instances == nil || info.Scopes == nil || info.Selections == nil || info.FileVersions == nil {
			t.Errorf("root %s has an unallocated Info map", p.ID)
		}
		if p.Fset != fset {
			t.Errorf("root %s has a different fileset", p.ID)
		}
		for i, file := range p.Syntax {
			name := p.CompiledGoFiles[i]
			if previous := rootFiles[name]; previous != nil && previous != file {
				t.Errorf("test variants did not share the AST for %s", filepath.Base(name))
			}
			rootFiles[name] = file
		}
	}
	deps := 0
	packages.Visit(roots, nil, func(p *packages.Package) {
		if rootSet[p] {
			return
		}
		deps++
		if p.Syntax != nil || p.TypesInfo != nil {
			t.Errorf("non-root %s retains Syntax or TypesInfo", p.ID)
		}
		if p.Types == nil || !p.Types.Complete() || p.IllTyped {
			t.Errorf("non-root %s has incomplete or failed types: %v", p.ID, typedPackageLoadErrors(p))
		}
	})
	if deps == 0 {
		t.Fatal("fixture did not load any dependencies")
	}
}
