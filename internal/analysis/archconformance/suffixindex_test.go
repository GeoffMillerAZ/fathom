package archconformance

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestUniqueSuffixIndexMatchesScan (CX-20): the suffix index answers every
// dot-led suffix exactly as the linear scan it replaced — the one node whose
// key ends with it, or nothing when none or several do.
func TestUniqueSuffixIndexMatchesScan(t *testing.T) {
	nodes := []core.SymbolNode{
		{Package: "a", Name: "Run"}, {Package: "b", Name: "Run"}, {Package: "b", Name: "Do"},
		{Package: "Handler", Name: "Handler.Serve", Kind: core.KindGoMethod}, {Package: "c", Name: "Serve"},
		{Package: "x.y", Name: "Deep"}, {Package: "b", Name: "Do"},
	}
	scan := func(suffix string) (core.SymbolNode, bool) {
		var match core.SymbolNode
		found := false
		for _, n := range nodes {
			if strings.HasSuffix(n.Key(), suffix) {
				if found {
					return core.SymbolNode{}, false
				}
				match, found = n, true
			}
		}
		return match, found
	}
	r := newEdgeResolver(nodes)
	for _, s := range []string{".Run", ".a.Run", ".Do", ".b.Do", ".Serve", ".Handler.Serve", ".Handler.Handler.Serve", ".c.Serve", ".Deep", ".y.Deep", ".x.y.Deep", ".Nope", ".", ".un"} {
		wn, wok := scan(s)
		gn, gok := r.uniqueSuffix(s)
		if wok != gok || wn.Key() != gn.Key() {
			t.Errorf("suffix %q: index (%v, %q), scan (%v, %q)", s, gok, gn.Key(), wok, wn.Key())
		}
	}
}
