package extractor

// regoextract.go — the Rego native dialect (LB-Q-rego, LB-F131; docs/design/
// language-breadth.md §13, engine "native", grammar_source native:pure-go).
// No allowlisted grammar parses real Rego (LB-Q-rego-rest: tree-sitter-rego
// da2a1f63 leaves an ERROR node in 74% of the files OPA accepts), and OPA's
// own parser (github.com/open-policy-agent/opa/v1/ast) would add 6.4 MB to
// the binary and be the tier-A oracle's own parser (ADR-048 amendment, wave
// pc). This is a reader of the structure the measured levels need, with no
// dependency:
//
//   - a tokenizer: identifiers, numbers, strings, raw strings, template
//     strings (`$"…{expr}…"`, whose interpolated expressions are read as
//     parenthesised terms), comments and punctuation;
//   - top-level statements: one starts at an identifier that opens a line at
//     bracket depth 0, unless the token before it continues an expression
//     (`if`, `:=`, `,`, an operator) or the identifier itself does (`else`,
//     `with`); a file whose brackets never balance is split again at every
//     identifier in column 0, so one unclosed body loses only itself;
//   - `package` (its path without a leading data), `import data.x.y [as z]`,
//     and one definition per rule head, at the head's line through its last
//     token (an else chain extends its rule): a function when the head has
//     arguments, else a rule (complete, partial, default and test rules
//     alike), named as OPA names it — the head's variable; for a two-term
//     reference, its first term when the head has a value (`p[k] := v`,
//     `p.q := 1`) or is a function's with a body (`a.b(x) if …`); else the
//     reference's terms up to the first that is neither a name nor a string,
//     joined with dots (`a.b.c := 1`, `items[x] if …`, `a.b(x) := 1`);
//   - calls by written name — a dotted name followed by `(` (`count`,
//     `object.get`, `util.f`, `data.lib.f`; `contains(s, x)` too, `set()` is a
//     literal) — and references rooted at data (`data.lib.roles[x]`), each
//     stored by its last (static) segment, as the oracle names it, and
//     attributed to the rule whose head, body or else chain holds it; an
//     infix operator is not written as a call, and a bare name (a rule of the
//     same package, a local) is not one: telling the two apart needs OPA's
//     compiler; a body chained to the one before it (`f(x) if {…} {…}`) is a
//     rule of its own, as in OPA's AST;
//   - for the file resolver (FileSpecs, the rego-package strategy): each data
//     import as a whole-module import bound to its alias or last segment, each
//     call or reference through an import's name by its first two segments
//     (`util.f`, a reference the resolver binds as it binds a call), and the
//     file's own package as a re-export, since a package's rules are visible
//     from every file that declares it.

import (
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	kindRegoPackage  core.SourceKind = "RegoPackage"
	kindRegoRule     core.SourceKind = "RegoRule"
	kindRegoFunction core.SourceKind = "RegoFunction"
)

// RegoExtractor implements Extractor and FileSpecExtractor for .rego files.
type RegoExtractor struct{}

var (
	_ Extractor         = (*RegoExtractor)(nil)
	_ FileSpecExtractor = (*RegoExtractor)(nil)
)

// NewRegoExtractor returns the Rego dialect's extractor.
func NewRegoExtractor() *RegoExtractor { return &RegoExtractor{} }

// Name is the dialect id, the file resolver's language key.
func (x *RegoExtractor) Name() string { return "rego" }

// Close releases nothing: the reader holds no module.
func (x *RegoExtractor) Close() {}

// Extract implements Extractor. Malformed input yields what its readable
// statements declare and no error.
func (x *RegoExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	return parseRego(src).nodes(service, relPath), nil
}

// ExtractFileSpecs implements FileSpecExtractor.
func (x *RegoExtractor) ExtractFileSpecs(relPath string, src []byte) (FileSpecs, error) {
	return parseRego(src).specs(), nil
}

// ExtractImportSpans implements FileSpecExtractor: an import is one line.
func (x *RegoExtractor) ExtractImportSpans(relPath string, src []byte) ([]ImportSpan, error) {
	f := parseRego(src)
	out := make([]ImportSpan, 0, len(f.imports))
	for _, s := range f.imports {
		out = append(out, ImportSpan{ImportSpec: s, StartLine: s.Line, EndLine: s.Line})
	}
	return out, nil
}

