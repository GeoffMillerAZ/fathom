package extractor

// hclwalk.go — what the native HCL tool dialects (terragruntdialect.go,
// hcltoolsdialect.go; LB-Q-terragrunt, LB-Q-hcl-tools) share over hclsyntax,
// the parser the terraform dialect (hclextract.go) already uses: a parsed
// body, entity and reference collection, and the helpers that read a literal
// out of an expression.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// hclParseBody parses src as HCL native syntax. A file with syntax errors
// still yields the body hclsyntax recovered (best effort, as the terraform
// dialect does); only a nil file is an error.
func hclParseBody(relPath string, src []byte) (*hclsyntax.Body, error) {
	f, diags := hclsyntax.ParseConfig(src, relPath, hcl.Pos{Line: 1, Column: 1})
	if f == nil {
		if diags.HasErrors() {
			return nil, fmt.Errorf("hcl parse %s: %s", relPath, diags.Error())
		}
		return nil, nil
	}
	body, _ := f.Body.(*hclsyntax.Body)
	return body, nil
}

// hclWalk collects one file's entities and cross-file references.
type hclWalk struct {
	service, relPath, pkg string
	nodes                 []core.SymbolNode
	cross                 []DocCrossFileRef
	seen                  map[string]int
	lines                 int
}

func newHCLWalk(service, relPath string, src []byte) *hclWalk {
	return &hclWalk{service: service, relPath: relPath, pkg: pkgFromPath(relPath), seen: map[string]int{}, lines: confLastLine(src)}
}

// add appends an entity and returns the name it was given: a name an earlier
// entity of the file already has (two unlabelled blocks, a provisioner type
// used twice) gets "#2", "#3", … so every entity of a file has its own.
func (w *hclWalk) add(name string, kind core.SourceKind, sig string, line, end int, refs []hclRef) string {
	w.seen[name]++
	if n := w.seen[name]; n > 1 {
		name = fmt.Sprintf("%s#%d", name, n)
		w.seen[name]++
	}
	if end < line {
		end = line
	}
	if len(sig) > 200 {
		sig = sig[:200]
	}
	w.nodes = append(w.nodes, core.SymbolNode{
		Service: w.service, Package: w.pkg, Name: name, Kind: kind, FilePath: w.relPath,
		Line: line, EndLine: end, Signature: sig, Tier: core.TierVerified, Calls: refKeys(refs),
	})
	return name
}

// ref records one cross-file reference.
func (w *hclWalk) ref(c DocCrossFileRef) { w.cross = append(w.cross, c) }

// result is the file's DocResult: entities by line then name, references by
// line, origin, edge, then target (deterministic).
func (w *hclWalk) result(roots ...string) DocResult {
	sort.SliceStable(w.nodes, func(i, j int) bool {
		a, b := w.nodes[i], w.nodes[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	sort.SliceStable(w.cross, func(i, j int) bool {
		a, b := w.cross[i], w.cross[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		return a.Target < b.Target
	})
	return DocResult{Nodes: w.nodes, CrossFile: w.cross, Roots: roots}
}

// hclSortedAttrs is a body's attributes in source order.
func hclSortedAttrs(b *hclsyntax.Body) []*hclsyntax.Attribute {
	if b == nil {
		return nil
	}
	out := make([]*hclsyntax.Attribute, 0, len(b.Attributes))
	for _, a := range b.Attributes {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SrcRange.Start.Byte != out[j].SrcRange.Start.Byte {
			return out[i].SrcRange.Start.Byte < out[j].SrcRange.Start.Byte
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// hclBodyRefs is the references the expressions of a body make, nested blocks
// included except the block types in skipBlock (their own entities') and the
// attributes in skipAttr (read some other way). key and fn are refCollector's.
func hclBodyRefs(b *hclsyntax.Body, skipBlock, skipAttr map[string]bool, key func(hcl.Traversal) string, fn func(string) string) []hclRef {
	if b == nil {
		return nil
	}
	c := &refCollector{key: key, fn: fn}
	for _, a := range hclSortedAttrs(b) {
		if !skipAttr[a.Name] {
			_ = hclsyntax.Walk(a, c)
		}
	}
	for _, blk := range b.Blocks {
		if !skipBlock[blk.Type] {
			_ = hclsyntax.Walk(blk, c)
		}
	}
	return c.refs
}

// hclExprRefs is the references one expression makes.
func hclExprRefs(x hclsyntax.Expression, key func(hcl.Traversal) string, fn func(string) string) []hclRef {
	if x == nil {
		return nil
	}
	c := &refCollector{key: key, fn: fn}
	_ = hclsyntax.Walk(x, c)
	return c.refs
}

// hclLiteralString is the value of an expression that is a string literal
// (an interpolation-free quoted or heredoc string); ok is false for anything
// that needs an evaluation context.
func hclLiteralString(x hclsyntax.Expression) (string, bool) {
	if x == nil {
		return "", false
	}
	v, d := x.Value(nil)
	if d.HasErrors() || !v.IsKnown() || v.IsNull() || v.Type() != cty.String {
		return "", false
	}
	return v.AsString(), true
}

// hclTupleItems is a list expression's elements (the expression itself for
// anything that is not a list).
func hclTupleItems(x hclsyntax.Expression) []hclsyntax.Expression {
	switch t := x.(type) {
	case *hclsyntax.TupleConsExpr:
		return t.Exprs
	case *hclsyntax.ParenthesesExpr:
		return hclTupleItems(t.Expression)
	}
	return []hclsyntax.Expression{x}
}

// hclExprText is an expression as written in src (one line, at most 200
// bytes).
func hclExprText(src []byte, x hclsyntax.Expression) string {
	r := x.Range()
	if r.Start.Byte < 0 || r.End.Byte > len(src) || r.Start.Byte >= r.End.Byte {
		return ""
	}
	s := strings.Join(strings.Fields(string(src[r.Start.Byte:r.End.Byte])), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// hclBlockLabel is a block's i-th label ("" when it has fewer).
func hclBlockLabel(b *hclsyntax.Block, i int) string {
	if i < len(b.Labels) {
		return b.Labels[i]
	}
	return ""
}

// hclKeyName is the literal key of an object constructor item: an identifier
// or a quoted string.
func hclKeyName(k hclsyntax.Expression) (string, bool) {
	if kw := hcl.ExprAsKeyword(k); kw != "" {
		return kw, true
	}
	return hclLiteralString(k)
}
