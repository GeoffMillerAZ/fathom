package callgraph

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The oracle deliberately resolves syntax twice, as the serving path did before
// W7-1. Do not replace this with the candidate's clone operation.
func twiceResolvedBoth(nodes []core.SymbolNode, typed []core.Edge) *Graph {
	g := Build(nodes)
	AddTypedEdges(g, typed)
	return g
}

func cloneBothOrders(nodes []core.SymbolNode) [][]core.SymbolNode {
	forward := slices.Clone(nodes)
	reverse := slices.Clone(nodes)
	slices.Reverse(reverse)
	sorted := slices.Clone(nodes)
	slices.SortStableFunc(sorted, func(a, b core.SymbolNode) int { return strings.Compare(a.Key(), b.Key()) })
	shuffled := slices.Clone(nodes)
	rand.New(rand.NewSource(218)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return [][]core.SymbolNode{forward, reverse, sorted, shuffled}
}

func cloneBothEdges(nodes []core.SymbolNode) []core.Edge {
	if len(nodes) < 2 {
		return nil
	}
	from, to := nodes[0], nodes[1]
	for _, n := range nodes {
		if n.Kind == core.KindGoMethod {
			to = n
			break
		}
	}
	return []core.Edge{
		{FromSymbol: from.Key(), ToSymbol: to.Key(), FromFile: from.FilePath, Kind: core.EdgeDirect},
		{FromSymbol: from.Key(), ToSymbol: to.Key(), FromFile: from.FilePath, Kind: core.EdgeIfaceDispatch},
		{FromSymbol: from.Key(), ToSymbol: nodes[len(nodes)-1].Key(), FromFile: from.FilePath, Kind: core.EdgeInstantiates},
		{FromSymbol: from.Key(), ToSymbol: "<unresolved:1>", FromFile: from.FilePath, Kind: core.EdgeUnresolved},
		{FromSymbol: "svc::initpkg.init", ToSymbol: to.Key(), FromFile: "initpkg/init.go", Kind: core.EdgeFuncValue},
		{FromSymbol: "svc::missing.Nope", ToSymbol: to.Key(), Kind: core.EdgeDirect},
		{FromSymbol: from.Key(), ToSymbol: "svc::missing.Nope", Kind: core.EdgeDirect},
		{},
	}
}

func cloneBothFixture(t *testing.T, root string) []core.SymbolNode {
	t.Helper()
	var nodes []core.SymbolNode
	ex := extractor.NewGoExtractor()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		ns, err := ex.Extract("svc", filepath.ToSlash(rel), b)
		if err != nil {
			return err
		}
		nodes = append(nodes, ns...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatalf("fixture extraction incomplete: %d nodes", len(nodes))
	}
	return nodes
}

func assertCloneBothEqual(t *testing.T, got, want *Graph) {
	t.Helper()
	// DeepEqual checks ALL Graph fields, including private indexes, nil/empty
	// distinctions, typed provenance, instantiation and dispatch metadata.
	if !reflect.DeepEqual(got, want) {
		t.Fatal("combined graph differs from twice-resolved oracle (all Graph fields)")
	}
	// Clone helpers trim spare adjacency capacity. Account for the exact
	// retained-capacity difference rather than requiring identical heap cost.
	var capacityDelta int64
	for _, pair := range [][2]map[string][]string{{got.callees, want.callees}, {got.callers, want.callers}, {got.byNameAll, want.byNameAll}, {got.instantiates, want.instantiates}} {
		for key, vs := range pair[0] {
			capacityDelta += int64(cap(vs)-cap(pair[1][key])) * strHdr
		}
	}
	if got.EstimatedBytes()-want.EstimatedBytes() != capacityDelta {
		t.Fatal("byte estimate does not account for cloned adjacency capacity")
	}
	for k := range want.nodes {
		if !reflect.DeepEqual(CallersDetail(got, k), CallersDetail(want, k)) ||
			!reflect.DeepEqual(CalleesDetail(got, k), CalleesDetail(want, k)) ||
			!reflect.DeepEqual(TestImpactDetail(got, k), TestImpactDetail(want, k)) ||
			!reflect.DeepEqual(CandidateKeys(got, k), CandidateKeys(want, k)) {
			t.Fatalf("public graph output differs for %s", k)
		}
	}
	if !reflect.DeepEqual(ProductEntryReach(got), ProductEntryReach(want)) {
		t.Fatal("product reach changed")
	}
}

func TestCloneBothTwiceResolvedEquality(t *testing.T) {
	sets := [][]core.SymbolNode{nil, {}, cloneBothFixture(t, "testdata/servedcalls"), cloneBothFixture(t, "../../connector/testdata/fauxtown")}
	for _, size := range []int{0, 1, 8, 64, 128} {
		sets = append(sets, indexAllocationFixture(size))
	}
	rng := rand.New(rand.NewSource(701))
	for seed := 0; seed < 12; seed++ {
		nodes := indexAllocationFixture(seed)
		for i := 0; i < 32; i++ {
			kind := core.KindGoMethod
			if i%7 == 0 {
				kind = core.KindGoField
			}
			nodes = append(nodes, core.SymbolNode{Service: "svc", Package: fmt.Sprintf("p%d", i%4), Name: fmt.Sprintf("R%d.M%d", i, rng.Intn(6)), Kind: kind, FilePath: fmt.Sprintf("p%d/f%d.go", i%4, i%3), Calls: []string{fmt.Sprintf("x.M%d", rng.Intn(6)), "alpha.Unique", "missing.Never"}})
		}
		sets = append(sets, nodes)
	}
	for si, nodes := range sets {
		for oi, order := range cloneBothOrders(nodes) {
			for ti, typed := range [][]core.Edge{nil, {}, cloneBothEdges(order)} {
				t.Run(fmt.Sprintf("set%d/order%d/typed%d", si, oi, ti), func(t *testing.T) {
					syntax := Build(order)
					before := Build(order)
					got := syntax.WithTypedEdges(typed)
					want := twiceResolvedBoth(order, typed)
					assertCloneBothEqual(t, got, want)
					if got == syntax {
						t.Fatal("combined graph aliases syntax graph")
					}
					assertCloneBothEqual(t, syntax, before)
					// Exercise mutation in both directions after augmentation, including
					// appending/sorting/pruning shared adjacency backing arrays.
					AddTypedEdges(got, cloneBothEdges(order))
					assertCloneBothEqual(t, syntax, before)
					frozen := twiceResolvedBoth(order, typed)
					AddTypedEdges(frozen, cloneBothEdges(order))
					AddTypedEdges(syntax, cloneBothEdges(order))
					assertCloneBothEqual(t, got, frozen)
				})
			}
		}
	}
}

// Freeze the previous pruning algorithm to price P1 independently. Every
// surviving edge was copied into a discarded buffer when no edge was removed.
func discardedCopyPrune(g *Graph) {
	if len(g.typedFiles) == 0 {
		return
	}
	for c, cs := range g.callees {
		if !g.typedFiles[g.nodes[c].FilePath] {
			continue
		}
		kept := cs[:0:0]
		for _, m := range cs {
			if _, typed := g.edgeKind[c+"\x00"+m]; !typed && g.nodes[m].Kind == core.KindGoMethod {
				g.callers[m] = removeOne(g.callers[m], c)
				continue
			}
			kept = append(kept, m)
		}
		if len(kept) != len(cs) {
			g.callees[c] = kept
		}
	}
}

func TestCloneBothPruneAvoidsDiscardedCopies(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "p", Name: "Caller", FilePath: "p/p.go", Calls: []string{"A", "B", "C"}},
		{Service: "svc", Package: "p", Name: "A"}, {Service: "svc", Package: "p", Name: "B"}, {Service: "svc", Package: "p", Name: "C"},
	}
	g := Build(nodes)
	g.typedFiles = map[string]bool{"p/p.go": true}
	want := Build(nodes)
	want.typedFiles = map[string]bool{"p/p.go": true}
	discardedCopyPrune(want)
	baseline := testing.AllocsPerRun(10, func() { discardedCopyPrune(g) })
	candidate := testing.AllocsPerRun(10, func() { g.dropTypedFileGuesses(nil) })
	assertCloneBothEqual(t, g, want)
	if candidate >= baseline {
		t.Fatalf("discarded pruning allocations remain: candidate %.0f baseline %.0f", candidate, baseline)
	}
}