// RegoRepairs is the number of repairs reading src took (an unclosed
// bracket or string, a stray closer): 0 for a file the reader read as
// written. The bench's error rate (ADR-048 amendment, wave pc).
func RegoRepairs(src []byte) int { return parseRego(src).repairs }

// RegoDeclarations is src's package path (without data.) and the names of
// the rules and functions it defines, sorted and distinct: what the
// rego-package import strategy maps an import path onto.
func RegoDeclarations(src []byte) (pkg string, rules []string) {
	f := parseRego(src)
	seen := map[string]bool{}
	for _, r := range f.rules {
		if !seen[r.name] {
			seen[r.name] = true
			rules = append(rules, r.name)
		}
	}
	sort.Strings(rules)
	return f.pkg, rules
}

// ── tokenizer ──

type regoTokKind uint8

const (
	regoIdent regoTokKind = iota
	regoString
	regoNumber
	regoPunct
)

type regoTok struct {
	kind          regoTokKind
	text          string // an identifier, a punctuation mark, a string's value
	off, end      int    // byte span in the source
	line, endLine int
	col           int  // byte column of off in its line
	lineStart     bool // the first token on its line
}

func (t regoTok) is(p string) bool { return t.kind == regoPunct && t.text == p }

func (t regoTok) ident(s string) bool { return t.kind == regoIdent && t.text == s }

type regoLexer struct {
	src     []byte
	toks    []regoTok
	line    int
	lineBeg int
	nl      bool
	repairs int
}

func lexRego(src []byte) ([]regoTok, int) {
	l := &regoLexer{src: src, line: 1, nl: true}
	i := 0
	if strings.HasPrefix(string(src), "\ufeff") {
		i = 3
	}
	l.lexTo(i, false)
	return l.toks, l.repairs
}

// emit appends a token starting at off, which is on the lexer's current line
// (a multi-line token is emitted at its start, before the lexer passes its
// newlines).
func (l *regoLexer) emit(kind regoTokKind, text string, off, end, endLine int) int {
	l.toks = append(l.toks, regoTok{kind: kind, text: text, off: off, end: end, line: l.line, endLine: endLine, col: off - l.lineBeg, lineStart: l.nl})
	l.nl = false
	return len(l.toks) - 1
}

func (l *regoLexer) newline(at int) { l.line++; l.lineBeg = at + 1 }

func isRegoIdentStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isRegoIdentByte(c byte) bool  { return isRegoIdentStart(c) || c >= '0' && c <= '9' }
func isRegoDigit(c byte) bool      { return c >= '0' && c <= '9' }
func regoSpace(c byte) bool        { return c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' }

func regoTwoCharPunct(a, b byte) (string, bool) {
	switch string([]byte{a, b}) {
	case ":=", "==", "!=", "<=", ">=":
		return string([]byte{a, b}), true
	}
	return "", false
}

// lexTo tokenizes from i to the end of the source or, inTmpl, to the `}`
// closing a template string's interpolation, returning the position after
// it.
func (l *regoLexer) lexTo(i int, inTmpl bool) int {
	src := l.src
	depth := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\n':
			l.newline(i)
			l.nl = true
			i++
		case regoSpace(c):
			i++
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '"':
			i = l.quoted(i)
		case c == '`':
			i = l.raw(i)
		case c == '$' && i+1 < len(src) && (src[i+1] == '"' || src[i+1] == '`'):
			i = l.template(i)
		case isRegoIdentStart(c):
			j := i + 1
			for j < len(src) && isRegoIdentByte(src[j]) {
				j++
			}
			l.emit(regoIdent, string(src[i:j]), i, j, l.line)
			i = j
		case isRegoDigit(c):
			j := i + 1
			for j < len(src) {
				d := src[j]
				if isRegoIdentByte(d) || d == '.' && j+1 < len(src) && isRegoDigit(src[j+1]) {
					j++
				} else if (d == '+' || d == '-') && (src[j-1] == 'e' || src[j-1] == 'E') {
					j++
				} else {
					break
				}
			}
			l.emit(regoNumber, string(src[i:j]), i, j, l.line)
			i = j
		default:
			if i+1 < len(src) {
				if p, ok := regoTwoCharPunct(c, src[i+1]); ok {
					l.emit(regoPunct, p, i, i+2, l.line)
					i += 2
					continue
				}
			}
			if inTmpl {
				switch c {
				case '{':
					depth++
				case '}':
					if depth == 0 {
						return i + 1
					}
					depth--
				}
			}
			l.emit(regoPunct, string(c), i, i+1, l.line)
			i++
		}
	}
	if inTmpl {
		l.repairs++ // an interpolation never closed
	}
	return i
}

