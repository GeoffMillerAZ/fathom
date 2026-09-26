package testisolate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) (root, module string) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				if m, ok := strings.CutPrefix(line, "module "); ok {
					return dir, strings.TrimSpace(m)
				}
			}
			t.Fatalf("%s/go.mod names no module", dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}

// TestEveryPackageWithDaemonDefaultsIsIsolated (RT-19, §10.4): every
// package whose tests can reach a daemon-URL, state-dir or call-log
// default — directly or through the module's own imports — runs under
// the isolating entry point.
func TestEveryPackageWithDaemonDefaultsIsIsolated(t *testing.T) {
	root, module := moduleRoot(t)
	need, err := packagesNeedingIsolation(root, module)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ResolverPackages {
		found := false
		for _, n := range need {
			found = found || n == r
		}
		if !found {
			t.Errorf("resolver package %s is not in the needing-isolation list — the walk is broken", r)
		}
	}
	for _, dir := range need {
		ok, err := usesEntryPoint(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			t.Errorf("%s: %v (unreadable is not isolated, I38)", dir, err)
			continue
		}
		if !ok {
			t.Errorf("%s reaches a daemon/state/call-log default but its tests do not run under testisolate.Main (add a testmain_test.go: func TestMain(m *testing.M) { testisolate.Main(m) })", dir)
		}
	}
}

// TestPackagesNeedingIsolation_Transitive: a fixture module where b
// imports a resolver and c imports b; d imports nothing; e has no tests.
func TestPackagesNeedingIsolation_Transitive(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/statedir/s.go", "package statedir\n")
	write("internal/statedir/s_test.go", "package statedir\n")
	write("internal/b/b.go", "package b\nimport _ \"example.com/m/internal/statedir\"\n")
	write("internal/b/b_test.go", "package b\n")
	write("internal/c/c_test.go", "package c_test\nimport _ \"example.com/m/internal/b\"\n")
	write("internal/d/d_test.go", "package d\nimport _ \"os\"\n")
	write("internal/e/e.go", "package e\nimport _ \"example.com/m/internal/b\"\n")
	write("third_party/x/x_test.go", "package x\nimport _ \"example.com/m/internal/b\"\n")
	got, err := packagesNeedingIsolation(root, "example.com/m")
	if err != nil {
		t.Fatal(err)
	}
	want := "internal/b,internal/c,internal/statedir"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
}

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPackagesNeedingIsolation_TestImportCycle (RT-19 review): test
// imports make per-directory cycles normal — q's external test imports p,
// and p imports q. q's test binary reaches statedir through p and r. The
// walk used to memoise "false" for a node it met while still on the DFS
// stack and ranged over a map, so q was missed ~90% of runs. Every run
// must now give the same, complete answer.
func TestPackagesNeedingIsolation_TestImportCycle(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"internal/statedir/s.go": "package statedir\n",
		"internal/p/p.go":        "package p\nimport (\n_ \"example.com/m/internal/q\"\n_ \"example.com/m/internal/r\"\n)\n",
		"internal/p/p_test.go":   "package p\n",
		"internal/q/q.go":        "package q\n",
		"internal/q/q_test.go":   "package q_test\nimport _ \"example.com/m/internal/p\"\n",
		"internal/r/r.go":        "package r\nimport _ \"example.com/m/internal/statedir\"\n",
		"internal/z/z.go":        "package z\nimport _ \"example.com/m/internal/q\"\n",
		"internal/z/z_test.go":   "package z\n",
		"internal/w/w_test.go":   "package w\nimport _ \"example.com/m/internal/q\"\n",
	})
	// z imports q (whose production code reaches nothing) and w's test
	// imports q: neither reaches statedir — q's TEST imports are not
	// compiled into its importers.
	want := "internal/p,internal/q"
	for i := 0; i < 50; i++ {
		got, err := packagesNeedingIsolation(root, "example.com/m")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(got, ",") != want {
			t.Fatalf("run %d: got %v, want %s", i, got, want)
		}
	}
}

// TestUsesEntryPointIsACall (RT-19 review): the entry point counts only
// as a call from TestMain — a mention in a comment or string does not.
func TestUsesEntryPointIsACall(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"a/a_test.go": "package a\n// TODO: call testisolate.Main(m) here\nvar s = \"testisolate.IsolateAndRun(\"\n",
		"b/b_test.go": "package b\nimport (\n\"testing\"\ntiso \"github.com/geoffmilleraz/fathom/internal/testisolate\"\n)\nfunc TestMain(m *testing.M) { tiso.Main(m) }\n",
		"c/c_test.go": "package c\nimport (\n\"os\"\n\"testing\"\n\"github.com/geoffmilleraz/fathom/internal/testisolate\"\n)\nfunc TestMain(m *testing.M) { os.Exit(testisolate.IsolateAndRun(m, testisolate.Options{})) }\n",
	})
	for dir, want := range map[string]bool{"a": false, "b": true, "c": true} {
		got, err := usesEntryPoint(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("usesEntryPoint(%s) = %v, want %v", dir, got, want)
		}
	}
}
