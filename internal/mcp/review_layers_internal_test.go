package mcp

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestReviewDiffLayers (GL-22): each changed file lands in exactly one
// layer by blast radius — leaf (nothing else calls it; a test caller does
// not count), shared-callee (another file calls it), cross-service
// provider (another service's client call joins a route it registers) —
// and the layers come in review order.
func TestReviewDiffLayers(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Handler", Kind: core.KindGoFunc, FilePath: "pkg/api.go", Line: 1, Calls: []string{"svc::pkg.Util"}},
		{Service: "svc", Package: "pkg", Name: "Util", Kind: core.KindGoFunc, FilePath: "pkg/util.go", Line: 1},
		{Service: "svc", Package: "pkg", Name: "Leaf", Kind: core.KindGoFunc, FilePath: "pkg/leaf.go", Line: 1},
		{Service: "svc", Package: "pkg", Name: "TestLeaf", Kind: core.KindGoFunc, FilePath: "pkg/leaf_test.go", Line: 1, Calls: []string{"svc::pkg.Leaf"}},
	}
	g := callgraph.Build(nodes)
	hops := []serviceref.Hop{
		{FromService: "web", FromFile: "client.ts", ToService: "svc", ToFile: "pkg/api.go"},
		{FromService: "mobile", FromFile: "c.kt", ToService: "svc", ToFile: "pkg/api.go"},
	}
	got := layerDiffByBlastRadius(g, nodes, []string{"pkg/util.go", "pkg/api.go", "pkg/leaf.go", "pkg/leaf.go"}, hops)
	want := []diffLayer{
		{Layer: diffLayerLeaf, Files: []diffLayerFile{{File: "pkg/leaf.go"}}},
		{Layer: diffLayerShared, Files: []diffLayerFile{{File: "pkg/util.go", CallerFiles: 1}}},
		{Layer: diffLayerProvider, Files: []diffLayerFile{{File: "pkg/api.go", Consumers: []string{"mobile", "web"}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("layers =\n%+v\nwant\n%+v", got, want)
	}

	// No graph, no hops: every file is a leaf, never an error.
	if got := layerDiffByBlastRadius(nil, nil, []string{"b.go", "a.go"}, nil); !reflect.DeepEqual(got, []diffLayer{{Layer: diffLayerLeaf, Files: []diffLayerFile{{File: "a.go"}, {File: "b.go"}}}}) {
		t.Fatalf("empty-evidence layers = %+v", got)
	}
}
