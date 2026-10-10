package fusion_test

// typed_nestedmodules_test.go — EW-59/EW-60 (docs/plans/efficiency-wave.md,
// O779): a repository whose binaries live in nested Go modules (a go.mod
// below the root, as kits/*/go.mod) had no typed edge out of any of them,
// because the typed pass was ONE packages.Load of "./..." from the root and
// "./..." never enters a directory holding its own go.mod. The syntax pass
// could not make up for it: two kits each declaring cmd.NewRoot leave the
// name-only call "cmd.NewRoot" ambiguous, so it binds to nothing — every
// nested main was a product entry that reached itself and nothing else, and
// verify read the whole module library-only.
//
// The fixture (testdata/nestedmodules) mirrors that shape with example.com
// names: a root module, two nested modules kits/a and kits/b each with a
// cmd/<x>/main.go calling cmd.NewRoot(), kits/a requiring an in-repo module
// outside itself (replace => ../../shared) and a vendored-style framework
// inside itself (replace => ./third_party/cli), and a cobra-shaped leaf
// wired only as a function value through a wrapper: RunE: runE(runDoctor, opts).

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// graphEvidence is verify.Evidence + verify.ProductEvidence over a real
// ingested graph — the same reads the CLI's verify makes (BuildBoth +
// ProductEntryReach), minus the store client.
type graphEvidence struct {
	nodes   []core.SymbolNode
	graph   *callgraph.Graph
	product *callgraph.ProductReach
}

func (e graphEvidence) FindSymbol(name string) []core.SymbolNode {
	var out []core.SymbolNode
	for _, n := range e.nodes {
		if n.Name == name || n.Key() == name {
			out = append(out, n)
		}
	}
	return out
}
func (e graphEvidence) TestReaches(symbol string) []string {
	return callgraph.TestImpact(e.graph, symbol)
}
func (e graphEvidence) Reachable(from, to string) bool       { return callgraph.Reachable(e.graph, from, to) }
func (e graphEvidence) Contradictions() []core.Contradiction { return nil }
func (e graphEvidence) SpecImplementors(string) []string     { return nil }
func (e graphEvidence) ProductEntryCount() int               { return len(e.product.Entries()) }
func (e graphEvidence) ProductReachedBy(s string) (string, bool) {
	return e.product.ReachedBy(s)
}

const nestedSvc = "nestedmodules"

