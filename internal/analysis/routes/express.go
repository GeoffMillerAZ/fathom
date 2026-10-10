package routes

// express.go — the Express reader (LB-F328) and the small JS/TS token
// scanner every Node reader (nest.go, fastify.go, express.go) shares.
//
// The scanner is not a parser: it splits source into identifiers, string
// and template literals, numbers, regular expressions and punctuators,
// drops comments, and pairs brackets. A route written inside a comment or
// a string is never a token, so it can never become a route. JSX text is
// not understood: an apostrophe in it opens a string that ends at the end
// of its line, which costs at most that line.

import (
	"bytes"
	"sort"
	"strings"
)

type jsTokKind uint8

const (
	tkIdent jsTokKind = iota + 1
	tkString
	tkTemplate
	tkNumber
	tkRegex
	tkPunct
)

// jsTok is one token. For a string, val is the decoded value; for a
// template, val is its text with each ${…} written as "\x00" and subs
// holds the substitutions' source text, trimmed.
type jsTok struct {
	kind jsTokKind
	val  string
	line int
	subs []string
}

func (t jsTok) is(kind jsTokKind, val string) bool { return t.kind == kind && t.val == val }
func (t jsTok) punct(val string) bool              { return t.kind == tkPunct && t.val == val }

// regexAfterWord: after these keywords a "/" opens a regular expression.
var regexAfterWord = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true,
	"yield": true, "await": true,
}

// multiPunct are the punctuators longer than one byte that the readers
// tell apart ("=" is an assignment, "==" is not); longest first.
var multiPunct = []string{"===", "!==", "...", "**=", "=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "+=", "-=", "*=", "/=", "++", "--"}

func isIdentByte(c byte, first bool) bool {
	switch {
	case c == '_' || c == '$' || c == '#':
		return true
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return !first
	case c >= 0x80:
		return true
	}
	return false
}

// lexJS splits src into tokens. It never fails: an unterminated string,
// template or comment ends the token where the input (or, for a string,
// the line) ends.
func lexJS(src []byte) []jsTok {
	l := jsLexer{src: src, line: 1}
	if bytes.HasPrefix(src, []byte("#!")) {
		for l.i < len(src) && src[l.i] != '\n' {
			l.i++
		}
	}
	l.run()
	return l.out
}

type jsLexer struct {
	src  []byte
	i    int
	line int
	out  []jsTok
}

func (l *jsLexer) regexAllowed() bool {
	if len(l.out) == 0 {
		return true
	}
	p := l.out[len(l.out)-1]
	switch p.kind {
	case tkIdent:
		return regexAfterWord[p.val]
	case tkPunct:
		switch p.val {
		case ")", "]", "++", "--":
			return false
		}
		return true
	}
	return false
}

func (l *jsLexer) run() {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '/':
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '*':
			end := bytes.Index(src[l.i+2:], []byte("*/"))
			stop := len(src)
			if end >= 0 {
				stop = l.i + 2 + end + 2
			}
			l.line += bytes.Count(src[l.i:stop], []byte("\n"))
			l.i = stop
		case c == '\'' || c == '"':
			l.str(c)
		case c == '`':
			start := l.line
			val, subs := l.template()
			l.out = append(l.out, jsTok{kind: tkTemplate, val: val, line: start, subs: subs})
		case c >= '0' && c <= '9' || c == '.' && l.i+1 < len(src) && src[l.i+1] >= '0' && src[l.i+1] <= '9':
			j := l.i + 1
			for j < len(src) && (isIdentByte(src[j], false) || src[j] == '.') {
				j++
			}
			l.out = append(l.out, jsTok{kind: tkNumber, val: string(src[l.i:j]), line: l.line})
			l.i = j
		case isIdentByte(c, true):
			j := l.i + 1
			for j < len(src) && isIdentByte(src[j], false) {
				j++
			}
			l.out = append(l.out, jsTok{kind: tkIdent, val: string(src[l.i:j]), line: l.line})
			l.i = j
		case c == '/' && l.regexAllowed():
			if !l.regex() {
				l.punct()
			}
		default:
			l.punct()
		}
	}
}

func (l *jsLexer) punct() {
	rest := l.src[l.i:]
	for _, p := range multiPunct {
		if bytes.HasPrefix(rest, []byte(p)) {
			if p == "?." && len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9' {
				continue // a ? .5 conditional, not optional chaining
			}
			l.out = append(l.out, jsTok{kind: tkPunct, val: p, line: l.line})
			l.i += len(p)
			return
		}
	}
	l.out = append(l.out, jsTok{kind: tkPunct, val: string(rest[:1]), line: l.line})
	l.i++
}

// str reads a quoted string; a newline ends an unterminated one.
func (l *jsLexer) str(q byte) {
	src := l.src
	var b strings.Builder
	j := l.i + 1
	for j < len(src) && src[j] != q && src[j] != '\n' {
		if src[j] == '\\' && j+1 < len(src) {
			j += l.escape(&b, j)
			continue
		}
		b.WriteByte(src[j])
		j++
	}
	if j < len(src) && src[j] == q {
		j++
	}
	l.out = append(l.out, jsTok{kind: tkString, val: b.String(), line: l.line})
	l.i = j
}

// escape decodes the escape at src[j] ('\\') into b and returns its length.
func (l *jsLexer) escape(b *strings.Builder, j int) int {
	src := l.src
	n := src[j+1]
	switch n {
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case '\n':
		l.line++ // a line continuation
	default:
		b.WriteByte(n)
	}
	return 2
}

