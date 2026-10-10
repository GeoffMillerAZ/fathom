package routes

// laravel.go — the Laravel route reader (LB-F331, wave c2 item phppy), and
// the small PHP reader it and symfony.go share: a lexer (strings, heredocs,
// comments, attributes, inline HTML) and a recursive-descent parser for the
// expression and statement subset a route file is written in. Nothing is
// executed: a route is read from the AST, with a group's attributes (prefix,
// controller) carried down its closure, and a loop over a literal array
// unrolled (the one evaluation the reader does, bounded).
//
// What the reader reads, on the facade `Route::` or a `$router` variable:
//
//   - get, post, put, patch, delete, options (one method each; Laravel's
//     automatic HEAD beside GET is not a row), any (ANY), match([...]);
//   - view (GET, ViewController.__invoke), redirect and permanentRedirect
//     (ANY, RedirectController.__invoke), fallback (GET {fallbackPlaceholder});
//   - resource and apiResource (7 and 5 actions, update as PUT and PATCH),
//     resources and apiResources, singleton and apiSingleton (creatable,
//     destroyable), honouring only/except chained or as options, nested
//     "photos.comments" names;
//   - group nesting: prefix()->group(closure), group(['prefix' => ...],
//     closure), controller()->group(closure) (a string action is then a
//     method of that controller), and any other attribute call before group;
//   - a group of a file (group(base_path('routes/x.php')), group(__DIR__.
//     '/x.php')) and a require/include of a route file inside a group are
//     Mounts under the group's prefix;
//   - bootstrap/app.php withRouting(web:, api:, apiPrefix:, then:, using:):
//     the api file(s) mounted under apiPrefix (default "api"), the web
//     file(s) under "", the closures read as route code;
//   - a handler [Controller::class, 'method'] or 'Controller@method' is
//     Controller.method (the short class name, as the PHP dialect names a
//     method), Controller::class is Controller.__invoke, a closure is "".
//
// A path built from a non-literal (a config() call, a variable) skips the
// route: precision over recall.

import (
	"bytes"
	"path"
	"strconv"
	"strings"
)

func init() {
	Register(Reader{Framework: "laravel", Langs: []string{"php"}, Read: readLaravel})
}

// ---------------------------------------------------------------------
// PHP lexer
// ---------------------------------------------------------------------

type phpTokKind uint8

const (
	ptEOF phpTokKind = iota
	ptIdent
	ptVar
	ptString
	ptNumber
	ptPunct
	ptAttrOpen // "#[", an attribute group's opener
)

// phpStrPart is one piece of a string literal: literal text, or (in a
// double-quoted string) an interpolated simple variable ($x, {$x}, ${x}).
// A complex interpolation ({$a->b}, $a[0]) is a part with dyn set.
type phpStrPart struct {
	lit string
	v   string
	dyn bool
}

type phpTok struct {
	kind  phpTokKind
	text  string // ident/var name (without $), punct, number
	parts []phpStrPart
	line  int
}

