package main

// lex.go — the oracle's own lexical scan of a CUE file. It proposes
// positions only: every identifier the cue binary's language server is then
// asked about (textDocument/definition), and every import declaration. It
// decides nothing the reading scores: what an identifier refers to, and
// whether it refers to anything at all, is the cue binary's answer. It shares
// no code with fathom and loads no CUE library: a small hand-written scanner
// over CUE's lexical grammar (comments, the four string forms with their #
// raw variants and \( \#( interpolations, identifiers, attributes).

import "unicode/utf8"

type tokKind int

const (
	tkIdent tokKind = iota
	tkString
	tkPunct
	tkNumber
	tkAttr
)

// token is one lexical token. Line is 1-based, Col the 0-based byte column
// and Col16 the 0-based UTF-16 column (the language server's default unit).
type token struct {
	Kind  tokKind
	Text  string
	Line  int
	Col   int
	Col16 int
	// Value is a string token's content without its quotes, when it holds
	// no interpolation (an import path).
	Value string
}

// identRole is what the scan says an identifier occurrence is, by its
// neighbours alone.
type identRole int

const (
	roleRef         identRole = iota // a candidate reference: the server is asked
	roleLabel                        // a field label being declared (`name:`)
	roleDecl                         // a let, alias or comprehension variable being declared
	roleKeyword                      // a keyword or a predeclared identifier
	roleImportAlias                  // an import's alias
	rolePackage                      // the package clause's name
)

// ident is one identifier occurrence and its role.
type ident struct {
	token
	Role identRole
}

// importSpec is one import declaration: the path as written (unquoted) and
// where its string starts.
type importSpec struct {
	Path  string
	Alias string
	Line  int
	Col   int // of the opening quote
	Col16 int
	// AliasAt is the alias identifier's token (zero when unaliased): the
	// language server resolves an aliased import at its alias.
	AliasAt token
}

// scanResult is a file's scan.
type scanResult struct {
	Idents  []ident
	Imports []importSpec
	Package string
	PkgLine int
}

var keywords = map[string]bool{
	"package": true, "import": true, "for": true, "if": true, "let": true,
	"true": true, "false": true, "null": true, "_": true,
	"bool": true, "string": true, "bytes": true, "number": true, "int": true, "float": true, "uint": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "int128": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uint128": true,
	"float32": true, "float64": true, "rune": true, "byte": true,
	"len": true, "close": true, "and": true, "or": true, "div": true, "mod": true, "quo": true, "rem": true,
}

// lexer walks src producing tokens; strings are one token each, and the
// expression inside each interpolation is tokenized in place.
type lexer struct {
	src  []byte
	pos  int
	line int
	bol  int // byte offset of the current line's start
	out  []token
}

func lex(src []byte) []token {
	l := &lexer{src: src, line: 1}
	l.expr(0)
	return l.out
}

func (l *lexer) col16(off int) int {
	n := 0
	for i := l.bol; i < off && i < len(l.src); {
		r, w := utf8.DecodeRune(l.src[i:])
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += w
	}
	return n
}

func (l *lexer) emit(k tokKind, start int, text string) {
	l.out = append(l.out, token{Kind: k, Text: text, Line: l.line, Col: start - l.bol, Col16: l.col16(start)})
}

func (l *lexer) newline() {
	l.line++
	l.bol = l.pos
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c == '#' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c >= 0x80
}

// expr tokenizes until EOF, or (depth > 0) until the ')' that closes an
// interpolation opened at depth.
func (l *lexer) expr(depth int) {
	parens := 0
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.pos++
			l.newline()
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case c == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '"' || c == '\'':
			l.str(0)
		case c == '#' && l.rawStart():
			hashes := 0
			for l.pos+hashes < len(l.src) && l.src[l.pos+hashes] == '#' {
				hashes++
			}
			l.str(hashes)
		case c == '@' && l.pos+1 < len(l.src) && isIdentStart(l.src[l.pos+1]):
			l.attr()
		case isIdentStart(c):
			start := l.pos
			l.pos++
			// `_#Name` and `#Name`: a definition marker, then the identifier.
			for l.pos < len(l.src) && (isIdentPart(l.src[l.pos]) || (l.src[l.pos] == '#' && l.pos == start+1 && l.src[start] == '_')) {
				l.pos++
			}
			l.emit(tkIdent, start, string(l.src[start:l.pos]))
		case c >= '0' && c <= '9':
			start := l.pos
			for l.pos < len(l.src) && (isIdentPart(l.src[l.pos]) || l.src[l.pos] == '.') {
				l.pos++
			}
			l.emit(tkNumber, start, string(l.src[start:l.pos]))
		default:
			if depth > 0 {
				if c == '(' {
					parens++
				} else if c == ')' {
					if parens == 0 {
						// the interpolation's close: a token of its own, so the
						// last name inside is never read as a label by the ':'
						// that may follow the string
						l.emit(tkPunct, l.pos, ")")
						l.pos++
						return
					}
					parens--
				}
			}
			l.emit(tkPunct, l.pos, string(c))
			l.pos++
		}
	}
}