// quoted reads a "…" string (one line, backslash escapes).
func (l *regoLexer) quoted(i int) int {
	src := l.src
	j := i + 1
	for j < len(src) && src[j] != '"' && src[j] != '\n' {
		if src[j] == '\\' && j+1 < len(src) && src[j+1] != '\n' {
			j++
		}
		j++
	}
	if j >= len(src) || src[j] != '"' {
		l.repairs++ // unterminated: the string ends at the line's end
		l.emit(regoString, string(src[i+1:j]), i, j, l.line)
		return j
	}
	v, err := strconv.Unquote(string(src[i : j+1]))
	if err != nil {
		v = string(src[i+1 : j])
	}
	l.emit(regoString, v, i, j+1, l.line)
	return j + 1
}

// raw reads a `…` string, which may span lines.
func (l *regoLexer) raw(i int) int {
	src := l.src
	k := l.emit(regoString, "", i, i, l.line)
	j := i + 1
	for j < len(src) && src[j] != '`' {
		if src[j] == '\n' {
			l.newline(j)
		}
		j++
	}
	if j >= len(src) {
		l.repairs++
	} else {
		j++
	}
	l.toks[k].text = strings.TrimSuffix(string(src[i+1:j]), "`")
	l.toks[k].end, l.toks[k].endLine = j, l.line
	return j
}

// template reads a template string, $"…" or $`…`: its text is one string
// token and each {expr} it interpolates is lexed in place, wrapped in
// parentheses, so a call written inside one is read as any other.
func (l *regoLexer) template(i int) int {
	src := l.src
	q := src[i+1]
	k := l.emit(regoString, "", i, i, l.line)
	j := i + 2
	for j < len(src) {
		c := src[j]
		switch {
		case c == '\\' && q == '"' && j+1 < len(src) && src[j+1] != '\n':
			j += 2
			continue
		case c == '\n' && q == '"':
			l.repairs++
			l.toks[k].end, l.toks[k].endLine = j, l.line
			return j
		case c == '\n':
			l.newline(j)
		case c == q:
			l.toks[k].end, l.toks[k].endLine = j+1, l.line
			return j + 1
		case c == '{':
			l.emit(regoPunct, "(", j, j+1, l.line)
			j = l.lexTo(j+1, true)
			l.emit(regoPunct, ")", j-1, j, l.line)
			continue
		}
		j++
	}
	l.repairs++
	l.toks[k].end, l.toks[k].endLine = j, l.line
	return j
}

// ── statements ──

// regoKeywords never start a reference.
var regoKeywords = map[string]bool{
	"if": true, "else": true, "not": true, "some": true, "every": true, "in": true,
	"with": true, "as": true, "contains": true, "default": true, "import": true,
	"package": true, "true": true, "false": true, "null": true,
}

// regoContinuesAfter: a statement whose last token is toks[i] goes on to the
// next line. A keyword written as a reference's segment (`import
// future.keywords.if`) is a name there, and ends nothing.
func regoContinuesAfter(toks []regoTok, i int) bool {
	t := toks[i]
	if t.kind == regoIdent && i > 0 && toks[i-1].is(".") {
		return false
	}
	switch t.kind {
	case regoPunct:
		switch t.text {
		case ":=", "=", "==", "!=", "<", ">", "<=", ">=", "+", "-", "*", "/", "%", "&", "|", ",", ".", ":", ";", "(", "[", "{":
			return true
		}
	case regoIdent:
		switch t.text {
		case "if", "else", "contains", "as", "in", "with", "not", "some", "every", "default", "import", "package":
			return true
		}
	}
	return false
}

// regoStarts: t can open a statement (a rule head, package or import).
func regoStarts(t regoTok) bool {
	if t.kind != regoIdent {
		return false
	}
	switch t.text {
	case "else", "if", "with", "as", "in", "contains", "not", "some", "every":
		return false
	}
	return true
}