// lexPHP tokenizes src. Text outside <?php ... ?> is skipped.
func lexPHP(src []byte) []phpTok {
	var toks []phpTok
	line := 1
	i, n := 0, len(src)
	inPHP := false
	for i < n {
		if !inPHP {
			j := bytes.Index(src[i:], []byte("<?"))
			if j < 0 {
				break
			}
			line += bytes.Count(src[i:i+j], []byte("\n"))
			i += j + 2
			if bytes.HasPrefix(src[i:], []byte("php")) {
				i += 3
			} else if i < n && src[i] == '=' {
				i++
			}
			inPHP = true
			continue
		}
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '?' && i+1 < n && src[i+1] == '>':
			inPHP = false
			i += 2
			toks = append(toks, phpTok{kind: ptPunct, text: ";", line: line})
		case c == '#' && i+1 < n && src[i+1] == '[':
			toks = append(toks, phpTok{kind: ptAttrOpen, text: "#[", line: line})
			i += 2
		case c == '#' || (c == '/' && i+1 < n && src[i+1] == '/'):
			for i < n && src[i] != '\n' {
				if src[i] == '?' && i+1 < n && src[i+1] == '>' {
					break
				}
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			j := bytes.Index(src[i+2:], []byte("*/"))
			end := n
			if j >= 0 {
				end = i + 2 + j + 2
			}
			line += bytes.Count(src[i:end], []byte("\n"))
			i = end
		case c == '\'':
			start := line
			var b strings.Builder
			i++
			for i < n && src[i] != '\'' {
				if src[i] == '\\' && i+1 < n && (src[i+1] == '\'' || src[i+1] == '\\') {
					b.WriteByte(src[i+1])
					i += 2
					continue
				}
				if src[i] == '\n' {
					line++
				}
				b.WriteByte(src[i])
				i++
			}
			i++
			toks = append(toks, phpTok{kind: ptString, parts: []phpStrPart{{lit: b.String()}}, line: start})
		case c == '"' || c == '`':
			start := line
			parts, ni, nl := lexPHPDouble(src, i+1, c)
			line += nl
			i = ni
			toks = append(toks, phpTok{kind: ptString, parts: parts, line: start})
		case c == '<' && bytes.HasPrefix(src[i:], []byte("<<<")):
			start := line
			lit, ni, nl, ok := lexPHPHeredoc(src, i)
			if !ok {
				toks = append(toks, phpTok{kind: ptPunct, text: "<<", line: line})
				i += 2
				continue
			}
			line += nl
			i = ni
			toks = append(toks, phpTok{kind: ptString, parts: []phpStrPart{{lit: lit, dyn: false}}, line: start})
		case c == '$' && i+1 < n && isPHPIdentStart(src[i+1]):
			j := i + 1
			for j < n && isPHPIdentChar(src[j]) {
				j++
			}
			toks = append(toks, phpTok{kind: ptVar, text: string(src[i+1 : j]), line: line})
			i = j
		case isPHPIdentStart(c) || (c == '\\' && i+1 < n && isPHPIdentStart(src[i+1])):
			j := i
			for j < n && (isPHPIdentChar(src[j]) || (src[j] == '\\' && j+1 < n && isPHPIdentStart(src[j+1]))) {
				j++
			}
			toks = append(toks, phpTok{kind: ptIdent, text: string(src[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9' || (c == '.' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9'):
			j := i + 1
			for j < n && (isPHPIdentChar(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, phpTok{kind: ptNumber, text: string(src[i:j]), line: line})
			i = j
		default:
			p := phpPunctAt(src[i:])
			toks = append(toks, phpTok{kind: ptPunct, text: p, line: line})
			i += len(p)
		}
	}
	return append(toks, phpTok{kind: ptEOF, line: line})
}

var phpPuncts = []string{
	"<<=", ">>=", "**=", "...", "<=>", "===", "!==", "??=", "?->",
	"::", "->", "=>", "==", "!=", "<>", "<=", ">=", "&&", "||", "??", "++", "--",
	"+=", "-=", "*=", "/=", ".=", "%=", "&=", "|=", "^=", "<<", ">>", "**",
}

func phpPunctAt(b []byte) string {
	for _, p := range phpPuncts {
		if bytes.HasPrefix(b, []byte(p)) {
			return p
		}
	}
	return string(b[:1])
}

func isPHPIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isPHPIdentChar(c byte) bool { return isPHPIdentStart(c) || (c >= '0' && c <= '9') }

// lexPHPDouble reads a double-quoted (or backtick) string body from i,
// decoding escapes and splitting out simple interpolations.
func lexPHPDouble(src []byte, i int, quote byte) ([]phpStrPart, int, int) {
	var parts []phpStrPart
	var b strings.Builder
	nl := 0
	n := len(src)
	flush := func() {
		if b.Len() > 0 {
			parts = append(parts, phpStrPart{lit: b.String()})
			b.Reset()
		}
	}
	for i < n && src[i] != quote {
		c := src[i]
		switch {
		case c == '\\' && i+1 < n:
			switch e := src[i+1]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\', '$', '"', '`':
				b.WriteByte(e)
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
			i += 2
		case c == '$' && i+1 < n && isPHPIdentStart(src[i+1]):
			j := i + 1
			for j < n && isPHPIdentChar(src[j]) {
				j++
			}
			flush()
			name := string(src[i+1 : j])
			dyn := j < n && (src[j] == '[' || (src[j] == '-' && j+1 < n && src[j+1] == '>'))
			parts = append(parts, phpStrPart{v: name, dyn: dyn})
			i = j
		case c == '{' && i+1 < n && src[i+1] == '$', c == '$' && i+1 < n && src[i+1] == '{':
			depth := 0
			j := i
			for j < n {
				if src[j] == '{' {
					depth++
				} else if src[j] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				j++
			}
			inner := strings.TrimSpace(string(src[i+1 : min(j, n)]))
			if c == '$' {
				inner = strings.TrimSpace(string(src[i+2 : min(j, n)]))
			}
			inner = strings.TrimPrefix(inner, "$")
			flush()
			simple := inner != "" && strings.IndexFunc(inner, func(r rune) bool {
				return !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
			}) < 0
			parts = append(parts, phpStrPart{v: inner, dyn: !simple})
			i = j + 1
		default:
			if c == '\n' {
				nl++
			}
			b.WriteByte(c)
			i++
		}
	}
	flush()
	return parts, i + 1, nl
}

// lexPHPHeredoc reads a heredoc or nowdoc starting at i ("<<<"); its body
// is one literal (an interpolation inside one is not a route path).
func lexPHPHeredoc(src []byte, i int) (string, int, int, bool) {
	j := i + 3
	for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
		j++
	}
	q := byte(0)
	if j < len(src) && (src[j] == '\'' || src[j] == '"') {
		q = src[j]
		j++
	}
	k := j
	for k < len(src) && isPHPIdentChar(src[k]) {
		k++
	}
	if k == j {
		return "", 0, 0, false
	}
	label := string(src[j:k])
	if q != 0 {
		k++
	}
	eol := bytes.IndexByte(src[k:], '\n')
	if eol < 0 {
		return "", 0, 0, false
	}
	body := k + eol + 1
	nl := 1
	p := body
	for p < len(src) {
		e := bytes.IndexByte(src[p:], '\n')
		lineEnd := len(src)
		if e >= 0 {
			lineEnd = p + e
		}
		t := bytes.TrimLeft(src[p:lineEnd], " \t")
		if bytes.HasPrefix(t, []byte(label)) && (len(t) == len(label) || !isPHPIdentChar(t[len(label)])) {
			end := lineEnd - len(t) + len(label)
			lit := ""
			if p > body {
				lit = string(src[body : p-1])
			}
			return lit, end, nl, true
		}
		if e < 0 {
			break
		}
		nl++
		p = lineEnd + 1
	}
	return "", 0, 0, false
}

// ---------------------------------------------------------------------
// PHP parser: the expression and statement subset route code is written in
// ---------------------------------------------------------------------

type phpKind uint8

const (
	pkOther phpKind = iota
	pkName
	pkVar
	pkStr
	pkNum
	pkArray
	pkCall       // x(args)
	pkStaticCall // x::name(args)
	pkMethodCall // x->name(args)
	pkProp       // x->name
	pkClassConst // x::name (X::class, X::CONST)
	pkClosure    // function () use () { body } / fn () => expr
	pkConcat     // x . y
	pkAssign     // x = y (and compound)
	pkNew        // new x(args)
	pkIndex      // x[y]
	pkRequire    // require/include x
	pkExprStmt   // x;
	pkBlock      // { body }
	pkForeach    // foreach (x as key => val) body
	pkReturn     // return x;
	pkFunc       // function name() { body } (also a method)
	pkClass      // class name { body }
	pkUse        // use A\B as C; (name = alias, str = target)
	pkNamespace  // namespace X;
)

type phpNode struct {
	kind  phpKind
	name  string
	parts []phpStrPart
	x, y  *phpNode
	args  []*phpNode // call args, array values, closure/arrow body as one expr
	keys  []*phpNode // array keys / named-arg names (pkName), nil where positional
	body  []*phpNode // statements
	attrs []*phpNode // attribute expressions on a class or method
	line  int
	key   string // foreach key variable
	val   string // foreach value variable
	extra string // class: parent; func: "method" for a class member
}

type phpParser struct {
	toks []phpTok
	pos  int
	// steps bounds the parse: a pathological file costs at most this many
	// token visits per token before the parser gives up on it.
	steps, maxSteps int
}

func parsePHP(src []byte) []*phpNode {
	toks := lexPHP(src)
	p := &phpParser{toks: toks, maxSteps: 64*len(toks) + 1024}
	return p.statements(false)
}

func (p *phpParser) peek() phpTok { return p.toks[p.pos] }

func (p *phpParser) peekAt(k int) phpTok {
	if p.pos+k < len(p.toks) {
		return p.toks[p.pos+k]
	}
	return p.toks[len(p.toks)-1]
}

func (p *phpParser) next() phpTok {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	p.steps++
	return t
}

func (p *phpParser) exhausted() bool { return p.steps > p.maxSteps || p.peek().kind == ptEOF }

func (p *phpParser) isPunct(s string) bool {
	t := p.peek()
	return t.kind == ptPunct && t.text == s
}

func (p *phpParser) isIdent(s string) bool {
	t := p.peek()
	return t.kind == ptIdent && strings.EqualFold(t.text, s)
}

func (p *phpParser) accept(s string) bool {
	if p.isPunct(s) {
		p.next()
		return true
	}
	return false
}

// skipBalanced skips one bracketed group starting at the current opener
// ("(", "[", "{" or "#[").
func (p *phpParser) skipBalanced() {
	depth := 0
	for !p.exhausted() {
		t := p.next()
		switch {
		case t.kind == ptAttrOpen, t.kind == ptPunct && (t.text == "(" || t.text == "[" || t.text == "{"):
			depth++
		case t.kind == ptPunct && (t.text == ")" || t.text == "]" || t.text == "}"):
			depth--
			if depth <= 0 {
				return
			}
		}
		if depth == 0 {
			return
		}
	}
}

// skipStatement skips to the end of the current statement (a ";" at depth 0,
// or before a "}" that closes the enclosing block).
func (p *phpParser) skipStatement() {
	for !p.exhausted() {
		t := p.peek()
		if t.kind == ptPunct {
			switch t.text {
			case ";":
				p.next()
				return
			case "}":
				return
			case "(", "[", "{":
				p.skipBalanced()
				continue
			}
		}
		if t.kind == ptAttrOpen {
			p.skipBalanced()
			continue
		}
		p.next()
	}
}

// statements parses until EOF, or the closing "}" when inBlock.
func (p *phpParser) statements(inBlock bool) []*phpNode {
	var out []*phpNode
	for !p.exhausted() {
		if p.isPunct("}") {
			if inBlock {
				p.next()
				return out
			}
			p.next()
			continue
		}
		start := p.pos
		if s := p.statement(); s != nil {
			out = append(out, s)
		}
		if p.pos == start {
			p.next()
		}
	}
	return out
}

func (p *phpParser) attributes() []*phpNode {
	var out []*phpNode
	for p.peek().kind == ptAttrOpen {
		p.next()
		for !p.exhausted() && !p.isPunct("]") {
			start := p.pos
			if e := p.expr(); e != nil {
				out = append(out, e)
			}
			if !p.accept(",") && p.pos == start {
				p.next()
			}
		}
		p.accept("]")
	}
	return out
}

var phpModifiers = map[string]bool{
	"public": true, "protected": true, "private": true, "static": true, "abstract": true,
	"final": true, "readonly": true, "var": true,
}

func (p *phpParser) statement() *phpNode {
	t := p.peek()
	line := t.line
	if t.kind == ptAttrOpen {
		attrs := p.attributes()
		s := p.statement()
		if s != nil && (s.kind == pkClass || s.kind == pkFunc) {
			s.attrs = append(attrs, s.attrs...)
		}
		return s
	}
	if t.kind == ptPunct {
		switch t.text {
		case ";":
			p.next()
			return nil
		case "{":
			p.next()
			return &phpNode{kind: pkBlock, body: p.statements(true), line: line}
		}
	}
	if t.kind == ptIdent {
		switch strings.ToLower(t.text) {
		case "namespace":
			if p.peekAt(1).kind == ptPunct && p.peekAt(1).text == "\\" {
				break
			}
			p.next()
			name := ""
			if p.peek().kind == ptIdent {
				name = p.next().text
			}
			if p.isPunct("{") {
				p.next()
				return &phpNode{kind: pkBlock, body: p.statements(true), line: line, name: name}
			}
			p.accept(";")
			return &phpNode{kind: pkNamespace, name: name, line: line}
		case "use":
			p.next()
			return p.useStatement(line)
		case "abstract", "final", "readonly", "class", "interface", "trait", "enum":
			if s := p.classDecl(); s != nil {
				return s
			}
		case "function":
			if p.peekAt(1).kind == ptIdent || (p.peekAt(1).kind == ptPunct && p.peekAt(1).text == "&" && p.peekAt(2).kind == ptIdent) {
				p.next()
				p.accept("&")
				name := p.next().text
				body := p.funcRest()
				return &phpNode{kind: pkFunc, name: name, body: body, line: line}
			}
		case "if", "elseif", "while", "for", "switch", "declare":
			p.next()
			if p.isPunct("(") {
				p.skipBalanced()
			}
			return p.controlBody(line)
		case "else", "do", "try", "finally":
			p.next()
			if strings.EqualFold(t.text, "else") && p.isIdent("if") {
				return p.statement()
			}
			return p.controlBody(line)
		case "catch":
			p.next()
			if p.isPunct("(") {
				p.skipBalanced()
			}
			return p.controlBody(line)
		case "foreach":
			p.next()
			return p.foreachStmt(line)
		case "return":
			p.next()
			var x *phpNode
			if !p.isPunct(";") {
				x = p.expr()
			}
			p.skipStatement()
			return &phpNode{kind: pkReturn, x: x, line: line}
		case "endif", "endforeach", "endwhile", "endfor", "endswitch", "enddeclare":
			p.skipStatement()
			return nil
		case "case", "default":
			for !p.exhausted() && !p.isPunct(":") && !p.isPunct(";") {
				p.next()
			}
			p.next()
			return nil
		}
	}
	x := p.expr()
	if x == nil {
		p.skipStatement()
		return nil
	}
	if !p.accept(";") {
		p.skipStatement()
	}
	return &phpNode{kind: pkExprStmt, x: x, line: line}
}

// controlBody parses the body of a control structure: a block, an
// alternative-syntax ":" block, or one statement.
func (p *phpParser) controlBody(line int) *phpNode {
	if p.accept(":") {
		var body []*phpNode
		for !p.exhausted() {
			if p.peek().kind == ptIdent {
				switch strings.ToLower(p.peek().text) {
				case "endif", "endforeach", "endwhile", "endfor", "endswitch", "enddeclare", "else", "elseif":
					return &phpNode{kind: pkBlock, body: body, line: line}
				}
			}
			start := p.pos
			if s := p.statement(); s != nil {
				body = append(body, s)
			}
			if p.pos == start {
				p.next()
			}
		}
		return &phpNode{kind: pkBlock, body: body, line: line}
	}
	if p.accept("{") {
		return &phpNode{kind: pkBlock, body: p.statements(true), line: line}
	}
	s := p.statement()
	if s == nil {
		return nil
	}
	return &phpNode{kind: pkBlock, body: []*phpNode{s}, line: line}
}

func (p *phpParser) foreachStmt(line int) *phpNode {
	if !p.accept("(") {
		return nil
	}
	iter := p.expr()
	n := &phpNode{kind: pkForeach, x: iter, line: line}
	if p.isIdent("as") {
		p.next()
		p.accept("&")
		if p.peek().kind == ptVar {
			first := p.next().text
			if p.accept("=>") {
				p.accept("&")
				n.key = first
				if p.peek().kind == ptVar {
					n.val = p.next().text
				}
			} else {
				n.val = first
			}
		}
	}
	for !p.exhausted() && !p.isPunct(")") {
		if p.isPunct("(") || p.isPunct("[") {
			p.skipBalanced()
			continue
		}
		p.next()
	}
	p.accept(")")
	if b := p.controlBody(line); b != nil {
		n.body = b.body
	}
	return n
}

// useStatement parses a top-level import ("use A\B [as C], D;", grouped
// "use A\{B, C as D};", "use function ...") into pkUse nodes in a block.
func (p *phpParser) useStatement(line int) *phpNode {
	if p.isIdent("function") || p.isIdent("const") {
		p.next()
	}
	blk := &phpNode{kind: pkBlock, line: line}
	for !p.exhausted() && !p.isPunct(";") {
		if p.peek().kind != ptIdent {
			p.next()
			continue
		}
		name := strings.TrimPrefix(p.next().text, "\\")
		if p.isPunct("\\") || (strings.HasSuffix(name, "\\") || p.isPunct("{")) {
			p.accept("\\")
			if p.accept("{") {
				base := strings.TrimSuffix(name, "\\")
				for !p.exhausted() && !p.isPunct("}") {
					if p.peek().kind != ptIdent {
						p.next()
						continue
					}
					sub := p.next().text
					alias := shortPHPName(sub)
					if p.isIdent("as") {
						p.next()
						alias = p.next().text
					}
					blk.body = append(blk.body, &phpNode{kind: pkUse, name: alias, extra: base + "\\" + sub, line: line})
				}
				p.accept("}")
				continue
			}
		}
		alias := shortPHPName(name)
		if p.isIdent("as") {
			p.next()
			alias = p.next().text
		}
		blk.body = append(blk.body, &phpNode{kind: pkUse, name: alias, extra: name, line: line})
	}
	p.accept(";")
	return blk
}

// classDecl parses a class-like declaration; its body holds its methods
// (pkFunc with extra "method", attributes attached).
func (p *phpParser) classDecl() *phpNode {
	save := p.pos
	for p.peek().kind == ptIdent && (strings.EqualFold(p.peek().text, "abstract") || strings.EqualFold(p.peek().text, "final") || strings.EqualFold(p.peek().text, "readonly")) {
		p.next()
	}
	if p.peek().kind != ptIdent {
		p.pos = save
		return nil
	}
	kw := strings.ToLower(p.peek().text)
	if kw != "class" && kw != "interface" && kw != "trait" && kw != "enum" {
		p.pos = save
		return nil
	}
	line := p.next().line
	if p.peek().kind != ptIdent {
		p.pos = save
		return nil
	}
	n := &phpNode{kind: pkClass, name: p.next().text, line: line}
	for !p.exhausted() && !p.isPunct("{") {
		if p.isIdent("extends") {
			p.next()
			if p.peek().kind == ptIdent {
				n.extra = p.next().text
			}
			continue
		}
		p.next()
	}
	if !p.accept("{") {
		return n
	}
	n.body = p.classBody()
	return n
}

func (p *phpParser) classBody() []*phpNode {
	var out []*phpNode
	for !p.exhausted() {
		if p.accept("}") {
			return out
		}
		start := p.pos
		attrs := p.attributes()
		for p.peek().kind == ptIdent && phpModifiers[strings.ToLower(p.peek().text)] {
			p.next()
		}
		if p.isIdent("function") {
			line := p.next().line
			p.accept("&")
			name := ""
			if p.peek().kind == ptIdent {
				name = p.next().text
			}
			body := p.funcRest()
			out = append(out, &phpNode{kind: pkFunc, name: name, body: body, attrs: attrs, line: line, extra: "method"})
			continue
		}
		if p.isIdent("use") || p.isIdent("const") || p.isIdent("case") || p.peek().kind == ptVar || p.peek().kind == ptIdent || p.isPunct("?") {
			p.skipStatement()
		}
		if p.pos == start {
			p.next()
		}
	}
	return out
}

// funcRest parses "(params) [: type] { body }" (or ";" for an abstract
// method) and returns the body.
func (p *phpParser) funcRest() []*phpNode {
	if p.isPunct("(") {
		p.skipBalanced()
	}
	for !p.exhausted() && !p.isPunct("{") && !p.isPunct(";") {
		if p.isPunct("(") {
			p.skipBalanced()
			continue
		}
		p.next()
	}
	if p.accept(";") {
		return nil
	}
	if !p.accept("{") {
		return nil
	}
	return p.statements(true)
}

// expr parses one expression (assignment level).
func (p *phpParser) expr() *phpNode { return p.binary(0) }

var phpBinPrec = map[string]int{
	"or": 1, "xor": 2, "and": 3,
	"=": 4, "+=": 4, "-=": 4, "*=": 4, "/=": 4, ".=": 4, "%=": 4, "&=": 4, "|=": 4, "^=": 4, "<<=": 4, ">>=": 4, "**=": 4, "??=": 4,
	"?": 5, "??": 6, "||": 7, "&&": 8, "|": 9, "^": 10, "&": 11,
	"==": 12, "!=": 12, "===": 12, "!==": 12, "<>": 12, "<=>": 12,
	"<": 13, "<=": 13, ">": 13, ">=": 13,
	".": 14, "<<": 15, ">>": 15, "+": 16, "-": 16, "*": 17, "/": 17, "%": 17,
	"instanceof": 18, "**": 19,
}

func (p *phpParser) binOp() (string, int) {
	t := p.peek()
	switch t.kind {
	case ptPunct:
		if pr, ok := phpBinPrec[t.text]; ok {
			return t.text, pr
		}
	case ptIdent:
		l := strings.ToLower(t.text)
		if l == "or" || l == "xor" || l == "and" || l == "instanceof" {
			return l, phpBinPrec[l]
		}
	}
	return "", -1
}

func (p *phpParser) binary(minPrec int) *phpNode {
	left := p.unary()
	if left == nil {
		return nil
	}
	for !p.exhausted() {
		op, pr := p.binOp()
		if pr < 0 || pr < minPrec {
			return left
		}
		line := p.next().line
		switch {
		case op == "?":
			if !p.isPunct(":") {
				p.binary(5)
			}
			p.accept(":")
			p.binary(5)
			left = &phpNode{kind: pkOther, line: line}
		case pr == 4: // assignment, right-associative
			right := p.binary(4)
			left = &phpNode{kind: pkAssign, name: op, x: left, y: right, line: line}
		case op == ".":
			right := p.binary(pr + 1)
			left = &phpNode{kind: pkConcat, x: left, y: right, line: line}
		default:
			right := p.binary(pr + 1)
			left = &phpNode{kind: pkOther, x: left, y: right, line: line}
		}
	}
	return left
}

var phpCasts = map[string]bool{"int": true, "integer": true, "bool": true, "boolean": true, "float": true, "double": true, "real": true, "string": true, "array": true, "object": true, "unset": true, "binary": true}

func (p *phpParser) unary() *phpNode {
	t := p.peek()
	if t.kind == ptPunct {
		switch t.text {
		case "!", "-", "+", "~", "@", "&":
			p.next()
			x := p.unary()
			if t.text == "@" || t.text == "&" {
				return x
			}
			return &phpNode{kind: pkOther, x: x, line: t.line}
		case "++", "--":
			p.next()
			return &phpNode{kind: pkOther, x: p.unary(), line: t.line}
		case "(":
			if c := p.peekAt(1); c.kind == ptIdent && phpCasts[strings.ToLower(c.text)] && p.peekAt(2).kind == ptPunct && p.peekAt(2).text == ")" {
				p.next()
				p.next()
				p.next()
				return &phpNode{kind: pkOther, x: p.unary(), line: t.line}
			}
		}
	}
	if t.kind == ptIdent {
		switch strings.ToLower(t.text) {
		case "new":
			p.next()
			n := &phpNode{kind: pkNew, line: t.line}
			if p.peek().kind == ptIdent || p.peek().kind == ptVar {
				nt := p.next()
				n.x = &phpNode{kind: pkName, name: nt.text, line: nt.line}
				if nt.kind == ptVar {
					n.x.kind = pkVar
				}
			} else if p.isIdent("class") {
				p.next()
			}
			if p.isPunct("(") {
				n.args, n.keys = p.callArgs()
			}
			if p.isPunct("{") { // anonymous class body
				p.skipBalanced()
			}
			return p.postfix(n)
		case "print", "clone", "yield", "throw":
			p.next()
			return &phpNode{kind: pkOther, x: p.expr(), line: t.line}
		case "require", "require_once", "include", "include_once":
			p.next()
			return &phpNode{kind: pkRequire, x: p.expr(), line: t.line}
		}
	}
	return p.postfix(p.primary())
}

func (p *phpParser) primary() *phpNode {
	t := p.peek()
	switch t.kind {
	case ptString:
		p.next()
		return &phpNode{kind: pkStr, parts: t.parts, line: t.line}
	case ptNumber:
		p.next()
		return &phpNode{kind: pkNum, name: t.text, line: t.line}
	case ptVar:
		p.next()
		return &phpNode{kind: pkVar, name: t.text, line: t.line}
	case ptAttrOpen:
		p.attributes()
		return p.primary()
	case ptPunct:
		switch t.text {
		case "[":
			p.next()
			return p.arrayItems("]", t.line)
		case "(":
			p.next()
			x := p.expr()
			for !p.exhausted() && !p.isPunct(")") {
				if p.isPunct("(") || p.isPunct("[") || p.isPunct("{") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			p.accept(")")
			return x
		}
		return nil
	case ptIdent:
		l := strings.ToLower(t.text)
		if l == "static" && p.peekAt(1).kind == ptIdent && (strings.EqualFold(p.peekAt(1).text, "function") || strings.EqualFold(p.peekAt(1).text, "fn")) {
			p.next()
			t = p.peek()
			l = strings.ToLower(t.text)
		}
		switch {
		case l == "function":
			p.next()
			p.accept("&")
			n := &phpNode{kind: pkClosure, line: t.line}
			if p.isPunct("(") {
				p.skipBalanced()
			}
			if p.isIdent("use") {
				p.next()
				if p.isPunct("(") {
					p.skipBalanced()
				}
			}
			for !p.exhausted() && !p.isPunct("{") && !p.isPunct(";") {
				p.next()
			}
			if p.accept("{") {
				n.body = p.statements(true)
			}
			return n
		case l == "fn":
			p.next()
			p.accept("&")
			n := &phpNode{kind: pkClosure, line: t.line}
			if p.isPunct("(") {
				p.skipBalanced()
			}
			for !p.exhausted() && !p.isPunct("=>") && !p.isPunct(";") {
				if p.isPunct("(") {
					p.skipBalanced()
					continue
				}
				p.next()
			}
			if p.accept("=>") {
				if body := p.expr(); body != nil {
					n.body = []*phpNode{{kind: pkExprStmt, x: body, line: body.line}}
				}
			}
			return n
		case l == "array" && p.peekAt(1).kind == ptPunct && p.peekAt(1).text == "(":
			p.next()
			p.next()
			return p.arrayItems(")", t.line)
		case l == "list" && p.peekAt(1).kind == ptPunct && p.peekAt(1).text == "(":
			p.next()
			p.skipBalanced()
			return &phpNode{kind: pkOther, line: t.line}
		case l == "match" && p.peekAt(1).kind == ptPunct && p.peekAt(1).text == "(":
			p.next()
			p.skipBalanced()
			if p.isPunct("{") {
				p.skipBalanced()
			}
			return &phpNode{kind: pkOther, line: t.line}
		case l == "isset" || l == "empty" || l == "exit" || l == "die":
			p.next()
			if p.isPunct("(") {
				p.skipBalanced()
			}
			return &phpNode{kind: pkOther, line: t.line}
		}
		p.next()
		return &phpNode{kind: pkName, name: t.text, line: t.line}
	}
	return nil
}

// arrayItems parses array items up to closer ("]" or ")").
func (p *phpParser) arrayItems(closer string, line int) *phpNode {
	n := &phpNode{kind: pkArray, line: line}
	for !p.exhausted() && !p.isPunct(closer) {
		if p.accept(",") {
			continue
		}
		p.accept("...")
		start := p.pos
		v := p.expr()
		var k *phpNode
		if p.accept("=>") {
			p.accept("&")
			k, v = v, p.expr()
		}
		if v != nil || k != nil {
			n.args = append(n.args, v)
			n.keys = append(n.keys, k)
		}
		if !p.isPunct(",") && !p.isPunct(closer) {
			if p.pos == start {
				p.next()
			} else {
				// an unparsed tail: skip to the next item
				for !p.exhausted() && !p.isPunct(",") && !p.isPunct(closer) {
					if p.isPunct("(") || p.isPunct("[") || p.isPunct("{") {
						p.skipBalanced()
						continue
					}
					p.next()
				}
			}
		}
	}
	p.accept(closer)
	return n
}

// callArgs parses "(args)": positional and named (name: expr) arguments.
func (p *phpParser) callArgs() ([]*phpNode, []*phpNode) {
	var args, keys []*phpNode
	p.next() // "("
	for !p.exhausted() && !p.isPunct(")") {
		if p.accept(",") {
			continue
		}
		var key *phpNode
		if t := p.peek(); t.kind == ptIdent && p.peekAt(1).kind == ptPunct && p.peekAt(1).text == ":" {
			p.next()
			p.next()
			key = &phpNode{kind: pkName, name: t.text, line: t.line}
		}
		p.accept("...")
		start := p.pos
		v := p.expr()
		if v == nil && p.pos == start {
			p.next()
			continue
		}
		args = append(args, v)
		keys = append(keys, key)
		for !p.exhausted() && !p.isPunct(",") && !p.isPunct(")") {
			if p.isPunct("(") || p.isPunct("[") || p.isPunct("{") {
				p.skipBalanced()
				continue
			}
			p.next()
		}
	}
	p.accept(")")
	return args, keys
}

func (p *phpParser) postfix(x *phpNode) *phpNode {
	if x == nil {
		return nil
	}
	for !p.exhausted() {
		t := p.peek()
		if t.kind != ptPunct {
			return x
		}
		switch t.text {
		case "(":
			args, keys := p.callArgs()
			x = &phpNode{kind: pkCall, x: x, args: args, keys: keys, line: x.line}
		case "::":
			p.next()
			nt := p.next()
			name := nt.text
			if nt.kind == ptPunct && nt.text == "{" {
				p.pos--
				p.skipBalanced()
				name = ""
			}
			if p.isPunct("(") {
				args, keys := p.callArgs()
				x = &phpNode{kind: pkStaticCall, x: x, name: name, args: args, keys: keys, line: t.line}
			} else {
				x = &phpNode{kind: pkClassConst, x: x, name: name, line: t.line}
			}
		case "->", "?->":
			p.next()
			nt := p.next()
			name := nt.text
			if nt.kind == ptPunct && nt.text == "{" {
				p.pos--
				p.skipBalanced()
				name = ""
			}
			if p.isPunct("(") {
				args, keys := p.callArgs()
				x = &phpNode{kind: pkMethodCall, x: x, name: name, args: args, keys: keys, line: t.line}
			} else {
				x = &phpNode{kind: pkProp, x: x, name: name, line: t.line}
			}
		case "[":
			p.next()
			var idx *phpNode
			if !p.isPunct("]") {
				idx = p.expr()
			}
			for !p.exhausted() && !p.isPunct("]") {
				p.next()
			}
			p.accept("]")
			x = &phpNode{kind: pkIndex, x: x, y: idx, line: t.line}
		case "++", "--":
			p.next()
		default:
			return x
		}
	}
	return x
}

// shortPHPName is a (possibly qualified) class name's last segment.
func shortPHPName(n string) string {
	if i := strings.LastIndexByte(n, '\\'); i >= 0 {
		return n[i+1:]
	}
	return n
}

// phpArg returns the call argument named name, else the positional
// argument at index pos (counting positional arguments only), else nil.
func phpArg(n *phpNode, pos int, name string) *phpNode {
	k := 0
	for i, a := range n.args {
		if i < len(n.keys) && n.keys[i] != nil {
			if name != "" && n.keys[i].name == name {
				return a
			}
			continue
		}
		if k == pos {
			return a
		}
		k++
	}
	return nil
}

// phpArrayGet returns the value under the string key key of an array
// literal.
func phpArrayGet(arr *phpNode, key string) *phpNode {
	if arr == nil || arr.kind != pkArray {
		return nil
	}
	for i, k := range arr.keys {
		if k != nil {
			if s, ok := phpLiteral(k, nil); ok && s == key {
				return arr.args[i]
			}
		}
	}
	return nil
}

// phpEnv binds variables to literal values while a loop is unrolled.
type phpEnv map[string]*phpNode

// phpLiteral folds n to a string: a literal, a concatenation of foldable
// parts, a simple interpolation of a bound variable, or a bound variable.
func phpLiteral(n *phpNode, env phpEnv) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.kind {
	case pkStr:
		var b strings.Builder
		for _, pt := range n.parts {
			if pt.v == "" && !pt.dyn {
				b.WriteString(pt.lit)
				continue
			}
			if pt.dyn {
				return "", false
			}
			bound, ok := env[pt.v]
			if !ok {
				return "", false
			}
			s, ok := phpLiteral(bound, nil)
			if !ok {
				return "", false
			}
			b.WriteString(s)
		}
		return b.String(), true
	case pkNum:
		return n.name, true
	case pkConcat:
		a, ok1 := phpLiteral(n.x, env)
		b, ok2 := phpLiteral(n.y, env)
		return a + b, ok1 && ok2
	case pkVar:
		if bound, ok := env[n.name]; ok {
			return phpLiteral(bound, nil)
		}
	}
	return "", false
}

// phpStrings folds n to a list of strings: a literal array of literals, or
// one literal; variadic call args are passed as several nodes.
func phpStrings(env phpEnv, ns ...*phpNode) []string {
	var out []string
	for _, n := range ns {
		if n == nil {
			continue
		}
		if n.kind == pkArray {
			for _, v := range n.args {
				if s, ok := phpLiteral(v, env); ok {
					out = append(out, s)
				}
			}
			continue
		}
		if s, ok := phpLiteral(n, env); ok {
			out = append(out, s)
		}
	}
	return out
}

// phpClassName returns the class X::class names (short form), or "".
func phpClassName(n *phpNode) string {
	if n != nil && n.kind == pkClassConst && strings.EqualFold(n.name, "class") && n.x != nil && n.x.kind == pkName {
		return shortPHPName(n.x.name)
	}
	return ""
}

// phpAliases maps each "use A\B as C" alias to the short name of the class
// it imports (C -> B), so a handler names the class as it is declared.
func phpAliases(stmts []*phpNode) map[string]string {
	out := map[string]string{}
	var walk func([]*phpNode)
	walk = func(ns []*phpNode) {
		for _, n := range ns {
			if n == nil {
				continue
			}
			switch n.kind {
			case pkUse:
				if real := shortPHPName(n.extra); real != n.name {
					out[n.name] = real
				}
			case pkBlock:
				walk(n.body)
			}
		}
	}
	walk(stmts)
	return out
}

// unalias resolves a short class name through the file's use aliases.
func unalias(aliases map[string]string, name string) string {
	if real, ok := aliases[name]; ok {
		return real
	}
	return name
}

// ---------------------------------------------------------------------
// Laravel evaluation
// ---------------------------------------------------------------------

// laravelGroup is the route-group state a statement is read under.
type laravelGroup struct {
	prefix     string
	controller string
}

type laravelReader struct {
	aliases map[string]string
	path    string
	dir     string // the file's directory (repo-relative)
	base    string // the Laravel application root (repo-relative)
	out     Result
	depth   int
	steps   int
}

// laravelMaxUnroll bounds one foreach's unrolled iterations, and
// laravelMaxSteps one file's evaluation.
const (
	laravelMaxUnroll = 512
	laravelMaxSteps  = 1 << 16
)

func readLaravel(path string, src []byte) Result {
	if isTestPath(path) {
		return Result{}
	}
	if !bytes.Contains(src, []byte("Route::")) && !bytes.Contains(src, []byte("withRouting")) && !bytes.Contains(src, []byte("$router->")) {
		return Result{}
	}
	stmts := parsePHP(src)
	r := &laravelReader{path: path, dir: pathDir(path), base: laravelBase(path), aliases: phpAliases(stmts)}
	r.stmts(stmts, laravelGroup{}, phpEnv{})
	return r.out
}

// isTestPath reports a file of a test suite (a tests/ or test/ directory,
// a test_*.py, *_test.py, *Test.php or conftest.py file): the routes a test
// registers for itself are not routes the application serves.
func isTestPath(p string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		if seg == "tests" || seg == "test" || seg == "Tests" {
			return true
		}
	}
	base := path.Base(p)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || strings.HasSuffix(base, "Test.php") || base == "conftest.py"
}

func pathDir(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

// laravelBase is the application root a file belongs to: the directory
// holding its app/, bootstrap/, routes/ or config/ ancestor.
func laravelBase(p string) string {
	best := -1
	for _, seg := range []string{"app/", "bootstrap/", "routes/", "config/", "src/"} {
		if strings.HasPrefix(p, seg) {
			return ""
		}
		if i := strings.Index(p, "/"+seg); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	if best < 0 {
		return pathDir(p)
	}
	return p[:best]
}

func (r *laravelReader) stmts(body []*phpNode, g laravelGroup, env phpEnv) {
	for _, s := range body {
		r.stmt(s, g, env)
	}
}

func (r *laravelReader) stmt(s *phpNode, g laravelGroup, env phpEnv) {
	if s == nil || r.steps > laravelMaxSteps {
		return
	}
	r.steps++
	switch s.kind {
	case pkExprStmt, pkReturn:
		r.expr(s.x, g, env)
	case pkBlock, pkFunc:
		r.stmts(s.body, g, env)
	case pkClass:
		for _, m := range s.body {
			r.stmts(m.body, g, env)
		}
	case pkForeach:
		r.foreach(s, g, env)
	}
}

// foreach unrolls a loop over a literal array (or a variable bound to one
// by an assignment in the file) with its key and value bound.
func (r *laravelReader) foreach(s *phpNode, g laravelGroup, env phpEnv) {
	arr := s.x
	if arr != nil && arr.kind == pkVar {
		arr = env[arr.name]
	}
	if arr == nil || arr.kind != pkArray {
		r.stmts(s.body, g, env)
		return
	}
	for i, v := range arr.args {
		if i >= laravelMaxUnroll {
			break
		}
		inner := phpEnv{}
		for k, b := range env {
			inner[k] = b
		}
		if s.val != "" {
			inner[s.val] = v
		}
		if s.key != "" {
			if k := arr.keys[i]; k != nil {
				inner[s.key] = k
			} else {
				inner[s.key] = &phpNode{kind: pkNum, name: strconv.Itoa(i)}
			}
		}
		r.stmts(s.body, g, inner)
	}
}

// laravelVerbs maps a route-registering method to its HTTP method.
var laravelVerbs = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE",
	"options": "OPTIONS", "any": "ANY",
}

// laravelAttrs are the RouteRegistrar attribute calls a group or a route
// chain may carry; prefix and controller change what is read.
var laravelAttrs = map[string]bool{
	"prefix": true, "controller": true, "middleware": true, "name": true, "as": true, "domain": true,
	"namespace": true, "where": true, "wherein": true, "wherenumber": true, "wherealpha": true,
	"wherealphanumeric": true, "whereuuid": true, "whereulid": true, "scopebindings": true,
	"withoutscopedbindings": true, "withoutmiddleware": true, "can": true, "missing": true,
	"withtrashed": true, "excludedmiddleware": true, "block": true, "lock": true,
}

// expr reads one expression statement: an assignment binds a literal; a
// route chain registers; a require mounts; withRouting mounts.
func (r *laravelReader) expr(e *phpNode, g laravelGroup, env phpEnv) {
	if e == nil {
		return
	}
	switch e.kind {
	case pkAssign:
		if e.x != nil && e.x.kind == pkVar && e.name == "=" && e.y != nil {
			switch e.y.kind {
			case pkArray, pkStr, pkConcat, pkNum:
				env[e.x.name] = e.y
				return
			}
		}
		r.expr(e.y, g, env)
		return
	case pkRequire:
		if t, ok := r.target(e.x, env); ok {
			r.mount(g.prefix, t, e.line)
		}
		return
	}
	chain := phpChain(e)
	if len(chain.links) == 0 {
		return
	}
	if chain.facade {
		r.chain(chain, g, env)
		return
	}
	// Not a route chain: look for withRouting(...) anywhere in it, and for
	// route code inside closures passed as arguments (a provider's
	// $this->routes(function () { ... })).
	for _, l := range chain.links {
		if strings.EqualFold(l.name, "withRouting") {
			r.withRouting(l, env)
			continue
		}
		for _, a := range l.args {
			if a != nil && a.kind == pkClosure {
				r.stmts(a.body, g, env)
			}
		}
	}
}

type phpLink struct {
	name string
	args []*phpNode
	keys []*phpNode
	line int
}

type phpChainT struct {
	facade bool // the chain starts on Route:: or a $router variable
	links  []phpLink
}

// phpChain decomposes a call chain into its links, innermost first.
func phpChain(e *phpNode) phpChainT {
	var links []phpLink
	cur := e
	for cur != nil {
		switch cur.kind {
		case pkMethodCall:
			links = append(links, phpLink{name: cur.name, args: cur.args, keys: cur.keys, line: cur.line})
			cur = cur.x
			continue
		case pkStaticCall:
			links = append(links, phpLink{name: cur.name, args: cur.args, keys: cur.keys, line: cur.line})
			facade := cur.x != nil && cur.x.kind == pkName && shortPHPName(cur.x.name) == "Route"
			for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
				links[i], links[j] = links[j], links[i]
			}
			return phpChainT{facade: facade, links: links}
		case pkCall:
			links = append(links, phpLink{name: "", args: cur.args, keys: cur.keys, line: cur.line})
			cur = cur.x
			continue
		case pkVar:
			facade := cur.name == "router"
			for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
				links[i], links[j] = links[j], links[i]
			}
			return phpChainT{facade: facade, links: links}
		}
		break
	}
	for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
		links[i], links[j] = links[j], links[i]
	}
	return phpChainT{links: links}
}

