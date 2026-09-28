package mcp

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// resolution_fixture_test.go is review F3's coverage (docs/plans/verbs.md
// VB-19): resolution.typed/resolution.heuristic and index_commit, asserted
// against known values, for each of the 8 instrumented tools —
// find_callers, references, blast_radius, symbol_context,
// safe_delete_check, rename_impact, test_impact, review_diff.
//
// Shared fixture: A calls B (a real, TYPED edge — A->B), B calls C (syntax
// only), D calls B (syntax only), TestFoo calls A (syntax only). One
// typed edge among four call edges means every tool below sees a REAL mix
// of typed and heuristic evidence, not a monochrome 0 or N.

// svcFixture is one service's complete answer surface for
// multiSvcBackend.
type svcFixture struct {
	nodes       []core.SymbolNode
	sources     map[string][]byte
	typedEdges  []core.Edge
	indexCommit string
	path        string
}

// multiSvcBackend is a from-scratch QueryInterface (never embedding the
// single-service-oriented fakes elsewhere in this package, which ignore
// their `service` argument) — genuinely keyed by service, so a fleet-wide
// references call can see two services disagree on index_commit.
type multiSvcBackend struct {
	svcs map[string]*svcFixture
}

func (b *multiSvcBackend) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b *multiSvcBackend) FindCallers(_ context.Context, service, symbol string) ([]core.SymbolNode, error) {
	f := b.svcs[service]
	if f == nil {
		return nil, nil
	}
	var out []core.SymbolNode
	for _, n := range f.nodes {
		for _, c := range n.Calls {
			if c == symbol || strings.HasSuffix(c, "."+symbol) {
				out = append(out, n)
				break
			}
		}
	}
	return out, nil
}

