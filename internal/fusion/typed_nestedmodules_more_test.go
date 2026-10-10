package fusion_test

// EW-59 edges of the per-module typed pass: a nested module whose go
// directive the local toolchain cannot satisfy is a disclosed failure that
// keeps its stored rows (K22: never a toolchain fetch); a repository whose
// only modules are nested ones gets typed edges; two nested modules
// declaring one module path store their packages apart.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func typedEdgesFrom(t *testing.T, st store.Store, svc, filePrefix string) int {
	t.Helper()
	typed, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	n := 0
	for _, e := range typed {
		if strings.HasPrefix(e.FromFile, filePrefix) && e.Kind == core.EdgeDirect {
			n++
		}
	}
	return n
}

func TestTypedPassNestedModuleNeedingNewerGoIsDisclosedAndKeepsRows(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod":                 "module example.com/root\n\ngo 1.21\n",
		"root.go":                "package root\n\nfunc A() int { return b() }\n\nfunc b() int { return 1 }\n",
		"kits/k/go.mod":          "module example.com/root/kits/k\n\ngo 1.21\n",
		"kits/k/cmd/k/main.go":   "package main\n\nfunc main() { run() }\n\nfunc run() {}\n",
		"kits/ok/go.mod":         "module example.com/root/kits/ok\n\ngo 1.21\n",
		"kits/ok/cmd/ok/main.go": "package main\n\nfunc main() { work() }\n\nfunc work() {}\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	svc := core.ServiceRef{ID: "newergo", Path: dir}
	if rep, err := pipe.Ingest(ctx, svc); err != nil || rep.TypedFailed {
		t.Fatalf("first Ingest: err=%v failed=%v %s", err, rep.TypedFailed, rep.TypedFailReason)
	}
	if typedEdgesFrom(t, st, "newergo", "kits/k/") == 0 {
		t.Fatal("first ingest: no typed edge from kits/k")
	}

	// kits/k now needs a toolchain this machine does not have.
	if err := os.WriteFile(filepath.Join(dir, "kits/k/go.mod"), []byte("module example.com/root/kits/k\n\ngo 1.999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kits/ok/cmd/ok/main.go"), []byte("package main\n\nfunc main() { work(); more() }\n\nfunc work() {}\n\nfunc more() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := pipe.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("second Ingest: %v (a nested module's load failure must never fail the ingest)", err)
	}
	if rep.TypedPackagesFailed == 0 || !rep.TypedFailed {
		t.Fatalf("second ingest: failed=%d TypedFailed=%v, want the kits/k load failure disclosed", rep.TypedPackagesFailed, rep.TypedFailed)
	}
	if !strings.Contains(rep.TypedFailReason, "kits/k/go.mod") || !strings.Contains(rep.TypedFailReason, "1.999") {
		t.Errorf("reason %q: want it to name kits/k/go.mod and the go version it needs", rep.TypedFailReason)
	}
	// The failed module's last good rows stand; the others re-resolve.
	if typedEdgesFrom(t, st, "newergo", "kits/k/") == 0 {
		t.Error("second ingest: kits/k's stored typed edges were purged although its load failed (fallback rows must stand)")
	}
	if typedEdgesFrom(t, st, "newergo", "kits/ok/") < 2 {
		t.Error("second ingest: kits/ok did not re-resolve (main -> more missing)")
	}
	if typedEdgesFrom(t, st, "newergo", "root.go") == 0 {
		t.Error("second ingest: the root module's typed edges are gone")
	}
}

func TestTypedPassRepoWithOnlyNestedModules(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"README.md":             "no root module\n",
		"tools/a/go.mod":        "module example.com/tools/a\n\ngo 1.21\n",
		"tools/a/cmd/a/main.go": "package main\n\nimport \"example.com/tools/a/lib\"\n\nfunc main() { lib.Do() }\n",
		"tools/a/lib/lib.go":    "package lib\n\nfunc Do() {}\n",
		"tools/b/go.mod":        "module example.com/tools/b\n\ngo 1.21\n",
		"tools/b/cmd/b/main.go": "package main\n\nimport \"example.com/tools/b/lib\"\n\nfunc main() { lib.Do() }\n",
		"tools/b/lib/lib.go":    "package lib\n\nfunc Do() {}\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	rep, err := pipe.Ingest(ctx, core.ServiceRef{ID: "nestedonly", Path: dir})
	if err != nil || rep.TypedFailed {
		t.Fatalf("Ingest: err=%v failed=%v %s", err, rep.TypedFailed, rep.TypedFailReason)
	}
	typed, _ := st.TypedEdges(ctx, "nestedonly")
	want := map[string]bool{
		core.SymbolKey("nestedonly", "tools/a/cmd/a/main", "main") + " -> " + core.SymbolKey("nestedonly", "tools/a/lib/lib", "Do"): false,
		core.SymbolKey("nestedonly", "tools/b/cmd/b/main", "main") + " -> " + core.SymbolKey("nestedonly", "tools/b/lib/lib", "Do"): false,
	}
	for _, e := range typed {
		if k := e.FromSymbol + " -> " + e.ToSymbol; e.Kind == core.EdgeDirect {
			if _, ok := want[k]; ok {
				want[k] = true
			}
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("missing typed edge %s (a repository without a root go.mod skipped the typed pass)", k)
		}
	}
}

func TestTypedPassNestedModulesSharingAModulePathStoreApart(t *testing.T) {
	cli := "package cli\n\nfunc Run() int { return step() }\n\nfunc step() int { return 0 }\n"
	dir := writeGoModule(t, map[string]string{
		"go.mod":                        "module example.com/twin\n\ngo 1.21\n",
		"root.go":                       "package twin\n",
		"kits/a/third_party/cli/go.mod": "module example.com/cli\n\ngo 1.21\n",
		"kits/a/third_party/cli/cli.go": cli,
		"kits/b/third_party/cli/go.mod": "module example.com/cli\n\ngo 1.21\n",
		"kits/b/third_party/cli/cli.go": cli,
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	if _, err := fusion.EnrichTypedEdges(ctx, st, "twin", dir, nil); err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	for _, prefix := range []string{"kits/a/", "kits/b/"} {
		if typedEdgesFrom(t, st, "twin", prefix) == 0 {
			t.Errorf("no typed edge from %s: two modules named example.com/cli overwrote each other's rows", prefix)
		}
	}
	keys, err := st.ListTypedEdgePackages(ctx, "twin")
	if err != nil {
		t.Fatal(err)
	}
	var cliKeys int
	for _, k := range keys {
		if strings.HasSuffix(k, "example.com/cli") {
			cliKeys++
		}
	}
	if cliKeys != 2 {
		t.Errorf("store keys %v: want one example.com/cli key per module", keys)
	}
}