// pendingRoute is a registration a chain made, still open to only/except.
type pendingRoute struct {
	resource *laravelResource
	routes   []Route
}

type laravelResource struct {
	name, handler string
	actions       []string
	singleton     bool
	line          int
	only, except  []string
	prefix        string
}

func (r *laravelReader) chain(c phpChainT, g laravelGroup, env phpEnv) {
	attrs := g
	var pend []*pendingRoute
	for _, l := range c.links {
		name := strings.ToLower(l.name)
		switch {
		case name == "group":
			if len(pend) > 0 {
				continue
			}
			r.group(l, attrs, env)
			attrs = g
		case laravelVerbs[name] != "":
			pend = append(pend, r.verbRoute(l, []string{laravelVerbs[name]}, attrs, env))
		case name == "match":
			ms := phpStrings(env, phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "methods"))
			var methods []string
			hasGet := false
			for _, m := range ms {
				if strings.EqualFold(m, "GET") {
					hasGet = true
				}
			}
			for _, m := range ms {
				m = strings.ToUpper(m)
				if m == "HEAD" && hasGet {
					continue
				}
				methods = append(methods, m)
			}
			if len(methods) > 0 && len(l.args) > 1 {
				shifted := phpLink{name: l.name, args: l.args[1:], keys: l.keys[1:], line: l.line}
				pend = append(pend, r.verbRoute(shifted, methods, attrs, env))
			}
		case name == "view":
			pend = append(pend, r.fixedRoute(l, "GET", "ViewController.__invoke", attrs, env))
		case name == "redirect" || name == "permanentredirect":
			pend = append(pend, r.fixedRoute(l, "ANY", "RedirectController.__invoke", attrs, env))
		case name == "fallback":
			pend = append(pend, r.fallback(l, attrs, env))
		case name == "resource" || name == "apiresource":
			if p := r.resource(l, name == "apiresource", false, attrs, env); p != nil {
				pend = append(pend, p)
			}
		case name == "singleton" || name == "apisingleton":
			if p := r.resource(l, name == "apisingleton", true, attrs, env); p != nil {
				pend = append(pend, p)
			}
		case name == "resources" || name == "apiresources" || name == "singletons" || name == "apisingletons":
			arr := phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "")
			if arr != nil && arr.kind == pkArray {
				for i, v := range arr.args {
					k := arr.keys[i]
					if k == nil {
						continue
					}
					one := phpLink{name: l.name, args: []*phpNode{k, v}, keys: []*phpNode{nil, nil}, line: l.line}
					if len(l.args) > 1 {
						one.args = append(one.args, l.args[1])
						one.keys = append(one.keys, nil)
					}
					if p := r.resource(one, strings.HasPrefix(name, "api"), strings.Contains(name, "singleton"), attrs, env); p != nil {
						pend = append(pend, p)
					}
				}
			}
		case name == "only" || name == "except":
			for _, p := range pend {
				if p != nil && p.resource != nil {
					list := phpStrings(env, l.args...)
					if name == "only" {
						p.resource.only = list
					} else {
						p.resource.except = append(p.resource.except, list...)
					}
				}
			}
		case name == "creatable":
			for _, p := range pend {
				if p != nil && p.resource != nil && p.resource.singleton {
					p.resource.actions = appendMissing(p.resource.actions, "create", "store", "destroy")
				}
			}
		case name == "destroyable":
			for _, p := range pend {
				if p != nil && p.resource != nil && p.resource.singleton {
					p.resource.actions = appendMissing(p.resource.actions, "destroy")
				}
			}
		case name == "prefix" && len(pend) == 0:
			if s, ok := phpLiteral(phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "prefix"), env); ok {
				attrs.prefix = joinLaravel(attrs.prefix, s)
			} else {
				attrs.prefix = joinLaravel(attrs.prefix, "{}")
			}
		case name == "controller" && len(pend) == 0:
			if c := r.classArg(phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "controller"), env); c != "" {
				attrs.controller = c
			}
		case name == "uses" || name == "action":
			for _, p := range pend {
				if p == nil {
					continue
				}
				h := r.handler(phpArg(&phpNode{args: l.args, keys: l.keys}, 0, ""), attrs, env)
				for i := range p.routes {
					p.routes[i].Handler = h
				}
			}
		}
	}
	for _, p := range pend {
		if p == nil {
			continue
		}
		if p.resource != nil {
			r.out.Routes = append(r.out.Routes, r.expandResource(p.resource)...)
			continue
		}
		r.out.Routes = append(r.out.Routes, p.routes...)
	}
}

