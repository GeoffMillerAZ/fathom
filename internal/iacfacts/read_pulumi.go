package iacfacts

import (
	"errors"
	"strings"
)

// ReadPulumiYAMLFile reads the resources of a Pulumi YAML program: each resource's
// properties as attribute paths with classes and spans. A ${name} or ${name.prop}
// interpolation of a resource is a reference, of a configuration entry with a default is
// defaulted, of a variable follows the variable, and a fn:: builtin is an expression
// (fn::secret keeps the class of its argument). Lists take [*] in a path. A resource that
// has a get section reads as a data source. Pulumi TypeScript and Python programs are not
// read.
//
// A program that does not parse reads as empty with SyntaxErrors set. A program past the
// product's limits is refused whole with ErrLimit.
func ReadPulumiYAMLFile(filePath string, src []byte, opts ...Option) (*ResourceSet, error) {
	set := &ResourceSet{Tool: ToolPulumi}
	root, err := parseTemplate(filePath, src)
	if errors.Is(err, errSyntax) {
		set.SyntaxErrors = 1
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	if root == nil || root.kind != nodeMap {
		return set, nil
	}
	res := root.get("resources")
	if res == nil || res.kind != nodeMap {
		return set, nil
	}
	pr := &pulumiReader{cfg: newConfig(opts), root: root, resolving: map[string]bool{}}
	w := &treeWalker{emitter: emitter{cfg: pr.cfg}, file: filePath, classify: pr.classifyNode}
	for _, p := range res.pairs {
		if p.val.kind != nodeMap {
			continue
		}
		typ := p.val.get("type")
		if typ == nil || typ.kind != nodeScalar {
			continue
		}
		r := Resource{
			Tool: ToolPulumi, Mode: ModeManaged, Type: typ.value, Name: p.key.value,
			Span: p.span, Multiplicity: Multiplicity{Kind: MultiSingle},
		}
		if p.val.get("get") != nil {
			r.Mode = ModeData
		}
		if opt := p.val.get("options"); opt != nil {
			if pv := opt.get("provider"); pv != nil && pv.kind == nodeScalar {
				r.Provider = strings.TrimSuffix(strings.TrimPrefix(pv.value, "${"), "}")
			}
		}
		if props := p.val.get("properties"); props != nil && props.kind == nodeMap {
			if _, handled := pr.classifyNode(props, 0); !handled {
				for _, pp := range props.pairs {
					w.walk(pp.val, pp.key.value, pp.span, &r.Attrs, 0)
				}
			}
		}
		if w.err != nil {
			return nil, w.err
		}
		set.Resources = append(set.Resources, r)
	}
	sortSet(set)
	return set, nil
}

type pulumiReader struct {
	cfg       config
	root      *node
	resolving map[string]bool
}

// pulumiBuiltins are the fn:: builtins a program may call.
var pulumiBuiltins = map[string]bool{
	"invoke": true, "join": true, "split": true, "toJSON": true, "toBase64": true, "fromBase64": true,
	"select": true, "readFile": true, "fileAsset": true, "stringAsset": true, "remoteAsset": true,
	"fileArchive": true, "remoteArchive": true, "assetArchive": true, "stackReference": true,
	"rawSecret": true, "secret": true,
}

// classifyNode classifies a string with interpolations and a fn:: call. It reports false
// for a node that is plain structure (a mapping, a list) or a plain scalar, which the walker
// descends into or emits as a literal. A scalar with ${} is classified here.
func (p *pulumiReader) classifyNode(n *node, depth int) (readRes, bool) {
	if n == nil {
		return readRes{}, false
	}
	if n.kind == nodeMap && len(n.pairs) == 1 && n.pairs[0].key.kind == nodeScalar && strings.HasPrefix(n.pairs[0].key.value, "fn::") {
		name := strings.TrimPrefix(n.pairs[0].key.value, "fn::")
		arg := n.pairs[0].val
		if !pulumiBuiltins[name] {
			return readRes{class: ClassUnknown}, true
		}
		if name == "secret" || name == "rawSecret" {
			if arg.kind == nodeScalar {
				if r, ok := p.classifyNode(arg, depth+1); ok {
					return r, true
				}
				return readRes{class: ClassLiteral, val: arg.scalarValue(), hasV: true}, true
			}
		}
		return readRes{class: ClassExpression, refs: p.collectRefs(arg, depth+1)}, true
	}
	if n.kind == nodeScalar && n.isStr() && strings.Contains(n.value, "${") {
		return p.interpolation(n.value, depth), true
	}
	return readRes{}, false
}

// interpolation classifies a string by its ${} parts. $${ is a literal.
func (p *pulumiReader) interpolation(s string, depth int) readRes {
	var parts []readRes
	for len(s) > 0 {
		i := strings.Index(s, "${")
		if i < 0 {
			parts = append(parts, readRes{class: ClassLiteral, val: s, hasV: true})
			break
		}
		if i > 0 && s[i-1] == '$' {
			parts = append(parts, readRes{class: ClassLiteral, val: s[:i-1] + "${", hasV: true})
			s = s[i+2:]
			continue
		}
		if i > 0 {
			parts = append(parts, readRes{class: ClassLiteral, val: s[:i], hasV: true})
		}
		s = s[i+2:]
		j := strings.IndexByte(s, '}')
		if j < 0 {
			return readRes{class: ClassUnknown}
		}
		parts = append(parts, p.variable(s[:j], depth))
		s = s[j+1:]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	out := foldRes(parts)
	if out.hasV {
		out.val, out.hasV = concatVals(parts)
	}
	return out
}

// variable classifies one ${name} or ${name.prop}.
func (p *pulumiReader) variable(expr string, depth int) readRes {
	name := expr
	if i := strings.IndexAny(expr, ".["); i >= 0 {
		name = expr[:i]
	}
	ref := expr
	if i := strings.IndexByte(expr, '['); i >= 0 {
		ref = expr[:i]
	}
	switch {
	case name == "pulumi":
		return readRes{class: ClassExpression}
	case p.root.get("resources").get(name) != nil:
		return readRes{class: ClassReference, refs: []string{ref}}
	case p.root.get("configuration").get(name) != nil:
		c := p.root.get("configuration").get(name)
		if d := c.get("default"); d != nil && d.kind == nodeScalar && ref == name {
			return readRes{class: ClassDefaulted, refs: []string{ref}, val: d.scalarValue(), hasV: true}
		}
		return readRes{class: ClassReference, refs: []string{ref}}
	case p.root.get("variables").get(name) != nil:
		if p.resolving[name] || depth > MaxDepth {
			return readRes{class: ClassUnknown, refs: []string{ref}}
		}
		p.resolving[name] = true
		v := p.root.get("variables").get(name)
		var inner readRes
		if r, ok := p.classifyNode(v, depth+1); ok {
			inner = r
		} else {
			inner = p.structural(v, depth+1)
		}
		delete(p.resolving, name)
		out := readRes{class: inner.class, refs: []string{ref}}
		if ref == name && inner.hasV {
			out.val, out.hasV = inner.val, true
		}
		return out
	}
	return readRes{class: ClassUnknown, refs: []string{ref}}
}

// structural classifies a mapping, list or scalar whole, by folding its parts.
func (p *pulumiReader) structural(n *node, depth int) readRes {
	if depth > MaxDepth {
		return readRes{class: ClassUnknown}
	}
	if r, ok := p.classifyNode(n, depth); ok {
		return r
	}
	switch n.kind {
	case nodeScalar:
		return readRes{class: ClassLiteral, val: n.scalarValue(), hasV: true}
	case nodeSeq:
		parts := make([]readRes, len(n.items))
		for i, it := range n.items {
			parts[i] = p.structural(it, depth+1)
		}
		out := foldRes(parts)
		if out.hasV {
			list := make([]any, len(parts))
			for i, q := range parts {
				list[i] = q.val
			}
			out.val = list
		}
		return out
	case nodeMap:
		parts := make([]readRes, len(n.pairs))
		for i, pr := range n.pairs {
			parts[i] = p.structural(pr.val, depth+1)
		}
		out := foldRes(parts)
		if out.hasV {
			m := make(map[string]any, len(parts))
			for i, pr := range n.pairs {
				m[pr.key.value] = parts[i].val
			}
			out.val = m
		}
		return out
	}
	return readRes{class: ClassUnknown}
}

// collectRefs lists the references a builtin's argument reads.
func (p *pulumiReader) collectRefs(n *node, depth int) []string {
	if n == nil || depth > MaxDepth {
		return nil
	}
	var out []string
	if n.kind == nodeScalar && n.isStr() {
		s := n.value
		for {
			i := strings.Index(s, "${")
			if i < 0 {
				break
			}
			if i > 0 && s[i-1] == '$' {
				s = s[i+2:]
				continue
			}
			s = s[i+2:]
			j := strings.IndexByte(s, '}')
			if j < 0 {
				break
			}
			e := s[:j]
			if k := strings.IndexByte(e, '['); k >= 0 {
				e = e[:k]
			}
			if !strings.HasPrefix(e, "pulumi") {
				out = append(out, e)
			}
			s = s[j+1:]
		}
	}
	for _, pr := range n.pairs {
		out = append(out, p.collectRefs(pr.val, depth+1)...)
	}
	for _, it := range n.items {
		out = append(out, p.collectRefs(it, depth+1)...)
	}
	return sortedRefs(out)
}
