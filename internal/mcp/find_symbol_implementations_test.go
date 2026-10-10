// Package mcp_test — SB-5 (docs/plans/strengths-bench.md; H1-T06): find_symbol
// on a Go interface carries its implementers.
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// implFakeBackend gives full control over FindSymbol and Sources (which
// extendedFakeBackend always answers empty) so implementationsOf's three
// tiers can each be exercised deterministically.
type implFakeBackend struct {
	extendedFakeBackend
	findSymbolNodes []core.SymbolNode
	sources         []core.SourceFile
}

func (b *implFakeBackend) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return b.findSymbolNodes, nil
}

func (b *implFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return b.sources, nil
}

var _ mcp.QueryInterface = (*implFakeBackend)(nil)

// writerInterfaceSource is the shared fixture: interface Writer{Write,
// Close}, two structurally-satisfying types FileWriter and NopWriter.
const writerInterfaceSource = `package sample

type Writer interface {
	Write(p []byte) (int, error)
	Close() error
}

type FileWriter struct{}

func (f *FileWriter) Write(p []byte) (int, error) { return 0, nil }

func (f *FileWriter) Close() error { return nil }

type NopWriter struct{}

func (NopWriter) Write(p []byte) (int, error) { return 0, nil }

func (NopWriter) Close() error { return nil }
`

func writerIfaceNode() core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: "sample", Name: "Writer", Kind: core.KindGoInterface, FilePath: "sample.go", Line: 3, Tier: core.TierVerified}
}

func writerFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		writerIfaceNode(),
		{Service: "svc", Package: "sample", Name: "FileWriter", Kind: core.KindGoType, FilePath: "sample.go", Line: 8, Tier: core.TierVerified},
		{Service: "svc", Package: "FileWriter", Name: "FileWriter.Write", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 10, Tier: core.TierVerified},
		{Service: "svc", Package: "FileWriter", Name: "FileWriter.Close", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 12, Tier: core.TierVerified},
		{Service: "svc", Package: "sample", Name: "NopWriter", Kind: core.KindGoType, FilePath: "sample.go", Line: 14, Tier: core.TierVerified},
		{Service: "svc", Package: "NopWriter", Name: "NopWriter.Write", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 16, Tier: core.TierVerified},
		{Service: "svc", Package: "NopWriter", Name: "NopWriter.Close", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 18, Tier: core.TierVerified},
	}
}

func findSymbolImplementations(t *testing.T, be *implFakeBackend) []any {
	t.Helper()
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4000, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_symbol",
			"arguments": map[string]any{"service": "svc", "name": "Writer"},
		},
	})
	p := xtraPayload(t, resp)
	impls, _ := p["implementations"].([]any)
	return impls
}

func TestFindSymbol_Interface_MethodSetTier(t *testing.T) {
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}},
		findSymbolNodes:     []core.SymbolNode{writerIfaceNode()},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(writerInterfaceSource), Lang: "go"}},
	}
	impls := findSymbolImplementations(t, be)
	if len(impls) != 2 {
		t.Fatalf("expected 2 implementers (FileWriter, NopWriter), got %d: %v", len(impls), impls)
	}
	names := []string{impls[0].(map[string]any)["name"].(string), impls[1].(map[string]any)["name"].(string)}
	if names[0] != "FileWriter" || names[1] != "NopWriter" {
		t.Errorf("expected sorted [FileWriter, NopWriter], got %v", names)
	}
	for _, raw := range impls {
		e := raw.(map[string]any)
		if e["via"] != "method_set" {
			t.Errorf("expected via=method_set, got %v", e["via"])
		}
		if e["file_path"] != "sample.go" {
			t.Errorf("expected file_path=sample.go, got %v", e["file_path"])
		}
	}
}

func TestFindSymbol_Interface_AssertionTier(t *testing.T) {
	src := writerInterfaceSource + "\nvar _ Writer = (*FileWriter)(nil)\n"
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}},
		findSymbolNodes:     []core.SymbolNode{writerIfaceNode()},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(src), Lang: "go"}},
	}
	impls := findSymbolImplementations(t, be)
	if len(impls) != 1 {
		t.Fatalf("expected exactly 1 asserted implementer, got %d: %v", len(impls), impls)
	}
	e := impls[0].(map[string]any)
	if e["name"] != "FileWriter" || e["via"] != "assertion" {
		t.Errorf("expected {name:FileWriter, via:assertion}, got %v", e)
	}
}