func appendMissing(list []string, add ...string) []string {
	for _, a := range add {
		found := false
		for _, l := range list {
			if l == a {
				found = true
			}
		}
		if !found {
			list = append(list, a)
		}
	}
	return list
}

// group reads a group(...) link: an attribute array and/or a closure or a
// route file.
func (r *laravelReader) group(l phpLink, attrs laravelGroup, env phpEnv) {
	inner := attrs
	for i, a := range l.args {
		if a == nil {
			continue
		}
		if i < len(l.keys) && l.keys[i] != nil && l.keys[i].name != "routes" && l.keys[i].name != "attributes" && l.keys[i].name != "callback" {
			continue
		}
		switch a.kind {
		case pkArray:
			if p := phpArrayGet(a, "prefix"); p != nil {
				if s, ok := phpLiteral(p, env); ok {
					inner.prefix = joinLaravel(inner.prefix, s)
				} else {
					inner.prefix = joinLaravel(inner.prefix, "{}")
				}
			}
			if c := phpArrayGet(a, "controller"); c != nil {
				if s := r.classArg(c, env); s != "" {
					inner.controller = s
				}
			}
		case pkClosure:
			r.depth++
			if r.depth < 32 {
				r.stmts(a.body, inner, env)
			}
			r.depth--
		default:
			if t, ok := r.target(a, env); ok {
				r.mount(inner.prefix, t, l.line)
			}
		}
	}
}

