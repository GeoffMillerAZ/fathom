// Package pysyntax is a small Python tokenizer: indentation, string
// prefixes, f-string fields and implicit joining inside brackets, as far as
// a reader of declarations needs. It never imports or runs anything, and it
// reports the places where its tokens may not be what Python reads.
package pysyntax

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
)

// Kind is a token's kind.
type Kind uint8

const (
	EOF Kind = iota
	Name
	Number
	String
	Op
	Newline
	Indent
	Dedent
)

// StrPart is one piece of a string literal: literal text in Lit, or an
// f-string replacement field (Dyn, its expression text in Expr).
type StrPart struct {
	Lit  string
	Dyn  bool
	Expr string
}

// Token is one token. Only a String token has Parts, and it has no Text.
type Token struct {
	Kind  Kind
	Text  string
	Parts []StrPart
	Line  int
}

// IssueKind names why a source may not tokenize the way Python reads it.
type IssueKind uint8

const (
	UnterminatedString IssueKind = iota + 1
	NestedFStringQuote
	OpenBracket
	NULByte
)

func (k IssueKind) String() string {
	switch k {
	case UnterminatedString:
		return "unterminated string"
	case NestedFStringQuote:
		return "nested f-string reuses its quote"
	case OpenBracket:
		return "bracket still open at end of file"
	case NULByte:
		return "NUL byte"
	}
	return ""
}

// Issue is one incompleteness at a 1-based line.
type Issue struct {
	Line int
	Kind IssueKind
}

var pyOps = []string{
	"**=", "//=", ">>=", "<<=", "...", "->", ":=", "**", "//", "<<", ">>", "<=", ">=", "==", "!=",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "@=",
}

func isPyIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isPyIdentChar(c byte) bool { return isPyIdentStart(c) || (c >= '0' && c <= '9') }

// Lex tokenizes src the way Python's tokenizer does, as far as a parser of
// declarations needs: NEWLINE ends a logical line outside brackets, INDENT
// and DEDENT bracket a suite, comments and blank lines are dropped. The
// issues name, by line, every place the tokens may not mean what Python
// reads; the tokens are the same either way.
func Lex(src []byte) ([]Token, []Issue) {
	var toks []Token
	var issues []Issue
	for k, l, last := 0, 1, 0; k < len(src); k++ {
		switch {
		case src[k] == '\n':
			l++
		case src[k] == 0 && l != last:
			issues = append(issues, Issue{Line: l, Kind: NULByte})
			last = l
		}
	}
	indents := []int{0}
	line := 1
	depth := 0
	openLine := 0
	i, n := 0, len(src)
	atLineStart := true
	for i < n {
		if atLineStart && depth == 0 {
			col := 0
			j := i
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\f') {
				if src[j] == '\t' {
					col = (col/8 + 1) * 8
				} else {
					col++
				}
				j++
			}
			if j >= n {
				break
			}
			if src[j] == '\n' || src[j] == '\r' || src[j] == '#' {
				for j < n && src[j] != '\n' {
					j++
				}
				if j < n {
					line++
					j++
				}
				i = j
				continue
			}
			if src[j] == '\\' && j+1 < n && (src[j+1] == '\n' || src[j+1] == '\r') {
				i = j
				atLineStart = false
				continue
			}
			if col > indents[len(indents)-1] {
				indents = append(indents, col)
				toks = append(toks, Token{Kind: Indent, Line: line})
			}
			for col < indents[len(indents)-1] {
				indents = indents[:len(indents)-1]
				toks = append(toks, Token{Kind: Dedent, Line: line})
			}
			i = j
			atLineStart = false
		}
		c := src[i]
		switch {
		case c == '\n':
			if depth == 0 {
				toks = append(toks, Token{Kind: Newline, Line: line})
				atLineStart = true
			}
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '\\' && i+1 < n && (src[i+1] == '\n' || (src[i+1] == '\r' && i+2 < n && src[i+2] == '\n')):
			if src[i+1] == '\r' {
				i++
			}
			i += 2
			line++
		case isPyIdentStart(c):
			j := i
			for j < n && isPyIdentChar(src[j]) {
				j++
			}
			word := string(src[i:j])
			if j < n && (src[j] == '\'' || src[j] == '"') && len(word) <= 2 && strings.Trim(strings.ToLower(word), "rbuf") == "" {
				parts, ni, nl, issue := lexString(src, j, strings.ToLower(word))
				if issue != 0 {
					issues = append(issues, Issue{Line: line, Kind: issue})
				}
				toks = append(toks, Token{Kind: String, Parts: parts, Line: line})
				line += nl
				i = ni
				continue
			}
			toks = append(toks, Token{Kind: Name, Text: word, Line: line})
			i = j
		case c == '\'' || c == '"':
			parts, ni, nl, issue := lexString(src, i, "")
			if issue != 0 {
				issues = append(issues, Issue{Line: line, Kind: issue})
			}
			toks = append(toks, Token{Kind: String, Parts: parts, Line: line})
			line += nl
			i = ni
		case c >= '0' && c <= '9' || (c == '.' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9'):
			j := i + 1
			for j < n && (isPyIdentChar(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, Token{Kind: Number, Text: string(src[i:j]), Line: line})
			i = j
		default:
			op := string(c)
			for _, o := range pyOps {
				if bytes.HasPrefix(src[i:], []byte(o)) {
					op = o
					break
				}
			}
			switch op {
			case "(", "[", "{":
				if depth == 0 {
					openLine = line
				}
				depth++
			case ")", "]", "}":
				if depth > 0 {
					depth--
				}
			}
			toks = append(toks, Token{Kind: Op, Text: op, Line: line})
			i += len(op)
		}
	}
	if depth > 0 {
		issues = append(issues, Issue{Line: openLine, Kind: OpenBracket})
	}
	if len(toks) > 0 && toks[len(toks)-1].Kind != Newline {
		toks = append(toks, Token{Kind: Newline, Line: line})
	}
	for len(indents) > 1 {
		indents = indents[:len(indents)-1]
		toks = append(toks, Token{Kind: Dedent, Line: line})
	}
	slices.SortStableFunc(issues, func(a, b Issue) int { return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Kind, b.Kind)) })
	return append(toks, Token{Kind: EOF, Line: line}), issues
}

