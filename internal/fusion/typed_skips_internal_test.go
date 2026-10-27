package fusion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeTypedFixture writes files (repo-relative path -> body) under a new
// temporary directory and returns it.
func writeTypedFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		name := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// typedCgoEnabled asks the go command itself, under the typed pass's
// environment, whether cgo is on: build.Default cannot see a missing C
// compiler or a CGO_ENABLED=0 the environment exports.
func typedCgoEnabled(t *testing.T, dir string) bool {
	t.Helper()
	cmd := exec.Command("go", "env", "CGO_ENABLED")
	cmd.Dir = dir
	cmd.Env = TypedEdgeEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go env CGO_ENABLED: %v", err)
	}
	return strings.TrimSpace(string(out)) == "1"
}

// TestTypedPassNamesEverySkippedPackage (EW-81, O1506): every package the
// typed pass's load returns is resolved, failed, or skipped with exactly one
// reason from a closed set, the counts add up to the packages seen, a
// package made only of cgo files resolves from its source, a "0 processed"
// failure names its real reasons, and the warning names each package of a
// class that loses evidence.
func TestTypedPassNamesEverySkippedPackage(t *testing.T) {
	root := writeTypedFixture(t, map[string]string{
		"go.mod":             "module example.com/fx\n\ngo 1.23\n",
		"a/a.go":             "package a\n\nfunc A() {}\n",
		"a/a_test.go":        "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { A() }\n",
		"b/b.go":             "package b\n\nfunc B() {}\n",
		"b/b_x_test.go":      "package b_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/fx/b\"\n)\n\nfunc TestB(t *testing.T) { b.B() }\n",
		"c/c.go":             "package c\n\nfunc C() {}\n",
		"c/c_x_test.go":      "package c_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/fx/c\"\n)\n\nfunc TestC(t *testing.T) { c.C() }\n",
		"e2e/e2e_test.go":    "package e2e\n\nimport \"testing\"\n\nfunc TestE2E(t *testing.T) {}\n",
		"tagged/tagged.go":   "//go:build fathom_never\n\npackage tagged\n\nfunc T() {}\n",
		"us/_gen.go":         "package us\n\nfunc Gen() {}\n",
		"cg/cg.go":           "package cg\n\n/*\nstatic int add(int a, int b) { return a + b; }\n*/\nimport \"C\"\n\nfunc Add(a, b int) int { return int(C.add(C.int(a), C.int(b))) }\n\nfunc Use() int { return Add(1, 2) }\n",
		"ignored/ignored.go": "package ignored\n",
	})
	allowed := map[string]bool{
		"a/a.go": true, "a/a_test.go": true,
		"b/b.go": true, "b/b_x_test.go": true,
		"c/c.go":           true,
		"e2e/e2e_test.go":  true,
		"tagged/tagged.go": true,
		"us/_gen.go":       true,
		"cg/cg.go":         true,
	}
	cgo := typedCgoEnabled(t, root)
	ctx := context.Background()
	st := store.NewMemoryStore()
	rep, err := EnrichTypedEdges(ctx, st, "fx", root, allowed)
	if err != nil {
		t.Fatal(err)
	}

	want := []TypedSkip{
		{Reason: typedSkipBuildConstraints, Count: 1, Names: []string{"example.com/fx/tagged"}},
		{Reason: typedSkipConnectorScope, Count: 1, Names: []string{"example.com/fx/c_test [example.com/fx/c.test]"}},
		{Reason: typedSkipOutside, Count: 1, Names: []string{"./us"}},
		{Reason: typedSkipTestMain, Count: 4, Names: []string{"example.com/fx/a.test", "example.com/fx/b.test", "example.com/fx/c.test", "example.com/fx/e2e.test"}},
		{Reason: typedSkipTestOnly, Count: 1, Names: []string{"example.com/fx/e2e"}},
	}
	resolved := 7 // a, a [a.test], b, b_test [b.test], c, cg, e2e [e2e.test]
	if !cgo {
		want[0] = TypedSkip{Reason: typedSkipBuildConstraints, Count: 2, Names: []string{"example.com/fx/cg", "example.com/fx/tagged"}}
		resolved = 6
	}
	if !reflect.DeepEqual(rep.Skips, want) {
		t.Errorf("Skips (cgo %v):\n got %+v\nwant %+v", cgo, rep.Skips, want)
	}
	if rep.PackagesTotal != 15 || rep.PackagesFailed != 0 || rep.PackagesLoaded+rep.PackagesCached != resolved || rep.PackagesSkipped != 15-resolved {
		t.Errorf("report total=%d loaded=%d cached=%d failed=%d skipped=%d, want 15 seen, %d resolved, 0 failed, %d skipped",
			rep.PackagesTotal, rep.PackagesLoaded, rep.PackagesCached, rep.PackagesFailed, rep.PackagesSkipped, resolved, 15-resolved)
	}
	if rep.PackagesTotal != rep.PackagesLoaded+rep.PackagesCached+rep.PackagesFailed+rep.PackagesSkipped {
		t.Errorf("R1 identity broken: %+v", rep)
	}
	sum := 0
	for _, s := range rep.Skips {
		sum += s.Count
	}
	if sum != rep.PackagesSkipped {
		t.Errorf("per-reason counts sum to %d, PackagesSkipped %d", sum, rep.PackagesSkipped)
	}

	if cgo {
		edges, err := st.TypedEdges(ctx, "fx")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range edges {
			if e.FromSymbol == "fx::cg/cg.Use" && e.ToSymbol == "fx::cg/cg.Add" && e.Kind == core.EdgeDirect {
				found = true
				if e.FromFile != "cg/cg.go" {
					t.Errorf("cg.Use -> cg.Add from %q, want the source file cg/cg.go", e.FromFile)
				}
			}
		}
		if !found {
			t.Errorf("a package whose every Go file imports \"C\" lost its typed edge cg.Use -> cg.Add; edges %v", edges)
		}
	} else {
		t.Log("cgo is off under the typed environment: cg is a build-constrained package, the edge assertion is skipped")
	}

	// The warning names each package of a class that loses evidence, never
	// a harmless one.
	line := typedSkipsLine("fx", rep.Skips, rep.NotLoaded)
	for _, w := range []string{"build constraints", "example.com/fx/tagged", "outside ./...", "./us"} {
		if !strings.Contains(line, w) {
			t.Errorf("warning %q does not name %q", line, w)
		}
	}
	for _, nw := range []string{typedSkipTestMain, typedSkipTestOnly, typedSkipConnectorScope, "a.test", "c_test"} {
		if strings.Contains(line, nw) {
			t.Errorf("warning %q names the harmless %q", line, nw)
		}
	}
	if typedSkipsLine("fx", []TypedSkip{{Reason: typedSkipTestMain, Count: 3, Names: []string{"x.test"}}}, nil) != "" {
		t.Errorf("a warning for harmless classes alone must be empty")
	}
	many := typedSkipsLine("fx", []TypedSkip{{Reason: typedSkipBuildConstraints, Count: 7, Names: []string{"p1", "p2", "p3", "p4", "p5"}}}, nil)
	if !strings.Contains(many, "7 package(s)") || !strings.Contains(many, "p1, p2, p3, p4, p5 and 2 more") {
		t.Errorf("warning over five names: %q", many)
	}

	t.Run("names sorted before the cap", func(t *testing.T) {
		var tally typedSkipTally
		for _, n := range []string{"g", "f", "e", "d", "c", "b", "a"} {
			tally.add(typedSkipTestMain, n)
		}
		got := tally.skips()
		want := []TypedSkip{{Reason: typedSkipTestMain, Count: 7, Names: []string{"a", "b", "c", "d", "e"}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tally %+v, want %+v", got, want)
		}
	})

	t.Run("reason rules", func(t *testing.T) {
		in := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
		cache := filepath.Join(t.TempDir(), "cache", "abc-d")
		cases := []struct {
			name string
			pkg  *packages.Package
			want string
		}{
			{"test main", &packages.Package{ID: "example.com/fx/a.test", GoFiles: []string{cache}, CompiledGoFiles: []string{cache}}, typedSkipTestMain},
			{"plain resolves", &packages.Package{ID: "example.com/fx/a", GoFiles: []string{in("a/a.go")}, CompiledGoFiles: []string{in("a/a.go")}}, ""},
			{"cgo-only resolves from GoFiles", &packages.Package{ID: "example.com/fx/cg", GoFiles: []string{in("cg/cg.go")}, CompiledGoFiles: []string{cache}}, ""},
			{"connector scope", &packages.Package{ID: "example.com/fx/c_test [example.com/fx/c.test]", GoFiles: []string{in("c/c_x_test.go")}, CompiledGoFiles: []string{in("c/c_x_test.go")}}, typedSkipConnectorScope},
			{"package named .test inside the repo is not a test main", &packages.Package{ID: "example.com/fx/odd.test", GoFiles: []string{in("odd.test/o.go")}, CompiledGoFiles: []string{in("odd.test/o.go")}}, typedSkipConnectorScope},
			{"cgo off", &packages.Package{ID: "example.com/fx/cg", IgnoredFiles: []string{in("cg/cg.go")}, Errors: []packages.Error{{Msg: "build constraints exclude all Go files"}}}, typedSkipBuildConstraints},
			{"tagged file, untagged test", &packages.Package{ID: "example.com/fx/m", IgnoredFiles: []string{in("m/m.go")}}, typedSkipBuildConstraints},
			{"test-only", &packages.Package{ID: "example.com/fx/e2e"}, typedSkipTestOnly},
			{"only an ignored file name", &packages.Package{ID: "./us", Errors: []packages.Error{{Msg: "no Go files"}}}, typedSkipOutside},
			{"build-constrained directory named x.test", &packages.Package{ID: "example.com/fx/suite.test", IgnoredFiles: []string{in("suite.test/x_windows.go")}, Errors: []packages.Error{{Msg: "build constraints exclude all Go files"}}}, typedSkipBuildConstraints},
			{"directory named x.test with only an ignored name", &packages.Package{ID: "./gen.test", Errors: []packages.Error{{Msg: "no Go files"}}}, typedSkipOutside},
			{"a non-Go ignored file is not a build constraint", &packages.Package{ID: "example.com/fx/e2e", IgnoredFiles: []string{in("e2e/helper.c")}}, typedSkipTestOnly},
		}
		for _, c := range cases {
			if got := typedSkipReason(c.pkg, root, allowed); got != c.want {
				t.Errorf("%s: typedSkipReason = %q, want %q", c.name, got, c.want)
			}
		}
		if got := typedSkipReason(&packages.Package{ID: "example.com/fx/a.test", GoFiles: []string{cache}}, root, nil); got != "" {
			t.Errorf("whole-module scope (nil allowed) skips nothing, got %q", got)
		}
	})

	t.Run("names", func(t *testing.T) {
		rootMod, tools := goModule{rel: "."}, goModule{rel: "tools"}
		for _, c := range []struct {
			m        goModule
			id, want string
		}{
			{rootMod, "example.com/fx/tagged", "example.com/fx/tagged"},
			{rootMod, "./us", "./us"},
			{rootMod, ".", "."},
			{tools, "./us", "./tools/us"},
			{tools, ".", "./tools"},
			// Two nested modules may declare one module path: the directory
			// tells their packages apart.
			{tools, "example.com/demo/t", "tools: example.com/demo/t"},
		} {
			if got := typedSkipName(c.m, c.id); got != c.want {
				t.Errorf("typedSkipName(%q, %q) = %q, want %q", c.m.rel, c.id, got, c.want)
			}
		}
	})

	t.Run("0 processed names its real reasons", func(t *testing.T) {
		only := writeTypedFixture(t, map[string]string{
			"go.mod":           "module example.com/tg\n\ngo 1.23\n",
			"tagged/tagged.go": "//go:build fathom_never\n\npackage tagged\n\nfunc T() {}\n",
		})
		p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{Concurrency: 2})
		var buf strings.Builder
		p.SetLogger(dlog.New(&buf, dlog.LevelDebug, time.Now))
		r, err := p.Ingest(ctx, core.ServiceRef{ID: "tg", Path: only})
		if err != nil {
			t.Fatal(err)
		}
		var zero, skips []string
		for _, l := range strings.Split(buf.String(), "\n") {
			if strings.Contains(l, "seen but 0 processed") {
				zero = append(zero, l)
			}
			if strings.Contains(l, "did not resolve") {
				skips = append(skips, l)
			}
		}
		if len(zero) != 1 || !strings.Contains(zero[0], "1 build constraints") || strings.Contains(zero[0], "connector scope filter") {
			t.Errorf("the 0-processed warning must name the real reason counts: %q", zero)
		}
		if len(skips) != 1 || !strings.Contains(skips[0], "build constraints 1 package(s): example.com/tg/tagged") {
			t.Errorf("the pass must warn once, naming the build-constrained package: %q\n%s", skips, buf.String())
		}
		if !r.TypedFailed {
			t.Fatalf("a load whose every package is skipped must stay a disclosed failure: %+v", r)
		}
		if !strings.HasPrefix(r.TypedFailReason, "1 package(s) seen but 0 processed") ||
			!strings.Contains(r.TypedFailReason, "1 build constraints") ||
			strings.Contains(r.TypedFailReason, "connector scope filter") ||
			strings.Contains(r.TypedFailReason, "example.com") {
			t.Fatalf("reason %q: want the prefix, the real reason count, no connector-scope claim and no package name (the reason is persisted)", r.TypedFailReason)
		}
	})
}