// rawStart reports whether the '#' at pos opens a raw string (#"…"#).
func (l *lexer) rawStart() bool {
	i := l.pos
	for i < len(l.src) && l.src[i] == '#' {
		i++
	}
	return i < len(l.src) && (l.src[i] == '"' || l.src[i] == '\'')
}

// str reads one string literal starting at pos (after `hashes` '#'s), with
// its interpolations tokenized in place.
func (l *lexer) str(hashes int) {
	start := l.pos
	startLine, startBol := l.line, l.bol
	l.pos += hashes
	q := l.src[l.pos]
	multi := l.pos+2 < len(l.src) && l.src[l.pos+1] == q && l.src[l.pos+2] == q
	if multi {
		l.pos += 3
	} else {
		l.pos++
	}
	var content []byte
	interpolated := false
	tok := token{Kind: tkString, Line: startLine, Col: start - startBol}
	// the column in UTF-16 units is computed against the start line
	saveBol := l.bol
	l.bol = startBol
	tok.Col16 = l.col16(start)
	l.bol = saveBol
	l.out = append(l.out, tok)
	idx := len(l.out) - 1
	closer := func() bool {
		if multi {
			if l.pos+2 >= len(l.src) || l.src[l.pos] != q || l.src[l.pos+1] != q || l.src[l.pos+2] != q {
				return false
			}
			for h := 0; h < hashes; h++ {
				if l.pos+3+h >= len(l.src) || l.src[l.pos+3+h] != '#' {
					return false
				}
			}
			l.pos += 3 + hashes
			return true
		}
		if l.src[l.pos] != q {
			return false
		}
		for h := 0; h < hashes; h++ {
			if l.pos+1+h >= len(l.src) || l.src[l.pos+1+h] != '#' {
				return false
			}
		}
		l.pos += 1 + hashes
		return true
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\n' {
			if !multi {
				break // an unterminated single-line string ends at the line
			}
			content = append(content, c)
			l.pos++
			l.newline()
			continue
		}
		if closer() {
			break
		}
		if c == '\\' {
			esc := l.pos + 1
			ok := true
			for h := 0; h < hashes; h++ {
				if esc >= len(l.src) || l.src[esc] != '#' {
					ok = false
					break
				}
				esc++
			}
			if ok && esc < len(l.src) && l.src[esc] == '(' {
				interpolated = true
				l.pos = esc + 1
				l.expr(1)
				continue
			}
			if ok && esc < len(l.src) {
				content = append(content, l.src[esc])
				l.pos = esc + 1
				continue
			}
		}
		content = append(content, c)
		l.pos++
	}
	l.out[idx].Text = string(l.src[start:min(l.pos, len(l.src))])
	if !interpolated {
		l.out[idx].Value = string(content)
	}
	// keep the string token before its interpolation's tokens: idx is fixed
}

// attr skips an attribute `@name(...)`: its body is the tool's text, never
// CUE references.
func (l *lexer) attr() {
	start := l.pos
	l.pos++
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.src) && l.src[l.pos] == '(' {
		depth := 0
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			if c == '\n' {
				l.pos++
				l.newline()
				continue
			}
			if c == '"' || c == '\'' {
				// a quoted argument may hold parentheses
				q := c
				l.pos++
				for l.pos < len(l.src) && l.src[l.pos] != q && l.src[l.pos] != '\n' {
					if l.src[l.pos] == '\\' {
						l.pos++
					}
					l.pos++
				}
				if l.pos < len(l.src) && l.src[l.pos] == q {
					l.pos++
				}
				continue
			}
			l.pos++
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
	}
	_ = start
}

