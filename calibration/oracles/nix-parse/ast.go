package main

// ast.go -- the reference parse. `nix-instantiate --parse` prints the parsed expression back as Nix text, fully
// parenthesised, with attribute sets merged and sorted and every path made absolute. This file reads that text
// back into a small tree: enough to attest a name (a key of the root set or let), to classify a binding's
// value (a function, or an application of a named function), to list the applications inside a binding, and to
// list the path literals. It reads the printer's grammar, not Nix's.

import (
	"fmt"
	"strings"
)

type nodeKind int

const (
	nOther nodeKind = iota
	nLambda
	nLet
	nWith
	nSet
	nApp
	nSelect
	nVar
	nPath
	nStr
	nList
)

type node struct {
	kind    nodeKind
	name    string // nVar: identifier; nPath: absolute path; nSelect: last attribute ("" when dynamic)
	rec     bool
	formals []string // nLambda
	arg     string   // nLambda: the single argument or the @ alias
	binds   map[string]*node
	order   []string // keys in print order
	body    *node    // nLambda, nLet, nWith
	head    *node    // nApp, nSelect
	kids    []*node  // everything else inside (arguments, elements, defaults, operands)
}

type atok struct {
	kind tokKind
	text string
}

// atoks tokenizes printed Nix.
func atoks(s string) []atok {
	var out []atok
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\n' || c == '\t':
			i++
		case c == '"':
			j := i + 1
			depth := 0
			for j < n {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == '$' && j+1 < n && s[j+1] == '{' {
					depth++
					j += 2
					continue
				}
				if depth > 0 && s[j] == '}' {
					depth--
					j++
					continue
				}
				if s[j] == '"' && depth == 0 {
					break
				}
				j++
			}
			out = append(out, atok{tString, s[i+1 : min(j, n)]})
			i = min(j+1, n)
		case c == '/' && i > 0 && s[i-1] == '(' && i+1 < n && s[i+1] == ' ': // the root path, `(/ + x)`
			out = append(out, atok{tPath, "/"})
			i++
		case c == '/' && i+1 < n && s[i+1] != ' ' && s[i+1] != '/' && (i == 0 || s[i-1] == ' ' || s[i-1] == '(' || s[i-1] == '[' || s[i-1] == '{'):
			j := i
			for j < n && s[j] != ' ' && s[j] != ')' && s[j] != ';' && s[j] != ']' && s[j] != '}' {
				j++
			}
			out = append(out, atok{tPath, s[i:j]})
			i = j
		case isIdentStart(c):
			j := i
			for j < n && isIdentChar(s[j]) {
				j++
			}
			out = append(out, atok{tIdent, s[i:j]})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (s[j] >= '0' && s[j] <= '9' || s[j] == '.' && j+1 < n && s[j+1] >= '0' && s[j+1] <= '9' || s[j] == 'e') {
				j++
			}
			out = append(out, atok{tNum, s[i:j]})
			i = j
		default:
			if strings.HasPrefix(s[i:], "...") {
				out = append(out, atok{tPunct, "..."})
				i += 3
				continue
			}
			for _, p := range []string{"//", "++", "&&", "||", "->", "==", "!=", "<=", ">=", "|>"} {
				if strings.HasPrefix(s[i:], p) {
					out = append(out, atok{tPunct, p})
					i += 2
					goto next
				}
			}
			out = append(out, atok{tPunct, string(c)})
			i++
		next:
		}
	}
	return out
}

type aparser struct {
	t   []atok
	i   int
	err error
}

func (p *aparser) peek() atok {
	if p.i < len(p.t) {
		return p.t[p.i]
	}
	return atok{tPunct, "\x00"}
}
func (p *aparser) at(s string) bool { t := p.peek(); return t.kind != tString && t.text == s }
func (p *aparser) atK(s string) bool {
	t := p.peek()
	return t.kind == tIdent && t.text == s
}
func (p *aparser) next() atok { t := p.peek(); p.i++; return t }
func (p *aparser) fail(f string, a ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(f+" at token %d", append(a, p.i)...)
	}
}