// template reads a template literal from l.i (the opening backquote).
func (l *jsLexer) template() (string, []string) {
	src := l.src
	var b strings.Builder
	var subs []string
	j := l.i + 1
	for j < len(src) && src[j] != '`' {
		switch {
		case src[j] == '\\' && j+1 < len(src):
			j += l.escape(&b, j)
		case src[j] == '$' && j+1 < len(src) && src[j+1] == '{':
			end := l.skipBalanced(j + 2)
			subs = append(subs, strings.TrimSpace(string(src[j+2:min(end, len(src))])))
			b.WriteByte(0)
			j = end + 1
		default:
			if src[j] == '\n' {
				l.line++
			}
			b.WriteByte(src[j])
			j++
		}
	}
	l.i = min(j+1, len(src))
	return b.String(), subs
}

// skipBalanced returns the index of the "}" closing a ${ substitution whose
// body starts at j, stepping over nested braces, strings, templates and
// comments, counting lines.
func (l *jsLexer) skipBalanced(j int) int {
	src := l.src
	depth := 0
	for j < len(src) {
		switch c := src[j]; {
		case c == '\n':
			l.line++
		case c == '{':
			depth++
		case c == '}':
			if depth == 0 {
				return j
			}
			depth--
		case c == '\'' || c == '"':
			k := j + 1
			for k < len(src) && src[k] != c && src[k] != '\n' {
				if src[k] == '\\' {
					k++
				}
				k++
			}
			j = k
		case c == '`':
			save := l.i
			l.i = j
			l.template()
			j = l.i - 1
			l.i = save
		case c == '/' && j+1 < len(src) && src[j+1] == '/':
			for j < len(src) && src[j] != '\n' {
				j++
			}
			continue
		}
		j++
	}
	return len(src)
}

// regex reads a regular expression literal; false when none closes on the
// line (the "/" is then a punctuator).
func (l *jsLexer) regex() bool {
	src := l.src
	j := l.i + 1
	inClass := false
	for j < len(src) && src[j] != '\n' {
		switch src[j] {
		case '\\':
			j++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				k := j + 1
				for k < len(src) && isIdentByte(src[k], false) {
					k++
				}
				l.out = append(l.out, jsTok{kind: tkRegex, val: string(src[l.i:k]), line: l.line})
				l.i = k
				return true
			}
		}
		j++
	}
	return false
}

// ---------------------------------------------------------------------
// The shared file model: tokens, bracket pairs, string constants, imports,
// functions and classes.
// ---------------------------------------------------------------------

// jsImport is what a local name was bound to by an import or require: the
// module specifier and the name the module exports ("default" for a
// default import or a plain require, "*" for a namespace).
type jsImport struct {
	spec string
	name string
}

// jsFunc is one function, arrow function or method: its parameter names
// and the token range of its body (an expression body ends before the
// token that ends the expression).
type jsFunc struct {
	params   []string
	open     int // the body's first token ("{" for a block)
	close    int // the body's last token
	callArg  int // when the function is an argument of a call: that call's "(" token, else -1
	argIndex int
}

// jsClass is one class declaration: its name, body range and the
// decorators written before it (each the index of its "@").
type jsClass struct {
	name        string
	open, close int
	decorators  []int
}

type jsFile struct {
	path    string
	toks    []jsTok
	match   []int // for "(", "[" and "{": the index of the closing token; -1 otherwise
	consts  map[string]string
	imports map[string]jsImport
	funcs   []jsFunc
	classes []jsClass
}

func newJSFile(p string, src []byte) *jsFile {
	f := &jsFile{path: p, toks: lexJS(src)}
	f.pairBrackets()
	f.consts = f.collectConsts()
	f.imports = f.collectImports()
	f.funcs = f.collectFuncs()
	f.classes = f.collectClasses()
	return f
}

func (f *jsFile) tok(i int) jsTok {
	if i < 0 || i >= len(f.toks) {
		return jsTok{}
	}
	return f.toks[i]
}

func (f *jsFile) pairBrackets() {
	f.match = make([]int, len(f.toks))
	var stack []int
	closer := map[string]string{")": "(", "]": "[", "}": "{"}
	for i, t := range f.toks {
		f.match[i] = -1
		if t.kind != tkPunct {
			continue
		}
		switch t.val {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			want := closer[t.val]
			// An unbalanced closer (JSX text, a lexing slip) pops back to
			// its own opener when one is open, else is ignored.
			for k := len(stack) - 1; k >= 0; k-- {
				if f.toks[stack[k]].val == want {
					f.match[stack[k]] = i
					stack = stack[:k]
					break
				}
			}
		}
	}
}

// closeOf is the matching close of the opener at i, or -1.
func (f *jsFile) closeOf(i int) int {
	if i < 0 || i >= len(f.match) {
		return -1
	}
	return f.match[i]
}

// args splits the bracketed list opening at open into its top-level
// comma-separated items, each a [lo, hi) token range. An empty list has
// none; a trailing comma adds none.
func (f *jsFile) args(open int) [][2]int {
	end := f.closeOf(open)
	if end < 0 {
		return nil
	}
	var out [][2]int
	lo := open + 1
	for i := open + 1; i < end; i++ {
		t := f.toks[i]
		if t.kind == tkPunct {
			switch t.val {
			case "(", "[", "{":
				if c := f.match[i]; c > 0 && c < end {
					i = c
				}
				continue
			case ",":
				out = append(out, [2]int{lo, i})
				lo = i + 1
				continue
			}
		}
	}
	if lo < end {
		out = append(out, [2]int{lo, end})
	}
	return out
}

