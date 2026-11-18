package deps_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func TestGoDefaultPGO(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cmd"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/control\n\ngo 1.26\n")
	write("default.pgo", "library profile is ignored")
	write("lib.go", "package lib\n")
	write("cmd/main.go", "package main\n\nfunc main() {}\n")
	repo := os.DirFS(root)
	reader, err := deps.New(deps.TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) deps.FileDeps {
		t.Helper()
		out, err := reader.ReadFiles(context.Background(), repo, []deps.Input{{Path: name}})
		if err != nil || len(out) != 1 {
			t.Fatalf("read %q: %v, %v", name, out, err)
		}
		return out[0]
	}
	if got := read("cmd/main.go"); !got.Complete || len(got.Edges) != 1 {
		t.Fatalf("missing package profile or profile in another directory: %+v", got)
	}
	if got := read("lib.go"); !got.Complete || len(got.Edges) != 1 {
		t.Fatalf("non-main package picked up a profile: %+v", got)
	}

	// The native dry-run shows profile selection without parsing the fixture data.
	profile := "cmd/default.pgo"
	write(profile, "synthetic dry-run profile")
	for _, mode := range []string{"auto", "off"} {
		cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-n", "-buildvcs=false", "-pgo="+mode, "-o", "dry-run", "./cmd")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOPROXY=off",
			"GOSUMDB=off", "GOWORK=off", "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native dry-run %s: %v\n%s", mode, err, out)
		}
		used := bytes.Contains(out, []byte(filepath.Join(root, profile)))
		t.Logf("native -pgo=%s selected default.pgo: %t", mode, used)
		if used != (mode == "auto") {
			t.Fatalf("native profile selection in %s mode: %t", mode, used)
		}
	}
	got := read("cmd/main.go")
	want := deps.Edge{From: "cmd/main.go", Kind: deps.BuildInput, Raw: profile, To: profile,
		Resolution: deps.File, Line: 1, Provenance: deps.Builtin}
	if !got.Complete || len(got.Edges) != 2 || got.Edges[0] != want {
		t.Errorf("missing native default profile edge: %+v", got)
	}

	if err := os.Remove(filepath.Join(root, profile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../default.pgo", filepath.Join(root, profile)); err != nil {
		t.Fatal(err)
	}
	got = read("cmd/main.go")
	if got.Complete || len(got.Edges) != 1 || len(got.Unknowns) != 1 || got.Unknowns[0].Code != deps.MissingConfig {
		t.Errorf("unsafe profile must be disclosed: %+v", got)
	}
}