func (b *multiSvcBackend) FindSymbol(_ context.Context, service, name string) ([]core.SymbolNode, error) {
	f := b.svcs[service]
	if f == nil {
		return nil, nil
	}
	var out []core.SymbolNode
	for _, n := range f.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (b *multiSvcBackend) ListAllSymbols(_ context.Context, service string) ([]core.SymbolNode, error) {
	f := b.svcs[service]
	if f == nil {
		return nil, nil
	}
	return f.nodes, nil
}

func (b *multiSvcBackend) Services(context.Context) ([]string, error) {
	out := make([]string, 0, len(b.svcs))
	for s := range b.svcs {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func (b *multiSvcBackend) Sources(_ context.Context, service string) ([]core.SourceFile, error) {
	f := b.svcs[service]
	if f == nil {
		return nil, nil
	}
	out := make([]core.SourceFile, 0, len(f.sources))
	for path, content := range f.sources {
		out = append(out, core.SourceFile{Path: path, Content: content})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (b *multiSvcBackend) IngestService(context.Context, string) error { return nil }

func (b *multiSvcBackend) ServiceMeta(_ context.Context, service string) (string, string, bool, error) {
	f := b.svcs[service]
	if f == nil {
		return "", "", false, nil
	}
	return f.path, f.indexCommit, true, nil
}

func (b *multiSvcBackend) ListCommits(context.Context, string) ([]CommitInfo, error) { return nil, nil }

func (b *multiSvcBackend) TypedEdges(_ context.Context, service string) ([]core.Edge, error) {
	f := b.svcs[service]
	if f == nil {
		return nil, nil
	}
	return f.typedEdges, nil
}

var _ QueryInterface = (*multiSvcBackend)(nil)

// resolutionFixtureNodes is the shared A/B/C/D/TestFoo graph every subtest
// below reads. One file per symbol keeps changed_files targeting (blast_
// radius, review_diff) unambiguous.
func resolutionFixtureNodes() (a, b, c, d, testFoo core.SymbolNode) {
	a = core.SymbolNode{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 1, Calls: []string{"B"}}
	b = core.SymbolNode{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/b.go", Line: 1, Calls: []string{"C"}}
	c = core.SymbolNode{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, FilePath: "pkg/c.go", Line: 1}
	d = core.SymbolNode{Service: "svc", Package: "pkg", Name: "D", Kind: core.KindGoFunc, FilePath: "pkg/d.go", Line: 1, Calls: []string{"B"}}
	testFoo = core.SymbolNode{Service: "svc", Package: "pkg", Name: "TestFoo", Kind: core.KindGoFunc, FilePath: "pkg/a_test.go", Line: 1, Calls: []string{"A"}}
	return
}

// newResolutionFixtureServer builds the one-service (typed A->B, heuristic
// B->C/D->B/TestFoo->A) server every single-service subtest below shares.
func newResolutionFixtureServer(t *testing.T) (*Server, core.SymbolNode, core.SymbolNode, core.SymbolNode, core.SymbolNode, core.SymbolNode) {
	t.Helper()
	a, b, c, d, testFoo := resolutionFixtureNodes()
	backend := &multiSvcBackend{svcs: map[string]*svcFixture{
		"svc": {
			nodes:       []core.SymbolNode{a, b, c, d, testFoo},
			typedEdges:  []core.Edge{{FromService: "svc", FromSymbol: a.Key(), ToSymbol: b.Key(), Kind: core.EdgeDirect}},
			indexCommit: "fixturecommit123",
			path:        "/repo",
		},
	}}
	return NewServer(backend), a, b, c, d, testFoo
}

func resolutionOf(t *testing.T, payload map[string]any) (typed, heuristic int) {
	t.Helper()
	m, ok := payload["resolution"].(map[string]any)
	if !ok {
		t.Fatalf("resolution missing or wrong type: %v", payload)
	}
	tf, _ := m["typed"].(float64)
	hf, _ := m["heuristic"].(float64)
	return int(tf), int(hf)
}

func TestResolution_FindCallers_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "find_callers", map[string]any{
		"service": "svc", "symbol": "B", "include_heuristic": true, "depth": 1,
	})
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 1 {
		t.Errorf("find_callers(B) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:1} (A typed, D heuristic)", typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf("find_callers index_commit = %v, want %q", payload["index_commit"], "fixturecommit123")
	}
}

func TestResolution_BlastRadius_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "blast_radius", map[string]any{
		"service": "svc", "changed_files": []any{"pkg/c.go"}, "include_heuristic": true,
	})
	// From C: depth1 = B (heuristic); depth2 = A (typed, A->B), D (heuristic,
	// D->B) — within the default depth-2 window. TestFoo (depth3) is outside
	// it and must NOT be counted.
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 2 {
		t.Errorf("blast_radius(pkg/c.go) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:2}", typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf("blast_radius index_commit = %v, want %q", payload["index_commit"], "fixturecommit123")
	}
}

func TestResolution_SymbolContext_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "B", "snippet": false})
	// callers of B: A (typed), D (heuristic); callees of B: C (heuristic).
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 2 {
		t.Errorf("symbol_context(B) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:2}", typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf("symbol_context index_commit = %v, want %q", payload["index_commit"], "fixturecommit123")
	}
}

// TestResolution_SafeDeleteCheck_MixedTiers: safe_delete_check's answer
// (unregistered as a standalone tool, now blast_radius's kind:"delete" —
// SR-24, F-13).
func TestResolution_SafeDeleteCheck_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "blast_radius", map[string]any{"kind": "delete", "service": "svc", "symbol": "B"})
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 1 {
		t.Errorf(`blast_radius(kind:"delete", B) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:1}`, typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf(`blast_radius(kind:"delete") index_commit = %v, want %q`, payload["index_commit"], "fixturecommit123")
	}
}

// TestResolution_RenameImpact_MixedTiers: rename_impact's answer
// (unregistered as a standalone tool, now blast_radius's kind:"rename" —
// SR-24, F-13).
func TestResolution_RenameImpact_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "blast_radius", map[string]any{"kind": "rename", "service": "svc", "symbol": "B"})
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 1 {
		t.Errorf(`blast_radius(kind:"rename", B) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:1}`, typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf(`blast_radius(kind:"rename") index_commit = %v, want %q`, payload["index_commit"], "fixturecommit123")
	}
}

func TestResolution_TestImpact_HeuristicOnly(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "test_impact", map[string]any{"service": "svc", "symbol": "C"})
	// TestFoo reaches C via TestFoo->A->B->C; the edge that first reaches
	// TestFoo (A->TestFoo, i.e. TestFoo's OWN call into A) is heuristic —
	// A->B being typed doesn't change TestFoo's own reach confidence.
	typed, heuristic := resolutionOf(t, payload)
	if typed != 0 || heuristic != 1 {
		t.Errorf("test_impact(C) resolution = {typed:%d heuristic:%d}, want {typed:0 heuristic:1}", typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf("test_impact index_commit = %v, want %q", payload["index_commit"], "fixturecommit123")
	}
}

