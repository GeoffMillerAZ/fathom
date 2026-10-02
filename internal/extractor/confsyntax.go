package extractor

// confsyntax.go — the one small lexer and block parser behind the native
// proxy-configuration dialects (LB-Q-nginx, LB-Q-haproxy, LB-Q-caddyfile;
// docs/design/language-breadth.md §13, engine "native", grammar_source
// native:pure-go). The three formats share their word rules (whitespace
// separates words, a quoted string is one word, a backslash escapes, a
// # at the start of a word begins a comment to the end of the line) and
// differ in what ends a statement, which confSyntax names:
//
//   - nginx (crossplane's token rules): { } and ; are words of their own
//     wherever they appear outside quotes; a quote only opens a string at
//     the start of a word (elsewhere it is an ordinary character); ${name}
//     keeps its braces inside a word; the body of a *_by_lua_block
//     directive is one opaque word (Lua strings and comments may hold
//     braces).
//   - HAProxy: a newline ends a statement; an unquoted, unescaped # starts a
//     comment anywhere on the line; quotes may open and close inside a word
//     ("a"b is one word, ab), single quotes take nothing literally.
//   - Caddyfile: a newline ends a statement unless a backslash precedes it;
//     braces are block delimiters only as words of their own (a placeholder
//     such as {host} is an ordinary word); a backtick string is raw; a
//     <<MARKER heredoc is one word.
//
// The parser above the lexer gives nginx and the Caddyfile the same tree
// (confNode: a statement, its arguments and, for a block, its children);
// HAProxy reads the lexer's lines directly. Malformed input never fails:
// an unterminated string or block ends at the end of the file, a stray
// closing brace is skipped, and each repair is counted.

import (
	"bytes"
	"strings"
)

type confTokKind uint8

const (
	confWord  confTokKind = iota
	confOpen              // { (nginx)
	confClose             // } (nginx)
	confSemi              // ; (nginx; also the end of an opaque Lua body)
	confEOL               // the end of a logical line (HAProxy, Caddyfile)
)

// confTok is one token: a word's text (quotes removed, escapes as each
// format keeps them), the line it starts on, and whether any of it was
// quoted (a quoted "{" is a word, never a block delimiter).
type confTok struct {
	kind   confTokKind
	text   string
	line   int
	quoted bool
}

// confSyntax selects one format's lexical rules.
type confSyntax struct {
	braces    bool // nginx: { } ; are tokens of their own
	lines     bool // HAProxy, Caddyfile: a newline ends a statement (confEOL)
	hashAny   bool // HAProxy: an unquoted, unescaped # starts a comment anywhere
	midQuotes bool // HAProxy: quotes open and close inside a word
	backticks bool // Caddyfile: `raw` strings
	heredocs  bool // Caddyfile: <<MARKER … MARKER
	continued bool // Caddyfile: a backslash before the newline continues the line
	luaBlocks bool // nginx: a *_by_lua_block body is one opaque word
}

var (
	nginxSyntax     = confSyntax{braces: true, luaBlocks: true}
	haproxySyntax   = confSyntax{lines: true, hashAny: true, midQuotes: true}
	caddyfileSyntax = confSyntax{lines: true, backticks: true, heredocs: true, continued: true}
)

func confSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' }

// confLexer is one file's lexing in progress.
type confLexer struct {
	src   []byte
	sx    confSyntax
	i     int
	line  int
	toks  []confTok
	fixes int // repairs: an unterminated string, Lua body or heredoc
	// nginx: the next word names a directive (after ; { } or at the start),
	// and the directive in progress is a *_by_lua_block.
	directive, lua bool
}

// confLex tokenizes src; fixes counts the repairs it made.
func confLex(src []byte, sx confSyntax) (toks []confTok, fixes int) {
	l := &confLexer{src: src, sx: sx, line: 1, directive: true}
	l.run()
	if sx.lines {
		l.eol()
	}
	return l.toks, l.fixes
}

func (l *confLexer) emit(t confTok) { l.toks = append(l.toks, t) }

// eol ends the logical line, once.
func (l *confLexer) eol() {
	if n := len(l.toks); n > 0 && l.toks[n-1].kind != confEOL {
		l.emit(confTok{kind: confEOL, line: l.toks[n-1].line})
	}
}

func (l *confLexer) run() {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			if l.sx.lines {
				l.eol()
			}
			l.line++
			l.i++
			continue
		case confSpace(c):
			l.i++
			continue
		case c == '#':
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
			continue
		case l.sx.continued && c == '\\' && l.continuation():
			continue
		case l.sx.braces && (c == '{' || c == '}' || c == ';'):
			l.punct(c)
			continue
		}
		l.word()
	}
}

