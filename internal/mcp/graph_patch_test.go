package mcp

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// patchFake serves one package's nodes at a fixed watermark and counts
// symbol loads.
type patchFake struct {
	refreshFake
	testStore
	nodes []core.SymbolNode
	loads int
}

func (f *patchFake) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	f.loads++
	return f.nodes, nil
}

func (f *patchFake) StoreWatermark(context.Context, string) (string, error) { return "w1", nil }

// TestPatchPackageGraph_KeepsMemoFreshWithoutRebuild (SU-4, now at the
// graph store, RT-74): after a package's nodes are patched, the SAME
// (service, watermark) is still a hit — no reload, no rebuild — and the
// graph served carries the new symbol, not the pre-refresh one. The
// resolved-only graph is dropped and rebuilds.
func TestPatchPackageGraph_KeepsMemoFreshWithoutRebuild(t *testing.T) {
	old := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Original", FilePath: "target.go", Line: 3}}
	f := &patchFake{nodes: old}
	s := NewServer(f)
	ctx := context.Background()
	if g := s.loadNavGraphs(ctx, f, "svc"); g == nil || f.loads != 1 {
		t.Fatalf("priming load: loads=%d", f.loads)
	}
	if !s.HasCachedGraph("svc") {
		t.Fatal("HasCachedGraph = false after priming")
	}

	fresh := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Changed", FilePath: "target.go", Line: 3}}
	s.PatchPackageGraph("svc", "main", fresh, nil)

	ng := s.loadNavGraphs(ctx, f, "svc")
	if f.loads != 1 {
		t.Fatalf("the patched key reloaded: loads=%d, want 1", f.loads)
	}
	// A node trivially reaches itself, so Reachable(k, k) is "k exists".
	for name, g := range map[string]*callgraph.Graph{"both": ng.Both, "syntax": ng.Syntax} {
		hasChanged := callgraph.Reachable(g, fresh[0].Key(), fresh[0].Key())
		hasOriginal := callgraph.Reachable(g, old[0].Key(), old[0].Key())
		if !hasChanged || hasOriginal {
			t.Fatalf("patched %s graph serves Changed=%v Original=%v, want true/false", name, hasChanged, hasOriginal)
		}
	}
	if got := ng.Snapshot.Named("Changed"); len(got) != 1 || len(ng.Snapshot.Named("Original")) != 0 {
		t.Fatalf("patched snapshot = %+v", ng.Nodes)
	}

	s.DropCachedGraphs("svc")
	if s.HasCachedGraph("svc") {
		t.Fatal("HasCachedGraph = true after DropCachedGraphs")
	}
}

// TestRefreshPatchesSnapshotWithNoGraphCached (review finding, O99): the
// iterating tools (find_symbol, search, references) read the snapshot, and
// a single-file refresh does not move the watermark it is keyed by — so a
// cached snapshot with no graph beside it is patched too, never left to
// serve the pre-refresh symbols until the next full ingest.
func TestRefreshPatchesSnapshotWithNoGraphCached(t *testing.T) {
	old := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Original", FilePath: "target.go", Line: 3}}
	f := &patchFake{nodes: old}
	s := NewServer(f)
	ctx := context.Background()
	if _, err := serviceSymbols(ctx, f, "svc"); err != nil || f.loads != 1 {
		t.Fatalf("priming load: loads=%d err=%v", f.loads, err)
	}
	if !s.HasCachedGraph("svc") {
		t.Fatal("HasCachedGraph = false with a cached snapshot: the refresh would skip the patch")
	}
	fresh := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Changed", FilePath: "target.go", Line: 3}}
	s.PatchPackageGraph("svc", "main", fresh, nil)
	nodes, err := serviceSymbols(ctx, f, "svc")
	if err != nil || f.loads != 1 {
		t.Fatalf("reload after patch: loads=%d err=%v", f.loads, err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Changed" {
		t.Fatalf("snapshot after refresh = %+v", nodes)
	}
}
