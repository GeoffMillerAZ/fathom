package fusion

// cdecls.go — the declaration scan behind the header-prototype binding
// (cproto.go, LB-Q-c-rest, O281): the function names a C/C++ header declares as
// prototypes. A prototype is no symbol — the dialects' queries capture a
// definition with a body, never a declaration — so the resolver reads the
// header's text. It is a token scan, not a parse: comments, string and
// character literals and preprocessor lines are dropped, a brace body is
// skipped whole, and each remaining file-scope statement that reads
// `<type tokens> name ( … ) …;` declares name. `extern "C" {` and a namespace
// block are transparent; a typedef, a function-pointer variable, a struct
// member and a macro invocation declare nothing.

import "bytes"

// scanDeclaredFunctions is the set of function names src declares as
// prototypes at file scope.
func scanDeclaredFunctions(src []byte) map[string]bool {
	toks := lexDecls(src)
	out := map[string]bool{}
	var stmt []string
	var stack []bool // open transparent braces
	skip := 0        // depth inside a skipped (non-transparent) brace body
	for _, t := range toks {
		if skip > 0 {
			switch t {
			case "{":
				skip++
			case "}":
				skip--
				if skip == 0 {
					stmt = nil
				}
			}
			continue
		}
		switch t {
		case "{":
			if transparentOpener(stmt) {
				stack = append(stack, true)
				stmt = nil
			} else {
				skip = 1
			}
		case "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			stmt = nil
		case ";":
			if name := declaredFunction(stmt); name != "" {
				out[name] = true
			}
			stmt = nil
		default:
			stmt = append(stmt, t)
		}
	}
	return out
}

// transparentOpener reports whether the statement so far opens a block whose
// declarations are file-scope ones: extern "C" { or a namespace.
func transparentOpener(stmt []string) bool {
	if len(stmt) == 2 && stmt[0] == "extern" && stmt[1] == `""` {
		return true
	}
	for i := 0; i < len(stmt) && i < 2; i++ {
		if stmt[i] == "namespace" {
			return true
		}
	}
	return false
}

// declKeywords are words that are never the name of a declared function.
var declKeywords = map[string]bool{
	"if": true, "while": true, "for": true, "switch": true, "return": true, "sizeof": true,
	"int": true, "char": true, "short": true, "long": true, "float": true, "double": true,
	"void": true, "signed": true, "unsigned": true, "struct": true, "union": true, "enum": true,
	"typedef": true, "static_assert": true, "_Static_assert": true, "defined": true,
	"_Alignof": true, "alignof": true, "typeof": true, "__typeof__": true, "decltype": true,
	"noexcept": true, "throw": true, "catch": true, "operator": true, "using": true,
	"bool": true, "_Bool": true, "auto": true, "const": true, "volatile": true,
}

// attrWords introduce a parenthesised attribute that is dropped before a
// statement is read.
var attrWords = map[string]bool{
	"__attribute__": true, "__declspec": true, "_Alignas": true, "alignas": true,
	"__asm__": true, "__asm": true, "asm": true, "__extension__": false,
}

// declaredFunction is the function name stmt declares, "" when it declares
// none.
func declaredFunction(stmt []string) string {
	var toks []string
	for i := 0; i < len(stmt); i++ {
		if attrWords[stmt[i]] && i+1 < len(stmt) && stmt[i+1] == "(" {
			depth := 0
			j := i + 1
			for ; j < len(stmt); j++ {
				if stmt[j] == "(" {
					depth++
				} else if stmt[j] == ")" {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			i = j
			continue
		}
		toks = append(toks, stmt[i])
	}
	for len(toks) >= 2 && toks[0] == "extern" && toks[1] == `""` {
		toks = toks[2:]
	}
	if len(toks) == 0 || toks[0] == "typedef" || toks[0] == "using" {
		return ""
	}
	open := -1
	for i, t := range toks {
		if t == "=" {
			return "" // a variable's initializer
		}
		if t == "operator" {
			return ""
		}
		if t == "(" {
			open = i
			break
		}
	}
	if open < 2 {
		return "" // no type before the name: a function-pointer variable or a macro invocation
	}
	name := toks[open-1]
	if !isIdentToken(name) || declKeywords[name] || toks[open-2] == ":" {
		return ""
	}
	return name
}

func isIdentToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

// lexDecls is src's tokens: identifiers, a literal as `""`, every other byte
// as itself; comments and preprocessor lines are dropped.
func lexDecls(src []byte) []string {
	var toks []string
	n := len(src)
	lineStart := true
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			lineStart = true
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < n && src[i+1] == '*':
			j := bytes.Index(src[i+2:], []byte("*/"))
			if j < 0 {
				return toks
			}
			i += j + 4
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '#' && lineStart:
			for i < n {
				if src[i] == '\\' && i+1 < n && (src[i+1] == '\n' || src[i+1] == '\r') {
					i += 2
					if i < n && src[i-1] == '\r' && src[i] == '\n' {
						i++
					}
					continue
				}
				if src[i] == '\n' {
					break
				}
				i++
			}
		case c == '"' || c == '\'':
			lineStart = false
			q := c
			i++
			for i < n && src[i] != q {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
			toks = append(toks, `""`)
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			lineStart = false
			j := i + 1
			for j < n && (src[j] == '_' || src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z' || src[j] >= '0' && src[j] <= '9') {
				j++
			}
			toks = append(toks, string(src[i:j]))
			i = j
		case c >= '0' && c <= '9':
			lineStart = false
			j := i + 1
			for j < n && (src[j] == '_' || src[j] == '.' || src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z' || src[j] >= '0' && src[j] <= '9') {
				j++
			}
			toks = append(toks, "0")
			i = j
		default:
			lineStart = false
			toks = append(toks, string(c))
			i++
		}
	}
	return toks
}