func (r *laravelReader) mount(prefix, target string, line int) {
	r.out.Mounts = append(r.out.Mounts, Mount{Prefix: laravelPath(prefix), Target: target, File: r.path, Line: line})
}

// target resolves a route file expression to a Mount Target: base_path(),
// app_path(), __DIR__ (and dirname(__DIR__)) concatenations, or a literal
// path relative to the application root.
func (r *laravelReader) target(e *phpNode, env phpEnv) (string, bool) {
	rel, ok := r.fileExpr(e, env)
	if !ok || rel == "" || rel == "." {
		return "", false
	}
	return normalizeTarget(rel), true
}

// fileExpr folds a file expression to a Target: "./x" or "../x" relative to
// the file's directory, or "/x" repo-relative.
func (r *laravelReader) fileExpr(e *phpNode, env phpEnv) (string, bool) {
	if e == nil {
		return "", false
	}
	switch e.kind {
	case pkConcat:
		left, ok := r.fileExpr(e.x, env)
		if !ok {
			return "", false
		}
		right, ok := phpLiteral(e.y, env)
		if !ok {
			return "", false
		}
		return left + right, true
	case pkName:
		if e.name == "__DIR__" {
			return ".", true
		}
	case pkCall:
		if e.x == nil || e.x.kind != pkName {
			return "", false
		}
		fn := strings.ToLower(shortPHPName(e.x.name))
		arg := phpArg(e, 0, "")
		switch fn {
		case "base_path", "app_path", "resource_path":
			s := ""
			if arg != nil {
				v, ok := phpLiteral(arg, env)
				if !ok {
					return "", false
				}
				s = v
			}
			sub := ""
			if fn == "app_path" {
				sub = "app"
			} else if fn == "resource_path" {
				sub = "resources"
			}
			return "/" + strings.TrimPrefix(path.Join(r.base, sub, s), "/"), true
		case "dirname":
			inner, ok := r.fileExpr(arg, env)
			if !ok {
				return "", false
			}
			levels := 1
			if l := phpArg(e, 1, "levels"); l != nil && l.kind == pkNum {
				if n, err := strconv.Atoi(l.name); err == nil && n > 0 {
					levels = n
				}
			}
			for i := 0; i < levels; i++ {
				inner += "/.."
			}
			return inner, true
		case "realpath":
			return r.fileExpr(arg, env)
		}
	case pkStr:
		s, ok := phpLiteral(e, env)
		if !ok || s == "" || strings.Contains(s, "://") {
			return "", false
		}
		if strings.HasPrefix(s, "/") {
			return "", false // an absolute file system path
		}
		return "/" + strings.TrimPrefix(path.Join(r.base, s), "/"), true
	}
	return "", false
}