// stringAt resolves the expression in [lo, hi) to a string: a string
// literal, a template whose substitutions are all string constants, a
// constant, or a "+" concatenation of those.
func (f *jsFile) stringAt(lo, hi int) (string, bool) {
	if lo >= hi {
		return "", false
	}
	// TypeScript's "as const" / "as string" after a literal.
	if hi-lo >= 3 && f.toks[hi-2].is(tkIdent, "as") {
		hi -= 2
	}
	var b strings.Builder
	want := true // an operand is expected next
	for i := lo; i < hi; i++ {
		t := f.toks[i]
		switch {
		case want && t.kind == tkString:
			b.WriteString(t.val)
		case want && t.kind == tkTemplate:
			s, ok := f.fillTemplate(t)
			if !ok {
				return "", false
			}
			b.WriteString(s)
		case want && t.kind == tkIdent:
			s, ok := f.consts[t.val]
			if !ok {
				return "", false
			}
			b.WriteString(s)
		case want && t.punct("("):
			c := f.closeOf(i)
			if c < 0 || c >= hi {
				return "", false
			}
			s, ok := f.stringAt(i+1, c)
			if !ok {
				return "", false
			}
			b.WriteString(s)
			i = c
		case !want && t.punct("+"):
			want = true
			continue
		default:
			return "", false
		}
		want = false
	}
	if want {
		return "", false
	}
	return b.String(), true
}

func (f *jsFile) fillTemplate(t jsTok) (string, bool) {
	if len(t.subs) == 0 {
		return t.val, true
	}
	var b strings.Builder
	k := 0
	for i := 0; i < len(t.val); i++ {
		if t.val[i] != 0 {
			b.WriteByte(t.val[i])
			continue
		}
		if k >= len(t.subs) {
			return "", false
		}
		s, ok := f.consts[t.subs[k]]
		if !ok {
			return "", false
		}
		b.WriteString(s)
		k++
	}
	return b.String(), true
}