// lexString reads a string literal at i (its quote) with the given prefix
// letters, returning its parts, the index after it, the number of newlines
// it spans, and the issue it raises (0 for none).
func lexString(src []byte, i int, prefix string) ([]StrPart, int, int, IssueKind) {
	raw := strings.Contains(prefix, "r")
	fstr := strings.Contains(prefix, "f")
	q := src[i]
	triple := i+2 < len(src) && src[i+1] == q && src[i+2] == q
	if triple {
		i += 3
	} else {
		i++
	}
	nl := 0
	closed := false
	var body []byte
	n := len(src)
	for i < n {
		c := src[i]
		if c == '\\' && i+1 < n {
			body = append(body, c, src[i+1])
			if src[i+1] == '\n' {
				nl++
			}
			i += 2
			continue
		}
		if c == q {
			if !triple {
				i++
				closed = true
				break
			}
			if i+2 < n && src[i+1] == q && src[i+2] == q {
				i += 3
				closed = true
				break
			}
		}
		if c == '\n' {
			if !triple {
				break
			}
			nl++
		}
		body = append(body, c)
		i++
	}
	decode := func(s string) string {
		if raw || !strings.Contains(s, "\\") {
			return s
		}
		var b strings.Builder
		for k := 0; k < len(s); k++ {
			if s[k] != '\\' || k+1 >= len(s) {
				b.WriteByte(s[k])
				continue
			}
			k++
			switch s[k] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\', '\'', '"':
				b.WriteByte(s[k])
			case '\n':
			default:
				b.WriteByte('\\')
				b.WriteByte(s[k])
			}
		}
		return b.String()
	}
	// A backslash-CRLF inside a one-line string ends it here though Python
	// continues it: a conservative issue, since fixing it changes tokens.
	var issue IssueKind
	switch {
	case !closed:
		issue = UnterminatedString
	case fstr && fieldOpenAtEnd(body):
		issue = NestedFStringQuote
	}
	if !fstr {
		return []StrPart{{Lit: decode(string(body))}}, i, nl, issue
	}
	var parts []StrPart
	var lit strings.Builder
	for k := 0; k < len(body); k++ {
		c := body[k]
		if c == '{' && k+1 < len(body) && body[k+1] == '{' {
			lit.WriteByte('{')
			k++
			continue
		}
		if c == '}' && k+1 < len(body) && body[k+1] == '}' {
			lit.WriteByte('}')
			k++
			continue
		}
		if c == '{' {
			d := 0
			j := k
			for j < len(body) {
				if body[j] == '{' {
					d++
				} else if body[j] == '}' {
					d--
					if d == 0 {
						break
					}
				}
				j++
			}
			if lit.Len() > 0 {
				parts = append(parts, StrPart{Lit: decode(lit.String())})
				lit.Reset()
			}
			expr := string(body[k+1 : min(j, len(body))])
			if cut := strings.IndexAny(expr, "!:="); cut > 0 && !strings.Contains(expr[:cut], "[") {
				expr = expr[:cut]
			}
			parts = append(parts, StrPart{Dyn: true, Expr: strings.TrimSpace(expr)})
			k = j
			continue
		}
		lit.WriteByte(c)
	}
	if lit.Len() > 0 {
		parts = append(parts, StrPart{Lit: decode(lit.String())})
	}
	return parts, i, nl, issue
}

// fieldOpenAtEnd reports whether an f-string body ends inside a replacement
// field, reading quotes inside fields (which the parts above do not): a
// field still open where the string ended means the string closed early,
// because a nested string reused this one's quote.
func fieldOpenAtEnd(body []byte) bool {
	depth := 0
	var quote byte
	for k := 0; k < len(body); k++ {
		c := body[k]
		switch {
		case quote != 0:
			if c == '\\' {
				k++
			} else if c == quote {
				quote = 0
			}
		case depth == 0 && (c == '{' || c == '}') && k+1 < len(body) && body[k+1] == c:
			k++
		case c == '{':
			depth++
		case c == '}' && depth > 0:
			depth--
		case depth > 0 && (c == '\'' || c == '"'):
			quote = c
		}
	}
	return depth > 0 || quote != 0
}
