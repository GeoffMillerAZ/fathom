package shellcmd

import (
	"strconv"
	"strings"
)

// maxDepth bounds nesting (substitutions, -c shells) so a hostile or
// fuzzed input cannot recurse without limit.
const maxDepth = 16

type tokKind int

const (
	tWord  tokKind = iota
	tOp            // a control operator: && || ; ;; ;& ;;& & | |& ( ) or a newline
	tRedir         // a redirection operator, with its descriptor
	tArith         // (( … )) at a command position
)

type token struct {
	kind    tokKind
	text    string // the operator, or the word's literal
	raw     string // a word's source text
	word    Word
	fd      int
	subst   []*Command
	heredoc *Heredoc
}

// unreadable is the lexer's and parser's one error: what it cannot read.
type unreadable struct{ reason string }

func (e *unreadable) Error() string { return e.reason }

func fail(reason string) error { return &unreadable{reason: reason} }

func reasonOf(err error) string {
	if u, ok := err.(*unreadable); ok {
		return u.reason
	}
	return err.Error()
}

type lexer struct {
	src       string
	i         int
	depth     int
	toks      []token
	pending   []*Heredoc // heredocs whose bodies start after the next newline
	wantDelim *Heredoc   // set after << or <<- until its delimiter word
}

func (l *lexer) peek(off int) byte {
	if l.i+off < len(l.src) {
		return l.src[l.i+off]
	}
	return 0
}

// run tokenizes from l.i. Nested (inside `$(` or `<(`), it stops after the
// unmatched `)` that closes the substitution.
func (l *lexer) run(nested bool) error {
	parens := 0
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == 0:
			return fail("NUL byte")
		case c == ' ' || c == '\t':
			l.i++
		case c == '\\' && l.peek(1) == '\n':
			l.i += 2
		case c == '\n':
			if err := l.op("\n", 1); err != nil {
				return err
			}
			if err := l.readHeredocs(); err != nil {
				return err
			}
		case c == '#':
			for l.i < len(l.src) && l.src[l.i] != '\n' {
				l.i++
			}
		case c == '(':
			if l.peek(1) == '(' && l.atCommandStart() {
				if err := l.arithCommand(); err != nil {
					return err
				}
				continue
			}
			parens++
			if err := l.op("(", 1); err != nil {
				return err
			}
		case c == ')':
			if nested && parens == 0 {
				l.i++
				if l.wantDelim != nil || len(l.pending) > 0 {
					return fail("unterminated heredoc")
				}
				return nil
			}
			parens--
			if err := l.op(")", 1); err != nil {
				return err
			}
		case c == ';':
			switch {
			case strings.HasPrefix(l.src[l.i:], ";;&"):
				if err := l.op(";;&", 3); err != nil {
					return err
				}
			case strings.HasPrefix(l.src[l.i:], ";;"):
				if err := l.op(";;", 2); err != nil {
					return err
				}
			case strings.HasPrefix(l.src[l.i:], ";&"):
				if err := l.op(";&", 2); err != nil {
					return err
				}
			default:
				if err := l.op(";", 1); err != nil {
					return err
				}
			}
		case c == '&':
			switch {
			case strings.HasPrefix(l.src[l.i:], "&&"):
				if err := l.op("&&", 2); err != nil {
					return err
				}
			case strings.HasPrefix(l.src[l.i:], "&>>"):
				l.redir("&>>", 3, -1)
			case strings.HasPrefix(l.src[l.i:], "&>"):
				l.redir("&>", 2, -1)
			default:
				if err := l.op("&", 1); err != nil {
					return err
				}
			}
		case c == '|':
			switch {
			case strings.HasPrefix(l.src[l.i:], "||"):
				if err := l.op("||", 2); err != nil {
					return err
				}
			case strings.HasPrefix(l.src[l.i:], "|&"):
				if err := l.op("|&", 2); err != nil {
					return err
				}
			default:
				if err := l.op("|", 1); err != nil {
					return err
				}
			}
		case c == '<' || c == '>':
			if l.peek(1) == '(' {
				if err := l.word(); err != nil {
					return err
				}
				continue
			}
			if err := l.redirOp(-1); err != nil {
				return err
			}
		case isDigit(c):
			j := l.i
			for j < len(l.src) && isDigit(l.src[j]) {
				j++
			}
			if j < len(l.src) && (l.src[j] == '<' || l.src[j] == '>') && !(j+1 < len(l.src) && l.src[j+1] == '(') {
				fd, err := strconv.Atoi(l.src[l.i:j])
				if err != nil {
					return fail("file descriptor out of range")
				}
				l.i = j
				if err := l.redirOp(fd); err != nil {
					return err
				}
				continue
			}
			if err := l.word(); err != nil {
				return err
			}
		default:
			if err := l.word(); err != nil {
				return err
			}
		}
	}
	if nested {
		return fail("unterminated $(")
	}
	if l.wantDelim != nil {
		return fail("heredoc without a delimiter")
	}
	if len(l.pending) > 0 {
		return fail("unterminated heredoc")
	}
	return nil
}

