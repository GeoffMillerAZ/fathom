package routes

// django.go — the Django and Django REST framework route reader (LB-F333,
// wave c2 item phppy), and the small Python reader it, fastapi.go and
// flask.go share: a tokenizer (indentation, string prefixes, f-strings,
// implicit joining inside brackets) and a recursive-descent parser for the
// statement and expression subset a urls.py or an application module is
// written in. Nothing is imported or executed.
//
// What the Django reader reads, at module level (and in if/try branches):
//
//   - urlpatterns = [...], += [...], .append(...), .extend(...), and a
//     list bound to another name and concatenated or included;
//   - path(route, view) with its <converter:name> segments as {name};
//     re_path(regex, view) and url(regex, view) with the regex read as a
//     template (anchors dropped, a named group {name}, an unnamed one {});
//   - include('a.b.urls') and include(module) as a Mount under the pattern
//     (a dotted Target, or "./x" for a relative import), include([...]) and
//     include(name) inline, include((patterns, 'app')) by its first item;
//   - a view: a function (handler its name), View.as_view() (the class),
//     a DRF ViewSet.as_view({'get': 'list'}) (one route per method,
//     ViewSet.action), a decorator call around one (csrf_exempt(v));
//   - a DRF router (DefaultRouter, SimpleRouter, or any *Router() a module
//     registers on): register(prefix, ViewSet) makes the list route
//     (GET list, POST create) and the detail route {pk} (GET retrieve, PUT
//     update, PATCH partial_update, DELETE destroy), the way DRF's router
//     maps them; a ViewSet class defined in the same module narrows that to
//     the actions it has (its mixins, or ReadOnlyModelViewSet) and adds its
//     @action extras (url_path, detail, methods); DefaultRouter adds the
//     API root and the .{format} suffix variants, and trailing_slash=False
//     drops the slash.
//
// Methods: Django's URL resolver is method-agnostic, so a function or a
// class-based view is ANY; a viewset route has the methods DRF maps.

import (
	"bytes"
	"regexp"
	"strings"
)

func init() {
	Register(Reader{Framework: "django", Langs: []string{"py"}, Read: readDjango})
}

// ---------------------------------------------------------------------
// Python tokenizer
// ---------------------------------------------------------------------

type pyTokKind uint8

const (
	qtEOF pyTokKind = iota
	qtName
	qtNumber
	qtString
	qtOp
	qtNewline
	qtIndent
	qtDedent
)

// pyStrPart is one piece of a string literal: literal text, or an
// f-string replacement field (dyn, its expression text in expr).
type pyStrPart struct {
	lit  string
	dyn  bool
	expr string
}

type pyTok struct {
	kind  pyTokKind
	text  string
	parts []pyStrPart
	line  int
}

var pyOps = []string{
	"**=", "//=", ">>=", "<<=", "...", "->", ":=", "**", "//", "<<", ">>", "<=", ">=", "==", "!=",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "@=",
}

func isPyIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isPyIdentChar(c byte) bool { return isPyIdentStart(c) || (c >= '0' && c <= '9') }

// lexPython tokenizes src the way Python's tokenizer does, as far as a
// parser of declarations needs: NEWLINE ends a logical line outside
// brackets, INDENT and DEDENT bracket a suite, comments and blank lines are
// dropped.
func lexPython(src []byte) []pyTok {
	var toks []pyTok
	indents := []int{0}
	line := 1
	depth := 0
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
				toks = append(toks, pyTok{kind: qtIndent, line: line})
			}
			for col < indents[len(indents)-1] {
				indents = indents[:len(indents)-1]
				toks = append(toks, pyTok{kind: qtDedent, line: line})
			}
			i = j
			atLineStart = false
		}
		c := src[i]
		switch {
		case c == '\n':
			if depth == 0 {
				toks = append(toks, pyTok{kind: qtNewline, line: line})
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
				parts, ni, nl := lexPyString(src, j, strings.ToLower(word))
				toks = append(toks, pyTok{kind: qtString, parts: parts, line: line})
				line += nl
				i = ni
				continue
			}
			toks = append(toks, pyTok{kind: qtName, text: word, line: line})
			i = j
		case c == '\'' || c == '"':
			parts, ni, nl := lexPyString(src, i, "")
			toks = append(toks, pyTok{kind: qtString, parts: parts, line: line})
			line += nl
			i = ni
		case c >= '0' && c <= '9' || (c == '.' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9'):
			j := i + 1
			for j < n && (isPyIdentChar(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, pyTok{kind: qtNumber, text: string(src[i:j]), line: line})
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
				depth++
			case ")", "]", "}":
				if depth > 0 {
					depth--
				}
			}
			toks = append(toks, pyTok{kind: qtOp, text: op, line: line})
			i += len(op)
		}
	}
	if len(toks) > 0 && toks[len(toks)-1].kind != qtNewline {
		toks = append(toks, pyTok{kind: qtNewline, line: line})
	}
	for len(indents) > 1 {
		indents = indents[:len(indents)-1]
		toks = append(toks, pyTok{kind: qtDedent, line: line})
	}
	return append(toks, pyTok{kind: qtEOF, line: line})
}

// lexPyString reads a string literal at i (its quote) with the given
// prefix letters, returning its parts, the index after it and the number
// of newlines it spans.
func lexPyString(src []byte, i int, prefix string) ([]pyStrPart, int, int) {
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
				break
			}
			if i+2 < n && src[i+1] == q && src[i+2] == q {
				i += 3
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
	if !fstr {
		return []pyStrPart{{lit: decode(string(body))}}, i, nl
	}
	var parts []pyStrPart
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
				parts = append(parts, pyStrPart{lit: decode(lit.String())})
				lit.Reset()
			}
			expr := string(body[k+1 : min(j, len(body))])
			if cut := strings.IndexAny(expr, "!:="); cut > 0 && !strings.Contains(expr[:cut], "[") {
				expr = expr[:cut]
			}
			parts = append(parts, pyStrPart{dyn: true, expr: strings.TrimSpace(expr)})
			k = j
			continue
		}
		lit.WriteByte(c)
	}
	if lit.Len() > 0 {
		parts = append(parts, pyStrPart{lit: decode(lit.String())})
	}
	return parts, i, nl
}

