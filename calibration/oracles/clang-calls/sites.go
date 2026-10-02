package main

import "strings"

func isIdent(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// notNames are words a name position can hold that name no callee.
var notNames = map[string]bool{"operator": true, "template": true, "typename": true, "decltype": true, "sizeof": true}

// calledName finds the callee written in src[from:to], an expression ending in
// a name (`f`, `ns::f`, `obj.m`, `p->template m<T>`): the last identifier
// outside a trailing template argument list. ok only when the expression is
// written as a call (`(` follows it).
func calledName(src []byte, from, to int) (string, int, bool) {
	end := to
	for end > from && isSpace(src[end-1]) {
		end--
	}
	if end > from && src[end-1] == '>' {
		depth := 0
		i := end - 1
		for ; i >= from; i-- {
			switch src[i] {
			case '>':
				depth++
			case '<':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if i < from {
			return "", 0, false
		}
		end = i
		for end > from && isSpace(src[end-1]) {
			end--
		}
	}
	start := end
	for start > from && isIdent(src[start-1]) {
		start--
	}
	name := string(src[start:end])
	if name == "" || notNames[name] || (name[0] >= '0' && name[0] <= '9') || (start > from && src[start-1] == '~') {
		return "", 0, false
	}
	k := to
	for k < len(src) && isSpace(src[k]) {
		k++
	}
	if k >= len(src) || src[k] != '(' {
		return "", 0, false
	}
	return name, start, true
}

// constructedType finds the type written at src[from:] as `T(`, `ns::T<A>(` or
// `T{` inside the node's span (to): the last identifier of the type path
// outside template argument lists.
func constructedType(src []byte, from, to int) (string, int, bool) {
	i := from
	if i+1 < len(src) && src[i] == ':' && src[i+1] == ':' {
		i += 2
	}
	name, at := "", 0
	for {
		s := i
		for i < len(src) && isIdent(src[i]) {
			i++
		}
		if i == s {
			return "", 0, false
		}
		name, at = string(src[s:i]), s
		for i < len(src) && isSpace(src[i]) {
			i++
		}
		if i < len(src) && src[i] == '<' {
			depth := 0
			for ; i < len(src); i++ {
				if src[i] == '<' {
					depth++
				} else if src[i] == '>' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
			}
			for i < len(src) && isSpace(src[i]) {
				i++
			}
		}
		if i+1 < len(src) && src[i] == ':' && src[i+1] == ':' {
			i += 2
			continue
		}
		break
	}
	if i >= len(src) || i >= to || (src[i] != '(' && src[i] != '{') || notNames[name] || (name[0] >= '0' && name[0] <= '9') {
		return "", 0, false
	}
	return name, at, true
}

// lastIdent is a type's own name: the last identifier of its spelling outside
// a trailing template argument list ("const fmt::basic_string_view<char>" is
// basic_string_view).
func lastIdent(t string) string {
	t = strings.TrimSpace(t)
	if strings.HasSuffix(t, ">") {
		depth := 0
		for i := len(t) - 1; i >= 0; i-- {
			switch t[i] {
			case '>':
				depth++
			case '<':
				depth--
			}
			if depth == 0 {
				t = strings.TrimSpace(t[:i])
				break
			}
		}
	}
	end := len(t)
	start := end
	for start > 0 && isIdent(t[start-1]) {
		start--
	}
	return t[start:end]
}