func (l *lexer) op(text string, n int) error {
	if l.wantDelim != nil {
		return fail("heredoc without a delimiter")
	}
	l.toks = append(l.toks, token{kind: tOp, text: text, fd: -1})
	l.i += n
	return nil
}

func (l *lexer) redir(text string, n, fd int) {
	l.toks = append(l.toks, token{kind: tRedir, text: text, fd: fd})
	l.i += n
}

// redirOp lexes a `<` or `>` operator at l.i.
func (l *lexer) redirOp(fd int) error {
	if l.wantDelim != nil {
		return fail("heredoc without a delimiter")
	}
	for _, op := range []string{"<<<", "<<-", "<<", "<>", "<&", "<", ">>", ">&", ">|", ">"} {
		if strings.HasPrefix(l.src[l.i:], op) {
			l.redir(op, len(op), fd)
			if op == "<<" || op == "<<-" {
				l.wantDelim = &Heredoc{StripTabs: op == "<<-"}
			}
			return nil
		}
	}
	return fail("unreadable redirect")
}

// atCommandStart reports whether the next token would begin a command.
func (l *lexer) atCommandStart() bool {
	if len(l.toks) == 0 {
		return true
	}
	t := l.toks[len(l.toks)-1]
	switch t.kind {
	case tOp:
		return t.text != ")"
	case tWord:
		if t.word.Quoted {
			return false
		}
		switch t.text {
		case "then", "do", "else", "elif", "if", "while", "until", "!", "{", "for", "time":
			return true
		}
	}
	return false
}

// arithCommand lexes `(( … ))` at a command position into one tArith token.
func (l *lexer) arithCommand() error {
	j := l.i + 2
	depth := 0
	for j < len(l.src) {
		switch l.src[j] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				if j+1 < len(l.src) && l.src[j+1] == ')' {
					l.toks = append(l.toks, token{kind: tArith, text: l.src[l.i+2 : j], fd: -1})
					l.i = j + 2
					return nil
				}
				return fail("unterminated ((")
			}
			depth--
		}
		j++
	}
	return fail("unterminated ((")
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameChar(c byte) bool { return isNameStart(c) || isDigit(c) }

func isBreak(c byte) bool {
	switch c {
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '<', '>':
		return true
	}
	return false
}

// word lexes one shell word at l.i.
func (l *lexer) word() error {
	start := l.i
	var b strings.Builder
	var w Word
	var subst []*Command
	brace := false
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == 0 {
			return fail("NUL byte")
		}
		if isBreak(c) {
			if (c == '<' || c == '>') && l.i == start && l.peek(1) == '(' {
				inner, end, err := l.nested(l.i + 2)
				if err != nil {
					return err
				}
				b.WriteString(l.src[l.i:end])
				subst = append(subst, inner)
				w.Dynamic = true
				l.i = end
				continue
			}
			break
		}
		switch c {
		case '\'':
			j := strings.IndexByte(l.src[l.i+1:], '\'')
			if j < 0 {
				return fail("unterminated '")
			}
			b.WriteString(l.src[l.i+1 : l.i+1+j])
			l.i += j + 2
			w.Quoted = true
		case '"':
			l.i++
			if err := l.double(&b, &w, &subst); err != nil {
				return err
			}
		case '\\':
			if l.i+1 >= len(l.src) {
				b.WriteByte('\\')
				l.i++
				continue
			}
			if l.src[l.i+1] == '\n' {
				l.i += 2
				continue
			}
			b.WriteByte(l.src[l.i+1])
			l.i += 2
			w.Quoted = true
		case '$':
			if err := l.dollar(&b, &w, &subst, false); err != nil {
				return err
			}
		case '`':
			if err := l.backtick(&b, &w, &subst); err != nil {
				return err
			}
		case '*', '?', '[':
			w.Glob = true
			b.WriteByte(c)
			l.i++
		case '{':
			brace = true
			b.WriteByte(c)
			l.i++
		default:
			b.WriteByte(c)
			l.i++
		}
	}
	w.Text = b.String()
	if brace && w.Text != "{" && (strings.Contains(w.Text, ",") || strings.Contains(w.Text, "..")) {
		w.Glob = true
	}
	t := token{kind: tWord, text: w.Text, raw: l.src[start:l.i], word: w, subst: subst, fd: -1}
	if h := l.wantDelim; h != nil {
		h.Delim = w.Text
		h.Quoted = w.Quoted
		t.heredoc = h
		l.pending = append(l.pending, h)
		l.wantDelim = nil
	}
	l.toks = append(l.toks, t)
	return nil
}

