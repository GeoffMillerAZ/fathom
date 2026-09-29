package memo

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func fixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "a", Name: "Handler", Kind: core.KindGoFunc, FilePath: "a/h.go", Line: 3, Calls: []string{"b.Store", "svc::c.Log"}},
		{Service: "svc", Package: "b", Name: "Store", Kind: core.KindGoFunc, FilePath: "b/s.go", Line: 7, Calls: []string{"c.Log"}},
		{Service: "svc", Package: "c", Name: "Log", Kind: core.KindGoFunc, FilePath: "c/l.go", Line: 1},
		{Service: "svc", Package: "d", Name: "log", Kind: core.KindGoFunc, FilePath: "d/l.go", Line: 2},
	}
}

// TestGraphStore_OneGraphPerServiceWatermark is RT-74 (§5, I51): every
// caller of a (store, service, watermark, kind) gets the one graph, built
// once however many ask at once; a new watermark builds anew and evicts
// the old; the kinds are separate keys; a scope without a watermark is
// built per call and never cached; and in the serving paths (internal/mcp
// and the console) no code builds a call graph but the graph store.
func TestGraphStore_OneGraphPerServiceWatermark(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	var loads, typedLoads atomic.Int32
	load := func(context.Context) ([]core.SymbolNode, error) { loads.Add(1); return fixtureNodes(), nil }
	typed := func(context.Context) ([]core.Edge, error) {
		typedLoads.Add(1)
		return []core.Edge{{FromSymbol: "svc::a.Handler", ToSymbol: "svc::d.log", Kind: core.EdgeDirect}}, nil
	}
	sc := Scope{Store: "gs-test", Service: "svc", Watermark: "w1"}

	var wg sync.WaitGroup
	graphs := make(chan any, 40)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, err := SnapshotFor(ctx, sc, load)
			if err != nil {
				t.Error(err)
				return
			}
			g, err := GraphFor(ctx, snap, GraphBoth, typed)
			if err != nil {
				t.Error(err)
				return
			}
			graphs <- g
		}()
	}
	wg.Wait()
	close(graphs)
	var first any
	for g := range graphs {
		if first == nil {
			first = g
		} else if g != first {
			t.Fatal("two callers at one (service, watermark, kind) got different graphs")
		}
	}
	if loads.Load() != 1 || typedLoads.Load() != 1 {
		t.Fatalf("loads = %d, typed loads = %d, want 1 each", loads.Load(), typedLoads.Load())
	}
	snap, _ := SnapshotFor(ctx, sc, load)
	syn, _ := GraphFor(ctx, snap, GraphSyntax, typed)
	if any(syn) == first {
		t.Fatal("syntax and both share one key")
	}
	if loads.Load() != 1 {
		t.Fatal("a second kind reloaded the symbols")
	}

	// A new watermark: one more load, and the old entries leave.
	sc2 := sc
	sc2.Watermark = "w2"
	snap2, _ := SnapshotFor(ctx, sc2, load)
	g2, _ := GraphFor(ctx, snap2, GraphBoth, typed)
	if any(g2) == first || loads.Load() != 2 {
		t.Fatalf("new watermark reused the old graph (loads %d)", loads.Load())
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
		t.Fatal("the superseded watermark's graph was not evicted")
	}

	// No watermark: built per call, never cached.
	nw := Scope{Store: "gs-test", Service: "fake"}
	before := loads.Load()
	for range 2 {
		s, _ := SnapshotFor(ctx, nw, load)
		_, _ = GraphFor(ctx, s, GraphSyntax, nil)
	}
	if loads.Load()-before != 2 {
		t.Fatal("a scope without a watermark was cached")
	}

	// The serving paths build no graph themselves — directly, or through
	// an analysis helper that builds one from a node slice (review
	// finding: the lexical scan once missed these). The graph-taking
	// variants (BlastRadiusOn, TestGapOn, FileNeighborhoodOn) are the
	// serving paths' form. conserve.Run/Residue, which read the store
	// directly, are the named exception (docs/plans/responsible-tenant.md
	// RT-74).
	build := regexp.MustCompile(`callgraph\.(Build|BuildBoth|BuildResolved)\(|impact\.(BlastRadius|BlastRadiusWithTypedEdges|TestGap)\(|findings\.FileNeighborhood\(`)
	var files []string
	mcpFiles, _ := filepath.Glob(filepath.Join("..", "mcp", "*.go"))
	consoleFiles, _ := filepath.Glob(filepath.Join("..", "..", "cmd", "fathom", "console_*.go"))
	files = append(append(files, mcpFiles...), consoleFiles...)
	if len(mcpFiles) == 0 || len(consoleFiles) == 0 {
		t.Fatal("serving-path sources not found")
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") {
				continue
			}
			if build.MatchString(line) {
				t.Errorf("%s builds a call graph outside the graph store: %s", f, trim)
			}
		}
	}
}

// TestSnapshot_NameIndexSharedByReaders is RT-75 (§5): one immutable
// snapshot per (store, service, watermark) serves every reader — a
// case-insensitive name index and a callee index answer point questions
// without a scan, lowered fields are stored once, and readers that sort
// get a copy, never the shared slice. It reads through SnapshotForExtra, the
// loader the product's readers call (the MCP navigation memo and the
// console's graph store), and pins that the loader's extra value rides on
// the one shared snapshot.
func TestSnapshot_NameIndexSharedByReaders(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	var loads atomic.Int32
	load := func(context.Context) ([]core.SymbolNode, any, error) {
		loads.Add(1)
		return fixtureNodes(), "disclosure", nil
	}
	sc := Scope{Store: "snap-test", Service: "svc", Watermark: "w1"}
	a, err := SnapshotForExtra(ctx, sc, load)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := SnapshotForExtra(ctx, sc, load)
	if a != b || loads.Load() != 1 {
		t.Fatalf("readers got different snapshots (loads %d)", loads.Load())
	}
	if a.Extra != "disclosure" {
		t.Fatalf("Extra = %v, want the loader's value kept on the snapshot", a.Extra)
	}
	if got := a.Named("LOG"); len(got) != 2 || got[0].Package != "c" || got[1].Package != "d" {
		t.Fatalf("Named(LOG) = %+v", got)
	}
	if got := a.Named("Nope"); len(got) != 0 {
		t.Fatalf("Named(Nope) = %+v", got)
	}
	callers := a.CallersOf("Log")
	if len(callers) != 2 || callers[0].Name != "Handler" || callers[1].Name != "Store" {
		t.Fatalf("CallersOf(Log) = %+v", callers)
	}
	if got := a.CallersOf("b.Store"); len(got) != 1 || got[0].Name != "Handler" {
		t.Fatalf("CallersOf(b.Store) = %+v", got)
	}
	if l := a.Lower(0); l.Name != "handler" || l.Ref != "a/h.go:3" || l.Kind != strings.ToLower(string(core.KindGoFunc)) {
		t.Fatalf("Lower(0) = %+v", l)
	}
	cp := a.NodesCopy()
	cp[0].Name = "Mutated"
	if a.Nodes[0].Name != "Handler" {
		t.Fatal("NodesCopy shares the snapshot's slice")
	}
	if st := snapshotGroup.stats(); st.Bytes <= 0 {
		t.Fatalf("snapshot bytes not accounted: %+v", st)
	}
}