// ingestNestedModules ingests testdata/nestedmodules through the real
// pipeline (syntax pass, then the typed pass) and returns its nodes and
// typed edges.
func ingestNestedModules(t *testing.T) ([]core.SymbolNode, []core.Edge) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "nestedmodules"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	rep, err := pipe.Ingest(ctx, core.ServiceRef{ID: nestedSvc, Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.TypedPackagesFailed != 0 || rep.TypedFailed {
		t.Errorf("typed pass: %d package(s) failed to load, want 0 and no failure (a replace directive must not break a nested module's load): %s", rep.TypedPackagesFailed, rep.TypedFailReason)
	}
	obs, err := st.QueryObservations(ctx, nestedSvc, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	nodes := make([]core.SymbolNode, len(obs))
	for i, o := range obs {
		nodes[i] = o.ToSymbolNode()
	}
	typed, err := st.TypedEdges(ctx, nestedSvc)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	return nodes, typed
}

func nestedKey(pkg, name string) string { return core.SymbolKey(nestedSvc, pkg, name) }

// TestTypedPassLoadsNestedModules is EW-59: each nested module's own typed
// edges exist, keyed by the repo-relative directory exactly as the syntax
// pass keys its nodes — an import of a nested module's package resolves to
// that module's own directory, and a replace directive into another in-repo
// module resolves to that module's.
func TestTypedPassLoadsNestedModules(t *testing.T) {
	_, typed := ingestNestedModules(t)
	mainA := nestedKey("kits/a/cmd/toola/main", "main")
	newRootA := nestedKey("kits/a/internal/cmd/cmd", "NewRoot")
	runDoctor := nestedKey("kits/a/internal/cmd/cmd", "runDoctor")
	mainB := nestedKey("kits/b/cmd/toolb/main", "main")
	newRootB := nestedKey("kits/b/internal/cmd/cmd", "NewRoot")
	banner := nestedKey("shared/shared", "Banner")

	has := func(from, to string, kind core.EdgeKind) bool {
		for _, e := range typed {
			if e.FromSymbol == from && e.ToSymbol == to && e.Kind == kind {
				return true
			}
		}
		return false
	}
	for _, want := range []struct {
		from, to string
		kind     core.EdgeKind
	}{
		{mainA, newRootA, core.EdgeDirect},
		{mainB, newRootB, core.EdgeDirect},
		{newRootA, runDoctor, core.EdgeFuncValue},
		// across modules, through replace => ../../shared
		{runDoctor, banner, core.EdgeDirect},
	} {
		if !has(want.from, want.to, want.kind) {
			t.Errorf("no %s typed edge %s -> %s (the typed pass never loaded the nested module)", want.kind, want.from, want.to)
		}
	}
	if has(mainA, newRootB, core.EdgeDirect) || has(mainB, newRootA, core.EdgeDirect) {
		t.Errorf("a main's typed edge crossed into the other kit's NewRoot")
	}
	var typedFromKitB int
	for _, e := range typed {
		if strings.HasPrefix(e.FromFile, "kits/b/") {
			typedFromKitB++
		}
	}
	if typedFromKitB == 0 {
		t.Errorf("no typed edge from any file in kits/b")
	}

}

// TestNestedModuleCobraLeafReadsWired is EW-60: a nested module's main is a
// product entry, it reaches a cobra-style leaf wired only as a function
// value through a wrapper (RunE: runE(runDoctor, opts)), and a verify row
// on the leaf reads wired: pass — the syntax graph alone cannot, because
// "cmd.NewRoot" names two kits' functions.
func TestNestedModuleCobraLeafReadsWired(t *testing.T) {
	nodes, typed := ingestNestedModules(t)
	mainA := nestedKey("kits/a/cmd/toola/main", "main")
	mainB := nestedKey("kits/b/cmd/toolb/main", "main")
	runDoctor := nestedKey("kits/a/internal/cmd/cmd", "runDoctor")

	if by, ok := callgraph.ProductEntryReach(callgraph.Build(nodes)).ReachedBy(runDoctor); ok {
		t.Errorf("the syntax graph alone reaches runDoctor from %s: the fixture no longer carries the ambiguous cmd.NewRoot this test exists for", by)
	}

	graph := callgraph.BuildBoth(nodes, typed)
	pr := callgraph.ProductEntryReach(graph)
	for _, entry := range []string{mainA, mainB} {
		found := false
		for _, e := range pr.Entries() {
			if e == entry {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not a product entry; entries: %v", entry, pr.Entries())
		}
	}
	if by, ok := pr.ReachedBy(runDoctor); !ok || by != mainA {
		t.Errorf("ProductEntryReach: runDoctor reached by %q (ok=%v), want %s", by, ok, mainA)
	}

	// The verify row on the leaf reads wired: pass.
	plan := verify.ParseMarkdownPlan(nestedSvc, []byte("- [x] The doctor leaf is wired. {id: NM-1, symbol: runDoctor, package: kits/a/internal/cmd, reachable_from: TestRunDoctor}\n"))
	vr := verify.VerifyPlan(plan, graphEvidence{nodes: nodes, graph: graph, product: pr})
	if len(vr.Tasks) != 1 {
		t.Fatalf("verify: %d task verdicts, want 1", len(vr.Tasks))
	}
	var wired *verify.Criterion
	for i, c := range vr.Tasks[0].Criteria {
		if c.Name == "wired" {
			wired = &vr.Tasks[0].Criteria[i]
		}
	}
	if wired == nil || wired.Status != verify.Pass {
		t.Errorf("NM-1 wired criterion = %+v, want pass (label %q)", wired, vr.Tasks[0].Label)
	}
}