func TestFindSymbol_Interface_TypedTier(t *testing.T) {
	typedEdges := []core.Edge{
		{FromService: "svc", FromSymbol: "svc::caller.Run", ToSymbol: "svc::FileWriter.FileWriter.Write", Kind: core.EdgeIfaceDispatch},
	}
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}, typedEdges: typedEdges},
		findSymbolNodes:     []core.SymbolNode{writerIfaceNode()},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(writerInterfaceSource), Lang: "go"}},
	}
	impls := findSymbolImplementations(t, be)
	if len(impls) != 1 {
		t.Fatalf("expected exactly 1 typed implementer, got %d: %v", len(impls), impls)
	}
	e := impls[0].(map[string]any)
	if e["name"] != "FileWriter" || e["via"] != "typed" {
		t.Errorf("expected {name:FileWriter, via:typed}, got %v", e)
	}
}

func TestFindSymbol_Interface_Deterministic(t *testing.T) {
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: writerFixtureNodes(), services: []string{"svc"}},
		findSymbolNodes:     []core.SymbolNode{writerIfaceNode()},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(writerInterfaceSource), Lang: "go"}},
	}
	a := findSymbolImplementations(t, be)
	b := findSymbolImplementations(t, be)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].(map[string]any)["name"] != b[i].(map[string]any)["name"] {
			t.Errorf("non-deterministic order at %d: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestFindSymbol_NonInterface_NoImplementationsField(t *testing.T) {
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		findSymbolNodes:     []core.SymbolNode{{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 10}},
	}
	srv := mcp.NewServer(be)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4001, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_symbol",
			"arguments": map[string]any{"service": "svc", "name": "A"},
		},
	})
	p := xtraPayload(t, resp)
	if _, has := p["implementations"]; has {
		t.Errorf("expected no implementations field on a non-interface hit, got %v", p["implementations"])
	}
}

func (b *implFakeBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := b.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}

// TestFindSymbol_Interface_MembersAreNotImplementers: K17 (wave lf2) indexes
// an interface's own methods as Iface.Method GoMethod symbols. Another
// interface whose methods cover Writer's is not one of its implementations,
// nor is Writer itself.
func TestFindSymbol_Interface_MembersAreNotImplementers(t *testing.T) {
	src := writerInterfaceSource + "\ntype WriteCloser interface {\n\tWrite(p []byte) (int, error)\n\tClose() error\n}\n"
	nodes := append(writerFixtureNodes(),
		core.SymbolNode{Service: "svc", Package: "Writer", Name: "Writer.Write", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 4, Tier: core.TierVerified},
		core.SymbolNode{Service: "svc", Package: "Writer", Name: "Writer.Close", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 5, Tier: core.TierVerified},
		core.SymbolNode{Service: "svc", Package: "sample", Name: "WriteCloser", Kind: core.KindGoInterface, FilePath: "sample.go", Line: 21, Tier: core.TierVerified},
		core.SymbolNode{Service: "svc", Package: "WriteCloser", Name: "WriteCloser.Write", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 22, Tier: core.TierVerified},
		core.SymbolNode{Service: "svc", Package: "WriteCloser", Name: "WriteCloser.Close", Kind: core.KindGoMethod, FilePath: "sample.go", Line: 23, Tier: core.TierVerified},
	)
	be := &implFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}},
		findSymbolNodes:     []core.SymbolNode{writerIfaceNode()},
		sources:             []core.SourceFile{{Path: "sample.go", Content: []byte(src), Lang: "go"}},
	}
	impls := findSymbolImplementations(t, be)
	var names []string
	for _, raw := range impls {
		names = append(names, raw.(map[string]any)["name"].(string))
	}
	if len(names) != 2 || names[0] != "FileWriter" || names[1] != "NopWriter" {
		t.Fatalf("implementations = %v, want [FileWriter NopWriter]: an interface is never an implementation", names)
	}
}