func TestCloneBothTypedPruningAndIsolation(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "p", Name: "Entry", Kind: core.KindGoFunc, FilePath: "p/p.go", Calls: []string{"x.Guess", "x.Keep", "Type"}},
		{Service: "svc", Package: "p", Name: "T.Guess", Kind: core.KindGoMethod, FilePath: "p/t.go"},
		{Service: "svc", Package: "p", Name: "T.Keep", Kind: core.KindGoMethod, FilePath: "p/t.go"},
		{Service: "svc", Package: "p", Name: "Type", Kind: core.KindGoType, FilePath: "p/t.go"},
	}
	edges := []core.Edge{
		{FromSymbol: nodes[0].Key(), ToSymbol: nodes[2].Key(), FromFile: "p/p.go", Kind: core.EdgeIfaceDispatch},
		{FromSymbol: nodes[0].Key(), ToSymbol: nodes[3].Key(), FromFile: "p/p.go", Kind: core.EdgeInstantiates},
	}
	syntax := Build(nodes)
	got := syntax.WithTypedEdges(edges)
	if !Reachable(syntax, nodes[0].Key(), nodes[1].Key()) {
		t.Fatal("fixture lacks syntax method guess")
	}
	if Reachable(got, nodes[0].Key(), nodes[1].Key()) {
		t.Fatal("typed pruning kept syntax-only method guess")
	}
	if !Reachable(got, nodes[0].Key(), nodes[2].Key()) || got.dispatchOut[nodes[0].Key()] != 1 {
		t.Fatal("typed dispatch lost")
	}
	if len(got.instantiates[nodes[0].Key()]) != 1 || !got.typedDirs["p"] {
		t.Fatal("typed instantiation/provenance lost")
	}
	assertCloneBothEqual(t, syntax, Build(nodes))
	assertCloneBothEqual(t, got, twiceResolvedBoth(nodes, edges))
	// A second clone starts from an already-typed graph, so every optional map
	// and every adjacency slice is populated before exercising isolation.
	frozen := twiceResolvedBoth(nodes, edges)
	cp := got.WithTypedEdges(nil)
	assertCloneBothEqual(t, cp, frozen)
	cp.nodes["sentinel"] = core.SymbolNode{Name: "sentinel"}
	cp.callees[nodes[0].Key()][0] = "sentinel"
	cp.callers[nodes[2].Key()][0] = "sentinel"
	cp.byName["sentinel"] = "sentinel"
	cp.byPkgName["sentinel"] = "sentinel"
	cp.byDirName["sentinel"] = "sentinel"
	cp.byNameAll[nodes[0].Name][0] = "sentinel"
	cp.bySuffix["sentinel"] = suffixCandidate{key: "sentinel", count: 1}
	cp.byDotSuffix["sentinel"] = suffixCandidate{key: "sentinel", count: 1}
	cp.edgeKind["sentinel"] = core.EdgeDirect
	cp.dispatchOut[nodes[0].Key()] = 99
	cp.instantiates[nodes[0].Key()][0] = "sentinel"
	cp.typedFiles["sentinel"] = true
	cp.typedDirs["sentinel"] = true
	assertCloneBothEqual(t, got, frozen)
}