// stringsAt resolves [lo, hi) to one string or an array literal of them.
func (f *jsFile) stringsAt(lo, hi int) ([]string, bool) {
	if lo < hi && f.toks[lo].punct("[") && f.closeOf(lo) == hi-1 {
		var out []string
		for _, a := range f.args(lo) {
			s, ok := f.stringAt(a[0], a[1])
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	}
	s, ok := f.stringAt(lo, hi)
	if !ok {
		return nil, false
	}
	return []string{s}, true
}

// collectConsts reads `const|let|var NAME = <string expression>` and
// `NAME: <string>` never: only declarations. A name declared twice with
// different values is dropped (which one a use sees is a scope question).
func (f *jsFile) collectConsts() map[string]string {
	out := map[string]string{}
	bad := map[string]bool{}
	f.consts = out
	for i := 0; i+3 < len(f.toks); i++ {
		t := f.toks[i]
		if t.kind != tkIdent || (t.val != "const" && t.val != "let" && t.val != "var") {
			continue
		}
		name := f.toks[i+1]
		if name.kind != tkIdent {
			continue
		}
		j := i + 2
		if f.toks[j].punct(":") { // a type annotation
			for j < len(f.toks) && !f.toks[j].punct("=") && !f.toks[j].punct(";") {
				j++
			}
		}
		if j >= len(f.toks) || !f.toks[j].punct("=") {
			continue
		}
		end := f.exprEnd(j + 1)
		if s, ok := f.stringAt(j+1, end); ok {
			if prev, seen := out[name.val]; seen && prev != s {
				bad[name.val] = true
			}
			out[name.val] = s
		}
	}
	for n := range bad {
		delete(out, n)
	}
	return out
}

// exprEnd returns the index just past the expression starting at i: the
// first top-level ",", ";", or closer of an enclosing bracket, or a line
// break followed by a token that cannot continue an expression.
func (f *jsFile) exprEnd(i int) int {
	for j := i; j < len(f.toks); j++ {
		t := f.toks[j]
		if t.kind == tkPunct {
			switch t.val {
			case "(", "[", "{":
				if c := f.match[j]; c > 0 {
					j = c
					continue
				}
				return j
			case ",", ";", ")", "]", "}":
				return j
			}
		}
		if j > i && t.line > f.toks[j-1].line && j+1 <= len(f.toks) {
			prev := f.toks[j-1]
			if (prev.kind != tkPunct || prev.val == ")" || prev.val == "]" || prev.val == "}") && t.kind != tkPunct {
				return j // ASI: a new statement on a new line
			}
		}
	}
	return len(f.toks)
}

// requireSpec reports whether [i] starts `require('spec')` and returns the
// specifier and the index of the call's ")".
func (f *jsFile) requireSpec(i int) (string, int, bool) {
	if !f.tok(i).is(tkIdent, "require") || !f.tok(i+1).punct("(") {
		return "", 0, false
	}
	c := f.closeOf(i + 1)
	if c != i+3 || f.tok(i+2).kind != tkString {
		return "", 0, false
	}
	return f.toks[i+2].val, c, true
}

// collectImports reads ES imports and CommonJS requires bound to a name.
func (f *jsFile) collectImports() map[string]jsImport {
	out := map[string]jsImport{}
	n := len(f.toks)
	for i := 0; i < n; i++ {
		t := f.toks[i]
		if t.is(tkIdent, "import") && i+1 < n && !f.toks[i+1].punct("(") && !f.toks[i+1].punct(".") {
			f.readImport(i, out)
			continue
		}
		spec, c, ok := f.requireSpec(i)
		if !ok {
			continue
		}
		name := "default"
		if f.tok(c+1).punct(".") && f.tok(c+2).kind == tkIdent {
			name = f.toks[c+2].val
		}
		// Walk back over `a = b = require(...)` and `const {x, y: z} = require(...)`.
		j := i - 1
		for j >= 1 && f.toks[j].punct("=") {
			lhs := f.toks[j-1]
			switch {
			case lhs.kind == tkIdent:
				out[lhs.val] = jsImport{spec: spec, name: name}
				j -= 2
				if j >= 0 && f.toks[j].punct(".") { // module.exports = require(...)
					j = -1
				}
				continue
			case lhs.punct("}"):
				open := -1
				for k := j - 1; k >= 0; k-- {
					if f.match[k] == j-1 {
						open = k
						break
					}
				}
				if open >= 0 {
					for _, a := range f.args(open) {
						local, exported := f.destructured(a[0], a[1])
						if local != "" {
							out[local] = jsImport{spec: spec, name: exported}
						}
					}
				}
			}
			break
		}
	}
	return out
}

// destructured reads one `{ a }`, `{ a: b }` or `{ a = 1 }` member.
func (f *jsFile) destructured(lo, hi int) (local, exported string) {
	if lo >= hi || f.toks[lo].kind != tkIdent {
		return "", ""
	}
	exported = f.toks[lo].val
	local = exported
	if lo+2 < hi+1 && f.tok(lo+1).punct(":") && f.tok(lo+2).kind == tkIdent && lo+2 < hi {
		local = f.toks[lo+2].val
	}
	return local, exported
}

func (f *jsFile) readImport(i int, out map[string]jsImport) {
	n := len(f.toks)
	j := i + 1
	if j < n && f.toks[j].is(tkIdent, "type") && j+1 < n && !f.toks[j+1].is(tkIdent, "from") {
		return // a type-only import binds no value
	}
	type bind struct{ local, name string }
	var binds []bind
	for j < n && !f.toks[j].is(tkIdent, "from") && f.toks[j].kind != tkString {
		t := f.toks[j]
		switch {
		case t.punct("*") && f.tok(j+1).is(tkIdent, "as") && f.tok(j+2).kind == tkIdent:
			binds = append(binds, bind{f.toks[j+2].val, "*"})
			j += 3
		case t.punct("{"):
			c := f.closeOf(j)
			if c < 0 {
				return
			}
			for _, a := range f.args(j) {
				lo := a[0]
				if f.tok(lo).is(tkIdent, "type") && a[1]-lo > 1 {
					continue
				}
				if f.tok(lo).kind != tkIdent && f.tok(lo).kind != tkString {
					continue
				}
				name := f.toks[lo].val
				local := name
				if a[1]-lo >= 3 && f.tok(lo+1).is(tkIdent, "as") {
					local = f.toks[lo+2].val
				}
				binds = append(binds, bind{local, name})
			}
			j = c + 1
		case t.kind == tkIdent && t.val != "as":
			binds = append(binds, bind{t.val, "default"})
			j++
		default:
			j++
		}
		if j-i > 400 {
			return
		}
	}
	if j < n && f.toks[j].is(tkIdent, "from") {
		j++
	}
	if j >= n || f.toks[j].kind != tkString {
		return
	}
	for _, b := range binds {
		out[b.local] = jsImport{spec: f.toks[j].val, name: b.name}
	}
}

// paramNames reads the parameter list opening at open: each parameter's
// own name (a destructured parameter names none), past decorators,
// accessibility modifiers, rest dots, type annotations and defaults.
func (f *jsFile) paramNames(open int) []string {
	var out []string
	for _, a := range f.args(open) {
		lo := a[0]
		for lo < a[1] {
			t := f.toks[lo]
			if t.punct("@") { // a parameter decorator: @Body() dto
				lo++
				if lo < a[1] && f.toks[lo].kind == tkIdent {
					lo++
				}
				for lo < a[1] && f.toks[lo].punct(".") && lo+1 < a[1] {
					lo += 2
				}
				if lo < a[1] && f.toks[lo].punct("(") {
					lo = f.closeOf(lo) + 1
				}
				continue
			}
			if t.punct("...") || t.kind == tkIdent && (t.val == "public" || t.val == "private" || t.val == "protected" || t.val == "readonly" || t.val == "override") {
				lo++
				continue
			}
			break
		}
		if lo < a[1] && f.toks[lo].kind == tkIdent {
			out = append(out, f.toks[lo].val)
		} else {
			out = append(out, "")
		}
	}
	return out
}

// funcKeywords never name a method: `if (x) {` is not a method `if`.
var funcKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true,
	"return": true, "with": true, "typeof": true, "await": true, "yield": true, "super": true,
	"import": true, "new": true, "delete": true, "void": true, "do": true, "else": true,
}

// collectFuncs finds functions (declarations and expressions), arrow
// functions and methods.
func (f *jsFile) collectFuncs() []jsFunc {
	var out []jsFunc
	n := len(f.toks)
	for i := 0; i < n; i++ {
		t := f.toks[i]
		var open int
		switch {
		case t.is(tkIdent, "function"):
			j := i + 1
			if f.tok(j).punct("*") {
				j++
			}
			if f.tok(j).kind == tkIdent {
				j++
			}
			if f.tok(j).punct("<") { // generic parameters
				for j < n && !f.toks[j].punct("(") {
					j++
				}
			}
			if !f.tok(j).punct("(") {
				continue
			}
			open = j
		case t.punct("=>"):
			// (a, b) => / a => / (a: T): R =>
			j := i - 1
			if f.tok(j).kind == tkIdent && !f.tok(j-1).punct(".") {
				body, end := f.arrowBody(i)
				out = append(out, f.fn([]string{f.toks[j].val}, body, end, j))
				continue
			}
			for k := j; k >= 0 && k >= i-60; k-- {
				if f.toks[k].punct(")") {
					for o := k - 1; o >= 0; o-- {
						if f.match[o] == k {
							body, end := f.arrowBody(i)
							out = append(out, f.fn(f.paramNames(o), body, end, o))
							break
						}
					}
					break
				}
			}
			continue
		case t.punct("(") && i > 0 && f.toks[i-1].kind == tkIdent && !funcKeywords[f.toks[i-1].val] && !f.tok(i-2).punct("."):
			// A method: `name(params) {` or `name(params): T {`.
			c := f.closeOf(i)
			if c < 0 {
				continue
			}
			k := c + 1
			if f.tok(k).punct(":") {
				for k < n && k < c+40 && !f.toks[k].punct("{") && !f.toks[k].punct(";") && !f.toks[k].punct("=>") {
					if f.toks[k].punct("(") || f.toks[k].punct("[") {
						if m := f.closeOf(k); m > 0 {
							k = m
						}
					}
					k++
				}
			}
			if !f.tok(k).punct("{") || !f.inClassOrObjectBody(i-1) {
				continue
			}
			bc := f.closeOf(k)
			if bc < 0 {
				continue
			}
			out = append(out, jsFunc{params: f.paramNames(i), open: k, close: bc, callArg: -1})
			continue
		default:
			continue
		}
		c := f.closeOf(open)
		if c < 0 {
			continue
		}
		k := c + 1
		for k < n && k < c+40 && !f.toks[k].punct("{") {
			k++
		}
		if !f.tok(k).punct("{") {
			continue
		}
		fn := f.fn(f.paramNames(open), k, f.closeOf(k), i)
		out = append(out, fn)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].open < out[b].open })
	return out
}

