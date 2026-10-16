package callgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// selectReachModule is a module where three error types reach one
// err.Error() call site through the error interface: p's PErr, built only
// by p.NewP, q's QErr, built only by q.NewQ, and p's GErr, built by a p
// package variable, so constructed for every test whose reach links p.
var selectReachModule = map[string]string{
	"go.mod":     "module example.com/sel\n\ngo 1.22\n",
	"use/use.go": "package use\n\nfunc Describe(err error) string { return err.Error() }\n",
	"p/p.go": `package p

type PErr struct{}

func (*PErr) Error() string { return "p" }

func NewP() error { return &PErr{} }

type GErr struct{}

func (GErr) Error() string { return "g" }

var Sentinel error = GErr{}

func Name() string { return "p" }
`,
	"q/q.go": `package q

type QErr struct{}

func (*QErr) Error() string { return "q" }

func NewQ() error { return &QErr{} }
`,
	"p/p_test.go": `package p_test

import (
	"testing"

	"example.com/sel/p"
	"example.com/sel/use"
)

func TestP(t *testing.T) { _ = use.Describe(p.NewP()) }
`,
	"q/q_test.go": `package q_test

import (
	"testing"

	"example.com/sel/q"
	"example.com/sel/use"
)

func TestQ(t *testing.T) { _ = use.Describe(q.NewQ()) }
`,
	"q/qp_test.go": `package q_test

import (
	"testing"

	"example.com/sel/p"
	"example.com/sel/q"
	"example.com/sel/use"
)

func TestQLinksP(t *testing.T) { _ = use.Describe(q.NewQ()) + p.Name() }
`,
}

func ingestModule(t *testing.T, service string, files map[string]string) *callgraph.Graph {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	t.Cleanup(pl.Close)
	if _, err := pl.Ingest(ctx, core.ServiceRef{ID: service, Path: root}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	typed, err := st.TypedEdges(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	if len(typed) == 0 {
		t.Fatal("the typed pass wrote no edges: the module did not type-check")
	}
	return callgraph.BuildBoth(nodes, typed)
}

// TestSelectionReach_DispatchNeedsConstruction is TQ-30's rule over a real
// ingest: an interface dispatch into a type's method counts for a test only
// when the test's reach constructs the type, or the initializer of a
// package that reach links does (TestQLinksP calls p.Name, so p's
// variable initializer runs and GErr counts; TestQ never links p).
// The plain forward reach (ForwardReach) still fans out to every
// implementation, and stays as it was.
func TestSelectionReach_DispatchNeedsConstruction(t *testing.T) {
	g := ingestModule(t, "sel", selectReachModule)
	const (
		testP  = "sel::p/p_test.TestP"
		testQ  = "sel::q/q_test.TestQ"
		testQP = "sel::q/q_test.TestQLinksP"
		pErr   = "sel::p/PErr.PErr.Error"
		qErr   = "sel::q/QErr.QErr.Error"
		gErr   = "sel::p/GErr.GErr.Error"
	)
	got := callgraph.SelectionReach(g, []string{testP, testQ, testQP}, []string{pErr, qErr, gErr})
	want := map[string][]callgraph.SelectionHit{
		testP:  {{Target: gErr, ViaDispatch: true}, {Target: pErr, ViaDispatch: true}},
		testQ:  {{Target: qErr, ViaDispatch: true}},
		testQP: {{Target: gErr, ViaDispatch: true}, {Target: qErr, ViaDispatch: true}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectionReach =\n%+v\nwant\n%+v", got, want)
	}

	plain := callgraph.ForwardReach(g, []string{testQ}).Keys(testQ)
	found := false
	for _, k := range plain {
		found = found || k == pErr
	}
	if !found {
		t.Errorf("ForwardReach(TestQ) = %v: the stored dispatch edge to PErr.Error must stay for every other reader", plain)
	}
}

// TestSelectionReach_NoFactsKeepsEveryImplementation: a graph whose typed
// edges carry no instantiation facts keeps the old reading.
func TestSelectionReach_NoFactsKeepsEveryImplementation(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "use", Name: "Describe", Kind: core.KindGoFunc, FilePath: "use/use.go"},
		{Service: "s", Package: "PErr", Name: "PErr.Error", Kind: core.KindGoMethod, FilePath: "p/p.go"},
		{Service: "s", Package: "q_test", Name: "TestQ", Kind: core.KindGoFunc, FilePath: "q/q_test.go", Calls: []string{"use.Describe"}},
	}
	typed := []core.Edge{
		{FromService: "s", FromFile: "use/use.go", FromSymbol: nodes[0].Key(), ToSymbol: nodes[1].Key(), Kind: core.EdgeIfaceDispatch},
		{FromService: "s", FromFile: "q/q_test.go", FromSymbol: nodes[2].Key(), ToSymbol: nodes[0].Key(), Kind: core.EdgeDirect},
	}
	g := callgraph.BuildBoth(nodes, typed)
	got := callgraph.SelectionReach(g, []string{nodes[2].Key()}, []string{nodes[1].Key()})
	want := map[string][]callgraph.SelectionHit{nodes[2].Key(): {{Target: nodes[1].Key(), ViaDispatch: true}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectionReach = %+v, want %+v", got, want)
	}
}