func (p *aparser) want(s string) {
	if !p.at(s) {
		p.fail("want %q, have %q", s, p.peek().text)
		return
	}
	p.i++
}

// parseAST reads one printed expression.
func parseAST(s string) (*node, error) {
	p := &aparser{t: atoks(s)}
	n := p.expr()
	if p.err == nil && p.i < len(p.t) {
		p.fail("trailing %q", p.peek().text)
	}
	return n, p.err
}

var binops = map[string]bool{"+": true, "-": true, "*": true, "/": true, "//": true, "++": true, "&&": true, "||": true, "->": true, "==": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true, "?": true}

// lambdaAhead reports whether a lambda starts at the current token.
func (p *aparser) lambdaAhead() bool {
	t := p.peek()
	if t.kind == tIdent && p.i+1 < len(p.t) && (p.t[p.i+1].text == ":" || p.t[p.i+1].text == "@") && p.t[p.i+1].kind == tPunct {
		return true
	}
	if t.kind == tPunct && t.text == "{" {
		depth := 0
		for j := p.i; j < len(p.t); j++ {
			if p.t[j].kind != tPunct {
				continue
			}
			switch p.t[j].text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				depth--
				if depth == 0 {
					return j+1 < len(p.t) && p.t[j+1].kind == tPunct && (p.t[j+1].text == ":" || p.t[j+1].text == "@")
				}
			}
		}
	}
	return false
}

func (p *aparser) expr() *node {
	if p.err != nil {
		return &node{}
	}
	switch {
	case p.atK("let"):
		p.next()
		n := &node{kind: nLet}
		p.bindings(n, "in")
		p.want("in")
		n.body = p.expr()
		return n
	case p.atK("if"):
		p.next()
		n := &node{}
		n.kids = append(n.kids, p.expr())
		if p.atK("then") {
			p.next()
		}
		n.kids = append(n.kids, p.expr())
		if p.atK("else") {
			p.next()
		}
		n.kids = append(n.kids, p.expr())
		return n
	case p.atK("with"):
		p.next()
		n := &node{kind: nWith}
		n.kids = append(n.kids, p.expr())
		p.want(";")
		n.body = p.expr()
		return n
	case p.atK("assert"):
		p.next()
		n := &node{}
		n.kids = append(n.kids, p.expr())
		p.want(";")
		n.kids = append(n.kids, p.expr())
		return n
	case p.lambdaAhead():
		return p.lambda()
	}
	return p.operands()
}

func (p *aparser) lambda() *node {
	n := &node{kind: nLambda}
	if p.peek().kind == tIdent {
		n.arg = p.next().text
		if p.at("@") {
			p.next()
			p.formalsInto(n)
		}
	} else {
		p.formalsInto(n)
		if p.at("@") {
			p.next()
			n.arg = p.next().text
		}
	}
	p.want(":")
	n.body = p.expr()
	return n
}

func (p *aparser) formalsInto(n *node) {
	p.want("{")
	for p.err == nil && !p.at("}") {
		switch {
		case p.at("..."):
			p.next()
		case p.peek().kind == tIdent:
			n.formals = append(n.formals, p.next().text)
			if p.at("?") {
				p.next()
				n.kids = append(n.kids, p.expr())
			}
		default:
			p.fail("formal %q", p.peek().text)
			return
		}
		if p.at(",") {
			p.next()
		}
	}
	p.want("}")
}

// operands reads atoms: an application (atom atom ...) or a binary chain (atom op atom ...).
func (p *aparser) operands() *node {
	first := p.unary()
	if p.err != nil {
		return first
	}
	items := []*node{first}
	binary := false
	for p.err == nil {
		t := p.peek()
		if t.kind == tPunct && binops[t.text] {
			p.next()
			binary = true
			if t.text == "?" {
				items = append(items, p.attrpath())
			} else {
				items = append(items, p.unary())
			}
			continue
		}
		if p.atomStart() {
			items = append(items, p.atom())
			continue
		}
		break
	}
	if binary || len(items) == 1 {
		if len(items) == 1 {
			return items[0]
		}
		return &node{kids: items}
	}
	return &node{kind: nApp, head: items[0], kids: items[1:]}
}