func regoDepth(t regoTok, depth int) (int, bool) {
	if t.kind != regoPunct {
		return depth, false
	}
	switch t.text {
	case "(", "[", "{":
		return depth + 1, false
	case ")", "]", "}":
		if depth == 0 {
			return 0, true // a stray closer
		}
		return depth - 1, false
	}
	return depth, false
}

// splitRego cuts toks into top-level statements, [lo, hi) each. col0 is the
// repair split: every identifier in column 0 that can open a statement
// does, whatever the depth. ok is false when the brackets never balance.
func splitRego(toks []regoTok, col0 bool) (stmts [][2]int, stray int, ok bool) {
	depth, start := 0, 0
	for i, t := range toks {
		if i > start && t.lineStart && regoStarts(t) &&
			((depth == 0 && !regoContinuesAfter(toks, i-1)) || (col0 && t.col == 0)) {
			stmts = append(stmts, [2]int{start, i})
			start, depth = i, 0
		}
		var bad bool
		if depth, bad = regoDepth(t, depth); bad {
			stray++
		}
	}
	if start < len(toks) {
		stmts = append(stmts, [2]int{start, len(toks)})
	}
	return stmts, stray, depth == 0
}

// ── the file ──

type regoRule struct {
	name      string
	fn        bool
	line, end int
	sig       string
	calls     []string
}

type regoFile struct {
	src      []byte
	pkg      string
	pkgLine  int
	pkgSig   string
	imports  []ImportSpec
	rules    []regoRule
	refs     map[string][]int
	repairs  int
	bindings map[string]bool // the names the file's imports bind
}

func parseRego(src []byte) *regoFile {
	toks, repairs := lexRego(src)
	f := &regoFile{src: src, repairs: repairs, refs: map[string][]int{}, bindings: map[string]bool{}}
	stmts, stray, ok := splitRego(toks, false)
	if !ok {
		f.repairs++
		stmts, stray, _ = splitRego(toks, true)
	}
	f.repairs += stray
	for _, s := range stmts { // package and imports first: a rule's references need the bindings
		ts := toks[s[0]:s[1]]
		switch {
		case ts[0].ident("package"):
			f.packageClause(ts)
		case ts[0].ident("import"):
			f.importClause(ts)
		}
	}
	for _, s := range stmts {
		ts := toks[s[0]:s[1]]
		if !ts[0].ident("package") && !ts[0].ident("import") {
			f.rule(ts)
		}
	}
	return f
}

// regoTermKind is what one term of a reference is.
type regoTermKind uint8

const (
	regoTermHead  regoTermKind = iota // the reference's first name
	regoTermDot                       // .name
	regoTermStr                       // ["name"]
	regoTermVar                       // [x]
	regoTermOther                     // any other bracketed term
)

type regoTerm struct {
	kind regoTermKind
	text string
}

// regoRef reads the reference whose first name is ts[i]: its terms and the
// index after it.
func regoRef(ts []regoTok, i int) ([]regoTerm, int) {
	terms := []regoTerm{{regoTermHead, ts[i].text}}
	j := i + 1
	for j < len(ts) {
		switch {
		case ts[j].is(".") && j+1 < len(ts) && ts[j+1].kind == regoIdent:
			terms = append(terms, regoTerm{regoTermDot, ts[j+1].text})
			j += 2
		case ts[j].is("["):
			k := regoClose(ts, j)
			inner := ts[j+1 : k]
			switch {
			case len(inner) == 1 && inner[0].kind == regoString:
				terms = append(terms, regoTerm{regoTermStr, inner[0].text})
			case len(inner) == 1 && inner[0].kind == regoIdent && !regoKeywords[inner[0].text]:
				terms = append(terms, regoTerm{regoTermVar, inner[0].text})
			default:
				terms = append(terms, regoTerm{kind: regoTermOther})
			}
			j = k + 1
		default:
			return terms, j
		}
	}
	return terms, j
}

// regoClose is the index of the bracket closing ts[open], or len(ts) when
// none does.
func regoClose(ts []regoTok, open int) int {
	depth := 0
	for k := open; k < len(ts); k++ {
		if ts[k].kind != regoPunct {
			continue
		}
		switch ts[k].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return k
			}
		}
	}
	return len(ts)
}