// normalizeTarget turns a folded "."-relative path into a Compose Target.
func normalizeTarget(t string) string {
	if strings.HasPrefix(t, "/") {
		return t
	}
	c := path.Clean(t)
	if strings.HasPrefix(c, "../") || c == ".." {
		return c
	}
	return "./" + c
}

// withRouting reads bootstrap/app.php's ->withRouting(...).
func (r *laravelReader) withRouting(l phpLink, env phpEnv) {
	call := &phpNode{args: l.args, keys: l.keys}
	apiPrefix := "api"
	if p := phpArg(call, -1, "apiPrefix"); p != nil {
		if s, ok := phpLiteral(p, env); ok {
			apiPrefix = s
		}
	}
	for _, k := range []string{"web", "api"} {
		v := phpArg(call, -1, k)
		if v == nil {
			continue
		}
		prefix := ""
		if k == "api" {
			prefix = apiPrefix
		}
		files := []*phpNode{v}
		if v.kind == pkArray {
			files = v.args
		}
		for _, f := range files {
			if t, ok := r.target(f, env); ok {
				r.mount(prefix, t, l.line)
			}
		}
	}
	for _, k := range []string{"then", "using"} {
		if v := phpArg(call, -1, k); v != nil && v.kind == pkClosure {
			r.stmts(v.body, laravelGroup{}, env)
		}
	}
}

