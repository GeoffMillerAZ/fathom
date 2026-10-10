package fusion

// identityxref.go — LB-F369 (wave c3, item cw; docs/design/language-breadth.md
// §4): the doc engine's `identity` cross-file references resolved. Such a
// value is an identity signature, TL-5's `key=value` form
// (`aws.cloudwatch.metric=Shop/Orders/Errors`,
// `aws.lambda.function_name=orders-api`), and it joins the entities signed
// with exactly that identity: a CloudWatch alarm to the metric filter whose
// metric it watches, a metric filter on `/aws/lambda/<name>` to the function
// of that name, in the referring file or any other, across dialects (a
// CloudFormation alarm, a Terraform metric filter). The join keeps the
// nearest targets — the referring file's own, else those under the deepest
// enclosing directory that holds one — so a repository of independent stacks
// (a samples monorepo) does not join one stack's alarm to another's filter
// when its own declares the metric. More than identityFanoutCap targets at
// the nearest level is no join: an identity that common names nothing.
// Edges are file-tier doc-xref edges, never a gate.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// IdentityStrategy is the doc engine's identity resolve strategy.
const IdentityStrategy = "identity"

// identityFanoutCap bounds the targets one identity reference joins.
const identityFanoutCap = 64

// ResolveIdentityXrefs resolves every file's identity references against the
// set's signed entities (deterministic, sorted, one edge per pair and edge
// name; never the referring entity itself).
func ResolveIdentityXrefs(files []DocFile) []DocXref {
	type target struct {
		file string
		n    core.SymbolNode
	}
	bySig := map[string][]target{}
	nodes := map[[2]string]core.SymbolNode{}
	roots := map[string]string{}
	for _, f := range files {
		for _, n := range f.Result.Nodes {
			if _, dup := nodes[[2]string{f.Path, n.Name}]; !dup {
				nodes[[2]string{f.Path, n.Name}] = n
			}
			if strings.Contains(n.Signature, "=") {
				bySig[n.Signature] = append(bySig[n.Signature], target{f.Path, n})
			}
		}
		if len(f.Result.Roots) > 0 {
			roots[f.Path] = f.Result.Roots[0]
		}
	}
	var out []DocXref
	seen := map[string]bool{}
	for _, f := range files {
		for _, c := range f.Result.CrossFile {
			if c.Strategy != IdentityStrategy || c.Raw == "" {
				continue
			}
			name := c.From
			if name == "" {
				name = roots[f.Path]
			}
			from, ok := nodes[[2]string{f.Path, name}]
			if !ok {
				continue
			}
			var cands []target
			for _, t := range bySig[c.Raw] {
				if t.n.Key() != from.Key() {
					cands = append(cands, t)
				}
			}
			cands = nearestTargets(f.Path, cands, func(t target) string { return t.file })
			if len(cands) > identityFanoutCap {
				continue
			}
			for _, t := range cands {
				id := f.Path + "\x00" + name + "\x00" + t.file + "\x00" + t.n.Name + "\x00" + c.Edge
				if seen[id] {
					continue
				}
				seen[id] = true
				out = append(out, DocXref{FromFile: f.Path, From: name, FromKey: from.Key(), ToFile: t.file, To: t.n.Name, ToKey: t.n.Key(), Edge: c.Edge, Strategy: IdentityStrategy})
			}
		}
	}
	sortDocXrefs(out)
	return out
}

// nearestTargets keeps the candidates nearest the referring file: those in
// the file itself, else those under the deepest enclosing directory of the
// file that holds any (the repository root last).
func nearestTargets[T any](referrer string, cands []T, fileOf func(T) string) []T {
	if len(cands) == 0 {
		return nil
	}
	var same []T
	for _, c := range cands {
		if fileOf(c) == referrer {
			same = append(same, c)
		}
	}
	if len(same) > 0 {
		return same
	}
	for dir := path.Dir(referrer); ; dir = path.Dir(dir) {
		var under []T
		for _, c := range cands {
			if dir == "." || strings.HasPrefix(fileOf(c), dir+"/") {
				under = append(under, c)
			}
		}
		if len(under) > 0 || dir == "." || dir == "/" {
			return under
		}
	}
}

// sortDocXrefs orders references by referring file, entity, referenced file,
// entity and edge.
func sortDocXrefs(xs []DocXref) {
	sort.Slice(xs, func(i, j int) bool {
		a, b := xs[i], xs[j]
		for _, p := range [][2]string{{a.FromFile, b.FromFile}, {a.From, b.From}, {a.ToFile, b.ToFile}, {a.To, b.To}, {a.Edge, b.Edge}} {
			if p[0] != p[1] {
				return p[0] < p[1]
			}
		}
		return false
	})
}