func (p *aparser) unary() *node {
	if p.at("!") || p.at("-") {
		p.next()
		return &node{kids: []*node{p.unary()}}
	}
	return p.atom()
}

func (p *aparser) atomStart() bool {
	t := p.peek()
	switch t.kind {
	case tIdent:
		switch t.text {
		case "in", "then", "else", "let", "if", "with", "assert":
			return false
		}
		return true
	case tString, tNum, tPath:
		return true
	case tPunct:
		return t.text == "(" || t.text == "{" || t.text == "["
	}
	return false
}

func (p *aparser) attrpath() *node {
	n := p.atom()
	return n
}

// atom: a primary and its `.attr` selections and `or` default.
func (p *aparser) atom() *node {
	n := p.primary()
	for p.err == nil && p.at(".") {
		p.next()
		sel := &node{kind: nSelect, head: n}
		t := p.next()
		switch {
		case t.kind == tIdent:
			sel.name = t.text
		case t.kind == tString:
			if strings.HasPrefix(t.text, "${") {
				sel.name = ""
			} else {
				sel.name = t.text
			}
		default:
			p.fail("attribute %q", t.text)
		}
		n = sel
	}
	if p.err == nil && p.atK("or") && n.kind == nSelect {
		p.next()
		n.kids = append(n.kids, p.atom())
	}
	return n
}

func (p *aparser) primary() *node {
	t := p.peek()
	switch {
	case t.kind == tIdent && t.text == "rec" && p.i+1 < len(p.t) && p.t[p.i+1].text == "{":
		p.next()
		n := p.set()
		n.rec = true
		return n
	case t.kind == tIdent:
		p.next()
		return &node{kind: nVar, name: t.text}
	case t.kind == tString:
		p.next()
		return &node{kind: nStr, name: t.text}
	case t.kind == tNum:
		p.next()
		return &node{}
	case t.kind == tPath:
		p.next()
		return &node{kind: nPath, name: t.text}
	case t.kind == tPunct && t.text == "(":
		p.next()
		n := p.expr()
		p.want(")")
		return n
	case t.kind == tPunct && t.text == "{":
		return p.set()
	case t.kind == tPunct && t.text == "[":
		p.next()
		n := &node{kind: nList}
		for p.err == nil && !p.at("]") {
			if !p.atomStart() {
				p.fail("list element %q", p.peek().text)
				break
			}
			n.kids = append(n.kids, p.atom())
		}
		p.want("]")
		return n
	}
	p.fail("unexpected %q", t.text)
	return &node{}
}

func (p *aparser) set() *node {
	p.want("{")
	n := &node{kind: nSet}
	p.bindings(n, "}")
	p.want("}")
	return n
}

// bindings reads `key = value;` and `inherit ... ;` until the terminator keyword or bracket.
func (p *aparser) bindings(n *node, stop string) {
	n.binds = map[string]*node{}
	for p.err == nil && !p.at(stop) && !p.atK(stop) {
		if p.atK("inherit") {
			p.next()
			var from *node
			if p.at("(") {
				p.next()
				from = p.expr()
				p.want(")")
			}
			for p.err == nil && !p.at(";") {
				t := p.next()
				if t.kind == tIdent || t.kind == tString {
					// `inherit (e) a` is `a = e.a`; a plain `inherit a` has no value of its own here
					var v *node
					if from != nil {
						v = &node{kind: nSelect, head: from, name: t.text}
					}
					n.binds[t.text] = v
					n.order = append(n.order, t.text)
				} else {
					p.fail("inherit %q", t.text)
				}
			}
			p.want(";")
			continue
		}
		t := p.next()
		if t.kind != tIdent && t.kind != tString {
			p.fail("binding key %q", t.text)
			return
		}
		p.want("=")
		v := p.expr()
		p.want(";")
		n.binds[t.text] = v
		n.order = append(n.order, t.text)
	}
}