// verbRoute reads get('/uri', action) and its siblings.
func (r *laravelReader) verbRoute(l phpLink, methods []string, g laravelGroup, env phpEnv) *pendingRoute {
	call := &phpNode{args: l.args, keys: l.keys}
	uri, ok := phpLiteral(phpArg(call, 0, "uri"), env)
	if !ok {
		return nil
	}
	h := r.handler(phpArg(call, 1, "action"), g, env)
	p := &pendingRoute{}
	for _, m := range methods {
		p.routes = append(p.routes, Route{Method: m, Path: laravelPath(joinLaravel(g.prefix, uri)), Handler: h, File: r.path, Line: l.line})
	}
	return p
}

func (r *laravelReader) fixedRoute(l phpLink, method, handler string, g laravelGroup, env phpEnv) *pendingRoute {
	uri, ok := phpLiteral(phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "uri"), env)
	if !ok {
		return nil
	}
	return &pendingRoute{routes: []Route{{Method: method, Path: laravelPath(joinLaravel(g.prefix, uri)), Handler: handler, File: r.path, Line: l.line}}}
}

func (r *laravelReader) fallback(l phpLink, g laravelGroup, env phpEnv) *pendingRoute {
	h := r.handler(phpArg(&phpNode{args: l.args, keys: l.keys}, 0, "action"), g, env)
	return &pendingRoute{routes: []Route{{Method: "GET", Path: laravelPath(joinLaravel(g.prefix, "{fallbackPlaceholder}")), Handler: h, File: r.path, Line: l.line}}}
}