// regoStaticPath is a reference's leading names and strings, the form a
// package path and an import path take.
func regoStaticPath(terms []regoTerm) []string {
	var out []string
	for _, t := range terms {
		if t.kind != regoTermHead && t.kind != regoTermDot && t.kind != regoTermStr {
			break
		}
		out = append(out, t.text)
	}
	return out
}

func (f *regoFile) packageClause(ts []regoTok) {
	if f.pkgLine != 0 || len(ts) < 2 || ts[1].kind != regoIdent {
		return
	}
	terms, _ := regoRef(ts, 1)
	segs := regoStaticPath(terms)
	if len(segs) > 0 && segs[0] == "data" {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return
	}
	f.pkg, f.pkgLine = strings.Join(segs, "."), ts[0].line
	f.pkgSig = f.text(ts)
}

// importClause records `import data.x.y [as z]`; an import of input, of
// rego.v1 or of future keywords names no file and binds nothing here.
func (f *regoFile) importClause(ts []regoTok) {
	if len(ts) < 2 || ts[1].kind != regoIdent {
		return
	}
	terms, j := regoRef(ts, 1)
	segs := regoStaticPath(terms)
	if len(segs) < 2 || segs[0] != "data" {
		return
	}
	local := segs[len(segs)-1]
	if j+1 < len(ts) && ts[j].ident("as") && ts[j+1].kind == regoIdent {
		local = ts[j+1].text
	}
	f.imports = append(f.imports, ImportSpec{LocalName: local, Module: strings.Join(segs, "."), Kind: ImportWhole, Line: ts[0].line})
	f.bindings[local] = true
}

// regoRuleName names a head as OPA does (see the file comment): a two-term
// reference names its first term when first is true — a function's head
// with a body, any other head with a value.
func regoRuleName(terms []regoTerm, first bool) string {
	if len(terms) == 1 || len(terms) == 2 && first {
		return terms[0].text
	}
	var segs []string
	for _, t := range terms {
		if t.kind == regoTermOther {
			break
		}
		segs = append(segs, t.text)
	}
	return strings.Join(segs, ".")
}

func (f *regoFile) rule(ts []regoTok) {
	i := 0
	if ts[0].ident("default") {
		i = 1
	}
	if i >= len(ts) || ts[i].kind != regoIdent || regoKeywords[ts[i].text] {
		return // not a rule head
	}
	terms, j := regoRef(ts, i)
	fn := j < len(ts) && ts[j].is("(")
	after := j
	if fn {
		after = regoClose(ts, j) + 1
	}
	hasValue := after < len(ts) && (ts[after].is(":=") || ts[after].is("="))
	hasBody, depth := false, 0
	for _, t := range ts[after:] {
		hasBody = hasBody || depth == 0 && t.ident("if")
		depth, _ = regoDepth(t, depth)
	}
	first := hasValue
	if fn {
		first = hasBody
	}
	name, sig := regoRuleName(terms, first), f.signature(ts)
	// A body chained to the one before it (`f(x) if { … } { … }`) is a rule
	// of its own in OPA's AST, from its brace to its end; the rule before it
	// ends where its body closes.
	from := j
	for _, seg := range regoChains(ts) {
		r := regoRule{name: name, fn: fn, line: ts[seg[0]].line, sig: sig}
		for _, t := range ts[seg[0]:seg[1]] {
			r.end = max(r.end, t.endLine)
		}
		r.calls = f.scan(ts[max(seg[0], from):seg[1]])
		f.rules = append(f.rules, r)
	}
}

// regoChains cuts a rule statement at each body chained to the one before
// it: a `{` directly after the `}` that closed a body at depth 0.
func regoChains(ts []regoTok) [][2]int {
	var out [][2]int
	depth, start := 0, 0
	for k, t := range ts {
		depth, _ = regoDepth(t, depth)
		if depth == 0 && t.is("}") && k+1 < len(ts) && ts[k+1].is("{") {
			out = append(out, [2]int{start, k + 1})
			start = k + 1
		}
	}
	return append(out, [2]int{start, len(ts)})
}