// double lexes the inside of a double-quoted string; l.i is past the `"`.
func (l *lexer) double(b *strings.Builder, w *Word, subst *[]*Command) error {
	w.Quoted = true
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch c {
		case 0:
			return fail("NUL byte")
		case '"':
			l.i++
			return nil
		case '\\':
			if l.i+1 >= len(l.src) {
				return fail("unterminated \"")
			}
			switch n := l.src[l.i+1]; n {
			case '$', '`', '"', '\\':
				b.WriteByte(n)
				l.i += 2
			case '\n':
				l.i += 2
			default:
				b.WriteByte('\\')
				l.i++
			}
		case '$':
			if err := l.dollar(b, w, subst, true); err != nil {
				return err
			}
		case '`':
			if err := l.backtick(b, w, subst); err != nil {
				return err
			}
		default:
			b.WriteByte(c)
			l.i++
		}
	}
	return fail("unterminated \"")
}

// dollar lexes an expansion or quoting form at a `$`.
func (l *lexer) dollar(b *strings.Builder, w *Word, subst *[]*Command, inDouble bool) error {
	start := l.i
	n := l.peek(1)
	switch {
	case n == '(' && l.peek(2) == '(':
		end, err := arithEnd(l.src, l.i+3)
		if err != nil {
			return err
		}
		b.WriteString(l.src[start:end])
		w.Dynamic = true
		l.i = end
	case n == '(':
		inner, end, err := l.nested(l.i + 2)
		if err != nil {
			return err
		}
		b.WriteString(l.src[start:end])
		*subst = append(*subst, inner)
		w.Dynamic = true
		l.i = end
	case n == '{':
		end, err := braceEnd(l.src, l.i+2)
		if err != nil {
			return err
		}
		b.WriteString(l.src[start:end])
		w.Dynamic = true
		l.i = end
	case n == '\'' && !inDouble:
		l.i += 2
		return l.ansiC(b, w)
	case n == '"' && !inDouble:
		l.i += 2
		return l.double(b, w, subst)
	case isNameStart(n):
		j := l.i + 1
		for j < len(l.src) && isNameChar(l.src[j]) {
			j++
		}
		b.WriteString(l.src[start:j])
		w.Dynamic = true
		l.i = j
	case isDigit(n) || strings.IndexByte("@*#?$!-", n) >= 0 && n != 0:
		b.WriteString(l.src[start : l.i+2])
		w.Dynamic = true
		l.i += 2
	default:
		b.WriteByte('$')
		l.i++
	}
	return nil
}

// nested lexes and parses a `$(` or `<(` body starting at at, returning the
// command and the index past its closing `)`.
func (l *lexer) nested(at int) (*Command, int, error) {
	if l.depth+1 > maxDepth {
		return nil, 0, fail("nested too deep")
	}
	sub := &lexer{src: l.src, i: at, depth: l.depth + 1}
	if err := sub.run(true); err != nil {
		return nil, 0, fail("in $(…): " + reasonOf(err))
	}
	for _, t := range sub.toks {
		if t.kind == tWord && !t.word.Quoted && t.text == "case" {
			return nil, 0, fail("case inside a command substitution")
		}
	}
	c, err := parseTokens(sub.toks, sub.depth)
	if err != nil {
		return nil, 0, fail("in $(…): " + reasonOf(err))
	}
	return c, sub.i, nil
}