// handler names an action: Controller.method, Controller.__invoke, or ""
// for a closure (and for what cannot be read).
func (r *laravelReader) handler(a *phpNode, g laravelGroup, env phpEnv) string {
	if a == nil {
		return ""
	}
	if a.kind == pkVar {
		if bound, ok := env[a.name]; ok {
			return r.handler(bound, g, nil)
		}
		return ""
	}
	switch a.kind {
	case pkClosure:
		return ""
	case pkArray:
		if uses := phpArrayGet(a, "uses"); uses != nil {
			return r.handler(uses, g, env)
		}
		if len(a.args) >= 1 && a.keys[0] == nil {
			cls := r.classArg(a.args[0], env)
			if cls == "" {
				return ""
			}
			if len(a.args) >= 2 && a.keys[1] == nil {
				if m, ok := phpLiteral(a.args[1], env); ok {
					return cls + "." + m
				}
				return ""
			}
			return cls + ".__invoke"
		}
		for i, v := range a.args {
			if a.keys[i] == nil && v != nil && v.kind == pkClosure {
				return ""
			}
		}
		return ""
	case pkClassConst:
		if c := phpClassName(a); c != "" {
			return unalias(r.aliases, c) + ".__invoke"
		}
	case pkStr, pkConcat:
		s, ok := phpLiteral(a, env)
		if !ok {
			return ""
		}
		if i := strings.IndexByte(s, '@'); i >= 0 {
			return shortPHPName(s[:i]) + "." + s[i+1:]
		}
		if g.controller != "" {
			return g.controller + "." + s
		}
		return shortPHPName(s) + ".__invoke"
	}
	return ""
}

// classArg names a controller argument: X::class or a class string.
func (r *laravelReader) classArg(a *phpNode, env phpEnv) string {
	if a != nil && a.kind == pkVar {
		if bound, ok := env[a.name]; ok {
			a = bound
		}
	}
	if c := phpClassName(a); c != "" {
		return unalias(r.aliases, c)
	}
	if s, ok := phpLiteral(a, env); ok && s != "" {
		return shortPHPName(s)
	}
	return ""
}

var laravelResourceActions = []string{"index", "create", "store", "show", "edit", "update", "destroy"}

// resource reads resource('name', Controller::class[, options]).
func (r *laravelReader) resource(l phpLink, api, singleton bool, g laravelGroup, env phpEnv) *pendingRoute {
	call := &phpNode{args: l.args, keys: l.keys}
	name, ok := phpLiteral(phpArg(call, 0, "name"), env)
	if !ok || name == "" {
		return nil
	}
	cls := r.classArg(phpArg(call, 1, "controller"), env)
	res := &laravelResource{name: name, handler: cls, singleton: singleton, line: l.line, prefix: g.prefix}
	switch {
	case singleton && api:
		res.actions = []string{"show", "update"}
	case singleton:
		res.actions = []string{"show", "edit", "update"}
	case api:
		res.actions = []string{"index", "store", "show", "update", "destroy"}
	default:
		res.actions = append([]string(nil), laravelResourceActions...)
	}
	if opts := phpArg(call, 2, "options"); opts != nil && opts.kind == pkArray {
		if o := phpArrayGet(opts, "only"); o != nil {
			res.only = phpStrings(env, o)
		}
		if e := phpArrayGet(opts, "except"); e != nil {
			res.except = phpStrings(env, e)
		}
	}
	return &pendingRoute{resource: res}
}

// expandResource is Laravel's ResourceRegistrar: index GET /photos, create
// GET /photos/create, store POST /photos, show GET /photos/{photo}, edit
// GET /photos/{photo}/edit, update PUT and PATCH /photos/{photo}, destroy
// DELETE /photos/{photo}; a singleton has no index and no {photo}.
func (r *laravelReader) expandResource(res *laravelResource) []Route {
	name := res.name
	prefix := res.prefix
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		prefix = joinLaravel(prefix, name[:i])
		name = name[i+1:]
	}
	segs := strings.Split(name, ".")
	base := ""
	for i, s := range segs {
		if i < len(segs)-1 {
			base = joinLaravel(base, s+"/{"+s+"}")
		} else {
			base = joinLaravel(base, s)
		}
	}
	last := segs[len(segs)-1]
	member := base + "/{" + last + "}"
	if res.singleton {
		member = base
	}
	want := func(a string) bool {
		if len(res.only) > 0 {
			found := false
			for _, o := range res.only {
				if o == a {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		for _, e := range res.except {
			if e == a {
				return false
			}
		}
		return true
	}
	var out []Route
	add := func(method, uri, action string) {
		h := ""
		if res.handler != "" {
			h = res.handler + "." + action
		}
		out = append(out, Route{Method: method, Path: laravelPath(joinLaravel(prefix, uri)), Handler: h, File: r.path, Line: res.line})
	}
	for _, a := range res.actions {
		if !want(a) {
			continue
		}
		switch a {
		case "index":
			add("GET", base, a)
		case "create":
			add("GET", base+"/create", a)
		case "store":
			add("POST", base, a)
		case "show":
			add("GET", member, a)
		case "edit":
			add("GET", member+"/edit", a)
		case "update":
			add("PUT", member, a)
			add("PATCH", member, a)
		case "destroy":
			add("DELETE", member, a)
		}
	}
	return out
}

// joinLaravel joins a group prefix and a uri the way Laravel's router does:
// both trimmed of "/", joined with one.
func joinLaravel(prefix, uri string) string {
	a, b := strings.Trim(prefix, "/"), strings.Trim(uri, "/")
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "/" + b
}

func laravelPath(uri string) string { return "/" + strings.Trim(uri, "/") }
