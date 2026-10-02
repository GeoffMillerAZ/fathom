package callgraph_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// servedSets is one graph's caller/callee answers over the servedcalls
// fixture, each edge written "key (kind)", kind "syntax" for a syntax-only
// edge.
type servedSets struct {
	Callers    map[string][]string `json:"callers"`
	Callees    map[string][]string `json:"callees"`
	Referrers  map[string][]string `json:"referrers,omitempty"`
	References map[string][]string `json:"references,omitempty"`
	TestImpact map[string][]string `json:"test_impact"`
}

// servedFixture ingests testdata/servedcalls (the Go extractor and the
// typed pass, as ingest runs them) and returns its nodes and typed edges.
func servedFixture(t *testing.T) ([]core.SymbolNode, []core.Edge) {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "servedcalls"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "served", Path: src}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, "served", "")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Key() < nodes[j].Key() })
	typed, err := st.TypedEdges(ctx, "served")
	if err != nil {
		t.Fatal(err)
	}
	return nodes, typed
}

func labelled(ds []callgraph.EdgeInfo) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		k := string(d.Kind)
		if k == "" {
			k = "syntax"
		}
		out = append(out, d.Key+" ("+k+")")
	}
	return out
}

// servedGolden is testdata/servedcalls.golden.json.
type servedGolden struct {
	Before servedSets `json:"before"`
	After  servedSets `json:"after"`
}

func readServedGolden(t *testing.T) servedGolden {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "servedcalls.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g servedGolden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// servedAnswers reads every answer the golden pins off g, for every node.
func servedAnswers(g *callgraph.Graph, nodes []core.SymbolNode) servedSets {
	s := servedSets{Callers: map[string][]string{}, Callees: map[string][]string{}, Referrers: map[string][]string{},
		References: map[string][]string{}, TestImpact: map[string][]string{}}
	put := func(m map[string][]string, k string, v []string) {
		if len(v) > 0 {
			m[k] = v
		}
	}
	for _, n := range nodes {
		k := n.Key()
		put(s.Callers, k, labelled(callgraph.CallersDetail(g, k)))
		put(s.Callees, k, labelled(callgraph.CalleesDetail(g, k)))
		put(s.Referrers, k, labelled(callgraph.ReferrersDetail(g, k)))
		put(s.References, k, labelled(callgraph.ReferencesDetail(g, k)))
		put(s.TestImpact, k, callgraph.TestImpact(g, k))
	}
	return s
}

// TestServedCalls_Golden is wave lg's E and D (LB-F199, LB-F200) on a real
// ingest of testdata/servedcalls: the served graph (BuildBoth) no longer
// answers a typed file's dependency call by bare name (Same's bytes.Equal
// and mu.Lock bound to the service's Store.Equal and Store.Lock), and a
// type written in Build is a reference, not a call, which callers and
// callees leave out and the reach walkers still follow. A build-excluded
// file, whose call sites the typed pass never saw, keeps its syntax edge.
func TestServedCalls_Golden(t *testing.T) {
	nodes, typed := servedFixture(t)
	golden := readServedGolden(t)
	got := servedAnswers(callgraph.BuildBoth(nodes, typed), nodes)
	if !reflect.DeepEqual(got, golden.After) {
		gb, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("served answers differ from testdata/servedcalls.golden.json \"after\":\n%s", gb)
	}

	// Every edge the pre-lg graph served is still served, or is now a
	// labelled reference, or is E's class: a syntax-only edge into a method
	// from a file the typed pass resolved. Nothing else left.
	kindOf := map[string]core.SourceKind{}
	fileOf := map[string]string{}
	for _, n := range nodes {
		kindOf[n.Key()], fileOf[n.Key()] = n.Kind, n.FilePath
	}
	typedFile := map[string]bool{}
	for _, e := range typed {
		typedFile[e.FromFile] = true
	}
	var dropped []string
	for callee, callers := range golden.Before.Callers {
		for _, c := range callers {
			caller, kind, _ := strings.Cut(strings.TrimSuffix(c, ")"), " (")
			switch {
			case slices.Contains(got.Callers[callee], c):
			case slices.Contains(got.Referrers[callee], caller+" ("+string(core.EdgeTypeRef)+")"):
			case kind == "syntax" && kindOf[callee] == core.KindGoMethod && typedFile[fileOf[caller]]:
				dropped = append(dropped, caller+" -> "+callee)
			default:
				t.Errorf("pre-lg edge %s -> %s (%s) is neither served, a reference, nor E's class", caller, callee, kind)
			}
		}
	}
	sort.Strings(dropped)
	want := []string{"served::app/app.Same -> served::svc/Store.Store.Equal", "served::app/app.Same -> served::svc/Store.Store.Lock"}
	if !reflect.DeepEqual(dropped, want) {
		t.Errorf("E dropped %v, want %v", dropped, want)
	}
}

// TestServedCalls_ReplacePackageEqualsFresh: a package patch of the served
// graph (CachedCallGraph.InvalidatePackage, memo.PatchPackage) answers
// exactly as a fresh BuildBoth over the same data, with the package's typed
// edges replaced and with them left alone (nil), so E and D hold on the
// patched graph too.
func TestServedCalls_ReplacePackageEqualsFresh(t *testing.T) {
	nodes, typed := servedFixture(t)
	fresh := servedAnswers(callgraph.BuildBoth(nodes, typed), nodes)

	var appNodes, stale []core.SymbolNode
	for _, n := range nodes {
		if n.Package == "app" {
			appNodes = append(appNodes, n)
			n.Calls = nil // before the edit, app called nothing
		}
		stale = append(stale, n)
	}
	var appTyped []core.Edge
	for _, e := range typed {
		if strings.HasPrefix(e.FromFile, "app/") {
			appTyped = append(appTyped, e)
		}
	}
	var otherTyped []core.Edge
	for _, e := range typed {
		if !strings.HasPrefix(e.FromFile, "app/") {
			otherTyped = append(otherTyped, e)
		}
	}

	patched := callgraph.BuildBoth(stale, otherTyped).ReplacePackage("app", appNodes, appTyped)
	if got := servedAnswers(patched, nodes); !reflect.DeepEqual(got, fresh) {
		t.Errorf("ReplacePackage(app, nodes, typed) differs from a fresh BuildBoth:\ngot  %+v\nwant %+v", got, fresh)
	}
	kept := callgraph.BuildBoth(nodes, typed).ReplacePackage("app", appNodes, nil)
	if got := servedAnswers(kept, nodes); !reflect.DeepEqual(got, fresh) {
		t.Errorf("ReplacePackage(app, nodes, nil) differs from a fresh BuildBoth (typed edges must be left alone):\ngot  %+v\nwant %+v", got, fresh)
	}
}
