package main

// src.go -- the root-expression reader. A Nix file's definitions are its root expression's own names (the
// dialect's rule, dialects/nix/tags.scm): the file's expression peeled through at most three wrappers (a
// function, a let, a with), then the bindings of the let or attribute set it ends in, and the formals of every
// function it passed. This reader finds them in the token stream, with their lines, independently of
// tree-sitter; the reference parse (ast.go) attests every name it reports.

type site struct {
	name    string
	line    int
	end     int
	pathLen int    // attributes in the key: 1 for `a = ...`, 3 for `a.b.c = ...`
	scope   string // "let" or "attr"
	dyn     bool   // the first attribute is a string or an interpolation (no identifier)
}

type param struct {
	name string
	line int
}

type rootInfo struct {
	sites  []site
	params []param
	wraps  int
}

const maxWrappers = 3

func isP(t token, s string) bool { return t.kind == tPunct && t.text == s }
func isK(t token, s string) bool { return t.kind == tIdent && t.text == s }

// matching returns the index of the token closing the bracket at i, or -1.
func matching(toks []token, i int) int {
	open := toks[i].text
	closeTok := map[string]string{"{": "}", "(": ")", "[": "]"}[open]
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].kind != tPunct {
			continue
		}
		switch toks[j].text {
		case open:
			depth++
		case closeTok:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// readRoot peels the root expression and collects its definitions.
func readRoot(toks []token) rootInfo {
	var info rootInfo
	i := 0
	for info.wraps <= maxWrappers && i < len(toks) {
		t := toks[i]
		switch {
		case isP(t, "{"):
			j := matching(toks, i)
			if j < 0 {
				return info
			}
			// formals: { ... } followed by `:` or `@`
			if j+1 < len(toks) && (isP(toks[j+1], ":") || isP(toks[j+1], "@")) {
				if info.wraps == maxWrappers {
					return info
				}
				info.params = append(info.params, formals(toks[i+1:j])...)
				k := j + 1
				if isP(toks[k], "@") && k+2 < len(toks) && toks[k+1].kind == tIdent && isP(toks[k+2], ":") {
					info.params = append(info.params, param{toks[k+1].text, toks[k+1].line})
					k += 2
				}
				i = k + 1
				info.wraps++
				continue
			}
			if j+1 < len(toks) { // `{ ... } // other`, `{ ... }.x`: the set is an operand, not the root
				return info
			}
			info.sites = append(info.sites, bindings(toks[i+1:j], "attr")...)
			return info
		case isK(t, "rec") && i+1 < len(toks) && isP(toks[i+1], "{"):
			j := matching(toks, i+1)
			if j < 0 {
				return info
			}
			if j+1 < len(toks) {
				return info
			}
			info.sites = append(info.sites, bindings(toks[i+2:j], "attr")...)
			return info
		case t.kind == tIdent && i+1 < len(toks) && isP(toks[i+1], ":") && !isKeyword(t.text):
			if info.wraps == maxWrappers {
				return info
			}
			info.params = append(info.params, param{t.text, t.line})
			i += 2
			info.wraps++
		case t.kind == tIdent && i+1 < len(toks) && isP(toks[i+1], "@") && !isKeyword(t.text):
			// name @ { formals } :
			if i+2 < len(toks) && isP(toks[i+2], "{") {
				j := matching(toks, i+2)
				if j > 0 && j+1 < len(toks) && isP(toks[j+1], ":") && info.wraps < maxWrappers {
					info.params = append(info.params, param{t.text, t.line})
					info.params = append(info.params, formals(toks[i+3:j])...)
					i = j + 2
					info.wraps++
					continue
				}
			}
			return info
		case isK(t, "let"):
			in := letEnd(toks, i)
			if in < 0 {
				return info
			}
			scope := "let"
			if info.wraps == maxWrappers {
				scope = "attr" // the dialect's terminal position reads a let's bindings as attribute-set keys
			}
			info.sites = append(info.sites, bindings(toks[i+1:in], scope)...)
			if info.wraps == maxWrappers {
				return info
			}
			i = in + 1
			info.wraps++
		case isK(t, "with"):
			if info.wraps == maxWrappers {
				return info
			}
			j := stmtEnd(toks, i+1)
			if j < 0 {
				return info
			}
			i = j + 1
			info.wraps++
		default:
			return info
		}
	}
	return info
}

func isKeyword(s string) bool {
	switch s {
	case "let", "in", "rec", "inherit", "if", "then", "else", "with", "assert":
		return true
	}
	return false
}

// letEnd returns the index of the `in` that closes the let opening at i.
func letEnd(toks []token, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		t := toks[j]
		switch {
		case isK(t, "let"):
			depth++
		case isK(t, "in"):
			depth--
			if depth == 0 {
				return j
			}
		case t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "["):
			if k := matching(toks, j); k > 0 {
				j = k
			}
		}
	}
	return -1
}

// stmtEnd returns the index of the `;` ending the statement that starts at i (nested brackets, lets, withs and
// asserts skipped). A `with e;` or `assert c;` owns the `;` that follows its head, at the let level it opens in.
func stmtEnd(toks []token, i int) int {
	pend := []int{0} // one counter per open let
	for j := i; j < len(toks); j++ {
		t := toks[j]
		switch {
		case t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "["):
			if k := matching(toks, j); k > 0 {
				j = k
			} else {
				return -1
			}
		case isK(t, "let"):
			pend = append(pend, 0)
		case isK(t, "in"):
			if len(pend) > 1 {
				pend = pend[:len(pend)-1]
			}
		case isK(t, "with") || isK(t, "assert"):
			pend[len(pend)-1]++
		case isP(t, ";"):
			if pend[len(pend)-1] > 0 {
				pend[len(pend)-1]--
				continue
			}
			if len(pend) > 1 {
				continue
			}
			return j
		}
	}
	return -1
}

// formals reads the names of a `{ a, b ? 1, ... }` formals list (the tokens between the braces).
func formals(toks []token) []param {
	var out []param
	expect := true
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "["):
			if k := matching(toks, i); k > 0 {
				i = k
			}
		case isP(t, ","):
			expect = true
		case t.kind == tIdent && expect && !isKeyword(t.text):
			out = append(out, param{t.text, t.line})
			expect = false
		default:
			expect = false
		}
	}
	return out
}

// bindings reads the bindings of a let or attribute set (the tokens between its delimiters) at depth one.
func bindings(toks []token, scope string) []site {
	var out []site
	for i := 0; i < len(toks); {
		t := toks[i]
		if isK(t, "inherit") {
			j := stmtEnd(toks, i)
			if j < 0 {
				return out
			}
			i = j + 1
			continue
		}
		// attrpath = value ;
		s := site{line: t.line, scope: scope}
		j := i
		first := true
		for j < len(toks) {
			a := toks[j]
			switch {
			case a.kind == tIdent && !isKeyword(a.text):
				if first {
					s.name = a.text
				}
			case a.kind == tString:
				if first {
					s.dyn = true
				}
			case a.kind == tInterp:
				if first {
					s.dyn = true
				}
			default:
				return out
			}
			first = false
			s.pathLen++
			j++
			if j < len(toks) && isP(toks[j], ".") {
				j++
				continue
			}
			break
		}
		if j >= len(toks) || !isP(toks[j], "=") {
			return out
		}
		e := stmtEnd(toks, j+1)
		if e < 0 {
			return out
		}
		s.end = toks[e].line
		if !s.dyn && s.name != "" {
			out = append(out, s)
		}
		i = e + 1
	}
	return out
}