// continuation consumes a backslash that ends its line (optionally before
// a carriage return); the next line continues this one.
func (l *confLexer) continuation() bool {
	j := l.i + 1
	if j < len(l.src) && l.src[j] == '\r' {
		j++
	}
	if j < len(l.src) && l.src[j] == '\n' {
		l.i = j + 1
		l.line++
		return true
	}
	return false
}

// punct emits nginx's { } ; — except the { of a *_by_lua_block directive,
// whose Lua body becomes the directive's last argument (one opaque word),
// ended by a ; at its closing brace, as crossplane reads it.
func (l *confLexer) punct(c byte) {
	l.i++
	l.directive = true
	if c == '{' && l.lua {
		l.lua = false
		start := l.line
		body, closed := l.luaBody()
		l.emit(confTok{kind: confWord, text: body, line: start, quoted: true})
		l.emit(confTok{kind: confSemi, text: ";", line: l.line})
		if closed {
			l.i++ // the closing brace
		} else {
			l.fixes++
		}
		return
	}
	kind := map[byte]confTokKind{'{': confOpen, '}': confClose, ';': confSemi}[c]
	l.emit(confTok{kind: kind, text: string(c), line: l.line})
}

// luaBody reads a Lua block's body up to (not including) its matching
// closing brace, skipping Lua strings, long brackets and comments.
func (l *confLexer) luaBody() (string, bool) {
	src := l.src
	start, depth := l.i, 1
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == '{':
			depth++
			l.i++
		case c == '}':
			depth--
			if depth == 0 {
				return string(src[start:l.i]), true
			}
			l.i++
		case c == '"' || c == '\'':
			l.i++
			for l.i < len(src) && src[l.i] != c {
				if src[l.i] == '\\' && l.i+1 < len(src) {
					l.i++
				}
				if src[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
			l.i++
		case c == '[' && luaLongOpen(src[l.i:]) >= 0:
			l.luaLong(luaLongOpen(src[l.i:]))
		case c == '-' && l.i+1 < len(src) && src[l.i+1] == '-':
			l.i += 2
			if l.i < len(src) && src[l.i] == '[' && luaLongOpen(src[l.i:]) >= 0 {
				l.luaLong(luaLongOpen(src[l.i:]))
				continue
			}
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
		default:
			l.i++
		}
	}
	return string(src[start:]), false
}

// luaLongOpen is the level of a Lua long bracket ([[, [=[, …) at the start
// of b, or -1.
func luaLongOpen(b []byte) int {
	if len(b) < 2 || b[0] != '[' {
		return -1
	}
	n := 1
	for n < len(b) && b[n] == '=' {
		n++
	}
	if n < len(b) && b[n] == '[' {
		return n - 1
	}
	return -1
}

func (l *confLexer) luaLong(level int) {
	closer := []byte("]" + strings.Repeat("=", level) + "]")
	l.i += level + 2
	for l.i < len(l.src) {
		if bytes.HasPrefix(l.src[l.i:], closer) {
			l.i += len(closer)
			return
		}
		if l.src[l.i] == '\n' {
			l.line++
		}
		l.i++
	}
}

// word reads one word: a string, a heredoc or a bare word.
func (l *confLexer) word() {
	src := l.src
	start := l.line
	c := src[l.i]
	var b strings.Builder
	quoted := false
	switch {
	case c == '"' || c == '\'' || (l.sx.backticks && c == '`'):
		if !l.sx.midQuotes {
			l.quoted(c, &b)
			l.finish(b.String(), start, true)
			return
		}
	case l.sx.heredocs && bytes.HasPrefix(src[l.i:], []byte("<<")):
		if text, ok := l.heredoc(); ok {
			l.finish(text, start, true)
			return
		}
	}
	for l.i < len(src) {
		c = src[l.i]
		if c == '\n' || confSpace(c) {
			break
		}
		if l.sx.braces && (c == ';' || c == '}') {
			break
		}
		if l.sx.braces && c == '{' {
			// ${name}: a parameter expansion keeps its braces.
			if s := b.String(); strings.HasSuffix(s, "$") {
				for l.i < len(src) && src[l.i] != '}' && src[l.i] != '\n' && !confSpace(src[l.i]) {
					b.WriteByte(src[l.i])
					l.i++
				}
				if l.i < len(src) && src[l.i] == '}' {
					b.WriteByte('}')
					l.i++
				}
				continue
			}
			break
		}
		if l.sx.hashAny && c == '#' {
			break
		}
		if l.sx.midQuotes && (c == '"' || c == '\'') {
			l.quoted(c, &b)
			quoted = true
			continue
		}
		if c == '\\' && l.i+1 < len(src) && src[l.i+1] != '\n' {
			if l.sx.midQuotes {
				// HAProxy: the backslash goes, the character stays.
				b.WriteByte(src[l.i+1])
			} else {
				b.WriteByte(c)
				b.WriteByte(src[l.i+1])
			}
			l.i += 2
			continue
		}
		if c == '\\' && l.sx.continued && l.continuation() {
			break
		}
		b.WriteByte(c)
		l.i++
	}
	l.finish(b.String(), start, quoted)
}

