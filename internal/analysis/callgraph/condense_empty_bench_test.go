package callgraph

import (
	"fmt"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func BenchmarkTransitiveCallerCountsMostlyIsolated(b *testing.B) {
	const n = 4096
	nodes := make([]core.SymbolNode, n)
	for i := range nodes {
		nodes[i] = core.SymbolNode{Service: "s", Package: "p", Name: fmt.Sprintf("Field%d", i), Kind: core.KindGoField}
	}
	g := Build(nodes)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = TransitiveCallerCounts(g)
	}
}
