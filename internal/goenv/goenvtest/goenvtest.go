// Package goenvtest builds the module fixtures the offline go tests share
// (ADR-083): a warm module cache filled from a file:// module proxy the
// test writes itself, so no test reaches the network to build one.
package goenvtest

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

// The warm fixture's modules: Module requires Dep at DepVersion.
const (
	Module     = "example.test/warm"
	Dep        = "example.test/dep"
	DepVersion = "v1.0.0"
)

// Warm is a module whose one dependency is in ModCache and whose go.sum
// lacks the dependency's zip hash: an inherited -mod=mod lets go list add
// it (go.sum goes from one line to two), and nothing else writes go.sum.
type Warm struct {
	Dir      string // the module root
	ModCache string // a module cache holding Dep@DepVersion, extracted
	GoSum    string // go.sum as written: the dependency's go.mod hash only
}

// Reset writes go.sum back to w.GoSum.
func (w Warm) Reset(t testing.TB) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(w.Dir, "go.sum"), []byte(w.GoSum), 0o644); err != nil {
		t.Fatal(err)
	}
}

// GoSumRewritten reports whether go.sum differs from w.GoSum, and its
// current text.
func (w Warm) GoSumRewritten(t testing.TB) (bool, string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(w.Dir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	return string(got) != w.GoSum, string(got)
}

// WarmModule builds the fixture. It skips the test when no go toolchain is
// on PATH.
func WarmModule(t testing.TB) Warm {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	depMod := "module " + Dep + "\n\ngo 1.22\n"
	write(t, filepath.Join(src, "go.mod"), depMod)
	write(t, filepath.Join(src, "dep.go"), "package dep\n\nfunc V() string { return \"v\" }\n")

	proxy := filepath.Join(root, "proxy")
	at := filepath.Join(proxy, filepath.FromSlash(Dep), "@v")
	write(t, filepath.Join(at, "list"), DepVersion+"\n")
	write(t, filepath.Join(at, DepVersion+".info"), `{"Version":"`+DepVersion+`","Time":"2026-01-01T00:00:00Z"}`+"\n")
	write(t, filepath.Join(at, DepVersion+".mod"), depMod)
	var zip bytes.Buffer
	if err := modzip.CreateFromDir(&zip, module.Version{Path: Dep, Version: DepVersion}, src); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(at, DepVersion+".zip"), zip.String())

	w := Warm{Dir: filepath.Join(root, "warm"), ModCache: filepath.Join(root, "mod")}
	write(t, filepath.Join(w.Dir, "go.mod"), "module "+Module+"\n\ngo 1.22\n\nrequire "+Dep+" "+DepVersion+"\n")
	write(t, filepath.Join(w.Dir, "warm.go"), "package warm\n\nimport \""+Dep+"\"\n\nfunc F() string { return dep.V() }\n\nfunc G() string { return F() }\n")
	t.Cleanup(func() { writable(w.ModCache) })

	// Fill the cache from the file proxy and let go write the full go.sum,
	// then drop the zip hash.
	env := append(os.Environ(), "GOWORK=off", "GOENV=off", "GOFLAGS=-mod=mod -modcacherw",
		"GOPROXY=file://"+filepath.ToSlash(proxy), "GOSUMDB=off", "GONOPROXY=", "GONOSUMDB=", "GOPRIVATE=", "GOINSECURE=",
		"GOMODCACHE="+w.ModCache, "GOTOOLCHAIN=local")
	for _, args := range [][]string{{"mod", "download", Dep + "@" + DepVersion}, {"mod", "tidy"}, {"list", "-deps", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir, cmd.Env = w.Dir, env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("goenvtest: go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	sum, err := os.ReadFile(filepath.Join(w.Dir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(string(sum)), "\n") {
		if strings.HasPrefix(line, Dep+" "+DepVersion+"/go.mod ") {
			kept = append(kept, line)
		}
	}
	if len(kept) != 1 {
		t.Fatalf("goenvtest: go.sum has no single go.mod hash for %s:\n%s", Dep, sum)
	}
	w.GoSum = kept[0] + "\n"
	w.Reset(t)
	return w
}

func write(t testing.TB, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writable makes every directory under dir writable, so the test's
// temporary directory can be removed even where the go command left a
// read-only module directory.
func writable(dir string) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
}
