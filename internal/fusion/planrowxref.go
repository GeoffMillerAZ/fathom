package fusion

// planrowxref.go — the resolvers of the fathom-plan-rows dialect's cross-file
// references (LB-F220 and LB-F221, wave fx item fmt):
//
//   - plan-row: `needs: <plan>.md#<ID>` names a row of another plan. The
//     extractor gives the plan's repository path as the reference's Target and
//     the row id is the fragment of Raw; resolveDocXrefs joins them to that
//     row's entity when the plan is among the files it was given (so the
//     language-breadth harness scores the same joins ingest stores).
//   - plan-symbol: `symbol:` and `reachable_from:` name a code symbol, in the
//     row's `package:` directory when it has one. Code is no doc file, so this
//     resolver reads the stored symbols (ResolvePlanSymbols); like the gate
//     (verify's underDir), a name that leaves more than one distinct symbol
//     after the package filter resolves to nothing, never first-match-wins.
//
// A `package:` anchor is the repo-path strategy, the doc engine's own.

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// planRowTarget splits a plan-row reference into the plan file and the row
// id it names (ok false for a reference that carries neither).
func planRowTarget(c extractor.DocCrossFileRef) (file, id string, ok bool) {
	i := strings.LastIndex(c.Raw, "#")
	if i < 0 || c.Target == "" || i == len(c.Raw)-1 {
		return "", "", false
	}
	return c.Target, c.Raw[i+1:], true
}

// PlanSymbolRef is one resolved plan-symbol reference: a plan row to the one
// code symbol it anchors.
type PlanSymbolRef struct {
	FromFile, From, FromKey string
	ToFile, To, ToKey       string
	Edge                    string
}

// ResolvePlanSymbols resolves every plan-symbol reference of files against
// the stored code symbols (deterministic, sorted). Doc chunks and plan rows
// are never a target: a heading that shares a bare name with a function is
// not what a row anchors (verify's codeMatches).
func ResolvePlanSymbols(files []DocFile, symbols []core.SymbolNode) []PlanSymbolRef {
	want := map[string]bool{}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy == extractor.PlanStrategySymbol {
				want[c.Raw] = true
			}
		}
	}
	if len(want) == 0 {
		return nil
	}
	byName := map[string][]core.SymbolNode{}
	for _, n := range symbols {
		if n.Kind == core.KindDocChunk || n.Kind == extractor.KindPlanRow {
			continue
		}
		bare := n.Name
		if i := strings.LastIndex(bare, "."); i >= 0 {
			bare = bare[i+1:] // a method is named Recv.Method; the anchor names the method
		}
		for sym := range want {
			if q, recv := planAnchorName(sym); (recv && n.Name == q) || (!recv && (n.Name == q || bare == q)) {
				byName[sym] = append(byName[sym], n)
			}
		}
	}
	rowKey := map[[2]string]string{}
	for _, f := range files {
		for _, n := range f.Result.Nodes {
			if n.Kind == extractor.KindPlanRow {
				rowKey[[2]string{f.Path, n.Name}] = n.Key()
			}
		}
	}
	var out []PlanSymbolRef
	seen := map[string]bool{}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy != extractor.PlanStrategySymbol {
				continue
			}
			from, ok := rowKey[[2]string{f.Path, c.From}]
			if !ok {
				continue
			}
			cands := byName[c.Raw]
			if c.Target != "" {
				var under []core.SymbolNode
				for _, n := range cands {
					if path.Dir(n.FilePath) == c.Target {
						under = append(under, n)
					}
				}
				cands = under
			}
			keys := map[string]core.SymbolNode{}
			for _, n := range cands {
				keys[n.Key()] = n
			}
			if len(keys) != 1 {
				continue
			}
			for k, n := range keys {
				id := f.Path + "\x00" + c.From + "\x00" + k
				if seen[id] {
					continue
				}
				seen[id] = true
				out = append(out, PlanSymbolRef{FromFile: f.Path, From: c.From, FromKey: from, ToFile: n.FilePath, To: n.Name, ToKey: k, Edge: c.Edge})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToKey, b.ToKey}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
	return out
}

// planReceiverRe is Go's doc-style "(*T).Method" or "(T).Method".
var planReceiverRe = regexp.MustCompile(`^\(\*?([A-Za-z_][A-Za-z0-9_]*)\)\.([A-Za-z_][A-Za-z0-9_]*)$`)

// planAnchorName is the name an anchor looks up: "T.Method" for a
// receiver-qualified one (recv true: only that method answers, as the gate's
// FindSymbol narrows it), the anchor as written otherwise.
func planAnchorName(sym string) (name string, recv bool) {
	if m := planReceiverRe.FindStringSubmatch(sym); m != nil {
		return m[1] + "." + m[2], true
	}
	return sym, false
}

// PlanSymbolEdges is the store form: one file-tier typed edge per (row, symbol).
func PlanSymbolEdges(refs []PlanSymbolRef) []core.Edge {
	seen := map[[2]string]bool{}
	var edges []core.Edge
	for _, r := range refs {
		k := [2]string{r.FromKey, r.ToKey}
		if seen[k] {
			continue
		}
		seen[k] = true
		edges = append(edges, core.Edge{FromFile: r.FromFile, FromSymbol: r.FromKey, ToSymbol: r.ToKey, Kind: core.EdgeFileResolved})
	}
	sortEdges(edges)
	return edges
}

// planSymbolEdges reads the service's stored symbols once, only when a plan
// of files carries a plan-symbol reference, and returns the row-to-symbol
// edges. A store that cannot answer gives none: the pass is additive.
func (p *Pipeline) planSymbolEdges(ctx context.Context, service string, files []DocFile) []core.Edge {
	has := false
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy == extractor.PlanStrategySymbol {
				has = true
			}
		}
	}
	if !has {
		return nil
	}
	obs, err := p.st.QueryObservations(ctx, service, "")
	if err != nil {
		return nil
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return PlanSymbolEdges(ResolvePlanSymbols(files, nodes))
}