// ---------------------------------------------------------------------
// Python parser
// ---------------------------------------------------------------------

type pyKind uint8

const (
	qOther pyKind = iota
	qName
	qStr
	qNum
	qList
	qTuple
	qDict
	qSet
	qCall
	qAttr
	qSubscript
	qBinOp
	qStarred
	qLambda
	qExprStmt
	qAssign
	qAugAssign
	qImport
	qFrom
	qDef
	qClass
	qBlock
	qReturn
	qIf
)

type pyNode struct {
	kind    pyKind
	name    string // name, attribute, operator, def/class name, module
	parts   []pyStrPart
	x, y    *pyNode
	args    []*pyNode // call args, list/tuple/set items, dict values
	keys    []string  // call keyword names ("" positional, "*", "**")
	dkeys   []*pyNode // dict keys
	targets []*pyNode
	body    []*pyNode
	orelse  []*pyNode
	decos   []*pyNode
	bases   []*pyNode
	names   [][2]string // import (name, alias)
	level   int         // relative import dots
	line    int
}

type pyParser struct {
	toks            []pyTok
	pos             int
	steps, maxSteps int
}

func parsePython(src []byte) []*pyNode {
	toks := lexPython(src)
	p := &pyParser{toks: toks, maxSteps: 64*len(toks) + 1024}
	return p.block(false)
}

func (p *pyParser) peek() pyTok { return p.toks[p.pos] }

func (p *pyParser) peekAt(k int) pyTok {
	if p.pos+k < len(p.toks) {
		return p.toks[p.pos+k]
	}
	return p.toks[len(p.toks)-1]
}

func (p *pyParser) next() pyTok {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	p.steps++
	return t
}

func (p *pyParser) done() bool { return p.steps > p.maxSteps || p.peek().kind == qtEOF }

func (p *pyParser) isOp(s string) bool {
	t := p.peek()
	return t.kind == qtOp && t.text == s
}

func (p *pyParser) isName(s string) bool {
	t := p.peek()
	return t.kind == qtName && t.text == s
}

func (p *pyParser) acceptOp(s string) bool {
	if p.isOp(s) {
		p.next()
		return true
	}
	return false
}

// skipLine skips to the end of the logical line (its NEWLINE consumed).
func (p *pyParser) skipLine() {
	for !p.done() {
		t := p.next()
		if t.kind == qtNewline {
			return
		}
	}
}

// block parses statements until EOF, or the DEDENT closing a suite.
func (p *pyParser) block(suite bool) []*pyNode {
	var out []*pyNode
	for !p.done() {
		t := p.peek()
		if t.kind == qtDedent {
			p.next()
			if suite {
				return out
			}
			continue
		}
		if t.kind == qtNewline || t.kind == qtIndent {
			p.next()
			continue
		}
		start := p.pos
		out = append(out, p.statement()...)
		if p.pos == start {
			p.next()
		}
	}
	return out
}

// suite parses the body after a compound statement's ":".
func (p *pyParser) suite() []*pyNode {
	if p.peek().kind == qtNewline {
		p.next()
		if p.peek().kind != qtIndent {
			return nil
		}
		p.next()
		return p.block(true)
	}
	return p.simpleStatements()
}

var pyCompound = map[string]bool{
	"if": true, "elif": true, "else": true, "while": true, "for": true, "try": true, "except": true,
	"finally": true, "with": true,
}

func (p *pyParser) statement() []*pyNode {
	t := p.peek()
	if t.kind == qtOp && t.text == "@" {
		var decos []*pyNode
		for p.isOp("@") {
			p.next()
			if d := p.expr(); d != nil {
				decos = append(decos, d)
			}
			p.skipLine()
		}
		stmts := p.statement()
		for _, s := range stmts {
			if s.kind == qDef || s.kind == qClass {
				s.decos = append(decos, s.decos...)
			}
		}
		return stmts
	}
	if t.kind == qtName {
		switch t.text {
		case "async":
			if nx := p.peekAt(1); nx.kind == qtName && (nx.text == "def" || nx.text == "for" || nx.text == "with") {
				p.next()
				return p.statement()
			}
		case "def":
			p.next()
			n := &pyNode{kind: qDef, line: t.line}
			if p.peek().kind == qtName {
				n.name = p.next().text
			}
			p.skipHeader()
			n.body = p.suite()
			return []*pyNode{n}
		case "class":
			p.next()
			n := &pyNode{kind: qClass, line: t.line}
			if p.peek().kind == qtName {
				n.name = p.next().text
			}
			if p.isOp("(") {
				p.next()
				for !p.done() && !p.isOp(")") {
					if p.acceptOp(",") {
						continue
					}
					if p.peek().kind == qtName && p.peekAt(1).kind == qtOp && p.peekAt(1).text == "=" {
						p.next()
						p.next()
						p.expr()
						continue
					}
					start := p.pos
					if b := p.expr(); b != nil {
						n.bases = append(n.bases, b)
					}
					if p.pos == start {
						p.next()
					}
				}
				p.acceptOp(")")
			}
			p.skipHeader()
			n.body = p.suite()
			return []*pyNode{n}
		case "if", "elif", "while":
			p.next()
			n := &pyNode{kind: qIf, line: t.line, x: p.expr()}
			p.skipHeader()
			n.body = p.suite()
			for p.peek().kind == qtName && (p.peek().text == "elif" || p.peek().text == "else") {
				if p.peek().text == "else" {
					p.next()
					p.skipHeader()
					n.orelse = append(n.orelse, p.suite()...)
					break
				}
				n.orelse = append(n.orelse, p.statement()...)
			}
			return []*pyNode{n}
		case "match", "case":
			if p.lineEndsWithColon() {
				p.next()
				p.skipHeader()
				return []*pyNode{{kind: qBlock, body: p.suite(), line: t.line}}
			}
		}
		if pyCompound[t.text] {
			p.next()
			p.skipHeader()
			return []*pyNode{{kind: qBlock, body: p.suite(), line: t.line}}
		}
	}
	return p.simpleStatements()
}