// inClassOrObjectBody reports whether the name token at i is a member of
// a class body or object literal: preceded by "{", ",", ";", "}", a
// decorator's ")", or a modifier.
func (f *jsFile) inClassOrObjectBody(i int) bool {
	p := f.tok(i - 1)
	switch {
	case p.kind == tkPunct:
		return p.val == "{" || p.val == "," || p.val == ";" || p.val == "}" || p.val == ")" || p.val == "*"
	case p.kind == tkIdent:
		switch p.val {
		case "async", "static", "public", "private", "protected", "override", "get", "set", "readonly":
			return true
		}
	}
	return false
}

// arrowBody returns the body range of the arrow whose "=>" is at i.
func (f *jsFile) arrowBody(i int) (int, int) {
	if f.tok(i + 1).punct("{") {
		return i + 1, f.closeOf(i + 1)
	}
	return i + 1, f.exprEnd(i+1) - 1
}

// fn builds a jsFunc and, when the function starts at start inside a
// call's argument list, records that call.
func (f *jsFile) fn(params []string, open, close, start int) jsFunc {
	j := jsFunc{params: params, open: open, close: close, callArg: -1}
	// `async` before the function belongs to it.
	if f.tok(start-1).is(tkIdent, "async") {
		start--
	}
	p := f.tok(start - 1)
	if p.punct("(") || p.punct(",") {
		// Find the call's "(" whose argument list holds start.
		for k := start - 1; k >= 0; k-- {
			if f.toks[k].punct("(") && f.match[k] > start {
				for ai, a := range f.args(k) {
					if a[0] == start {
						j.callArg, j.argIndex = k, ai
					}
				}
				break
			}
			if (f.toks[k].punct(")") || f.toks[k].punct("]") || f.toks[k].punct("}")) && k < start-1 {
				for o := k - 1; o >= 0; o-- {
					if f.match[o] == k {
						k = o
						break
					}
				}
			}
		}
	}
	return j
}

// enclosing returns the innermost function whose body holds token i.
func (f *jsFile) enclosing(i int) (jsFunc, bool) {
	best := -1
	for k, fn := range f.funcs {
		if fn.open > i {
			break
		}
		if fn.open <= i && i <= fn.close {
			if best < 0 || fn.open >= f.funcs[best].open {
				best = k
			}
		}
	}
	if best < 0 {
		return jsFunc{}, false
	}
	return f.funcs[best], true
}

// collectClasses finds class declarations with their decorators.
func (f *jsFile) collectClasses() []jsClass {
	var out []jsClass
	for i, t := range f.toks {
		if !t.is(tkIdent, "class") || f.tok(i-1).punct(".") {
			continue
		}
		name := ""
		j := i + 1
		if f.tok(j).kind == tkIdent && f.tok(j).val != "extends" && f.tok(j).val != "implements" {
			name = f.toks[j].val
			j++
		}
		for j < len(f.toks) && !f.toks[j].punct("{") {
			if f.toks[j].punct("(") || f.toks[j].punct("<") && false {
				if c := f.closeOf(j); c > 0 {
					j = c
				}
			}
			j++
		}
		c := f.closeOf(j)
		if c < 0 {
			continue
		}
		cls := jsClass{name: name, open: j, close: c}
		// Decorators before `export default abstract class`.
		k := i - 1
		for k >= 0 && f.toks[k].kind == tkIdent && (f.toks[k].val == "export" || f.toks[k].val == "default" || f.toks[k].val == "abstract" || f.toks[k].val == "declare") {
			k--
		}
		cls.decorators = f.decoratorsBefore(k)
		out = append(out, cls)
	}
	return out
}

// decoratorsBefore walks back from k (the token before a declaration) over
// `@Name(...)` / `@a.b(...)` / `@Name` decorators and returns each "@"
// index in source order.
func (f *jsFile) decoratorsBefore(k int) []int {
	var out []int
	for k >= 0 {
		end := k
		if f.toks[end].punct(")") {
			o := -1
			for x := end - 1; x >= 0 && x >= end-4000; x-- {
				if f.match[x] == end {
					o = x
					break
				}
			}
			if o < 0 {
				break
			}
			end = o - 1
		}
		// end is the decorator's name's last identifier.
		s := end
		for s >= 2 && f.toks[s].kind == tkIdent && f.toks[s-1].punct(".") && f.toks[s-2].kind == tkIdent {
			s -= 2
		}
		if s < 1 || f.toks[s].kind != tkIdent || !f.toks[s-1].punct("@") {
			break
		}
		out = append([]int{s - 1}, out...)
		k = s - 2
	}
	return out
}

