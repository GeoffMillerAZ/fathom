package iacfacts

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ReadCloudFormationFile reads the resources of an AWS CloudFormation or SAM template in
// YAML or JSON: each resource's Properties as attribute paths with classes and spans,
// intrinsic functions (the long Fn:: form and the short YAML tags such as !Ref, !GetAtt,
// !Sub and !If), a resource's Condition as its multiplicity, and parameter defaults. A
// !Ref to a parameter with a default reads as defaulted, and a parameter of an SSM
// parameter type does not, because its default is a path and not the value.
//
// Lists take [*] in a path. A template that does not parse reads as empty with
// SyntaxErrors set. A template past the product's limits is refused whole with ErrLimit.
func ReadCloudFormationFile(filePath string, src []byte, opts ...Option) (*ResourceSet, error) {
	set := &ResourceSet{Tool: ToolCloudFormation}
	root, info, err := parseTemplate(filePath, src)
	if errors.Is(err, errSyntax) {
		set.SyntaxErrors = 1
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	set.SyntaxErrors = info.syntaxErrors()
	res := root.get("Resources")
	if root == nil || root.kind != nodeMap || res == nil || res.kind != nodeMap {
		return set, nil
	}
	cr := &cfnReader{params: map[string]cfnParam{}, conds: root.get("Conditions"),
		classMemo: map[*node]cfnClass{}, refMemo: map[*node][]string{}}
	cr.cfg = newConfig(opts)
	if ps := root.get("Parameters"); ps != nil && ps.kind == nodeMap {
		for _, p := range ps.pairs {
			prm := cfnParam{}
			if def := p.val.get("Default"); def != nil && def.kind == nodeScalar {
				prm.def = def
			}
			if t := p.val.get("Type"); t != nil && strings.HasPrefix(t.value, "AWS::SSM::Parameter::Value<") {
				prm.indirect = true
			}
			if ne := p.val.get("NoEcho"); ne != nil && ne.kind == nodeScalar && strings.EqualFold(ne.value, "true") {
				prm.noEcho = true
			}
			cr.params[p.key.value] = prm
		}
	}
	w := &treeWalker{emitter: emitter{cfg: cr.cfg}, file: filePath, classify: cr.classifyIntrinsic}
	for _, p := range res.pairs {
		var typ *node
		if p.val.kind == nodeMap {
			typ = p.val.get("Type")
		}
		if typ == nil || typ.kind != nodeScalar {
			// Fn::ForEach, an alias, a merge-only entry, a scalar: not a resource the reader can name.
			set.Dropped = append(set.Dropped, p.key.value)
			continue
		}
		r := Resource{
			Tool: ToolCloudFormation, Mode: ModeManaged, Type: typ.value, Name: p.key.value,
			Span: p.span, Multiplicity: Multiplicity{Kind: MultiSingle},
		}
		for _, cp := range p.val.pairs {
			if cp.key.value == "Condition" && cp.val.kind == nodeScalar {
				r.Multiplicity = cr.condition(cp.val, cp.span)
			}
		}
		if props := p.val.get("Properties"); props != nil {
			ires, intrinsic := cr.classifyIntrinsic(props, 0)
			switch {
			case !intrinsic && props.kind == nodeMap:
				for _, pp := range props.pairs {
					w.walk(pp.val, pp.key.value, pp.span, &r.Attrs, 0)
				}
			case props.kind == nodeScalar && props.typ == "null":
				// `Properties:` with nothing under it sets nothing.
			default:
				// An intrinsic (!If, Fn::Transform), an alias, a list or a scalar: the shape of the
				// resource is not known, so every attribute is.
				w.emit(filePath, &r.Attrs, "", readRes{class: ClassUnknown, refs: ires.refs}, props.span)
				set.SyntaxErrors++
			}
		}
		if w.err != nil {
			return nil, w.err
		}
		if cr.over {
			return nil, limitErr(filePath, fmt.Sprintf("more than %d intrinsic-function visits", MaxReadVisits))
		}
		set.Resources = append(set.Resources, r)
	}
	sort.Strings(set.Dropped)
	sortSet(set)
	return set, nil
}

type cfnParam struct {
	def      *node
	indirect bool // the default is a path the service resolves, not the value
	noEcho   bool // NoEcho: the value is a secret and is never kept, whatever the caller asks
}

type cfnReader struct {
	cfg    config
	params map[string]cfnParam
	conds  *node
	// classMemo and refMemo make classification linear: a node is classified once, however many ways it is
	// reached (an intrinsic's argument, a Sub definition used twice, a condition). nodes counts the first
	// visits against MaxNodes, and over is set when the budget is spent.
	classMemo map[*node]cfnClass
	refMemo   map[*node][]string
	nodes     int
	over      bool
}

// cfnClass is a memoized classifyIntrinsic answer.
type cfnClass struct {
	res readRes
	ok  bool
}

// spend counts one first visit of a node and reports whether the budget still holds.
func (c *cfnReader) spend() bool {
	if c.over {
		return false
	}
	if c.nodes++; c.nodes > MaxReadVisits {
		c.over = true
		return false
	}
	return true
}

// condition reads a resource's Condition: the multiplicity is the condition, with the
// class of its definition.
func (c *cfnReader) condition(name *node, span Span) Multiplicity {
	m := Multiplicity{Kind: MultiCondition, Class: ClassUnknown, Refs: []string{name.value}, Span: span}
	if def := c.conds.get(name.value); def != nil {
		if res, ok := c.classifyIntrinsic(def, 0); ok {
			m.Class = res.class
			m.Refs = append(m.Refs, res.refs...)
		} else {
			m.Class = ClassUnknown
		}
	}
	return m
}

// intrinsic reports whether n is an intrinsic function, and its normalized name (Ref,
// GetAtt, Sub, If) and argument.
func intrinsic(n *node) (string, *node, bool) {
	if n == nil {
		return "", nil, false
	}
	if n.tag != "" {
		arg := *n
		arg.tag = ""
		return strings.TrimPrefix(n.tag, "!"), &arg, true
	}
	if n.kind == nodeMap && len(n.pairs) == 1 && n.pairs[0].key.kind == nodeScalar {
		k := n.pairs[0].key.value
		if k == "Ref" {
			return "Ref", n.pairs[0].val, true
		}
		if strings.HasPrefix(k, "Fn::") {
			return strings.TrimPrefix(k, "Fn::"), n.pairs[0].val, true
		}
	}
	return "", nil, false
}

// classifyIntrinsic classifies an intrinsic function. It reports false for any other node.
func (c *cfnReader) classifyIntrinsic(n *node, depth int) (readRes, bool) {
	if _, _, ok := intrinsic(n); !ok {
		return readRes{}, false
	}
	if m, hit := c.classMemo[n]; hit {
		return m.res, m.ok
	}
	if !c.spend() {
		return readRes{class: ClassUnknown}, true
	}
	res, ok := c.classifyIntrinsicOnce(n, depth)
	c.classMemo[n] = cfnClass{res, ok}
	return res, ok
}

func (c *cfnReader) classifyIntrinsicOnce(n *node, depth int) (readRes, bool) {
	name, arg, _ := intrinsic(n)
	if depth > MaxDepth {
		return readRes{class: ClassUnknown}, true
	}
	switch name {
	case "Ref":
		if arg.kind != nodeScalar {
			return readRes{class: ClassUnknown}, true
		}
		return c.ref(arg.value), true
	case "GetAtt":
		switch {
		case arg.kind == nodeScalar:
			return readRes{class: ClassReference, refs: []string{arg.value}}, true
		case arg.kind == nodeSeq && len(arg.items) == 2 && arg.items[0].kind == nodeScalar && arg.items[1].kind == nodeScalar:
			return readRes{class: ClassReference, refs: []string{arg.items[0].value + "." + arg.items[1].value}}, true
		}
		return readRes{class: ClassExpression, refs: c.collectRefs(arg, depth+1)}, true
	case "ImportValue":
		if arg.kind == nodeScalar {
			return readRes{class: ClassReference, refs: []string{"export:" + arg.value}}, true
		}
		return readRes{class: ClassExpression, refs: c.collectRefs(arg, depth+1)}, true
	case "Sub":
		return c.sub(arg, depth), true
	case "If":
		refs := c.collectRefs(arg, depth+1)
		if arg.kind == nodeSeq && len(arg.items) > 0 && arg.items[0].kind == nodeScalar {
			refs = sortedRefs(append(refs, arg.items[0].value))
		}
		return readRes{class: ClassExpression, refs: refs}, true
	case "Join", "Select", "Split", "FindInMap", "Base64", "Cidr", "GetAZs", "Equals", "And", "Or", "Not",
		"Transform", "Length", "ToJsonString", "Condition", "Contains", "EachMemberEquals", "EachMemberIn", "RefAll",
		"ValueOf", "ValueOfAll", "Implies", "Include":
		return readRes{class: ClassExpression, refs: c.collectRefs(arg, depth+1)}, true
	}
	if strings.HasPrefix(name, "ForEach::") {
		return readRes{class: ClassExpression, refs: c.collectRefs(arg, depth+1)}, true
	}
	return readRes{class: ClassUnknown}, true
}

// ref classifies a Ref.
func (c *cfnReader) ref(name string) readRes {
	if strings.HasPrefix(name, "AWS::") {
		return readRes{class: ClassExpression, refs: []string{name}}
	}
	if p, ok := c.params[name]; ok && p.def != nil && !p.indirect {
		return readRes{class: ClassDefaulted, refs: []string{name}, val: p.def.scalarValue(), hasV: c.cfg.values && !p.noEcho}
	}
	return readRes{class: ClassReference, refs: []string{name}}
}

// sub classifies Fn::Sub: a string with ${Name} and ${Res.Attr} variables, and an
// optional map that defines more.
func (c *cfnReader) sub(arg *node, depth int) readRes {
	str := arg
	var defs *node
	if arg.kind == nodeSeq {
		if len(arg.items) != 2 {
			return readRes{class: ClassUnknown}
		}
		str, defs = arg.items[0], arg.items[1]
	}
	if str.kind != nodeScalar || !str.isStr() {
		return readRes{class: ClassUnknown}
	}
	var parts []readRes
	s := str.value
	for len(s) > 0 {
		i := strings.Index(s, "${")
		if i < 0 {
			parts = append(parts, readRes{class: ClassLiteral, val: s, hasV: c.cfg.values})
			break
		}
		if i > 0 {
			parts = append(parts, readRes{class: ClassLiteral, val: s[:i], hasV: c.cfg.values})
		}
		s = s[i+2:]
		j := strings.IndexByte(s, '}')
		if j < 0 {
			return readRes{class: ClassUnknown}
		}
		v := s[:j]
		s = s[j+1:]
		if lit, escaped := strings.CutPrefix(v, "!"); escaped {
			parts = append(parts, readRes{class: ClassLiteral, val: "${" + lit + "}", hasV: c.cfg.values})
			continue
		}
		if d := defs.get(v); d != nil {
			if r, ok := c.classifyIntrinsic(d, depth+1); ok {
				parts = append(parts, r)
			} else if d.kind == nodeScalar {
				parts = append(parts, readRes{class: ClassLiteral, val: d.scalarValue(), hasV: c.cfg.values})
			} else {
				parts = append(parts, readRes{class: ClassExpression})
			}
			continue
		}
		if strings.Contains(v, ".") {
			parts = append(parts, readRes{class: ClassReference, refs: []string{v}})
		} else {
			parts = append(parts, c.ref(v))
		}
	}
	if len(parts) == 0 {
		return readRes{class: ClassLiteral, val: "", hasV: c.cfg.values}
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

// collectRefs lists the references an intrinsic's argument reads. A node's answer is computed once.
func (c *cfnReader) collectRefs(n *node, depth int) []string {
	if n == nil || depth > MaxDepth {
		return nil
	}
	if refs, hit := c.refMemo[n]; hit {
		return refs
	}
	if !c.spend() {
		return nil
	}
	out := c.collectRefsOnce(n, depth)
	c.refMemo[n] = out
	return out
}

func (c *cfnReader) collectRefsOnce(n *node, depth int) []string {
	var lists [][]string
	if r, ok := c.classifyIntrinsic(n, depth); ok {
		name, arg, _ := intrinsic(n)
		switch {
		case name == "Ref" || name == "GetAtt" || name == "Sub" || name == "ImportValue":
			return sortedRefs(r.refs)
		case r.class == ClassExpression:
			// The classification already read the whole argument.
			return sortedRefs(r.refs)
		}
		lists = append(lists, r.refs)
		n = arg
	}
	for _, p := range n.pairs {
		lists = append(lists, c.collectRefs(p.val, depth+1))
	}
	for _, it := range n.items {
		lists = append(lists, c.collectRefs(it, depth+1))
	}
	return unionRefs(lists...)
}
