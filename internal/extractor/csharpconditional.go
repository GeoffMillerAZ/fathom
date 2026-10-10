package extractor

// csharpconditional.go — the csharp dialect's `preprocess: "csharp-conditionals"`
// texts (O148, LB-F53): a C# file whose tree has a syntax error is parsed
// again with its #if directives blanked, then with each group read as its
// first branch. Moved from the retired CGO csharpextract.go (LB-F393), which
// read them alike; tsextract.go's preprocessor table names the function.

import "bytes"

// csharpConditionalTexts is the texts a C# file that does not parse as
// written is parsed as instead, in order (O148): src with each #if group's
// directive lines (#if, #elif, #else, #endif) blanked, every branch kept;
// then src with each group read as its first branch, its #elif and #else
// branches blanked too. tree-sitter-c-sharp parses a group as a node whose
// branches must each hold whole declarations, so a group that closes one
// type and opens another, or whose branches are two headers of one body
// (#if A / T() / #else / void M() / #endif, then the body), leaves the
// file's namespace_declaration unparsed; without the directives it parses,
// and where two branches still collide, one branch of each does. The first
// branch is the one a group's own condition selects, and it keeps a group
// with no #else whole (a file-wide #if HAVE_ASYNC included), which the
// compiler's reading with no symbol defined drops. Blanking turns each byte
// but a line break into a space, so byte offsets, lines and columns are
// unchanged. A directive is found as the compiler finds it: the first token
// of a line outside a block comment and a verbatim or raw string of the code
// read, and any line of a blanked branch. nil when src has no #if group.
func csharpConditionalTexts(src []byte) [][]byte {
	all, ok := csharpBlankConditionals(src, false)
	if !ok {
		return nil
	}
	first, _ := csharpBlankConditionals(src, true)
	return [][]byte{all, first}
}

// csharpBlankConditionals is src with every #if group's directive lines
// blanked, and, when firstOnly, every line of its #elif and #else branches;
// ok is false when src has no #if group.
func csharpBlankConditionals(src []byte, firstOnly bool) (out []byte, ok bool) {
	var (
		skip []bool // per open group: whether the current branch is blanked
		lex  csharpLexState
	)
	blanked := func() bool {
		for _, s := range skip {
			if s {
				return true
			}
		}
		return false
	}
	blank := func(start, end int) {
		if out == nil {
			out = append([]byte(nil), src...)
		}
		for i := start; i < end; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	for start := 0; start < len(src); {
		end := len(src)
		if i := bytes.IndexByte(src[start:], '\n'); i >= 0 {
			end = start + i + 1
		}
		line := src[start:end]
		if start == 0 {
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}
		off := blanked()
		kw := ""
		if off || !lex.open() {
			kw = csharpDirective(line)
		}
		switch {
		case kw == "if":
			skip = append(skip, false)
		case (kw == "elif" || kw == "else") && len(skip) > 0:
			skip[len(skip)-1] = firstOnly
		case kw == "endif" && len(skip) > 0:
			skip = skip[:len(skip)-1]
		default:
			switch {
			case off:
				blank(start, end)
			case kw == "":
				lex.scan(line) // a directive's own text (#region …) is no code
			}
			start = end
			continue
		}
		ok = true
		blank(start, end)
		start = end
	}
	if !ok {
		return nil, false
	}
	return out, true
}

// csharpDirective is the keyword of the preprocessing directive a line
// starts with ("if", "endif", "region" …), "" when it starts with none.
func csharpDirective(line []byte) string {
	s := bytes.TrimLeft(line, " \t\v\f")
	if len(s) == 0 || s[0] != '#' {
		return ""
	}
	s = bytes.TrimLeft(s[1:], " \t")
	i := 0
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		i++
	}
	return string(s[:i])
}

// csharpLexState carries, from one line of compiled code to the next, the
// multi-line constructs a directive cannot start inside: a block comment,
// a verbatim string (@"…", $@"…") and a raw string ("""…""").
type csharpLexState struct {
	comment  bool
	verbatim bool
	raw      int // the open raw string's quote count, 0 outside one
}

func (s *csharpLexState) open() bool { return s.comment || s.verbatim || s.raw > 0 }

// scan advances s over one line of compiled code.
func (s *csharpLexState) scan(line []byte) {
	for i := 0; i < len(line); {
		switch {
		case s.comment:
			j := bytes.Index(line[i:], []byte("*/"))
			if j < 0 {
				return
			}
			s.comment = false
			i += j + 2
		case s.verbatim:
			i = s.verbatimEnd(line, i)
		case s.raw > 0:
			j := bytes.Index(line[i:], bytes.Repeat([]byte{'"'}, s.raw))
			if j < 0 {
				return
			}
			i += j + s.raw
			s.raw = 0
		default:
			i = s.token(line, i)
		}
	}
}

// verbatimEnd is the offset past the verbatim string's closing quote ("" is
// an escaped quote), or the line's end when it continues.
func (s *csharpLexState) verbatimEnd(line []byte, i int) int {
	for ; i < len(line); i++ {
		if line[i] != '"' {
			continue
		}
		if i+1 < len(line) && line[i+1] == '"' {
			i++
			continue
		}
		s.verbatim = false
		return i + 1
	}
	return i
}

// token steps over the code token at i: a comment or a string literal whole
// (opening a multi-line one on s), any other byte alone.
func (s *csharpLexState) token(line []byte, i int) int {
	c := line[i]
	switch {
	case c == '/' && i+1 < len(line) && line[i+1] == '/':
		return len(line)
	case c == '/' && i+1 < len(line) && line[i+1] == '*':
		s.comment = true
		return i + 2
	case c == '\'':
		return csharpQuoted(line, i+1, '\'')
	case c == '"' || c == '@' || c == '$':
		j := i
		verbatim := false
		for j < len(line) && (line[j] == '@' || line[j] == '$') {
			verbatim = verbatim || line[j] == '@'
			j++
		}
		if j >= len(line) || line[j] != '"' {
			return j + 1 // an @identifier or a stray $
		}
		if verbatim {
			s.verbatim = true
			return j + 1
		}
		q := j
		for q < len(line) && line[q] == '"' {
			q++
		}
		switch n := q - j; {
		case n >= 3:
			s.raw = n
			return q
		case n == 2:
			return q // ""
		}
		return csharpQuoted(line, j+1, '"')
	}
	return i + 1
}

// csharpQuoted is the offset past the quote closing a regular string or
// character literal whose content starts at i (a backslash escapes the next
// byte), or the line's end.
func csharpQuoted(line []byte, i int, quote byte) int {
	for ; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(line)
}