// quoted appends the string opened by q at l.i to b and moves past its
// closing quote: nginx unescapes only the quote character, the Caddyfile
// the quote and a newline, HAProxy every escaped character inside double
// quotes; single quotes (HAProxy) and backticks (Caddyfile) are literal. An
// unterminated string runs to the end of the file (a repair).
func (l *confLexer) quoted(q byte, b *strings.Builder) {
	src := l.src
	l.i++
	literal := q == '`' || (l.sx.midQuotes && q == '\'')
	for l.i < len(src) {
		c := src[l.i]
		if c == q {
			l.i++
			return
		}
		if c == '\n' {
			if l.sx.midQuotes {
				// HAProxy: a string never spans lines; the newline ends it.
				l.fixes++
				return
			}
			l.line++
		}
		if c == '\\' && !literal && l.i+1 < len(src) {
			next := src[l.i+1]
			switch {
			case l.sx.midQuotes, next == q, l.sx.lines && next == '\n':
				if next == '\n' {
					l.line++
				}
				b.WriteByte(next)
			default:
				b.WriteByte(c)
				b.WriteByte(next)
			}
			l.i += 2
			continue
		}
		b.WriteByte(c)
		l.i++
	}
	l.fixes++
}

// heredoc reads a Caddyfile <<MARKER heredoc: the lines up to the one that
// starts (after its indentation) with MARKER, whose indentation is removed
// from every line; words after the closing MARKER continue the statement
// (`HTML 200`). ok is false when << does not open one (it is then an
// ordinary word).
func (l *confLexer) heredoc() (string, bool) {
	src := l.src
	j := l.i + 2
	k := j
	for k < len(src) && !confSpace(src[k]) && src[k] != '\n' {
		k++
	}
	marker := string(src[j:k])
	if marker == "" || k >= len(src) || src[k] != '\n' {
		return "", false
	}
	l.i = k + 1
	l.line++
	var lines []string
	for l.i <= len(src) {
		e := l.i
		for e < len(src) && src[e] != '\n' {
			e++
		}
		ln := string(src[l.i:e])
		trimmed := strings.TrimLeft(ln, " \t")
		if rest, ok := strings.CutPrefix(trimmed, marker); ok && (rest == "" || confSpace(rest[0])) {
			indent := ln[:len(ln)-len(trimmed)]
			for n, s := range lines {
				lines[n] = strings.TrimPrefix(s, indent)
			}
			l.i += len(indent) + len(marker)
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, ln)
		if e >= len(src) {
			l.i = e
			l.fixes++
			return strings.Join(lines, "\n"), true
		}
		l.i = e + 1
		l.line++
	}
	l.fixes++
	return strings.Join(lines, "\n"), true
}

// finish emits a word and tracks nginx's directive position.
func (l *confLexer) finish(text string, line int, quoted bool) {
	l.emit(confTok{kind: confWord, text: text, line: line, quoted: quoted})
	if l.sx.luaBlocks {
		if l.directive {
			l.lua = strings.HasSuffix(text, "_by_lua_block")
		}
		l.directive = false
	}
}

// confNode is one statement: its name (the first word), its arguments, the
// lines it spans and, for a block, its children.
type confNode struct {
	name      string
	args      []confTok
	line, end int
	block     bool
	children  []*confNode
}

// argText is the i-th argument's text, "" past the end.
func (n *confNode) argText(i int) string {
	if i < len(n.args) {
		return n.args[i].text
	}
	return ""
}

// argsText joins the arguments' texts with single spaces.
func (n *confNode) argsText() string {
	parts := make([]string, len(n.args))
	for i, a := range n.args {
		parts[i] = a.text
	}
	return strings.Join(parts, " ")
}

func newConfNode(words []confTok, end int) *confNode {
	n := &confNode{line: words[0].line, end: end, name: words[0].text}
	n.args = append(n.args, words[1:]...)
	return n
}

// confLastLine is the line of src's last character (a file's EOF line).
func confLastLine(src []byte) int {
	n := 1 + strings.Count(string(src), "\n")
	if len(src) > 0 && src[len(src)-1] == '\n' {
		n--
	}
	return max(n, 1)
}

// nginxParser builds the statement tree from nginx tokens.
type nginxParser struct {
	toks  []confTok
	i     int
	eof   int // the file's last line: an unclosed block ends there
	close int // the line of the brace that closed the block just read
	fixes int
}