// lineEndsWithColon reports whether the current logical line ends with ":"
// (a soft-keyword match/case statement).
func (p *pyParser) lineEndsWithColon() bool {
	for k := p.pos; k < len(p.toks); k++ {
		if p.toks[k].kind == qtNewline || p.toks[k].kind == qtEOF {
			return k > p.pos && p.toks[k-1].kind == qtOp && p.toks[k-1].text == ":"
		}
	}
	return false
}

// skipHeader skips a compound statement's header to its ":".
func (p *pyParser) skipHeader() {
	depth := 0
	for !p.done() {
		t := p.peek()
		if t.kind == qtNewline && depth == 0 {
			return
		}
		p.next()
		if t.kind == qtOp {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case ":":
				if depth == 0 {
					return
				}
			}
		}
	}
}

func (p *pyParser) simpleStatements() []*pyNode {
	var out []*pyNode
	for !p.done() {
		if s := p.simple(); s != nil {
			out = append(out, s)
		}
		if p.acceptOp(";") {
			if p.peek().kind == qtNewline {
				p.next()
				return out
			}
			continue
		}
		if p.peek().kind == qtNewline {
			p.next()
			return out
		}
		p.skipLine()
		return out
	}
	return out
}

func (p *pyParser) simple() *pyNode {
	t := p.peek()
	if t.kind == qtName {
		switch t.text {
		case "import":
			p.next()
			n := &pyNode{kind: qImport, line: t.line}
			for !p.done() && p.peek().kind != qtNewline && !p.isOp(";") {
				if p.acceptOp(",") {
					continue
				}
				mod := p.dotted()
				alias := ""
				if p.isName("as") {
					p.next()
					alias = p.next().text
				}
				if mod == "" {
					p.next()
					continue
				}
				n.names = append(n.names, [2]string{mod, alias})
			}
			return n
		case "from":
			p.next()
			n := &pyNode{kind: qFrom, line: t.line}
			for p.isOp(".") || p.isOp("...") {
				n.level += len(p.next().text)
			}
			if !p.isName("import") {
				n.name = p.dotted()
			}
			if p.isName("import") {
				p.next()
			}
			paren := p.acceptOp("(")
			for !p.done() && !(p.peek().kind == qtNewline && !paren) && !p.isOp(";") {
				if p.acceptOp(",") {
					continue
				}
				if paren && p.acceptOp(")") {
					break
				}
				if p.peek().kind == qtNewline {
					p.next()
					continue
				}
				nm := p.next()
				if nm.kind != qtName && !(nm.kind == qtOp && nm.text == "*") {
					continue
				}
				alias := ""
				if p.isName("as") {
					p.next()
					alias = p.next().text
				}
				n.names = append(n.names, [2]string{nm.text, alias})
			}
			return n
		case "return":
			p.next()
			n := &pyNode{kind: qReturn, line: t.line}
			if p.peek().kind != qtNewline && !p.isOp(";") {
				n.x = p.exprList()
			}
			return n
		case "pass", "break", "continue", "del", "global", "nonlocal", "raise", "assert":
			for !p.done() && p.peek().kind != qtNewline && !p.isOp(";") {
				if p.isOp("(") || p.isOp("[") || p.isOp("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			return nil
		}
	}
	first := p.exprList()
	if first == nil {
		return nil
	}
	if p.isOp(":") { // annotation
		p.next()
		p.expr()
		if !p.isOp("=") {
			return nil
		}
	}
	if p.isOp("=") {
		n := &pyNode{kind: qAssign, line: t.line, targets: []*pyNode{first}}
		for p.acceptOp("=") {
			v := p.exprList()
			n.targets = append(n.targets, v)
		}
		n.x = n.targets[len(n.targets)-1]
		n.targets = n.targets[:len(n.targets)-1]
		return n
	}
	if t := p.peek(); t.kind == qtOp && strings.HasSuffix(t.text, "=") && t.text != "==" && t.text != "<=" && t.text != ">=" && t.text != "!=" {
		p.next()
		return &pyNode{kind: qAugAssign, name: t.text, targets: []*pyNode{first}, x: p.exprList(), line: first.line}
	}
	return &pyNode{kind: qExprStmt, x: first, line: first.line}
}

func (p *pyParser) dotted() string {
	var b strings.Builder
	for p.peek().kind == qtName {
		b.WriteString(p.next().text)
		if !p.isOp(".") {
			break
		}
		p.next()
		b.WriteByte('.')
	}
	return b.String()
}

func (p *pyParser) skipBalanced() {
	depth := 0
	for !p.done() {
		t := p.next()
		if t.kind == qtOp {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			}
		}
		if depth <= 0 {
			return
		}
	}
}

// exprList parses "a, b, c" (a tuple when it has a comma).
func (p *pyParser) exprList() *pyNode {
	first := p.starExpr()
	if first == nil || !p.isOp(",") {
		return first
	}
	n := &pyNode{kind: qTuple, args: []*pyNode{first}, line: first.line}
	for p.acceptOp(",") {
		t := p.peek()
		if t.kind == qtNewline || (t.kind == qtOp && (t.text == "=" || t.text == ")" || t.text == ";" || t.text == ":")) {
			break
		}
		if e := p.starExpr(); e != nil {
			n.args = append(n.args, e)
		} else {
			break
		}
	}
	return n
}

func (p *pyParser) starExpr() *pyNode {
	if p.isOp("*") {
		t := p.next()
		return &pyNode{kind: qStarred, x: p.expr(), line: t.line}
	}
	return p.expr()
}

func (p *pyParser) expr() *pyNode {
	if p.isName("lambda") {
		t := p.next()
		depth := 0
		for !p.done() {
			if p.isOp("(") || p.isOp("[") || p.isOp("{") {
				depth++
			} else if p.isOp(")") || p.isOp("]") || p.isOp("}") {
				depth--
			} else if p.isOp(":") && depth == 0 {
				break
			}
			p.next()
		}
		p.acceptOp(":")
		return &pyNode{kind: qLambda, x: p.expr(), line: t.line}
	}
	x := p.orTest()
	if x == nil {
		return nil
	}
	if p.isName("if") {
		save := p.pos
		p.next()
		cond := p.orTest()
		if p.isName("else") && cond != nil {
			p.next()
			other := p.expr()
			return &pyNode{kind: qBinOp, name: "ifelse", x: x, y: other, line: x.line}
		}
		p.pos = save
	}
	if p.isOp(":=") {
		p.next()
		return p.expr()
	}
	return x
}

func (p *pyParser) orTest() *pyNode {
	x := p.andTest()
	for x != nil && p.isName("or") {
		p.next()
		x = &pyNode{kind: qBinOp, name: "or", x: x, y: p.andTest(), line: x.line}
	}
	return x
}

func (p *pyParser) andTest() *pyNode {
	x := p.notTest()
	for x != nil && p.isName("and") {
		p.next()
		x = &pyNode{kind: qBinOp, name: "and", x: x, y: p.notTest(), line: x.line}
	}
	return x
}

func (p *pyParser) notTest() *pyNode {
	if p.isName("not") {
		t := p.next()
		return &pyNode{kind: qOther, x: p.notTest(), line: t.line}
	}
	return p.comparison()
}

var pyBinPrec = map[string]int{
	"|": 1, "^": 2, "&": 3, "<<": 4, ">>": 4, "+": 5, "-": 5, "*": 6, "/": 6, "//": 6, "%": 6, "@": 6,
}

func (p *pyParser) comparison() *pyNode {
	x := p.binary(1)
	for x != nil {
		t := p.peek()
		isCmp := t.kind == qtOp && (t.text == "<" || t.text == ">" || t.text == "==" || t.text == ">=" || t.text == "<=" || t.text == "!=")
		if t.kind == qtName && (t.text == "in" || t.text == "is" || (t.text == "not" && p.peekAt(1).kind == qtName && p.peekAt(1).text == "in")) {
			isCmp = true
		}
		if !isCmp {
			return x
		}
		p.next()
		if t.text == "not" || (t.text == "is" && p.isName("not")) {
			p.next()
		}
		x = &pyNode{kind: qBinOp, name: "cmp", x: x, y: p.binary(1), line: x.line}
	}
	return x
}

func (p *pyParser) binary(minPrec int) *pyNode {
	x := p.unary()
	for x != nil {
		t := p.peek()
		pr, ok := pyBinPrec[t.text]
		if t.kind != qtOp || !ok || pr < minPrec {
			return x
		}
		p.next()
		y := p.binary(pr + 1)
		x = &pyNode{kind: qBinOp, name: t.text, x: x, y: y, line: x.line}
	}
	return x
}

func (p *pyParser) unary() *pyNode {
	t := p.peek()
	if t.kind == qtOp && (t.text == "-" || t.text == "+" || t.text == "~") {
		p.next()
		return &pyNode{kind: qOther, x: p.unary(), line: t.line}
	}
	if t.kind == qtName && t.text == "await" {
		p.next()
	}
	x := p.postfix(p.atom())
	if x != nil && p.isOp("**") {
		p.next()
		return &pyNode{kind: qBinOp, name: "**", x: x, y: p.unary(), line: x.line}
	}
	return x
}

func (p *pyParser) atom() *pyNode {
	t := p.peek()
	switch t.kind {
	case qtName:
		switch t.text {
		case "if", "else", "for", "in", "and", "or", "not", "is", "lambda", "import", "from", "as", "def", "class", "return", "with", "yield":
			if t.text == "yield" {
				p.next()
				if p.isName("from") {
					p.next()
				}
				return &pyNode{kind: qOther, x: p.exprList(), line: t.line}
			}
			return nil
		}
		p.next()
		return &pyNode{kind: qName, name: t.text, line: t.line}
	case qtNumber:
		p.next()
		return &pyNode{kind: qNum, name: t.text, line: t.line}
	case qtString:
		n := &pyNode{kind: qStr, line: t.line}
		for p.peek().kind == qtString {
			n.parts = append(n.parts, p.next().parts...)
		}
		return n
	case qtOp:
		switch t.text {
		case "(":
			p.next()
			return p.seq(")", qTuple, t.line)
		case "[":
			p.next()
			return p.seq("]", qList, t.line)
		case "{":
			p.next()
			return p.dictOrSet(t.line)
		case "...":
			p.next()
			return &pyNode{kind: qOther, line: t.line}
		}
	}
	return nil
}

// seq parses a parenthesized expression, a tuple or a list; a
// comprehension is qOther.
func (p *pyParser) seq(closer string, kind pyKind, line int) *pyNode {
	n := &pyNode{kind: kind, line: line}
	commas := 0
	for !p.done() && !p.isOp(closer) {
		if p.acceptOp(",") {
			commas++
			continue
		}
		start := p.pos
		e := p.starExpr()
		if p.isName("for") || p.isName("async") {
			for !p.done() && !p.isOp(closer) {
				if p.isOp("(") || p.isOp("[") || p.isOp("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			p.acceptOp(closer)
			return &pyNode{kind: qOther, line: line}
		}
		if e != nil {
			n.args = append(n.args, e)
		} else if p.pos == start {
			p.next()
		}
	}
	p.acceptOp(closer)
	if kind == qTuple && len(n.args) == 1 && commas == 0 {
		return n.args[0]
	}
	return n
}

func (p *pyParser) dictOrSet(line int) *pyNode {
	n := &pyNode{kind: qSet, line: line}
	for !p.done() && !p.isOp("}") {
		if p.acceptOp(",") {
			continue
		}
		if p.acceptOp("**") {
			n.kind = qDict
			v := p.expr()
			n.dkeys = append(n.dkeys, nil)
			n.args = append(n.args, v)
			continue
		}
		start := p.pos
		k := p.starExpr()
		if p.isName("for") || p.isName("async") {
			for !p.done() && !p.isOp("}") {
				if p.isOp("(") || p.isOp("[") || p.isOp("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			p.acceptOp("}")
			return &pyNode{kind: qOther, line: line}
		}
		if p.acceptOp(":") {
			n.kind = qDict
			v := p.expr()
			if p.isName("for") {
				continue
			}
			n.dkeys = append(n.dkeys, k)
			n.args = append(n.args, v)
			continue
		}
		if k != nil {
			n.args = append(n.args, k)
		} else if p.pos == start {
			p.next()
		}
	}
	p.acceptOp("}")
	return n
}

func (p *pyParser) postfix(x *pyNode) *pyNode {
	for x != nil && !p.done() {
		t := p.peek()
		if t.kind != qtOp {
			return x
		}
		switch t.text {
		case ".":
			p.next()
			if p.peek().kind != qtName {
				return x
			}
			x = &pyNode{kind: qAttr, x: x, name: p.next().text, line: x.line}
		case "(":
			p.next()
			x = p.callArgs(x)
		case "[":
			p.next()
			var idx *pyNode
			if !p.isOp("]") {
				idx = p.expr()
			}
			for !p.done() && !p.isOp("]") {
				if p.isOp("(") || p.isOp("[") || p.isOp("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			p.acceptOp("]")
			x = &pyNode{kind: qSubscript, x: x, y: idx, line: x.line}
		default:
			return x
		}
	}
	return x
}

func (p *pyParser) callArgs(fn *pyNode) *pyNode {
	n := &pyNode{kind: qCall, x: fn, line: fn.line}
	for !p.done() && !p.isOp(")") {
		if p.acceptOp(",") {
			continue
		}
		key := ""
		switch {
		case p.isOp("*"):
			p.next()
			key = "*"
		case p.isOp("**"):
			p.next()
			key = "**"
		case p.peek().kind == qtName && p.peekAt(1).kind == qtOp && p.peekAt(1).text == "=":
			key = p.next().text
			p.next()
		}
		start := p.pos
		v := p.expr()
		if p.isName("for") || p.isName("async") {
			for !p.done() && !p.isOp(")") && !p.isOp(",") {
				if p.isOp("(") || p.isOp("[") || p.isOp("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			v = &pyNode{kind: qOther, line: n.line}
		}
		if v == nil && p.pos == start {
			p.next()
			continue
		}
		n.args = append(n.args, v)
		n.keys = append(n.keys, key)
	}
	p.acceptOp(")")
	return n
}

// ---------------------------------------------------------------------
// Python helpers shared by the Django, FastAPI and Flask readers
// ---------------------------------------------------------------------

// pyArg returns a call's keyword argument name, else its positional
// argument at index pos (counting positional arguments only), else nil.
func pyArg(call *pyNode, pos int, name string) *pyNode {
	if call == nil || call.kind != qCall {
		return nil
	}
	k := 0
	for i, a := range call.args {
		switch call.keys[i] {
		case "":
			if k == pos {
				return a
			}
			k++
		case "*", "**":
		default:
			if name != "" && call.keys[i] == name {
				return a
			}
		}
	}
	return nil
}

// pyStr folds a string literal, or a concatenation (+) of literals, to its
// text; an f-string's replacement field makes it non-literal.
func pyStr(n *pyNode) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.kind {
	case qStr:
		var b strings.Builder
		for _, pt := range n.parts {
			if pt.dyn {
				return "", false
			}
			b.WriteString(pt.lit)
		}
		return b.String(), true
	case qBinOp:
		if n.name == "+" {
			a, ok1 := pyStr(n.x)
			b, ok2 := pyStr(n.y)
			return a + b, ok1 && ok2
		}
	}
	return "", false
}

// pyStrings folds a string, or a list/tuple/set of strings.
func pyStrings(n *pyNode) []string {
	if n == nil {
		return nil
	}
	switch n.kind {
	case qList, qTuple, qSet:
		var out []string
		for _, a := range n.args {
			if s, ok := pyStr(a); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := pyStr(n); ok {
		return []string{s}
	}
	return nil
}

// pyDotted renders a name or an attribute chain ("a.b.c"), else "".
func pyDotted(n *pyNode) string {
	switch {
	case n == nil:
		return ""
	case n.kind == qName:
		return n.name
	case n.kind == qAttr:
		if b := pyDotted(n.x); b != "" {
			return b + "." + n.name
		}
	}
	return ""
}

// pyLastName is the last segment of a dotted name.
func pyLastName(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// pyImports maps each name a module binds by import to the module it
// names, as a Compose Target: a dotted module ("a.b.c"), or "./x" and
// "../x" for a relative import (from . import x, from .x import y).
// `from M import N` binds N to M's member N, recorded as the module "M.N"
// (a submodule, or the member of M: pyModuleTarget decides).
type pyImport struct {
	module string // the module holding the name (a Target form)
	member string // the imported member, "" for a module import
}

func pyImports(stmts []*pyNode) map[string]pyImport {
	out := map[string]pyImport{}
	var walk func([]*pyNode)
	walk = func(ns []*pyNode) {
		for _, s := range ns {
			switch s.kind {
			case qImport:
				for _, nm := range s.names {
					if nm[1] != "" {
						out[nm[1]] = pyImport{module: nm[0]}
					} else {
						top := nm[0]
						if i := strings.IndexByte(top, '.'); i >= 0 {
							top = top[:i]
						}
						out[top] = pyImport{module: top}
					}
				}
			case qFrom:
				mod := s.name
				if s.level > 0 {
					rel := "."
					if s.level > 1 {
						rel = strings.TrimSuffix(strings.Repeat("../", s.level-1), "/")
					}
					if mod != "" {
						mod = rel + "/" + strings.ReplaceAll(mod, ".", "/")
					} else {
						mod = rel
					}
				}
				for _, nm := range s.names {
					alias := nm[1]
					if alias == "" {
						alias = nm[0]
					}
					out[alias] = pyImport{module: mod, member: nm[0]}
				}
			case qIf, qBlock, qDef:
				walk(s.body)
				walk(s.orelse)
			}
		}
	}
	walk(stmts)
	return out
}

// pyModuleTarget is the Mount Target for a module-valued name: an imported
// module, or a member imported from a module (its module when the member
// is an object, its submodule when the member is a module: asSubmodule).
func pyModuleTarget(imp pyImport, asSubmodule bool) string {
	mod := imp.module
	if imp.member != "" && asSubmodule {
		if strings.HasPrefix(mod, ".") {
			mod = strings.TrimSuffix(mod, "/") + "/" + imp.member
		} else {
			mod = mod + "." + imp.member
		}
	}
	if strings.HasPrefix(mod, ".") {
		if mod == "." || mod == ".." {
			return ""
		}
		return normalizeTarget(mod)
	}
	return mod
}

// pyClasses indexes a module's class definitions (top level and nested in
// if/try blocks) by name.
func pyClasses(stmts []*pyNode) map[string]*pyNode {
	out := map[string]*pyNode{}
	var walk func([]*pyNode)
	walk = func(ns []*pyNode) {
		for _, s := range ns {
			switch s.kind {
			case qClass:
				out[s.name] = s
			case qIf, qBlock:
				walk(s.body)
				walk(s.orelse)
			}
		}
	}
	walk(stmts)
	return out
}

// pyMethods lists a class body's methods by name, in order.
func pyMethods(cls *pyNode) []*pyNode {
	var out []*pyNode
	if cls == nil {
		return nil
	}
	for _, s := range cls.body {
		if s.kind == qDef {
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------------
// Django
// ---------------------------------------------------------------------

var (
	djangoConverterRe = regexp.MustCompile(`<(?:[A-Za-z_][A-Za-z0-9_]*:)?([A-Za-z_][A-Za-z0-9_]*)>`)
	djangoGroupRe     = regexp.MustCompile(`^\(\?P<([A-Za-z_][A-Za-z0-9_]*)>`)
)

// djangoRouteTemplate spells a path() route as a template.
func djangoRouteTemplate(route string) string {
	return djangoConverterRe.ReplaceAllString(route, "{$1}")
}

// djangoRegexTemplate spells a re_path() regex as a template: anchors
// dropped, a named group {name}, an unnamed group {}, an escaped character
// itself, an optional slash dropped.
func djangoRegexTemplate(rx string) string {
	s := strings.TrimPrefix(rx, "^")
	s = strings.TrimSuffix(s, `\Z`)
	s = strings.TrimSuffix(s, "$")
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '(':
			name := ""
			if m := djangoGroupRe.FindStringSubmatch(s[i:]); m != nil {
				name = m[1]
			}
			depth := 0
			j := i
			for j < len(s) {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == '(' {
					depth++
				} else if s[j] == ')' {
					depth--
					if depth == 0 {
						break
					}
				}
				j++
			}
			b.WriteString("{" + name + "}")
			i = j + 1
			if i < len(s) && (s[i] == '?' || s[i] == '*' || s[i] == '+') {
				i++
			}
		case c == '\\' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i += 2
		case c == '?' && strings.HasSuffix(b.String(), "/"):
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// djangoJoin joins Django pattern pieces: Django concatenates them.
func djangoJoin(prefix, piece string) string { return prefix + piece }

func djangoPath(p string) string { return "/" + strings.TrimLeft(p, "/") }

// drfRoute is one DRF router route shape.
type drfRoute struct {
	detail  bool
	mapping [][2]string // method, action
}

var drfListRoute = drfRoute{mapping: [][2]string{{"GET", "list"}, {"POST", "create"}}}
var drfDetailRoute = drfRoute{detail: true, mapping: [][2]string{{"GET", "retrieve"}, {"PUT", "update"}, {"PATCH", "partial_update"}, {"DELETE", "destroy"}}}

// drfMixinActions are the actions each DRF mixin and stock viewset
// provides, for a ViewSet class defined in the module.
var drfMixinActions = map[string][]string{
	"ListModelMixin":         {"list"},
	"CreateModelMixin":       {"create"},
	"RetrieveModelMixin":     {"retrieve"},
	"UpdateModelMixin":       {"update", "partial_update"},
	"DestroyModelMixin":      {"destroy"},
	"ModelViewSet":           {"list", "create", "retrieve", "update", "partial_update", "destroy"},
	"ReadOnlyModelViewSet":   {"list", "retrieve"},
	"GenericViewSet":         {},
	"ViewSet":                {},
	"ViewSetMixin":           {},
	"GenericAPIView":         {},
	"APIView":                {},
	"CreateAPIView":          {},
	"ListAPIView":            {},
	"RetrieveAPIView":        {},
	"DestroyAPIView":         {},
	"UpdateAPIView":          {},
	"ListCreateAPIView":      {},
	"RetrieveUpdateAPIView":  {},
	"RetrieveDestroyAPIView": {},
}

type djangoRouter struct {
	defaultRouter bool
	trailing      string
	rootView      string
	registry      []djangoRegistration
}

type djangoRegistration struct {
	prefix  string
	viewset *pyNode
	line    int
}

type djangoReader struct {
	path    string
	imports map[string]pyImport
	classes map[string]*pyNode
	env     map[string]*pyNode
	routers map[string]*djangoRouter
	out     Result
	depth   int
}

func readDjango(path string, src []byte) Result {
	if isTestPath(path) {
		return Result{}
	}
	if !bytes.Contains(src, []byte("urlpatterns")) && !(bytes.Contains(src, []byte("Router(")) && bytes.Contains(src, []byte(".register("))) {
		return Result{}
	}
	stmts := parsePython(src)
	r := &djangoReader{path: path, imports: pyImports(stmts), classes: pyClasses(stmts), env: map[string]*pyNode{}, routers: map[string]*djangoRouter{}}
	var patterns []*pyNode
	r.module(stmts, &patterns)
	for _, p := range patterns {
		r.pattern(p, "")
	}
	return r.out
}

// module evaluates module-level statements, collecting urlpatterns'
// entries in order.
func (r *djangoReader) module(stmts []*pyNode, patterns *[]*pyNode) {
	for _, s := range stmts {
		switch s.kind {
		case qIf, qBlock:
			if s.kind == qIf && djangoDebugOnly(s.x) {
				r.module(s.orelse, patterns)
				continue
			}
			r.module(s.body, patterns)
			r.module(s.orelse, patterns)
		case qAssign:
			for _, t := range s.targets {
				if t.kind == qAttr && t.name == "APIRootView" {
					if rt := r.routers[pyDotted(t.x)]; rt != nil {
						rt.rootView = pyLastName(pyDotted(s.x))
					}
					continue
				}
				if t.kind != qName {
					continue
				}
				if t.name == "urlpatterns" {
					*patterns = r.patternList(s.x, nil)
					continue
				}
				r.bind(t.name, s.x)
			}
		case qAugAssign:
			if len(s.targets) == 1 && s.targets[0].kind == qName && s.name == "+=" {
				name := s.targets[0].name
				if name == "urlpatterns" {
					*patterns = append(*patterns, r.patternList(s.x, nil)...)
					continue
				}
				if prev, ok := r.env[name]; ok {
					r.env[name] = &pyNode{kind: qBinOp, name: "+", x: prev, y: s.x, line: s.line}
				}
			}
		case qExprStmt:
			r.exprStmt(s.x, patterns)
		}
	}
}

// djangoDebugOnly reports a test of settings.DEBUG (a development-only
// branch: its patterns are not served in production).
func djangoDebugOnly(test *pyNode) bool {
	d := pyDotted(test)
	return d == "settings.DEBUG" || d == "DEBUG"
}

func (r *djangoReader) bind(name string, v *pyNode) {
	if v == nil {
		return
	}
	if v.kind == qCall {
		fn := pyDotted(v.x)
		if strings.HasSuffix(pyLastName(fn), "Router") {
			rt := &djangoRouter{defaultRouter: !strings.Contains(pyLastName(fn), "Simple"), trailing: "/"}
			if ts := pyArg(v, -1, "trailing_slash"); ts != nil {
				if ts.kind == qName && ts.name == "False" {
					rt.trailing = ""
				} else if s, ok := pyStr(ts); ok {
					rt.trailing = s
				}
			}
			r.routers[name] = rt
			return
		}
	}
	r.env[name] = v
}

func (r *djangoReader) exprStmt(e *pyNode, patterns *[]*pyNode) {
	if e == nil || e.kind != qCall || e.x == nil || e.x.kind != qAttr {
		return
	}
	recv := pyDotted(e.x.x)
	switch e.x.name {
	case "register":
		rt := r.routers[recv]
		if rt == nil {
			return
		}
		prefix, ok := pyStr(pyArg(e, 0, "prefix"))
		if !ok {
			return
		}
		rt.registry = append(rt.registry, djangoRegistration{prefix: prefix, viewset: pyArg(e, 1, "viewset"), line: e.line})
	case "append", "extend":
		arg := pyArg(e, 0, "")
		if arg == nil {
			return
		}
		if e.x.name == "append" {
			arg = &pyNode{kind: qList, args: []*pyNode{arg}, line: e.line}
		}
		if recv == "urlpatterns" {
			*patterns = append(*patterns, r.patternList(arg, nil)...)
			return
		}
		if prev, ok := r.env[recv]; ok {
			r.env[recv] = &pyNode{kind: qBinOp, name: "+", x: prev, y: arg, line: e.line}
		}
	}
}

// patternList resolves an expression to its pattern entries: a list or a
// tuple literal, a name bound to one, a concatenation, a router's urls,
// format_suffix_patterns(x).
func (r *djangoReader) patternList(e *pyNode, seen map[string]bool) []*pyNode {
	if e == nil {
		return nil
	}
	switch e.kind {
	case qList, qTuple:
		return e.args
	case qName:
		if seen == nil {
			seen = map[string]bool{}
		}
		if seen[e.name] {
			return nil
		}
		seen[e.name] = true
		if v, ok := r.env[e.name]; ok {
			return r.patternList(v, seen)
		}
	case qBinOp:
		if e.name == "+" {
			return append(r.patternList(e.x, seen), r.patternList(e.y, seen)...)
		}
	case qAttr:
		if e.name == "urls" {
			if _, ok := r.routers[pyDotted(e.x)]; ok {
				return []*pyNode{e}
			}
		}
	case qCall:
		if pyLastName(pyDotted(e.x)) == "format_suffix_patterns" {
			return r.patternList(pyArg(e, 0, "urlpatterns"), seen)
		}
		return []*pyNode{e}
	}
	return nil
}

// pattern reads one urlpatterns entry under prefix.
func (r *djangoReader) pattern(e *pyNode, prefix string) {
	if e == nil || r.depth > 16 {
		return
	}
	if e.kind == qAttr && e.name == "urls" {
		if rt := r.routers[pyDotted(e.x)]; rt != nil {
			r.routerRoutes(rt, prefix)
		}
		return
	}
	if e.kind != qCall {
		return
	}
	fn := pyLastName(pyDotted(e.x))
	view := pyArg(e, 1, "view")
	if view == nil {
		return
	}
	isInclude := view.kind == qCall && pyLastName(pyDotted(view.x)) == "include"
	var piece string
	switch fn {
	case "path":
		route := pyArg(e, 0, "route")
		s, ok := pyStr(route)
		if !ok {
			// A mount point named by a setting (path(settings.BASE_PATH,
			// include(...))) is read at the root, the setting's default;
			// any other non-literal route is skipped.
			if !isInclude || pyDotted(route) == "" {
				return
			}
			s = ""
		}
		piece = djangoRouteTemplate(s)
	case "re_path", "url":
		s, ok := pyStr(pyArg(e, 0, "route"))
		if !ok {
			s, ok = pyStr(pyArg(e, 0, "regex"))
		}
		if !ok {
			return
		}
		piece = djangoRegexTemplate(s)
	default:
		return
	}
	full := djangoJoin(prefix, piece)
	if isInclude {
		r.include(view, full, e.line)
		return
	}
	r.view(view, full, e.line)
}

// include reads include(...) under prefix: a module (a Mount), a list or a
// name bound to one (inline), a router's urls, a (patterns, app) tuple.
func (r *djangoReader) include(call *pyNode, prefix string, line int) {
	arg := pyArg(call, 0, "arg")
	if arg != nil && arg.kind == qTuple && len(arg.args) > 0 {
		arg = arg.args[0]
	}
	if arg == nil {
		return
	}
	if s, ok := pyStr(arg); ok {
		r.out.Mounts = append(r.out.Mounts, Mount{Prefix: djangoPath(prefix), Target: s, File: r.path, Line: line})
		return
	}
	if arg.kind == qName {
		if imp, ok := r.imports[arg.name]; ok {
			if t := pyModuleTarget(imp, true); t != "" {
				r.out.Mounts = append(r.out.Mounts, Mount{Prefix: djangoPath(prefix), Target: t, File: r.path, Line: line})
			}
			return
		}
	}
	if arg.kind == qAttr && arg.name == "urls" {
		if rt := r.routers[pyDotted(arg.x)]; rt != nil {
			r.routerRoutes(rt, prefix)
			return
		}
	}
	if arg.kind == qAttr && arg.name == "urlpatterns" {
		if imp, ok := r.imports[pyDotted(arg.x)]; ok {
			if t := pyModuleTarget(imp, true); t != "" {
				r.out.Mounts = append(r.out.Mounts, Mount{Prefix: djangoPath(prefix), Target: t, File: r.path, Line: line})
			}
		}
		return
	}
	r.depth++
	for _, p := range r.patternList(arg, nil) {
		r.pattern(p, prefix)
	}
	r.depth--
}

// view emits the route(s) one view expression serves at full.
func (r *djangoReader) view(v *pyNode, full string, line int) {
	for k := 0; k < 4 && v != nil && v.kind == qName; k++ {
		bound, ok := r.env[v.name]
		if !ok {
			break
		}
		v = bound
	}
	if v == nil {
		return
	}
	switch v.kind {
	case qName, qAttr:
		if name := pyLastName(pyDotted(v)); name != "" {
			r.out.Routes = append(r.out.Routes, Route{Method: "ANY", Path: djangoPath(full), Handler: name, File: r.path, Line: line})
		}
	case qCall:
		callee := v.x
		if callee != nil && callee.kind == qAttr && callee.name == "as_view" {
			cls := pyLastName(pyDotted(callee.x))
			if cls == "" {
				return
			}
			if actions := pyArg(v, 0, "actions"); actions != nil && actions.kind == qDict {
				for i, k := range actions.dkeys {
					m, ok1 := pyStr(k)
					a, ok2 := pyStr(actions.args[i])
					if ok1 && ok2 {
						r.out.Routes = append(r.out.Routes, Route{Method: strings.ToUpper(m), Path: djangoPath(full), Handler: cls + "." + a, File: r.path, Line: line})
					}
				}
				return
			}
			r.out.Routes = append(r.out.Routes, Route{Method: "ANY", Path: djangoPath(full), Handler: cls, File: r.path, Line: line})
			return
		}
		// A decorator applied as a call (csrf_exempt(view), login_required(View.as_view())).
		if inner := pyArg(v, 0, ""); inner != nil && len(v.args) >= 1 {
			r.view(inner, full, line)
		}
	}
}

// routerRoutes expands a DRF router's registrations under prefix, the way
// DRF's SimpleRouter.get_urls() and DefaultRouter build them.
func (r *djangoReader) routerRoutes(rt *djangoRouter, prefix string) {
	if rt.defaultRouter {
		root := rt.rootView
		if root == "" {
			root = "APIRootView"
		}
		line := 0
		if len(rt.registry) > 0 {
			line = rt.registry[0].line
		}
		r.out.Routes = append(r.out.Routes,
			Route{Method: "ANY", Path: djangoPath(prefix), Handler: root, File: r.path, Line: line},
			Route{Method: "ANY", Path: djangoPath(prefix + "{format}"), Handler: root, File: r.path, Line: line})
	}
	for _, reg := range rt.registry {
		vs := pyLastName(pyDotted(reg.viewset))
		if vs == "" {
			continue
		}
		cls := r.classes[vs]
		have := r.viewsetActions(cls)
		base := strings.TrimSuffix(reg.prefix, "/")
		emit := func(path string, method, action string) {
			r.out.Routes = append(r.out.Routes, Route{Method: method, Path: djangoPath(djangoJoin(prefix, path)), Handler: vs + "." + action, File: r.path, Line: reg.line})
			if rt.defaultRouter {
				fp := strings.TrimSuffix(path, "/") + ".{format}"
				r.out.Routes = append(r.out.Routes, Route{Method: method, Path: djangoPath(djangoJoin(prefix, fp)), Handler: vs + "." + action, File: r.path, Line: reg.line})
			}
		}
		for _, shape := range []drfRoute{drfListRoute, drfDetailRoute} {
			path := base + rt.trailing
			if shape.detail {
				path = base + "/{pk}" + rt.trailing
			}
			for _, ma := range shape.mapping {
				if have != nil && !have[ma[1]] {
					continue
				}
				emit(path, ma[0], ma[1])
			}
		}
		for _, x := range r.viewsetExtras(cls) {
			path := base + "/" + x.urlPath + rt.trailing
			if x.detail {
				path = base + "/{pk}/" + x.urlPath + rt.trailing
			}
			for _, m := range x.methods {
				emit(path, m, x.name)
			}
		}
	}
}

// viewsetActions returns the standard actions a ViewSet class defined in
// the module provides (its own methods, its stock bases' mixins), or nil
// when the class is not in the module or a base is not known (all of them
// are then assumed, as a ModelViewSet).
func (r *djangoReader) viewsetActions(cls *pyNode) map[string]bool {
	if cls == nil {
		return nil
	}
	out := map[string]bool{}
	for _, m := range pyMethods(cls) {
		out[m.name] = true
	}
	for _, b := range cls.bases {
		name := pyLastName(pyDotted(b))
		acts, ok := drfMixinActions[name]
		if !ok {
			if local := r.classes[name]; local != nil && local != cls {
				for a := range r.viewsetActions(local) {
					out[a] = true
				}
				continue
			}
			return nil
		}
		for _, a := range acts {
			out[a] = true
		}
	}
	return out
}

type drfExtra struct {
	name    string
	urlPath string
	detail  bool
	methods []string
}

// viewsetExtras reads a ViewSet's @action methods.
func (r *djangoReader) viewsetExtras(cls *pyNode) []drfExtra {
	var out []drfExtra
	for _, m := range pyMethods(cls) {
		for _, d := range m.decos {
			if d.kind != qCall || pyLastName(pyDotted(d.x)) != "action" {
				continue
			}
			x := drfExtra{name: m.name, urlPath: m.name, methods: []string{"GET"}}
			if det := pyArg(d, -1, "detail"); det != nil && det.kind == qName && det.name == "True" {
				x.detail = true
			}
			if up, ok := pyStr(pyArg(d, -1, "url_path")); ok {
				x.urlPath = up
			}
			if ms := pyStrings(pyArg(d, -1, "methods")); len(ms) > 0 {
				x.methods = nil
				for _, mm := range ms {
					x.methods = append(x.methods, strings.ToUpper(mm))
				}
			}
			out = append(out, x)
		}
	}
	return out
}
