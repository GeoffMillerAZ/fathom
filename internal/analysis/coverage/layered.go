package coverage

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// LabeledProfile pairs a parsed Profile with the test LAYER whose run
// produced it — a CALLER-declared label (CLI: `--profile
// integration=workspace/cov/integration.out`; MCP/review: a
// `coverage_profiles: [{layer, path}]` entry), never guessed from
// per-test data, which a standard Go cover profile never carries (see
// JoinSymbols' own doc comment). A bare, unlabeled profile defaults to
// Layer "unit" — internal/testlayer.Classify's own default for a plain
// `_test.go` file with no distinguishing build tag or directory; there is
// no way to inspect a profile's own test files to confirm or override
// that guess, so "unit" is the caller's honest fallback, not fathom's
// inference.
type LabeledProfile struct {
	Layer   string
	Profile Profile
}

// LayerStat is one layer's (or the cross-layer "Any" union's) statement
// coverage.
type LayerStat struct {
	Statements int
	Covered    int
	Percent    float64
}

// SymbolLayerCoverage is one symbol's coverage broken out per test layer,
// plus Any: the union across every supplied layer.
type SymbolLayerCoverage struct {
	Symbol  string
	File    string
	Line    int
	EndLine int
	// ByLayer holds an entry ONLY for a layer whose profile actually has
	// block data for this symbol — a layer with none is simply absent,
	// never a fabricated 0%.
	ByLayer map[string]LayerStat
	// Any is the union: a statement counts as covered in Any if ANY
	// labeled profile's block at that EXACT source span (stable across
	// profiles of the same compiled source) had Count>0 — computed by
	// deduping spans (unionBlocks), not by summing layers, so a statement
	// covered by only one of N layers is still counted once, not N times.
	Any LayerStat
}

// CoveringLayers returns the sorted layer names in ByLayer whose
// Covered>0 — the layers that actually exercise this symbol.
func (s SymbolLayerCoverage) CoveringLayers() []string {
	var out []string
	for layer, st := range s.ByLayer {
		if st.Covered > 0 {
			out = append(out, layer)
		}
	}
	sort.Strings(out)
	return out
}

// UncoveredAt returns the sorted layer names in ByLayer whose Covered==0
// — a layer that DID run over this symbol's source but never executed it,
// as distinct from a layer with no data for it at all (simply absent from
// ByLayer, and never listed here).
func (s SymbolLayerCoverage) UncoveredAt() []string {
	var out []string
	for layer, st := range s.ByLayer {
		if st.Covered == 0 {
			out = append(out, layer)
		}
	}
	sort.Strings(out)
	return out
}

// JoinLayeredSymbols is JoinSymbols' multi-profile, per-layer wrapper: it
// runs the SAME block-to-symbol join (joinBlocks, shared with JoinSymbols)
// once per distinct layer label — profiles sharing a label have their
// blocks summed together first, so pass at most one profile per layer for
// a precise per-layer percentage (two profiles under the same label sum
// their statement blocks rather than deduping, unlike Any's union) — and
// once more over the union of every profile's blocks (unionBlocks) for the
// cross-layer Any view. Output is sorted by (File, Line): deterministic
// given identical inputs.
func JoinLayeredSymbols(profiles []LabeledProfile, symbols []core.SymbolNode) []SymbolLayerCoverage {
	byLayerBlocks := make(map[string][]Block, len(profiles))
	for _, lp := range profiles {
		byLayerBlocks[lp.Layer] = append(byLayerBlocks[lp.Layer], lp.Profile.Blocks...)
	}
	perLayer := make(map[string]map[string]*blockAccum, len(byLayerBlocks))
	for layer, blocks := range byLayerBlocks {
		perLayer[layer] = joinBlocks(blocks, symbols)
	}

	anyAccum := joinBlocks(unionBlocks(profiles), symbols)

	out := make([]SymbolLayerCoverage, 0, len(anyAccum))
	for key, a := range anyAccum {
		sc := SymbolLayerCoverage{
			Symbol:  key,
			File:    a.sym.FilePath,
			Line:    a.sym.Line,
			EndLine: a.sym.EndLine,
			ByLayer: map[string]LayerStat{},
			Any:     LayerStat{Statements: a.statements, Covered: a.covered, Percent: percentOf(a.statements, a.covered)},
		}
		for layer, accs := range perLayer {
			if la, ok := accs[key]; ok {
				sc.ByLayer[layer] = LayerStat{Statements: la.statements, Covered: la.covered, Percent: percentOf(la.statements, la.covered)}
			}
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// unionBlocks dedupes blocks across every labeled profile by EXACT source
// span (ImportPath + start/end line/col) — stable across profiles built
// from the same compiled source — marking the merged span covered if ANY
// profile's block at that span had Count>0. NumStmt is taken from
// whichever profile's block is seen first (it is a property of the
// compiled source, not of the run, so it should be identical across
// profiles that share the same build).
func unionBlocks(profiles []LabeledProfile) []Block {
	type key struct {
		path                    string
		sl, sc, el, ec, numStmt int
	}
	merged := make(map[key]bool) // true once any profile covered this span
	var order []key
	for _, lp := range profiles {
		for _, b := range lp.Profile.Blocks {
			k := key{b.ImportPath, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt}
			if _, seen := merged[k]; !seen {
				order = append(order, k)
				merged[k] = false
			}
			if b.Count > 0 {
				merged[k] = true
			}
		}
	}
	out := make([]Block, 0, len(order))
	for _, k := range order {
		count := 0
		if merged[k] {
			count = 1
		}
		out = append(out, Block{
			ImportPath: k.path,
			StartLine:  k.sl,
			StartCol:   k.sc,
			EndLine:    k.el,
			EndCol:     k.ec,
			NumStmt:    k.numStmt,
			Count:      count,
		})
	}
	return out
}
