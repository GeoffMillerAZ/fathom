package main

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// cgofree_test.go — CS-9 (docs/plans/cloud-sessions.md), ADR-066: the Linux
// binary is pure Go. `task build` and scripts/cloud-setup.sh build it with
// CGO_ENABLED=0, so no C toolchain is needed to build fathom on Linux, and
// darwin keeps cgo ONLY for this package's two proc readers (host_darwin.go,
// rss_darwin.go: proc_pid_rusage and proc_listchildpids have no pure-Go
// path). The last CGO dependency, the go-tree-sitter bindings (ADR-013), left
// with LB-F19.
//
// The check reads `go list -deps` over this package, not a build: the load
// is cheap enough for every gate. Under CGO_ENABLED=0 a cgo file is silently
// excluded from its package (go list reports no CgoFiles at all, and the
// failure would only surface at link time), so the C-source check runs with
// CGO_ENABLED=1, where a cgo dependency shows itself, and the CGO_ENABLED=0
// load must still resolve every package without error.

// goListPkg is the subset of `go list -json` the guard reads.
type goListPkg struct {
	ImportPath   string
	Standard     bool
	CgoFiles     []string
	CFiles       []string
	CXXFiles     []string
	MFiles       []string
	SwigFiles    []string
	SwigCXXFiles []string
	Error        *struct{ Err string }
}

// cSources names the files of p that need a C toolchain to build.
func (p goListPkg) cSources() []string {
	var out []string
	for _, files := range [][]string{p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.SwigFiles, p.SwigCXXFiles} {
		out = append(out, files...)
	}
	slices.Sort(out)
	return out
}

// listDeps runs `go list -e -deps -json` over this package for one target.
func listDeps(t *testing.T, goos, goarch, cgo string) []goListPkg {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	cmd := exec.CommandContext(t.Context(), "go", "list", "-e", "-deps", "-json=ImportPath,Standard,CgoFiles,CFiles,CXXFiles,MFiles,SwigFiles,SwigCXXFiles,Error", ".")
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED="+cgo, "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("GOOS=%s GOARCH=%s CGO_ENABLED=%s go list -deps: %v\n%s", goos, goarch, cgo, err, stderr.String())
	}
	var pkgs []goListPkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goListPkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	if len(pkgs) == 0 {
		t.Fatalf("GOOS=%s GOARCH=%s CGO_ENABLED=%s go list -deps listed no packages", goos, goarch, cgo)
	}
	return pkgs
}

// cgoDeps maps each non-stdlib package carrying C sources to those files.
func cgoDeps(pkgs []goListPkg) map[string][]string {
	got := map[string][]string{}
	for _, p := range pkgs {
		if p.Standard {
			continue
		}
		if files := p.cSources(); len(files) > 0 {
			got[p.ImportPath] = files
		}
	}
	return got
}

// TestLinuxBuildIsCgoFree (CS-9, ADR-066): for linux/amd64 and linux/arm64,
// the CGO_ENABLED=0 load of cmd/fathom resolves every package, and no
// non-stdlib package in its dependency set carries a cgo or C/C++/ObjC/SWIG
// file even with cgo enabled, so the pure-Go build drops nothing.
func TestLinuxBuildIsCgoFree(t *testing.T) {
	for _, goarch := range []string{"amd64", "arm64"} {
		t.Run("linux/"+goarch, func(t *testing.T) {
			for _, p := range listDeps(t, "linux", goarch, "0") {
				if p.Error != nil {
					t.Errorf("CGO_ENABLED=0 load: %s: %s", p.ImportPath, p.Error.Err)
				}
			}
			got := cgoDeps(listDeps(t, "linux", goarch, "1"))
			for _, path := range slices.Sorted(maps.Keys(got)) {
				files := got[path]
				t.Errorf("%s carries C sources %s: the Linux build is CGO_ENABLED=0 (ADR-066); retire the dependency or record a decision", path, strings.Join(files, " "))
			}
		})
	}
}

// TestDarwinCgoIsOnlyTheProcReaders (CS-9, ADR-066): on darwin the only C
// sources in cmd/fathom's dependency set are this package's two proc
// readers, which `task build` keeps cgo for.
func TestDarwinCgoIsOnlyTheProcReaders(t *testing.T) {
	const self = "github.com/geoffmilleraz/fathom/cmd/fathom"
	want := []string{"host_darwin.go", "rss_darwin.go"}
	for _, goarch := range []string{"arm64", "amd64"} {
		t.Run("darwin/"+goarch, func(t *testing.T) {
			got := cgoDeps(listDeps(t, "darwin", goarch, "1"))
			for _, path := range slices.Sorted(maps.Keys(got)) {
				files := got[path]
				if path == self && slices.Equal(files, want) {
					continue
				}
				t.Errorf("%s carries C sources %s: darwin keeps cgo only for %s %v (ADR-066)", path, strings.Join(files, " "), self, want)
			}
			if _, ok := got[self]; !ok {
				t.Errorf("%s lists no cgo files under darwin/%s CGO_ENABLED=1: want %v (the proc readers moved; update ADR-066 and this guard)", self, goarch, want)
			}
		})
	}
}