func TestResolution_ReviewDiff_MixedTiers(t *testing.T) {
	srv, _, _, _, _, _ := newResolutionFixtureServer(t)
	payload := callTool(t, srv, "review_diff", map[string]any{"service": "svc", "changed_files": []any{"pkg/c.go"}})
	// review_diff's impact dimension walks the UNBOUNDED both-kinds graph
	// (not blast_radius's depth-capped default): from C, every hop is
	// included — B(heuristic), A(typed), D(heuristic), TestFoo(heuristic).
	typed, heuristic := resolutionOf(t, payload)
	if typed != 1 || heuristic != 3 {
		t.Errorf("review_diff(pkg/c.go) resolution = {typed:%d heuristic:%d}, want {typed:1 heuristic:3}", typed, heuristic)
	}
	if got, ok := payload["index_commit"].(string); !ok || got != "fixturecommit123" {
		t.Errorf("review_diff index_commit = %v, want %q", payload["index_commit"], "fixturecommit123")
	}
}

// TestResolution_References_SingleService covers references' own
// resolution/index_commit on a single, cleanly-attributable service.
func TestResolution_References_SingleService(t *testing.T) {
	foo := core.SymbolNode{Service: "svc1", Package: "pkg", Name: "Foo", Kind: core.KindGoFunc, FilePath: "pkg/foo.go", Line: 1}
	backend := &multiSvcBackend{svcs: map[string]*svcFixture{
		"svc1": {
			nodes:       []core.SymbolNode{foo},
			sources:     map[string][]byte{"pkg/foo.go": []byte("package pkg\n\nfunc Foo() {}\n")},
			indexCommit: "commitA",
		},
	}}
	srv := NewServer(backend)
	payload := callTool(t, srv, "references", map[string]any{"query": "Foo", "services": []any{"svc1"}})
	if got, ok := payload["index_commit"].(string); !ok || got != "commitA" {
		t.Errorf("references (single service) index_commit = %v, want %q", payload["index_commit"], "commitA")
	}
	typed, heuristic := resolutionOf(t, payload)
	if typed+heuristic == 0 {
		t.Errorf("references(Foo) resolution = {typed:%d heuristic:%d}, want a nonzero mix (1 definition + 1 occurrence)", typed, heuristic)
	}
}

// TestResolution_References_DisagreeingServices_OmitsIndexCommit is
// review F3's required "index_commit omitted" case: a fleet-wide query
// spanning two services recorded at DIFFERENT commits has no single
// honest watermark to report.
func TestResolution_References_DisagreeingServices_OmitsIndexCommit(t *testing.T) {
	foo1 := core.SymbolNode{Service: "svc1", Package: "pkg", Name: "Foo", Kind: core.KindGoFunc, FilePath: "pkg/foo.go", Line: 1}
	foo2 := core.SymbolNode{Service: "svc2", Package: "pkg", Name: "Foo", Kind: core.KindGoFunc, FilePath: "pkg/foo.go", Line: 1}
	backend := &multiSvcBackend{svcs: map[string]*svcFixture{
		"svc1": {
			nodes:       []core.SymbolNode{foo1},
			sources:     map[string][]byte{"pkg/foo.go": []byte("package pkg\n\nfunc Foo() {}\n")},
			indexCommit: "commitA",
		},
		"svc2": {
			nodes:       []core.SymbolNode{foo2},
			sources:     map[string][]byte{"pkg/foo.go": []byte("package pkg\n\nfunc Foo() {}\n")},
			indexCommit: "commitB", // deliberately different from svc1
		},
	}}
	srv := NewServer(backend)
	payload := callTool(t, srv, "references", map[string]any{"query": "Foo", "services": []any{"svc1", "svc2"}})
	if _, present := payload["index_commit"]; present {
		t.Errorf("references (disagreeing services) index_commit = %v, want omitted", payload["index_commit"])
	}
	// resolution is still reported — mix is envelope-level meta, unrelated
	// to whether a single commit can be named.
	if _, ok := payload["resolution"].(map[string]any); !ok {
		t.Errorf("references (disagreeing services) resolution missing: %v", payload)
	}
}

func (b *multiSvcBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := b.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}
