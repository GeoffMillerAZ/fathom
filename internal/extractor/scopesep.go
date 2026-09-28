package extractor

import "strings"

// nameNormalizer is the manifest's qualify.scope_sep rule as a function
// (nil when the dialect declares no scope operator, which is every dialect
// but C++): applied to a definition's @name text and to a call's callee text
// so a scope path spelled with the language's own operator lands on the
// graph's one separator (containerSep).
func (r *tsDialectRules) nameNormalizer() func(string) string {
	if r.scopeSep == "" {
		return nil
	}
	sep, to := r.scopeSep, r.containerSep
	if to == "" {
		to = "."
	}
	return func(s string) string { return normalizeScopedName(s, sep, to) }
}

// normalizeScopedName rewrites a scope-qualified name (C++'s
// "ns::Box<T>::make") to one spelling per symbol: the scope operator becomes
// to, whitespace around it goes, a leading (global-scope) operator is
// dropped and template argument lists — part of a type's spelling, not of the
// symbol's key — are removed. Everything from an `operator` keyword on
// (operator+, operator<<, operator(), operator new[], a conversion operator)
// is the function's own name and keeps its spelling, whitespace runs
// collapsed. A name with unbalanced angle brackets is not guessed at: they
// are left in place.
func normalizeScopedName(name, sep, to string) string {
	if !strings.Contains(name, sep) && !strings.Contains(name, "<") {
		return name
	}
	head, tail := name, ""
	if i := operatorKeyword(name); i >= 0 {
		head, tail = name[:i], strings.Join(strings.Fields(name[i:]), " ")
	}
	head = stripTemplateArgs(head)
	head = strings.Join(strings.Fields(head), "")
	head = strings.ReplaceAll(head, sep, to)
	head = strings.TrimPrefix(head, to)
	return head + tail
}

// operatorKeyword is the byte index of an `operator` keyword in name (at
// identifier boundaries, so my_operator does not count), or -1.
func operatorKeyword(name string) int {
	const kw = "operator"
	for from := 0; from < len(name); {
		i := strings.Index(name[from:], kw)
		if i < 0 {
			return -1
		}
		i += from
		before := i == 0 || !isIdentByte(name[i-1])
		after := i+len(kw) >= len(name) || !isIdentByte(name[i+len(kw)])
		if before && after {
			return i
		}
		from = i + len(kw)
	}
	return -1
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// stripTemplateArgs removes every balanced <...> group (a '>' inside
// parentheses, as in Foo<(1 > 0)>, does not close one). Unbalanced input is
// returned unchanged.
func stripTemplateArgs(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	depth, paren := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '<' && paren == 0:
			depth++
		case c == '>' && paren == 0 && depth > 0:
			depth--
		case c == '(' && depth > 0:
			paren++
		case c == ')' && depth > 0 && paren > 0:
			paren--
		case depth == 0:
			b.WriteByte(c)
		}
	}
	if depth != 0 || paren != 0 {
		return s
	}
	return b.String()
}