// decorator reads the decorator at "@" index at: its (last) name and the
// "(" of its arguments, or -1.
func (f *jsFile) decorator(at int) (string, int) {
	j := at + 1
	name := f.tok(j).val
	for f.tok(j+1).punct(".") && f.tok(j+2).kind == tkIdent {
		j += 2
		name = f.toks[j].val
	}
	if f.tok(j + 1).punct("(") {
		return name, j + 1
	}
	return name, -1
}

// relSpec reports whether a module specifier names a file of the repo.
func relSpec(spec string) bool {
	return strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == ".."
}

// mountTarget turns a relative import specifier into a Mount target,
// stripping a JS extension the import names (the compose step resolves
// "./x" to x.js, x.ts or x/index.*).
func mountTarget(spec string) string {
	if spec == "." || spec == ".." {
		return spec + "/"
	}
	for _, ext := range []string{".js", ".mjs", ".cjs", ".ts", ".mts", ".cts", ".jsx", ".tsx"} {
		if strings.HasSuffix(spec, ext) {
			return strings.TrimSuffix(spec, ext)
		}
	}
	return spec
}

// mountPrefix joins a mount's prefix pieces; the root is "" (a mount
// that adds no path).
func mountPrefix(parts ...string) string {
	if p := joinRoute(parts...); p != "/" {
		return p
	}
	return ""
}

// joinRoute joins route path pieces with one "/" between them.
func joinRoute(parts ...string) string {
	var segs []string
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			segs = append(segs, p)
		}
	}
	return "/" + strings.Join(segs, "/")
}

// ---------------------------------------------------------------------
// Express (LB-F328)
// ---------------------------------------------------------------------

func init() {
	Register(Reader{Framework: "express", Langs: []string{"js", "ts"}, Read: readExpress})
}

// expressMethods are the router methods that register a route; "all"
// matches every method (ANY, the Go readers' spelling).
var expressMethods = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE", "patch": "PATCH",
	"head": "HEAD", "options": "OPTIONS", "all": "ANY",
}

// otherNodeFrameworks are imports that make a file not an Express file
// even when its receivers are named app or router.
var otherNodeFrameworks = []string{"fastify", "koa", "@koa/router", "koa-router", "hono", "restify", "@hapi/hapi", "@nestjs/", "elysia", "h3"}

// expressPrefilter: a file none of these appear in registers no Express route.
var expressPrefilter = [][]byte{[]byte("express"), []byte("Router"), []byte("app."), []byte("router.")}

// readExpress reads Express routes and mounts (LB-F328).
//
//   - app.METHOD(path, …, handler), router.METHOD(…) and
//     router.route(path).get(h).post(h2) register routes; "all" is ANY.
//     A receiver is a name bound to express(), express.Router() or a
//     Router imported from express, or any name app or router (a
//     parameter, an import, a global) in a file that imports no other
//     web framework; a call needs a path and a handler to be a route.
//   - app.use(path, router) composes in the file when router is a router
//     of the same file; when it is an import of a repo file
//     (require('./x'), import x from './x') it is a Mount of that file
//     under the path; a use() with no path mounts under "". A use() is
//     never a route.
//   - require('./routes')(app) mounts ./routes under app's own prefix.
//   - The handler is the last argument: a function name, the exported
//     name of a module member (user.list is "list"), Class.method for a
//     class's member, the innermost named argument of a wrapper call
//     (asyncHandler(list) is "list"), and "" for an inline function.
func readExpress(p string, src []byte) Result {
	if !anyContains(src, expressPrefilter) {
		return Result{}
	}
	f := newJSFile(p, src)
	x := expressReader{f: f, roots: map[string]bool{}, routers: map[string]bool{}, prefixes: map[string][]string{}}
	if !x.gate() {
		return Result{}
	}
	x.bindReceivers()
	x.composeInFile()
	x.read()
	sortRoutes(x.out.Routes)
	sortMounts(x.out.Mounts)
	return x.out
}

func anyContains(src []byte, needles [][]byte) bool {
	for _, n := range needles {
		if bytes.Contains(src, n) {
			return true
		}
	}
	return false
}

type expressReader struct {
	f        *jsFile
	express  map[string]bool // names bound to the express module
	routerFn map[string]bool // names bound to express's Router
	roots    map[string]bool // receivers that are apps (express())
	routers  map[string]bool // receivers that are routers (Router())
	prefixes map[string][]string
	out      Result
}

// gate decides whether the file is an Express file.
func (x *expressReader) gate() bool {
	x.express, x.routerFn = map[string]bool{}, map[string]bool{}
	for local, imp := range x.f.imports {
		for _, o := range otherNodeFrameworks {
			if imp.spec == o || strings.HasPrefix(imp.spec, o) && strings.HasSuffix(o, "/") {
				return false
			}
		}
		if imp.spec != "express" {
			// A module bound to the name express is Express wherever it is
			// loaded from (a vendored copy, a monorepo path).
			if local == "express" && (imp.name == "default" || imp.name == "*") {
				x.express[local] = true
			}
			continue
		}
		switch imp.name {
		case "default", "*":
			x.express[local] = true
		case "Router":
			x.routerFn[local] = true
		}
	}
	return true
}