// parseNginxTree reads src into statements; fixes counts the repairs (an
// unterminated string or block, a stray brace or semicolon, a statement a
// closing brace or the end of the file cut short).
func parseNginxTree(src []byte) ([]*confNode, int) {
	toks, fixes := confLex(src, nginxSyntax)
	p := &nginxParser{toks: toks, eof: confLastLine(src)}
	nodes := p.block(0)
	return nodes, fixes + p.fixes
}

func (p *nginxParser) block(depth int) []*confNode {
	var out []*confNode
	var cur []confTok
	flush := func(end int) {
		if len(cur) > 0 {
			out = append(out, newConfNode(cur, end))
			cur = nil
		}
	}
	for p.i < len(p.toks) {
		t := p.toks[p.i]
		p.i++
		switch t.kind {
		case confWord:
			cur = append(cur, t)
		case confSemi:
			if len(cur) == 0 {
				p.fixes++
				continue
			}
			flush(t.line)
		case confOpen:
			if len(cur) == 0 {
				// A block with no name: read its children so what they
				// declare survives.
				p.fixes++
				cur = []confTok{{kind: confWord, line: t.line}}
			}
			n := newConfNode(cur, t.line)
			cur = nil
			n.block = true
			n.children = p.block(depth + 1)
			n.end = p.close
			out = append(out, n)
		case confClose:
			if len(cur) > 0 {
				p.fixes++
				flush(cur[len(cur)-1].line)
			}
			if depth == 0 {
				p.fixes++
				continue
			}
			p.close = t.line
			return out
		}
	}
	if len(cur) > 0 {
		p.fixes++
		flush(cur[len(cur)-1].line)
	}
	if depth > 0 {
		p.fixes++
	}
	p.close = p.eof
	return out
}

// confLines splits line-oriented tokens (HAProxy, Caddyfile) into logical
// lines of words.
func confLines(toks []confTok) [][]confTok {
	var out [][]confTok
	var cur []confTok
	for _, t := range toks {
		if t.kind == confEOL {
			if len(cur) > 0 {
				out = append(out, cur)
			}
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// confIsOpen and confIsClose: a Caddyfile brace is a delimiter only as an
// unquoted word of its own.
func confIsOpen(t confTok) bool  { return !t.quoted && t.text == "{" }
func confIsClose(t confTok) bool { return !t.quoted && t.text == "}" }

// caddyParser builds the statement tree from Caddyfile lines: a line whose
// last word is { opens a block for the statement it starts, a line holding
// only } closes the innermost one.
type caddyParser struct {
	lines [][]confTok
	i     int
	eof   int
	close int
	fixes int
}

// parseCaddyTree reads src into statements, with parseNginxTree's repairs.
func parseCaddyTree(src []byte) ([]*confNode, int) {
	toks, fixes := confLex(src, caddyfileSyntax)
	p := &caddyParser{lines: confLines(toks), eof: confLastLine(src)}
	nodes := p.block(0)
	return nodes, fixes + p.fixes
}

func (p *caddyParser) block(depth int) []*confNode {
	var out []*confNode
	for p.i < len(p.lines) {
		ln := p.lines[p.i]
		p.i++
		if len(ln) == 1 && confIsClose(ln[0]) {
			if depth == 0 {
				p.fixes++
				continue
			}
			p.close = ln[0].line
			return out
		}
		// A site's address list may continue on the next line after a
		// trailing comma.
		for depth == 0 && strings.HasSuffix(ln[len(ln)-1].text, ",") && p.i < len(p.lines) {
			ln = append(append([]confTok(nil), ln...), p.lines[p.i]...)
			p.i++
		}
		open := confIsOpen(ln[len(ln)-1])
		words := ln
		if open {
			words = ln[:len(ln)-1]
		}
		if len(words) == 0 {
			// A bare { — the global options block at the top, or a stray one.
			words = []confTok{{kind: confWord, line: ln[0].line}}
		}
		last := words[len(words)-1].line
		n := newConfNode(words, last)
		if open {
			n.block = true
			n.children = p.block(depth + 1)
			n.end = p.close
		}
		out = append(out, n)
	}
	if depth > 0 {
		p.fixes++
	}
	p.close = p.eof
	return out
}

// ProxyDialectExtractor is the stand-alone extractor of a native proxy
// dialect (nginx, haproxy, caddyfile); NativeDialectExtractor asks it first.
func ProxyDialectExtractor(id string) (Extractor, bool) {
	switch id {
	case "nginx":
		return NewNginxExtractor(), true
	case "haproxy":
		return NewHAProxyExtractor(), true
	case "caddyfile":
		return NewCaddyfileExtractor(), true
	}
	return nil, false
}