// signature is the head as written: from the statement's first token to
// its body (`if`, `{`, `else`) or the end of the head's line at depth 0,
// whitespace collapsed.
func (f *regoFile) signature(ts []regoTok) string {
	depth, stop := 0, len(ts)
	for k := 1; k < len(ts); k++ {
		t := ts[k]
		if depth == 0 && (t.ident("if") || t.ident("else") || t.line > ts[0].line ||
			t.is("{") && !(ts[k-1].is(":=") || ts[k-1].is("=") || ts[k-1].ident("contains"))) {
			stop = k
			break
		}
		depth, _ = regoDepth(t, depth)
	}
	return f.text(ts[:stop])
}

// text is the source ts spans, whitespace collapsed.
func (f *regoFile) text(ts []regoTok) string {
	end := ts[0].end
	for _, t := range ts {
		end = max(end, t.end)
	}
	return capString(strings.Join(strings.Fields(string(f.src[ts[0].off:end])), " "), 200)
}

// scan reads one rule's calls and data references, each by its last
// segment as the oracle names it (in order, distinct), and records the
// references the file resolver binds: every call or reference through one
// of the file's import bindings, by its first two segments (`util.f`,
// `r.users`).
func (f *regoFile) scan(ts []regoTok) []string {
	var calls []string
	seen := map[string]bool{}
	add := func(segs []string) {
		c := segs[len(segs)-1]
		if !seen[c] {
			seen[c] = true
			calls = append(calls, c)
		}
	}
	for k := 0; k < len(ts); k++ {
		t := ts[k]
		if t.kind != regoIdent || k > 0 && ts[k-1].is(".") {
			continue
		}
		// contains is a keyword and a builtin: contains(s, x) is a call.
		if regoKeywords[t.text] && !(t.text == "contains" && k+1 < len(ts) && ts[k+1].is("(")) {
			continue
		}
		terms, n := regoRef(ts, k)
		called := n < len(ts) && ts[n].is("(")
		dotted := true
		for _, tm := range terms[1:] {
			dotted = dotted && tm.kind == regoTermDot
		}
		if f.bindings[t.text] {
			name := t.text
			if len(terms) > 1 && (terms[1].kind == regoTermDot || terms[1].kind == regoTermStr) {
				name += "." + terms[1].text
			}
			if ls := f.refs[name]; len(ls) == 0 || ls[len(ls)-1] != t.line {
				f.refs[name] = append(ls, t.line)
			}
		}
		switch {
		case called && dotted:
			segs := make([]string, len(terms))
			for x, tm := range terms {
				segs[x] = tm.text
			}
			if !(len(segs) == 1 && segs[0] == "set" && n+1 < len(ts) && ts[n+1].is(")")) {
				add(segs)
			}
			k = n - 1 // the arguments are read next
		case called:
		case t.text == "data" && len(terms) > 1:
			if segs := regoStaticPath(terms); len(segs) > 1 {
				add(segs)
			}
		}
	}
	return calls
}

func (f *regoFile) nodes(service, relPath string) []core.SymbolNode {
	pkg := f.pkg
	if pkg == "" {
		pkg = pkgFromPath(relPath)
	}
	node := func(name string, kind core.SourceKind, line, end int, sig string, calls []string) core.SymbolNode {
		return core.SymbolNode{
			Service: service, Package: pkg, Name: name, Kind: kind, FilePath: relPath,
			Line: line, EndLine: max(end, line), Signature: sig, Calls: calls, Tier: core.TierPreScan,
		}
	}
	var out []core.SymbolNode
	if f.pkg != "" {
		out = append(out, node(f.pkg, kindRegoPackage, f.pkgLine, f.pkgLine, f.pkgSig, nil))
	}
	for _, r := range f.rules {
		kind := kindRegoRule
		if r.fn {
			kind = kindRegoFunction
		}
		out = append(out, node(r.name, kind, r.line, r.end, r.sig, r.calls))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

func (f *regoFile) specs() FileSpecs {
	fs := FileSpecs{Imports: append([]ImportSpec{}, f.imports...)}
	names := make([]string, 0, len(f.refs))
	for n := range f.refs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fs.Refs = append(fs.Refs, RefSpec{Name: n, Lines: f.refs[n]})
	}
	if f.pkg != "" {
		fs.Exports = []ExportSpec{{Module: "data." + f.pkg, Line: f.pkgLine}}
	}
	return fs
}