// bindReceivers finds names bound to express() and Router().
func (x *expressReader) bindReceivers() {
	f := x.f
	for i := range f.toks {
		kind := x.creation(i)
		if kind == "" {
			continue
		}
		// Walk back over `a = b = <creation>` (module.exports = … ends it).
		j := i - 1
		for j >= 1 && f.toks[j].punct("=") {
			lhs := f.toks[j-1]
			if lhs.kind != tkIdent {
				break
			}
			if f.tok(j - 2).punct(".") {
				k := j - 1 // module.exports = …: step over the member expression
				for f.tok(k-1).punct(".") && f.tok(k-2).kind == tkIdent {
					k -= 2
				}
				j = k - 1
				continue
			}
			if kind == "app" {
				x.roots[lhs.val] = true
			} else {
				x.routers[lhs.val] = true
			}
			j -= 2
		}
	}
}

// creation reports what the expression starting at i creates: "app" for
// express() / require('express')(), "router" for express.Router(),
// Router(), new Router() or require('express').Router().
func (x *expressReader) creation(i int) string {
	f := x.f
	t := f.tok(i)
	if t.is(tkIdent, "new") {
		i++
		t = f.tok(i)
	}
	if spec, c, ok := f.requireSpec(i); ok && spec == "express" {
		if f.tok(c + 1).punct("(") {
			return "app"
		}
		if f.tok(c+1).punct(".") && f.tok(c+2).is(tkIdent, "Router") && f.tok(c+3).punct("(") {
			return "router"
		}
		return ""
	}
	if t.kind != tkIdent || f.tok(i-1).punct(".") {
		return ""
	}
	switch {
	case x.express[t.val] && f.tok(i+1).punct("("):
		return "app"
	case x.express[t.val] && f.tok(i+1).punct(".") && f.tok(i+2).is(tkIdent, "Router") && f.tok(i+3).punct("("):
		return "router"
	case x.routerFn[t.val] && f.tok(i+1).punct("("):
		return "router"
	}
	return ""
}

// receiver reports whether the identifier at i receives route calls.
func (x *expressReader) receiver(i int) bool {
	t := x.f.tok(i)
	if t.kind != tkIdent || x.f.tok(i-1).punct(".") {
		return false
	}
	if x.roots[t.val] || x.routers[t.val] {
		return true
	}
	return t.val == "app" || t.val == "router"
}

// prefixesOf returns the in-file prefixes of a receiver ({""} unless it is
// a router mounted in this file).
func (x *expressReader) prefixesOf(name string) []string {
	if p, ok := x.prefixes[name]; ok && len(p) > 0 {
		return p
	}
	return []string{""}
}

// composeInFile resolves `X.use(path, r)` where r is a router of this file:
// r's routes are read under X's prefixes joined with path. A fixpoint over
// chains of in-file routers, at most eight deep.
func (x *expressReader) composeInFile() {
	type edge struct{ parent, child, path string }
	var edges []edge
	f := x.f
	for i := range f.toks {
		if !x.receiver(i) || !f.tok(i+1).punct(".") || !f.tok(i+2).is(tkIdent, "use") || !f.tok(i+3).punct("(") {
			continue
		}
		args := f.args(i + 3)
		if len(args) == 0 {
			continue
		}
		last := args[len(args)-1]
		if last[1]-last[0] != 1 || f.toks[last[0]].kind != tkIdent || !x.routers[f.toks[last[0]].val] {
			continue
		}
		paths := []string{""}
		if len(args) > 1 {
			if ps, ok := f.stringsAt(args[0][0], args[0][1]); ok {
				paths = ps
			}
		}
		for _, p := range paths {
			edges = append(edges, edge{f.toks[i].val, f.toks[last[0]].val, p})
		}
	}
	for round := 0; round < 8; round++ {
		next := map[string][]string{}
		for _, e := range edges {
			for _, pp := range x.prefixesOf(e.parent) {
				next[e.child] = appendUnique(next[e.child], joinRoute(pp, e.path))
			}
		}
		if equalPrefixes(next, x.prefixes) {
			break
		}
		x.prefixes = next
	}
}

func appendUnique(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}

func equalPrefixes(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		w := b[k]
		if len(v) != len(w) {
			return false
		}
		for i := range v {
			if v[i] != w[i] {
				return false
			}
		}
	}
	return true
}

func (x *expressReader) read() {
	f := x.f
	for i := range f.toks {
		if !x.receiver(i) || !f.tok(i+1).punct(".") || f.tok(i+2).kind != tkIdent || !f.tok(i+3).punct("(") {
			// require('./routes')(app) / routes(app)
			x.callWithReceiver(i)
			continue
		}
		name := f.toks[i].val
		m := f.toks[i+2].val
		switch {
		case m == "use":
			x.use(i, name)
		case m == "route":
			x.routeChain(i, name)
		case expressMethods[m] != "":
			args := f.args(i + 3)
			if len(args) < 2 {
				continue // app.get('env') reads a setting
			}
			paths, ok := f.stringsAt(args[0][0], args[0][1])
			if !ok || !routePaths(paths) {
				continue
			}
			h := x.handler(args[len(args)-1])
			for _, pre := range x.prefixesOf(name) {
				for _, p := range paths {
					x.out.Routes = append(x.out.Routes, Route{Method: expressMethods[m], Path: joinRoute(pre, p), Handler: h, File: f.path, Line: f.toks[i].line})
				}
			}
		}
	}
}

// routePaths reports whether every path is a route path ("/…" or "*").
func routePaths(ps []string) bool {
	for _, p := range ps {
		if !strings.HasPrefix(p, "/") && p != "*" {
			return false
		}
	}
	return len(ps) > 0
}