// scan classifies a file's identifiers by their neighbours and collects its
// package clause and imports.
func scan(src []byte) scanResult {
	toks := lex(src)
	var res scanResult
	next := func(i int) (token, bool) {
		if i+1 < len(toks) {
			return toks[i+1], true
		}
		return token{}, false
	}
	isPunct := func(t token, ok bool, s string) bool { return ok && t.Kind == tkPunct && t.Text == s }
	inFor := false
	importBlock := false
	importLine := false
	for i, t := range toks {
		if t.Kind != tkIdent {
			if t.Kind == tkPunct && t.Text == ")" && importBlock {
				importBlock = false
			}
			if t.Kind == tkString && importLine {
				importLine = false // `import "path"` and `import alias "path"` end at their path
			}
			continue
		}
		id := ident{token: t, Role: roleRef}
		n1, ok1 := next(i)
		var prev token
		hasPrev := i > 0
		if hasPrev {
			prev = toks[i-1]
		}
		switch {
		case t.Text == "package" && !isPunct(n1, ok1, ":"):
			id.Role = roleKeyword
			if ok1 && n1.Kind == tkIdent {
				res.Package, res.PkgLine = n1.Text, n1.Line
			}
		case hasPrev && prev.Kind == tkIdent && prev.Text == "package" && (i < 2 || !(toks[i-2].Kind == tkPunct && toks[i-2].Text == ".")):
			id.Role = rolePackage
		case t.Text == "import" && !isPunct(n1, ok1, ":"):
			id.Role = roleKeyword
			if isPunct(n1, ok1, "(") {
				importBlock = true
			} else {
				importLine = true
			}
		case (importBlock || importLine) && ok1 && n1.Kind == tkString:
			id.Role = roleImportAlias
		case t.Text == "for" && !isPunct(n1, ok1, ":"):
			id.Role, inFor = roleKeyword, true
		case t.Text == "in" && inFor:
			id.Role, inFor = roleKeyword, false
		case inFor:
			id.Role = roleDecl
		case t.Text == "let" && !isPunct(n1, ok1, ":"):
			id.Role = roleKeyword
		case hasPrev && prev.Kind == tkIdent && prev.Text == "let" && !(i >= 2 && toks[i-2].Kind == tkPunct && toks[i-2].Text == "."):
			id.Role = roleDecl
		case isPunct(n1, ok1, "=") && !(i+2 < len(toks) && toks[i+2].Kind == tkPunct && toks[i+2].Text == "="):
			id.Role = roleDecl // an alias, `X=label` or `X = expr`
		case isPunct(n1, ok1, ":"):
			id.Role = roleLabel
		case (isPunct(n1, ok1, "?") || isPunct(n1, ok1, "!")) && i+2 < len(toks) && toks[i+2].Kind == tkPunct && toks[i+2].Text == ":":
			id.Role = roleLabel
		case hasPrev && prev.Kind == tkPunct && prev.Text == ".":
			id.Role = roleRef // a selector
		case keywords[t.Text]:
			id.Role = roleKeyword
		}
		res.Idents = append(res.Idents, id)
	}
	// imports: a string token that follows `import` or an alias inside an
	// import declaration
	importBlock, importLine = false, false
	for i, t := range toks {
		switch {
		case t.Kind == tkIdent && t.Text == "import" && !(i+1 < len(toks) && toks[i+1].Kind == tkPunct && toks[i+1].Text == ":"):
			if i+1 < len(toks) && toks[i+1].Kind == tkPunct && toks[i+1].Text == "(" {
				importBlock = true
			} else {
				importLine = true
			}
		case t.Kind == tkPunct && t.Text == ")" && importBlock:
			importBlock = false
		case t.Kind == tkString && (importBlock || importLine):
			alias := ""
			var aliasAt token
			if i > 0 && toks[i-1].Kind == tkIdent && toks[i-1].Text != "import" {
				alias, aliasAt = toks[i-1].Text, toks[i-1]
			}
			if t.Value != "" {
				res.Imports = append(res.Imports, importSpec{Path: t.Value, Alias: alias, Line: t.Line, Col: t.Col, Col16: t.Col16, AliasAt: aliasAt})
			}
			importLine = false
		case t.Kind == tkIdent && importLine && !(i+1 < len(toks) && toks[i+1].Kind == tkString):
			importLine = false
		}
	}
	return res
}
