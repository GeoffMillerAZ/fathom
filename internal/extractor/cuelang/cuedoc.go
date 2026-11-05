package cuelang

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// sortCUECross orders a file's cross-file references deterministically: by
// line, then strategy, origin and value.
func sortCUECross(xs []extractor.DocCrossFileRef) {
	sort.SliceStable(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		for _, p := range [][2]string{{a.Strategy, b.Strategy}, {a.From, b.From}, {a.Raw, b.Raw}, {a.Target, b.Target}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
}
