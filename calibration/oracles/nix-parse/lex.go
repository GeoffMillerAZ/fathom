package main

// lex.go -- a lexer for Nix source text, written for this oracle (never fathom's tree-sitter path). It
// yields the tokens the root-expression reader (src.go) needs, each with its 1-based line: identifiers and
// keywords, strings (an interpolation inside one is skipped, not entered), paths, punctuation. Comments are
// dropped. A string or path that holds an interpolation is flagged, because it names nothing statically.

import "strings"

type tokKind int

const (
	tIdent tokKind = iota
	tString
	tPath
	tNum
	tPunct
	tInterp // ${ ... } used as an attribute name
)

type token struct {
	kind   tokKind
	text   string // identifier, punctuation or path text; a string's unquoted body (best effort)
	line   int
	interp bool // a string or path holding ${...}
	indent bool // an indented ('') string
}

func isIdentStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIdentChar(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9' || c == '\'' || c == '-'
}

func isPathChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("._-+/~", c) >= 0
}

// lex returns the tokens of src. It never fails: unknown bytes are skipped.
func lex(src []byte) []token {
	var toks []token
	line := 1
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"':
			t, j, dl := lexString(src, i, line)
			toks = append(toks, t)
			line += dl
			i = j
		case c == '\'' && i+1 < n && src[i+1] == '\'':
			t, j, dl := lexIndented(src, i, line)
			toks = append(toks, t)
			line += dl
			i = j
		case c == '$' && i+1 < n && src[i+1] == '{':
			j, dl := skipInterp(src, i+2)
			toks = append(toks, token{kind: tInterp, text: "${}", line: line})
			line += dl
			i = j
		case isIdentStart(c):
			j := i
			for j < n && isIdentChar(src[j]) {
				j++
			}
			// a path or a URI that starts with an identifier: a/b, https://x
			if j < n && src[j] == '/' && j+1 < n && src[j+1] != '/' && src[j+1] != '*' && isPathChar(src[j+1]) && spaceBefore(src, i) {
				k := j
				for k < n && isPathChar(src[k]) {
					k++
				}
				toks = append(toks, token{kind: tPath, text: string(src[i:k]), line: line})
				i = k
				continue
			}
			toks = append(toks, token{kind: tIdent, text: string(src[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (src[j] >= '0' && src[j] <= '9' || src[j] == '.' && j+1 < n && src[j+1] >= '0' && src[j+1] <= '9' || src[j] == 'e') {
				j++
			}
			toks = append(toks, token{kind: tNum, text: string(src[i:j]), line: line})
			i = j
		case c == '.' && i+1 < n && (src[i+1] == '/' || src[i+1] == '.' && i+2 < n && src[i+2] == '/'):
			t, j, dl := lexPath(src, i, line)
			toks = append(toks, t)
			line += dl
			i = j
		case c == '~' && i+1 < n && src[i+1] == '/':
			t, j, dl := lexPath(src, i, line)
			toks = append(toks, t)
			line += dl
			i = j
		case c == '/' && i+1 < n && src[i+1] != '/' && src[i+1] != '*' && src[i+1] != ' ' && src[i+1] != '\n' && src[i+1] != '=' && isPathChar(src[i+1]):
			t, j, dl := lexPath(src, i, line)
			toks = append(toks, t)
			line += dl
			i = j
		case c == '<' && i+1 < n && isIdentStart(src[i+1]):
			j := i + 1
			for j < n && (isPathChar(src[j]) || src[j] == '/') {
				j++
			}
			if j < n && src[j] == '>' {
				toks = append(toks, token{kind: tPath, text: string(src[i : j+1]), line: line})
				i = j + 1
				continue
			}
			toks = append(toks, token{kind: tPunct, text: "<", line: line})
			i++
		default:
			if p := punct(src[i:]); p != "" {
				toks = append(toks, token{kind: tPunct, text: p, line: line})
				i += len(p)
			} else {
				i++
			}
		}
	}
	return toks
}

// spaceBefore reports whether the byte before i is not an identifier or path character (so a/b is a path
// only when it starts a token).
func spaceBefore(src []byte, i int) bool {
	if i == 0 {
		return true
	}
	c := src[i-1]
	return !(isIdentChar(c) || c == '.' || c == '/')
}

func punct(b []byte) string {
	for _, p := range []string{"...", "//", "++", "&&", "||", "->", "==", "!=", "<=", ">=", "|>", "<|"} {
		if strings.HasPrefix(string(b[:min(len(b), len(p))]), p) {
			return p
		}
	}
	if strings.IndexByte("{}()[];:,.@=?!+-*/<>&|", b[0]) >= 0 {
		return string(b[:1])
	}
	return ""
}

// lexPath scans a path literal from i; an interpolation inside it flags the token.
func lexPath(src []byte, i, line int) (token, int, int) {
	n := len(src)
	j := i
	t := token{kind: tPath, line: line}
	dl := 0
	for j < n {
		if src[j] == '$' && j+1 < n && src[j+1] == '{' {
			k, d := skipInterp(src, j+2)
			t.interp = true
			dl += d
			j = k
			continue
		}
		if !isPathChar(src[j]) {
			break
		}
		j++
	}
	t.text = string(src[i:j])
	return t, j, dl
}

// skipInterp returns the index after the } that closes an interpolation whose body starts at i, and the
// newlines skipped. Strings and comments inside it are honoured.
func skipInterp(src []byte, i int) (int, int) {
	depth := 1
	dl := 0
	n := len(src)
	for i < n && depth > 0 {
		switch c := src[i]; {
		case c == '\n':
			dl++
			i++
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			i++
		case c == '"':
			_, j, d := lexString(src, i, 0)
			dl += d
			i = j
		case c == '\'' && i+1 < n && src[i+1] == '\'':
			_, j, d := lexIndented(src, i, 0)
			dl += d
			i = j
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		default:
			i++
		}
	}
	return i, dl
}

func lexString(src []byte, i, line int) (token, int, int) {
	n := len(src)
	t := token{kind: tString, line: line}
	j := i + 1
	dl := 0
	var b strings.Builder
	for j < n && src[j] != '"' {
		switch {
		case src[j] == '\\' && j+1 < n:
			if src[j+1] == '\n' {
				dl++
			}
			b.WriteByte(src[j+1])
			j += 2
		case src[j] == '$' && j+1 < n && src[j+1] == '{':
			k, d := skipInterp(src, j+2)
			t.interp = true
			dl += d
			j = k
		default:
			if src[j] == '\n' {
				dl++
			}
			b.WriteByte(src[j])
			j++
		}
	}
	t.text = b.String()
	return t, min(j+1, n), dl
}

func lexIndented(src []byte, i, line int) (token, int, int) {
	n := len(src)
	t := token{kind: tString, line: line, indent: true}
	j := i + 2
	dl := 0
	var b strings.Builder
	for j < n {
		switch {
		case src[j] == '\'' && j+1 < n && src[j+1] == '\'':
			if j+2 < n && src[j+2] == '\'' { // ''' is an escaped ''
				b.WriteString("''")
				j += 3
				continue
			}
			if j+2 < n && src[j+2] == '$' { // ''$ is a literal $
				b.WriteByte('$')
				j += 3
				continue
			}
			if j+2 < n && src[j+2] == '\\' && j+3 < n { // ''\x is an escape
				b.WriteByte(src[j+3])
				j += 4
				continue
			}
			t.text = b.String()
			return t, j + 2, dl
		case src[j] == '$' && j+1 < n && src[j+1] == '{':
			k, d := skipInterp(src, j+2)
			t.interp = true
			dl += d
			j = k
		default:
			if src[j] == '\n' {
				dl++
			}
			b.WriteByte(src[j])
			j++
		}
	}
	t.text = b.String()
	return t, n, dl
}
