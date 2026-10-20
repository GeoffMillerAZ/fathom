package main

import "path"

// xref.go -- the cross-file references the reference parse attests. A name selected from an imported file and
// applied is a reference to that file: `let u = import ./u.nix; in u.f 1`, `(import ./u.nix).f 1`,
// `inherit (import ./u.nix) f;` then `f 1`. The imported file's root attribute set must define the name. The
// scope is read as Nix reads it: a let binding or a function argument named like the import shadows it.

type scopeVal struct {
	imp    string // the subject file a binding imports
	member string // non-empty: the binding is that file's attribute `member`
}

type scope map[string]*scopeVal // a nil entry shadows

func (s scope) with(add map[string]*scopeVal) scope {
	out := make(scope, len(s)+len(add))
	for k, v := range s {
		out[k] = v
	}
	for k, v := range add {
		out[k] = v
	}
	return out
}

type cand struct{ from, to, attr string }

type xr struct {
	subject string
	cands   []cand
}

func (x *xr) rel(abs string) string {
	r := path.Clean(abs)
	if len(r) > len(x.subject) && r[:len(x.subject)] == x.subject && r[len(x.subject)] == '/' {
		return resolveNix(x.subject, r[len(x.subject)+1:], false)
	}
	return ""
}

// importOf is the subject file n imports: `import <path> args...`, or a name bound to one.
func (x *xr) importOf(n *node, sc scope) string {
	if n == nil {
		return ""
	}
	switch n.kind {
	case nApp:
		if n.head.kind == nVar && n.head.name == "import" && sc["import"] == nil && len(n.kids) > 0 && n.kids[0].kind == nPath {
			return x.rel(n.kids[0].name)
		}
	case nVar:
		if v := sc[n.name]; v != nil && v.member == "" {
			return v.imp
		}
	}
	return ""
}

// memberOf is (file, attribute) when n selects an attribute of an imported file, directly or through a name.
func (x *xr) memberOf(n *node, sc scope) (string, string) {
	if n == nil {
		return "", ""
	}
	switch n.kind {
	case nSelect:
		if t := x.importOf(n.head, sc); t != "" && n.name != "" {
			return t, n.name
		}
	case nVar:
		if v := sc[n.name]; v != nil && v.member != "" {
			return v.imp, v.member
		}
	}
	return "", ""
}

func (x *xr) bind(n *node, sc scope) *scopeVal {
	if t := x.importOf(n, sc); t != "" {
		return &scopeVal{imp: t}
	}
	if t, a := x.memberOf(n, sc); t != "" {
		return &scopeVal{imp: t, member: a}
	}
	return nil
}

func (x *xr) letScope(n *node, sc scope) scope {
	add := map[string]*scopeVal{}
	sc2 := sc.with(nil)
	for _, k := range n.order { // a let is recursive; one in-order pass resolves the imports a later binding names
		add[k] = nil
	}
	sc2 = sc.with(add)
	for _, k := range n.order {
		if v := x.bind(n.binds[k], sc2); v != nil {
			add[k] = v
			sc2 = sc.with(add)
		}
	}
	return sc2
}

func (x *xr) visit(n *node, sc scope, from string) {
	if n == nil {
		return
	}
	switch n.kind {
	case nLambda:
		add := map[string]*scopeVal{}
		for _, f := range n.formals {
			add[f] = nil
		}
		if n.arg != "" {
			add[n.arg] = nil
		}
		sc = sc.with(add)
	case nLet:
		sc = x.letScope(n, sc)
	case nApp:
		if t, a := x.memberOf(n.head, sc); t != "" {
			x.cands = append(x.cands, cand{from, t, a})
		}
	}
	x.visit(n.head, sc, from)
	x.visit(n.body, sc, from)
	for _, k := range n.kids {
		x.visit(k, sc, from)
	}
	for _, k := range n.order {
		x.visit(n.binds[k], sc, from)
	}
}

// root walks the root expression as rootScopes does, visiting each definition's value.
func (x *xr) root(n *node, sc scope) {
	for wraps := 0; n != nil && wraps <= maxWrappers; wraps++ {
		switch n.kind {
		case nLambda:
			if wraps == maxWrappers {
				return
			}
			add := map[string]*scopeVal{}
			for _, f := range n.formals {
				add[f] = nil
			}
			if n.arg != "" {
				add[n.arg] = nil
			}
			sc = sc.with(add)
			n = n.body
		case nWith:
			if wraps == maxWrappers {
				return
			}
			n = n.body
		case nLet:
			sc = x.letScope(n, sc)
			for _, k := range n.order {
				x.visit(n.binds[k], sc, k)
			}
			if wraps == maxWrappers {
				return
			}
			n = n.body
		case nSet:
			for _, k := range n.order {
				x.visit(n.binds[k], sc, k)
			}
			return
		default:
			return
		}
	}
}
