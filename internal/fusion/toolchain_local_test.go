package fusion_test

// K22 (wave lf2, LB-F341, O654 point 3): the automatic typed pass never
// fetches or runs a toolchain a repository names. Every automatic `go`
// invocation (packages.Load for ingest, the typed pass and the daemon's
// auto-refresh; `go env GOWORK` in the repository; `go env <key>` for the
// environment repair) runs with GOTOOLCHAIN=local, so a go.mod's
// `go`/`toolchain` line can make the local toolchain refuse, never switch.
//
// The stand-in: with GOTOOLCHAIN=auto the go command looks for the named
// toolchain on PATH (`go1.99.0`) before it downloads one, so a script of
// that name on PATH records every run. GOPROXY=off keeps the download
// itself impossible, and a private GOMODCACHE shows nothing was fetched.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestTypedPassNeverSwitchesToolchain(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "switched")
	stub := "#!/bin/sh\necho \"$*\" >> '" + marker + "'\necho 'stand-in go1.99.0: a repository-named toolchain ran' >&2\nexit 3\n"
	if err := os.WriteFile(filepath.Join(bin, "go1.99.0"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	gopath := filepath.Join(root, "gopath")
	modcache := filepath.Join(gopath, "pkg", "mod")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOMODCACHE", modcache)
	t.Setenv("GOWORK", "") // an inherited GOWORK=off would hide the repository's own `go env GOWORK` run

	src := "package a\n\nfunc F() { G() }\n\nfunc G() {}\n"
	fixture := func(name, gomod string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	noSwitch := func(label string) {
		t.Helper()
		if raw, err := os.ReadFile(marker); err == nil {
			t.Fatalf("%s: the repository-named toolchain ran (go %s) — an automatic go invocation honoured go.mod's toolchain line", label, strings.TrimSpace(string(raw)))
		}
		if hits, _ := filepath.Glob(filepath.Join(modcache, "golang.org", "toolchain@*")); len(hits) > 0 {
			t.Fatalf("%s: a toolchain was fetched into GOMODCACHE: %v", label, hits)
		}
	}
	ingest := func(id, dir string) (fusion.Report, fusion.TypedEdgeStatus, []core.Edge) {
		t.Helper()
		ctx := context.Background()
		st := store.NewMemoryStore()
		p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
		rep, err := p.Ingest(ctx, core.ServiceRef{ID: id, Path: dir})
		if err != nil {
			t.Fatalf("%s: Ingest: %v", id, err)
		}
		status, known := p.TypedEdgeStatus(id)
		if !known {
			t.Fatalf("%s: no typed-edge status recorded", id)
		}
		edges, err := st.TypedEdges(ctx, id)
		if err != nil {
			t.Fatalf("%s: TypedEdges: %v", id, err)
		}
		obs, err := st.QueryObservations(ctx, id, "")
		if err != nil {
			t.Fatalf("%s: QueryObservations: %v", id, err)
		}
		if len(obs) == 0 {
			t.Fatalf("%s: no symbols indexed — the syntax pass must stand whatever the typed pass does", id)
		}
		return rep, status, edges
	}

	// Fixture A: the go line itself names a toolchain newer than the local
	// one. The local toolchain refuses; ingest degrades to syntax edges and
	// names the refusal.
	dirA := fixture("a", "module example.com/a\n\ngo 1.99.0\n\ntoolchain go1.99.0\n")
	repA, stA, edgesA := ingest("k22a", dirA)
	noSwitch("go 1.99.0")
	if !stA.Failed || !repA.TypedFailed {
		t.Fatalf("go 1.99.0: typed pass reported healthy (status %+v, report failed=%v) — the refusal must be disclosed", stA, repA.TypedFailed)
	}
	for _, want := range []string{"1.99.0", "GOTOOLCHAIN=local"} {
		if !strings.Contains(stA.Reason, want) || !strings.Contains(repA.TypedFailReason, want) {
			t.Fatalf("go 1.99.0: the named reason lacks %q:\n status: %s\n report: %s", want, stA.Reason, repA.TypedFailReason)
		}
	}
	if len(edgesA) != 0 {
		t.Fatalf("go 1.99.0: %d typed edges stored from a refused load", len(edgesA))
	}

	// Fixture B: only the toolchain line is new; the go line (1.21) is one
	// the local toolchain satisfies, so it type-checks locally.
	dirB := fixture("b", "module example.com/b\n\ngo 1.21\n\ntoolchain go1.99.0\n")
	_, stB, edgesB := ingest("k22b", dirB)
	noSwitch("toolchain go1.99.0")
	if stB.Failed {
		t.Fatalf("toolchain go1.99.0 under go 1.21: typed pass failed: %s", stB.Reason)
	}
	direct := false
	for _, e := range edgesB {
		if e.Kind == core.EdgeDirect && strings.HasSuffix(e.FromSymbol, ".F") && strings.HasSuffix(e.ToSymbol, ".G") {
			direct = true
		}
	}
	if !direct {
		t.Fatalf("toolchain go1.99.0 under go 1.21: no typed F -> G edge from the local toolchain; got %v", edgesB)
	}

	// The environment repair asks `go env GOPATH` in this process's own
	// directory, which for a CLI ingest is the repository itself.
	t.Chdir(dirA)
	t.Setenv("GOPATH", "")
	env := fusion.TypedEdgeEnv()
	noSwitch("go env in the repository")
	if last := env[len(env)-1]; last != "GOTOOLCHAIN=local" {
		t.Fatalf("TypedEdgeEnv's last entry is %q; GOTOOLCHAIN=local must be appended last so nothing inherited overrides it", last)
	}
}