func TestCloneBothPruneMatchesDiscardedCopyOracle(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		nodes := []core.SymbolNode{{Service: "svc", Package: "p", Name: "Caller", FilePath: "p/p.go"}}
		for i := 0; i < 4; i++ {
			n := core.SymbolNode{Service: "svc", Package: "p", Name: fmt.Sprintf("T.M%d", i), Kind: core.KindGoMethod}
			if i == 2 {
				n.Kind = core.KindGoFunc
			}
			nodes = append(nodes, n)
			nodes[0].Calls = append(nodes[0].Calls, n.Key())
		}
		setup := func() *Graph {
			g := Build(nodes)
			g.typedFiles = map[string]bool{"p/p.go": true}
			g.edgeKind = map[string]core.EdgeKind{}
			for i, n := range nodes[1:] {
				if mask&(1<<i) != 0 {
					g.edgeKind[nodes[0].Key()+"\x00"+n.Key()] = core.EdgeDirect
				}
			}
			return g
		}
		got, want := setup(), setup()
		gBefore := Build(nodes)
		discardedCopyPrune(want)
		got.dropTypedFileGuesses(nil)
		assertCloneBothEqual(t, got, want)
		// Idempotence and the explicit caller subset are independent entry modes.
		got.dropTypedFileGuesses([]string{nodes[0].Key()})
		assertCloneBothEqual(t, got, want)
		if gBefore.EdgeCount() != 4 {
			t.Fatal("pruning fixture incomplete")
		}
	}
}

func TestCloneBothPreservesPatchIneligibility(t *testing.T) {
	nodes := indexAllocationFixture(8)
	fresh := Build(nodes)
	if !fresh.CanReuseSyntax() {
		t.Fatal("fresh syntax is ineligible")
	}
	patched := fresh.WithPackagePatch("not-present", nil, nil)
	if patched.CanReuseSyntax() {
		t.Fatal("package-patched syntax is eligible")
	}
	cloned := patched.WithTypedEdges(cloneBothEdges(nodes))
	if cloned.CanReuseSyntax() {
		t.Fatal("cloning promoted an approximate graph")
	}
	if !fresh.CanReuseSyntax() {
		t.Fatal("patch mutated held fresh graph provenance")
	}
	if !fresh.WithTypedEdges(nil).CanReuseSyntax() {
		t.Fatal("clone lost fresh graph provenance")
	}
}
