package impact

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// stdlibdispatch.go — the reach `fathom tests select --safe` does not
// select by itself (O887): a test that reaches a changed symbol only
// through a dispatch into a method of a standard-library interface every
// error path and format call goes through (error's Error, fmt.Stringer's
// String, fmt.GoStringer's GoString). Every test that calls Error() on any
// error reaches every constructed error type's Error that way, so one
// edited or moved Error method selected most of a package's tests.

// StdlibDispatchOnly is what SplitStdlibDispatch took out of a selection.
type StdlibDispatchOnly struct {
	Methods []string // the interface methods those reaches went through, sorted ("error.Error")
	Tests   []string // the tests' canonical keys, sorted
}

// stdlibDispatchMethod names the standard-library interface method a Go
// method symbol implements, by its name and the signature the extractor
// records. A stored dispatch edge names the implementing method, never the
// interface its call site went through, so the method's own name and
// signature stand for the interface.
func stdlibDispatchMethod(n core.SymbolNode) (string, bool) {
	if n.Kind != core.KindGoMethod {
		return "", false
	}
	switch n.Name[strings.LastIndex(n.Name, ".")+1:] + " " + n.Signature {
	case "Error func Error() string":
		return "error.Error", true
	case "String func String() string":
		return "fmt.Stringer.String", true
	case "GoString func GoString() string":
		return "fmt.GoStringer.GoString", true
	}
	return "", false
}

// SplitStdlibDispatch reads res (TestGapSelectOn over g, built from nodes
// and typed) again over a graph without the interface-dispatch edges into a
// standard-library interface method (stdlibDispatchMethod), and returns res
// keeping only the tests that still reach a seed, with the tests it took
// out and the methods their reach went through. Gap is unchanged: a seed
// only those tests reach is reached, not unprotected. With no such edge,
// or no test taken out, res comes back as it was.
func SplitStdlibDispatch(g *callgraph.Graph, nodes []core.SymbolNode, typed []core.Edge, seeds []string, res TestGapResult) (TestGapResult, StdlibDispatchOnly) {
	if g == nil || len(res.AffectedTests) == 0 {
		return res, StdlibDispatchOnly{}
	}
	label := map[string]string{}
	for _, n := range nodes {
		if l, ok := stdlibDispatchMethod(n); ok {
			label[n.Key()] = l
		}
	}
	if len(label) == 0 {
		return res, StdlibDispatchOnly{}
	}
	// A dropped edge stays in the list aimed at a key no node has: the graph
	// then still counts its file as one the typed pass answered, and drops
	// that file's syntax guesses into Go methods (callgraph.AddTypedEdges),
	// which would otherwise put the same call back by name.
	kept := make([]core.Edge, 0, len(typed))
	dropped := map[string]bool{}
	for _, e := range typed {
		if e.Kind == core.EdgeIfaceDispatch && label[e.ToSymbol] != "" {
			dropped[e.ToSymbol] = true
			e.ToSymbol += "\x00stdlib-dispatch"
		}
		kept = append(kept, e)
	}
	if len(dropped) == 0 {
		return res, StdlibDispatchOnly{}
	}

	still := map[string]bool{}
	for _, t := range TestGapSelectOn(callgraph.BuildBoth(nodes, kept), nodes, seeds).AffectedTests {
		still[t.Test] = true
	}
	var out StdlibDispatchOnly
	affected := make([]TestReach, 0, len(res.AffectedTests))
	for _, t := range res.AffectedTests {
		if still[t.Test] {
			affected = append(affected, t)
			continue
		}
		out.Tests = append(out.Tests, t.Test)
	}
	if len(out.Tests) == 0 {
		return res, StdlibDispatchOnly{}
	}
	sort.Strings(out.Tests)

	// The methods named are the dropped targets that are a seed or reach
	// one and that a test taken out reaches: the ones its reach went through.
	isSeed := make(map[string]bool, len(seeds))
	for _, s := range seeds {
		isSeed[s] = true
	}
	targets := make([]string, 0, len(dropped))
	for k := range dropped {
		if isSeed[k] || callgraph.ReachableAny(g, k, seeds) {
			targets = append(targets, k)
		}
	}
	sort.Strings(targets)
	methods := map[string]bool{}
	for _, hits := range callgraph.SelectionReach(g, out.Tests, targets) {
		for _, h := range hits {
			methods[label[h.Target]] = true
		}
	}
	for m := range methods {
		out.Methods = append(out.Methods, m)
	}
	sort.Strings(out.Methods)
	res.AffectedTests = affected
	return res, out
}