// backtick lexes a `…` command substitution at l.i.
func (l *lexer) backtick(b *strings.Builder, w *Word, subst *[]*Command) error {
	start := l.i
	var inner strings.Builder
	j := l.i + 1
	for {
		if j >= len(l.src) {
			return fail("unterminated `")
		}
		c := l.src[j]
		if c == '\\' && j+1 < len(l.src) {
			if n := l.src[j+1]; n == '`' || n == '\\' || n == '$' {
				inner.WriteByte(n)
				j += 2
				continue
			}
		}
		if c == '`' {
			break
		}
		inner.WriteByte(c)
		j++
	}
	cmd := parseAt(inner.String(), l.depth+1)
	if cmd.Unreadable {
		return fail("in `…`: " + cmd.Reason)
	}
	b.WriteString(l.src[start : j+1])
	*subst = append(*subst, cmd)
	w.Dynamic = true
	l.i = j + 1
	return nil
}

// ansiC lexes the inside of a $'…' string; l.i is past the opening quote.
func (l *lexer) ansiC(b *strings.Builder, w *Word) error {
	w.Quoted = true
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == '\'' {
			l.i++
			return nil
		}
		if c != '\\' {
			b.WriteByte(c)
			l.i++
			continue
		}
		if l.i+1 >= len(l.src) {
			return fail("unterminated $'")
		}
		n := l.src[l.i+1]
		l.i += 2
		switch n {
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'e', 'E':
			b.WriteByte(27)
		case 'f':
			b.WriteByte(12)
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte(11)
		case '\\', '\'', '"', '?':
			b.WriteByte(n)
		case 'x':
			v, k := 0, 0
			for k < 2 && l.i < len(l.src) && isHex(l.src[l.i]) {
				v = v*16 + hexVal(l.src[l.i])
				l.i++
				k++
			}
			if k == 0 {
				b.WriteString("\\x")
			} else {
				b.WriteByte(byte(v))
			}
		default:
			if n >= '0' && n <= '7' {
				v, k := int(n-'0'), 1
				for k < 3 && l.i < len(l.src) && l.src[l.i] >= '0' && l.src[l.i] <= '7' {
					v = v*8 + int(l.src[l.i]-'0')
					l.i++
					k++
				}
				b.WriteByte(byte(v))
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(n)
		}
	}
	return fail("unterminated $'")
}

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) int {
	switch {
	case isDigit(c):
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return int(c-'A') + 10
}

// arithEnd finds the end of `$((…))` whose body starts at at.
func arithEnd(src string, at int) (int, error) {
	depth := 0
	for j := at; j < len(src); j++ {
		switch src[j] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				if j+1 < len(src) && src[j+1] == ')' {
					return j + 2, nil
				}
				return 0, fail("unterminated $((")
			}
			depth--
		}
	}
	return 0, fail("unterminated $((")
}

// braceEnd finds the end of `${…}` whose body starts at at.
func braceEnd(src string, at int) (int, error) {
	depth := 1
	for j := at; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '\'':
			k := strings.IndexByte(src[j+1:], '\'')
			if k < 0 {
				return 0, fail("unterminated ${")
			}
			j += k + 1
		case '"':
			k := j + 1
			for k < len(src) && src[k] != '"' {
				if src[k] == '\\' {
					k++
				}
				k++
			}
			if k >= len(src) {
				return 0, fail("unterminated ${")
			}
			j = k
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1, nil
			}
		}
	}
	return 0, fail("unterminated ${")
}

// readHeredocs reads the bodies of the pending heredocs, in order, from the
// line after the newline just lexed.
func (l *lexer) readHeredocs() error {
	for _, h := range l.pending {
		var body strings.Builder
		for {
			if l.i >= len(l.src) {
				return fail("unterminated heredoc")
			}
			line, next := l.src[l.i:], len(l.src)
			if k := strings.IndexByte(line, '\n'); k >= 0 {
				line, next = line[:k], l.i+k+1
			}
			if h.StripTabs {
				line = strings.TrimLeft(line, "\t")
			}
			l.i = next
			if line == h.Delim {
				break
			}
			if strings.IndexByte(line, 0) >= 0 {
				return fail("NUL byte")
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
		h.Body = body.String()
		h.Dynamic = !h.Quoted && strings.ContainsAny(h.Body, "$`")
	}
	l.pending = nil
	return nil
}