// routeChain reads router.route(path).get(h).post(h2)….
func (x *expressReader) routeChain(i int, name string) {
	f := x.f
	args := f.args(i + 3)
	if len(args) != 1 {
		return
	}
	paths, ok := f.stringsAt(args[0][0], args[0][1])
	if !ok || !routePaths(paths) {
		return
	}
	j := f.closeOf(i + 3)
	for j > 0 && f.tok(j+1).punct(".") && f.tok(j+2).kind == tkIdent && f.tok(j+3).punct("(") {
		m := expressMethods[f.toks[j+2].val]
		margs := f.args(j + 3)
		if m != "" && len(margs) > 0 {
			h := x.handler(margs[len(margs)-1])
			for _, pre := range x.prefixesOf(name) {
				for _, p := range paths {
					x.out.Routes = append(x.out.Routes, Route{Method: m, Path: joinRoute(pre, p), Handler: h, File: f.path, Line: f.toks[j+2].line})
				}
			}
		}
		j = f.closeOf(j + 3)
	}
}

// use reads app.use([path,] …, target): a Mount when target is a repo
// module; an in-file router was composed by composeInFile.
func (x *expressReader) use(i int, name string) {
	f := x.f
	args := f.args(i + 3)
	if len(args) == 0 {
		return
	}
	paths := []string{""}
	if len(args) > 1 {
		ps, ok := f.stringsAt(args[0][0], args[0][1])
		if ok && routePaths(ps) {
			paths = ps
		} else if ok {
			return // a path that is not one
		}
	}
	spec, ok := x.moduleOf(args[len(args)-1])
	if !ok {
		return
	}
	for _, pre := range x.prefixesOf(name) {
		for _, p := range paths {
			x.out.Mounts = append(x.out.Mounts, Mount{Prefix: mountPrefix(pre, p), Target: mountTarget(spec), File: f.path, Line: f.toks[i].line})
		}
	}
}

// callWithReceiver reads `require('./routes')(app)` and `routes(app)` for
// an imported routes: the module registers on app, so it is mounted under
// app's prefixes.
func (x *expressReader) callWithReceiver(i int) {
	f := x.f
	t := f.tok(i)
	var spec string
	var open int
	if s, c, ok := f.requireSpec(i); ok && f.tok(c+1).punct("(") {
		spec, open = s, c+1
	} else if t.kind == tkIdent && !f.tok(i-1).punct(".") && f.tok(i+1).punct("(") {
		imp, ok := f.imports[t.val]
		if !ok {
			return
		}
		spec, open = imp.spec, i+1
	} else {
		return
	}
	if !relSpec(spec) {
		return
	}
	for _, a := range f.args(open) {
		if a[1]-a[0] == 1 && x.receiver(a[0]) {
			for _, pre := range x.prefixesOf(f.toks[a[0]].val) {
				x.out.Mounts = append(x.out.Mounts, Mount{Prefix: mountPrefix(pre), Target: mountTarget(spec), File: f.path, Line: t.line})
			}
			return
		}
	}
}

// moduleOf resolves a use() target to the repo module it names: an
// imported name, require('./x'), require('./x').router, or x.router for
// an imported x.
func (x *expressReader) moduleOf(a [2]int) (string, bool) {
	f := x.f
	lo, hi := a[0], a[1]
	if spec, c, ok := f.requireSpec(lo); ok {
		if c == hi-1 || c+3 == hi && f.tok(c+1).punct(".") {
			return spec, relSpec(spec)
		}
		return "", false
	}
	t := f.tok(lo)
	if t.kind != tkIdent {
		return "", false
	}
	if hi-lo == 1 || hi-lo == 3 && f.tok(lo+1).punct(".") && f.tok(lo+2).kind == tkIdent {
		if imp, ok := f.imports[t.val]; ok && relSpec(imp.spec) {
			return imp.spec, true
		}
	}
	return "", false
}

// handler names the function an argument passes (see readExpress).
func (x *expressReader) handler(a [2]int) string {
	return jsHandlerName(x.f, a[0], a[1])
}

// jsHandlerName names the function the expression [lo, hi) passes, the
// way the TS/JS dialect names its definition.
func jsHandlerName(f *jsFile, lo, hi int) string {
	if lo >= hi {
		return ""
	}
	t := f.toks[lo]
	// x.bind(this) / x.bind(null)
	if hi-lo >= 4 && f.toks[hi-1].punct(")") && f.tok(hi-4).is(tkIdent, "bind") && f.tok(hi-5).punct(".") {
		o := hi - 1
		for k := lo; k < hi; k++ {
			if f.match[k] == hi-1 {
				o = k
				break
			}
		}
		return jsHandlerName(f, lo, o-2)
	}
	switch {
	case t.kind == tkIdent && hi-lo == 1:
		if t.val == "undefined" || t.val == "null" {
			return ""
		}
		if imp, ok := f.imports[t.val]; ok && imp.name != "default" && imp.name != "*" {
			return imp.name
		}
		return t.val
	case t.kind == tkIdent && hi-lo == 3 && f.toks[lo+1].punct(".") && f.toks[lo+2].kind == tkIdent:
		obj, member := t.val, f.toks[lo+2].val
		if obj == "this" {
			if cls := f.classAt(lo); cls != "" {
				return cls + "." + member
			}
			return member
		}
		if _, ok := f.imports[obj]; ok {
			return member
		}
		for _, c := range f.classes {
			if c.name == obj {
				return obj + "." + member
			}
		}
		return member
	case t.kind == tkIdent && f.tok(lo+1).punct("(") && f.closeOf(lo+1) == hi-1 && t.val != "function":
		// A wrapper call: name its last argument that names a function.
		args := f.args(lo + 1)
		for k := len(args) - 1; k >= 0; k-- {
			if h := jsHandlerName(f, args[k][0], args[k][1]); h != "" {
				return h
			}
		}
	}
	return ""
}

// classAt returns the name of the class whose body holds token i.
func (f *jsFile) classAt(i int) string {
	name := ""
	for _, c := range f.classes {
		if c.open < i && i < c.close {
			name = c.name
		}
	}
	return name
}


