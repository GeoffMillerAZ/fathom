package logtemplate

// lex.go — the little lexing the reader needs, per language: where strings
// and comments are (so a call inside either is never read), a call's
// top-level arguments, and a string literal's decoded text with its
// interpolation holes (a Python f-string's {x}, a JavaScript template's
// ${x}, PHP's "$x" and "{$x}"). It is not a parser: an expression it cannot
// read yields no template, never a guessed one.

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// lang is a reader language.
type lang int

const (
	langPython lang = iota
	langJava
	langNode
	langPHP
)

// seg is one decoded part of a string literal: literal text, or a hole
// spelled as the source writes it.
type seg struct {
	text string
	hole bool
}

// isIdent reports whether b can continue an identifier.
func isIdent(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// stringStart reports whether a string literal starts at i, and where its
// opening quote is (after a Python prefix).
func stringStart(l lang, src string, i int) (quote int, ok bool) {
	if i >= len(src) {
		return 0, false
	}
	c := src[i]
	switch c {
	case '"', '\'':
		return i, true
	case '`':
		return i, l == langNode
	}
	if l != langPython || i > 0 && isIdent(src[i-1]) {
		return 0, false
	}
	j := i
	for j < len(src) && j-i < 2 && strings.IndexByte("rRuUbBfF", src[j]) >= 0 {
		j++
	}
	if j > i && j < len(src) && (src[j] == '"' || src[j] == '\'') {
		return j, true
	}
	return 0, false
}

// commentEnd returns the end of a comment starting at i, if one does.
func commentEnd(l lang, src string, i int) (int, bool) {
	if i >= len(src) {
		return 0, false
	}
	line := func(from int) int {
		if k := strings.IndexByte(src[from:], '\n'); k >= 0 {
			return from + k
		}
		return len(src)
	}
	switch {
	case l == langPython && src[i] == '#':
		return line(i), true
	case l == langPHP && src[i] == '#' && !(i+1 < len(src) && src[i+1] == '['):
		return line(i), true
	case l != langPython && strings.HasPrefix(src[i:], "//"):
		return line(i), true
	case l != langPython && strings.HasPrefix(src[i:], "/*"):
		if k := strings.Index(src[i+2:], "*/"); k >= 0 {
			return i + 2 + k + 2, true
		}
		return len(src), true
	}
	return 0, false
}

// scanString reads the string literal starting at i (a Python prefix
// included) and returns its end and decoded segments. ok is false for an
// unterminated literal.
func scanString(l lang, src string, i int) (end int, segs []seg, ok bool) {
	q, isStr := stringStart(l, src, i)
	if !isStr {
		return 0, nil, false
	}
	switch l {
	case langPython:
		return scanPython(src, i, q)
	case langJava:
		if strings.HasPrefix(src[q:], `"""`) {
			return scanJavaTextBlock(src, q)
		}
		return scanCLike(src, q, langJava)
	case langNode:
		if src[q] == '`' {
			return scanTemplateLiteral(src, q)
		}
		return scanCLike(src, q, langNode)
	default:
		if src[q] == '\'' {
			return scanPHPSingle(src, q)
		}
		return scanPHPDouble(src, q)
	}
}

// builder accumulates segments, merging adjacent literal text.
type builder struct {
	segs []seg
	lit  strings.Builder
}

func (b *builder) text(s string) { b.lit.WriteString(s) }
func (b *builder) byteText(c byte) {
	b.lit.WriteByte(c)
}
func (b *builder) hole(spell string) {
	b.flush()
	b.segs = append(b.segs, seg{text: spell, hole: true})
}
func (b *builder) flush() {
	if b.lit.Len() > 0 {
		b.segs = append(b.segs, seg{text: b.lit.String()})
		b.lit.Reset()
	}
}
func (b *builder) done() []seg {
	b.flush()
	return b.segs
}

// decodeEscape decodes the escape at src[i] == '\\' for a C-like or Python
// string, returning the decoded text and the index after it.
func decodeEscape(src string, i int, l lang) (string, int) {
	if i+1 >= len(src) {
		return `\`, i + 1
	}
	c := src[i+1]
	switch c {
	case 'n':
		return "\n", i + 2
	case 't':
		return "\t", i + 2
	case 'r':
		return "\r", i + 2
	case '\\', '\'', '"', '`':
		return string(c), i + 2
	case '$':
		if l == langNode || l == langPHP {
			return "$", i + 2
		}
	case '0':
		if l != langPHP && (i+2 >= len(src) || src[i+2] < '0' || src[i+2] > '7') {
			return "\x00", i + 2
		}
	case 'f':
		return "\f", i + 2
	case 'b':
		if l != langPHP {
			return "\b", i + 2
		}
	case 'v':
		return "\v", i + 2
	case 's':
		if l == langJava {
			return " ", i + 2
		}
	case '\n':
		if l != langJava {
			return "", i + 2 // a line continuation
		}
	case 'x':
		if i+3 < len(src) {
			if v, err := strconv.ParseUint(src[i+2:i+4], 16, 8); err == nil {
				return string(rune(v)), i + 4
			}
		}
	case 'u':
		if i+2 < len(src) && src[i+2] == '{' {
			if k := strings.IndexByte(src[i+2:], '}'); k > 0 {
				if v, err := strconv.ParseUint(src[i+3:i+2+k], 16, 32); err == nil && utf8.ValidRune(rune(v)) {
					return string(rune(v)), i + 3 + k
				}
			}
		}
		if i+5 < len(src) {
			if v, err := strconv.ParseUint(src[i+2:i+6], 16, 32); err == nil {
				return string(rune(v)), i + 6
			}
		}
	}
	return src[i : i+2], i + 2 // kept as written
}

// scanCLike reads a Java or JavaScript '...' or "..." literal: escapes
// decoded, terminated by its quote or (unterminated) a newline.
func scanCLike(src string, q int, l lang) (int, []seg, bool) {
	quote := src[q]
	var b builder
	for i := q + 1; i < len(src); {
		c := src[i]
		switch {
		case c == quote:
			return i + 1, b.done(), true
		case c == '\n':
			return i, nil, false
		case c == '\\':
			s, n := decodeEscape(src, i, l)
			b.text(s)
			i = n
			continue
		default:
			b.byteText(c)
		}
		i++
	}
	return len(src), nil, false
}

// scanJavaTextBlock reads a Java text block: the content after the opening
// line, with the lines' common indentation removed.
func scanJavaTextBlock(src string, q int) (int, []seg, bool) {
	start := q + 3
	nl := strings.IndexByte(src[start:], '\n')
	if nl < 0 {
		return len(src), nil, false
	}
	start += nl + 1
	k := strings.Index(src[start:], `"""`)
	if k < 0 {
		return len(src), nil, false
	}
	body := src[start : start+k]
	lines := strings.Split(body, "\n")
	indent := -1
	for li, ln := range lines {
		if strings.TrimSpace(ln) == "" && li != len(lines)-1 {
			continue
		}
		n := len(ln) - len(strings.TrimLeft(ln, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	var out strings.Builder
	for li, ln := range lines {
		if len(ln) >= indent && indent > 0 {
			ln = ln[indent:]
		}
		out.WriteString(strings.TrimRight(ln, " \t"))
		if li < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	var b builder
	text := out.String()
	for i := 0; i < len(text); {
		if text[i] == '\\' {
			s, n := decodeEscape(text, i, langJava)
			b.text(s)
			i = n
			continue
		}
		b.byteText(text[i])
		i++
	}
	return start + k + 3, b.done(), true
}

// scanTemplateLiteral reads a JavaScript template literal: ${...} holes,
// escapes decoded in the literal text.
func scanTemplateLiteral(src string, q int) (int, []seg, bool) {
	var b builder
	for i := q + 1; i < len(src); {
		c := src[i]
		switch {
		case c == '`':
			return i + 1, b.done(), true
		case c == '\\':
			s, n := decodeEscape(src, i, langNode)
			b.text(s)
			i = n
			continue
		case c == '$' && i+1 < len(src) && src[i+1] == '{':
			end, ok := matchBrace(langNode, src, i+1)
			if !ok {
				return len(src), nil, false
			}
			b.hole(src[i:end])
			i = end
			continue
		default:
			b.byteText(c)
		}
		i++
	}
	return len(src), nil, false
}

// matchBrace returns the index after the '}' matching the '{' at open,
// skipping strings and comments inside.
func matchBrace(l lang, src string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(src); {
		if e, ok := commentEnd(l, src, i); ok && i > open {
			i = e
			continue
		}
		if i > open {
			if _, isStr := stringStart(l, src, i); isStr {
				e, _, ok := scanString(l, src, i)
				if !ok {
					return 0, false
				}
				i = e
				continue
			}
		}
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
		i++
	}
	return 0, false
}

// scanPython reads a Python literal whose prefix starts at i and quote at
// q: raw, f-string and triple-quoted forms.
func scanPython(src string, i, q int) (int, []seg, bool) {
	prefix := strings.ToLower(src[i:q])
	raw := strings.Contains(prefix, "r")
	fstr := strings.Contains(prefix, "f")
	quote := src[q : q+1]
	triple := strings.HasPrefix(src[q:], strings.Repeat(quote, 3))
	closer := quote
	start := q + 1
	if triple {
		closer = strings.Repeat(quote, 3)
		start = q + 3
	}
	var b builder
	for j := start; j < len(src); {
		if strings.HasPrefix(src[j:], closer) {
			return j + len(closer), b.done(), true
		}
		c := src[j]
		switch {
		case c == '\n' && !triple:
			return j, nil, false
		case c == '\\':
			if raw {
				b.text(src[j:min(j+2, len(src))])
				j += 2
				continue
			}
			s, n := decodeEscape(src, j, langPython)
			b.text(s)
			j = n
			continue
		case fstr && c == '{':
			if j+1 < len(src) && src[j+1] == '{' {
				b.byteText('{')
				j += 2
				continue
			}
			end, ok := matchFStringHole(src, j, closer)
			if !ok {
				return len(src), nil, false
			}
			b.hole(src[j:end])
			j = end
			continue
		case fstr && c == '}' && j+1 < len(src) && src[j+1] == '}':
			b.byteText('}')
			j += 2
			continue
		default:
			b.byteText(c)
		}
		j++
	}
	return len(src), nil, false
}

// matchFStringHole returns the index after the '}' closing the f-string
// hole at open; a single-quoted literal's hole never crosses a line.
func matchFStringHole(src string, open int, closer string) (int, bool) {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		case '\n':
			if len(closer) == 1 {
				return 0, false
			}
		}
	}
	return 0, false
}

// scanPHPSingle reads a PHP single-quoted literal: only \\ and \' escape.
func scanPHPSingle(src string, q int) (int, []seg, bool) {
	var b builder
	for i := q + 1; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\'':
			return i + 1, b.done(), true
		case c == '\\' && i+1 < len(src) && (src[i+1] == '\\' || src[i+1] == '\''):
			b.byteText(src[i+1])
			i++
		default:
			b.byteText(c)
		}
	}
	return len(src), nil, false
}

// scanPHPDouble reads a PHP double-quoted literal: escapes decoded, and
// $name (with one ->prop or [key]), {$expr} and ${expr} as holes.
func scanPHPDouble(src string, q int) (int, []seg, bool) {
	var b builder
	for i := q + 1; i < len(src); {
		c := src[i]
		switch {
		case c == '"':
			return i + 1, b.done(), true
		case c == '\\':
			s, n := decodeEscape(src, i, langPHP)
			b.text(s)
			i = n
			continue
		case c == '{' && i+1 < len(src) && src[i+1] == '$':
			end, ok := matchBrace(langPHP, src, i)
			if !ok {
				return len(src), nil, false
			}
			b.hole(src[i:end])
			i = end
			continue
		case c == '$' && i+1 < len(src) && src[i+1] == '{':
			end, ok := matchBrace(langPHP, src, i+1)
			if !ok {
				return len(src), nil, false
			}
			b.hole(src[i:end])
			i = end
			continue
		case c == '$' && i+1 < len(src) && (isIdent(src[i+1]) && src[i+1] != '$' && (src[i+1] < '0' || src[i+1] > '9')):
			j := i + 1
			for j < len(src) && isIdent(src[j]) && src[j] != '$' {
				j++
			}
			switch {
			case strings.HasPrefix(src[j:], "->") && j+2 < len(src) && isIdent(src[j+2]):
				j += 2
				for j < len(src) && isIdent(src[j]) && src[j] != '$' {
					j++
				}
			case j < len(src) && src[j] == '[':
				if k := strings.IndexByte(src[j:], ']'); k > 0 && !strings.ContainsAny(src[j:j+k], "\"\n") {
					j += k + 1
				}
			}
			b.hole(src[i:j])
			i = j
			continue
		default:
			b.byteText(c)
		}
		i++
	}
	return len(src), nil, false
}

// codeMask marks every byte of src that is code: not inside a string
// literal or a comment. A string that cannot be read ends at its line, so
// one odd literal (a JavaScript regular expression holding a quote) hides
// at most its own line.
func codeMask(l lang, src string) []bool {
	mask := make([]bool, len(src))
	for i := 0; i < len(src); {
		if e, ok := commentEnd(l, src, i); ok {
			i = e
			continue
		}
		if l == langPHP && strings.HasPrefix(src[i:], "<<<") {
			if e := heredocEnd(src, i); e > i {
				i = e
				continue
			}
		}
		if _, isStr := stringStart(l, src, i); isStr {
			e, _, ok := scanString(l, src, i)
			if !ok {
				if k := strings.IndexByte(src[i:], '\n'); k >= 0 {
					e = i + k
				} else {
					e = len(src)
				}
			}
			if e <= i {
				e = i + 1
			}
			i = e
			continue
		}
		if l == langJava && src[i] == '\'' {
			i++
			continue
		}
		mask[i] = true
		i++
	}
	return mask
}

// heredocEnd returns the end of a PHP heredoc or nowdoc starting at i.
func heredocEnd(src string, i int) int {
	j := i + 3
	for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
		j++
	}
	quoted := j < len(src) && (src[j] == '\'' || src[j] == '"')
	if quoted {
		j++
	}
	k := j
	for k < len(src) && isIdent(src[k]) && src[k] != '$' {
		k++
	}
	if k == j {
		return i
	}
	id := src[j:k]
	nl := strings.IndexByte(src[k:], '\n')
	if nl < 0 {
		return i
	}
	for p := k + nl + 1; p < len(src); {
		e := strings.IndexByte(src[p:], '\n')
		lineEnd := len(src)
		if e >= 0 {
			lineEnd = p + e
		}
		if t := strings.TrimLeft(src[p:lineEnd], " \t"); strings.HasPrefix(t, id) && (len(t) == len(id) || !isIdent(t[len(id)])) {
			return lineEnd
		}
		if e < 0 {
			break
		}
		p = lineEnd + 1
	}
	return len(src)
}

// argSpan is one argument's [start, end) in src, whitespace trimmed.
type argSpan struct{ start, end int }

// splitArgs reads the call arguments after the '(' at open: the top-level
// comma-separated spans and the index after the closing ')'.
func splitArgs(l lang, src string, open int) ([]argSpan, int, bool) {
	var out []argSpan
	depth := 0
	argStart := open + 1
	add := func(end int) {
		s, e := argStart, end
		for s < e && isSpace(src[s]) {
			s++
		}
		for e > s && isSpace(src[e-1]) {
			e--
		}
		if e > s {
			out = append(out, argSpan{s, e})
		}
	}
	for i := open; i < len(src); {
		if e, ok := commentEnd(l, src, i); ok {
			i = e
			continue
		}
		if _, isStr := stringStart(l, src, i); isStr {
			e, _, ok := scanString(l, src, i)
			if !ok {
				return nil, 0, false
			}
			i = e
			continue
		}
		switch src[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				add(i)
				return out, i + 1, true
			}
		case ',':
			if depth == 1 {
				add(i)
				argStart = i + 1
			}
		}
		i++
	}
	return nil, 0, false
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
