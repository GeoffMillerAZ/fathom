package routes

// spring.go — the Spring MVC route reader (LB-F322, wave c2) and the Java
// declaration scanner it shares with jaxrs.go.
//
// A route is a method carrying @RequestMapping, @GetMapping, @PostMapping,
// @PutMapping, @DeleteMapping or @PatchMapping, under its type's own
// @RequestMapping prefix (Spring's class-plus-method composition, inside
// the file: no mount). Paths come from value or path, a string or an array;
// an absent or empty path is the prefix alone; a reference to a String
// constant declared in the same file is folded, and a route whose path
// names anything else is dropped rather than guessed (a ceiling the
// reading counts). Methods come from the annotation's name, else from
// method = RequestMethod.X (a single value or an array), united with the
// type's own; none is ANY. A type annotated @FeignClient is a client, never
// a server. The handler is the java dialect's name for the method:
// Class.method, Outer.Inner.method for a nested type.
//
// The scanner is a byte-level Java lexer (comments, string and text-block
// literals, char literals) plus a declaration walk that keeps only what a
// route reader needs: each type's qualified name, kind and annotations,
// each method's name and annotations, and the file's String constants.
// Method bodies, initializers and annotation arguments are skipped by
// bracket matching, so a mapping-like text inside one is never a route.

import (
	"bytes"
	"strings"
)

func init() {
	Register(Reader{Framework: "spring", Langs: []string{"java"}, Read: readSpring})
}

// springMappings maps a method-level mapping annotation to its fixed HTTP
// method ("" for @RequestMapping, which reads method =).
var springMappings = map[string]string{
	"RequestMapping": "", "GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT",
	"DeleteMapping": "DELETE", "PatchMapping": "PATCH",
}

// readSpring is the spring reader.
func readSpring(path string, src []byte) Result {
	if !bytes.Contains(src, []byte("Mapping")) {
		return Result{}
	}
	jf := scanJava(src)
	var out []Route
	for _, t := range jf.types {
		if t.has("FeignClient") {
			continue
		}
		prefixes := []string{""}
		var typeMethods []string
		if a, ok := t.ann("RequestMapping"); ok {
			ps, ok := a.paths(jf.consts)
			if !ok {
				continue // the prefix names something this file does not declare
			}
			if len(ps) > 0 {
				prefixes = ps
			}
			typeMethods = a.idents("method")
		}
		for _, m := range t.methods {
			for _, a := range m.anns {
				fixed, ok := springMappings[a.name]
				if !ok {
					continue
				}
				paths, ok := a.paths(jf.consts)
				if !ok {
					continue
				}
				if len(paths) == 0 {
					paths = []string{""}
				}
				methods := []string{fixed}
				if fixed == "" {
					methods = unionUpper(typeMethods, a.idents("method"))
					if len(methods) == 0 {
						methods = []string{"ANY"}
					}
				} else if len(typeMethods) > 0 {
					methods = unionUpper(typeMethods, methods)
				}
				for _, pre := range prefixes {
					for _, p := range paths {
						full := springCombine(pre, p)
						for _, hm := range methods {
							out = append(out, Route{Method: hm, Path: full, Handler: t.name + "." + m.name, File: path, Line: a.line})
						}
					}
				}
			}
		}
	}
	return Result{Routes: out}
}

// springCombine joins a type-level and a method-level pattern as Spring
// does: each gains a leading "/" when it has none, they join with exactly
// one "/", and two empty patterns are the root.
func springCombine(prefix, p string) string {
	full := joinPathSegments(leadSlash(prefix), leadSlash(p))
	if full == "" {
		return "/"
	}
	return full
}

func leadSlash(s string) string {
	if s != "" && !strings.HasPrefix(s, "/") {
		return "/" + s
	}
	return s
}

// unionUpper is a ∪ b, upper-cased, in first-seen order.
func unionUpper(a, b []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range append(append([]string(nil), a...), b...) {
		s = strings.ToUpper(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// -----------------------------------------------------------------
// The Java declaration scanner (shared with jaxrs.go)
// -----------------------------------------------------------------

type jtokKind uint8

const (
	jIdent  jtokKind = iota
	jString          // val holds the literal's value
	jPunct           // val holds one byte of punctuation
	jOther           // numbers, char literals
)

type jtok struct {
	kind jtokKind
	val  string
	line int
}

func (t jtok) is(p string) bool { return t.kind == jPunct && t.val == p }

// jann is one annotation: its simple name (the last segment of a qualified
// one), its argument tokens (between the parentheses) and its line.
type jann struct {
	name string
	args []jtok
	line int
}

// jmethod is one method or constructor declaration with its annotations.
type jmethod struct {
	name string
	anns []jann
	line int
}

// jtype is one type declaration: name qualified by its enclosing types
// (the java dialect's container chain), kind keyword, annotations and the
// methods declared directly in its body.
type jtype struct {
	name    string
	kind    string // class, interface, enum, record, @interface
	anns    []jann
	methods []jmethod
}

func (t *jtype) has(name string) bool { _, ok := t.ann(name); return ok }

func (t *jtype) ann(name string) (jann, bool) { return annNamed(t.anns, name) }

func annNamed(anns []jann, name string) (jann, bool) {
	for _, a := range anns {
		if a.name == name {
			return a, true
		}
	}
	return jann{}, false
}

// jfile is a scanned Java file: its types in declaration order and the
// String constants it declares.
type jfile struct {
	types  []*jtype
	consts *jconsts
}

// jconsts are a file's String constants by simple name, folded on demand
// so a constant may use one declared further down (or in a nested type) as
// Java allows. A name declared more than once with different values, or
// in a cycle, has no value.
type jconsts struct {
	decls map[string][][]jtok // name -> each declaration's initializer tokens
	memo  map[string]string
	bad   map[string]bool
	busy  map[string]bool
}

func newJConsts() *jconsts {
	return &jconsts{decls: map[string][][]jtok{}, memo: map[string]string{}, bad: map[string]bool{}, busy: map[string]bool{}}
}

// value folds the constant name.
func (c *jconsts) value(name string) (string, bool) {
	if v, ok := c.memo[name]; ok {
		return v, true
	}
	decls, ok := c.decls[name]
	if !ok || c.bad[name] || c.busy[name] {
		return "", false
	}
	c.busy[name] = true
	defer delete(c.busy, name)
	val := ""
	for i, d := range decls {
		v, ok := foldStrings(d, c)
		if !ok || (i > 0 && v != val) {
			c.bad[name] = true
			return "", false
		}
		val = v
	}
	c.memo[name] = val
	return val, true
}

// lexJava tokenizes src. Comments and whitespace are dropped; string and
// text-block literals become jString with their value (escapes \" and \\
// resolved, others kept); an unterminated literal ends the token stream.
func lexJava(src []byte) []jtok {
	var out []jtok
	line := 1
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"':
			start := line
			i += 3
			var b strings.Builder
			for i < n && !(src[i] == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"') {
				if src[i] == '\\' && i+1 < n {
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
			if i >= n {
				return out
			}
			i += 3
			out = append(out, jtok{kind: jString, val: b.String(), line: start})
		case c == '"':
			i++
			var b strings.Builder
			for i < n && src[i] != '"' && src[i] != '\n' {
				if src[i] == '\\' && i+1 < n {
					if e := src[i+1]; e == '"' || e == '\\' || e == '\'' {
						b.WriteByte(e)
					} else {
						b.WriteByte('\\')
						b.WriteByte(e)
					}
					i += 2
					continue
				}
				b.WriteByte(src[i])
				i++
			}
			if i >= n || src[i] != '"' {
				return out
			}
			i++
			out = append(out, jtok{kind: jString, val: b.String(), line: line})
		case c == '\'':
			i++
			for i < n && src[i] != '\'' && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
			out = append(out, jtok{kind: jOther, val: "'", line: line})
		case isJavaIdentStart(c):
			j := i + 1
			for j < n && isJavaIdentPart(src[j]) {
				j++
			}
			out = append(out, jtok{kind: jIdent, val: string(src[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < n && (isJavaIdentPart(src[j]) || (src[j] == '.' && j+1 < n && src[j+1] >= '0' && src[j+1] <= '9')) {
				j++
			}
			out = append(out, jtok{kind: jOther, val: string(src[i:j]), line: line})
			i = j
		default:
			out = append(out, jtok{kind: jPunct, val: string(c), line: line})
			i++
		}
	}
	return out
}

func isJavaIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isJavaIdentPart(c byte) bool { return isJavaIdentStart(c) || (c >= '0' && c <= '9') }

// matchClose returns the index of the token closing the bracket open at
// toks[i] ("(", "{" or "["), or len(toks) when it never closes.
func matchClose(toks []jtok, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].kind != jPunct {
			continue
		}
		switch toks[j].val {
		case "(", "{", "[":
			depth++
		case ")", "}", "]":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks)
}

// javaTypeKeywords are the declaration keywords that open a type body.
var javaTypeKeywords = map[string]bool{"class": true, "interface": true, "enum": true, "record": true}

// scanJava walks src's declarations.
func scanJava(src []byte) jfile {
	toks := lexJava(src)
	jf := jfile{consts: newJConsts()}
	type frame struct {
		t         *jtype
		enumConst bool // an enum body before its first ";": constants, not members
	}
	var stack []frame
	var pending []jann
	memberStart := 0 // index of the current member's first token
	endMember := func(next int) {
		pending = nil
		memberStart = next
	}
	for i := 0; i < len(toks); {
		t := toks[i]
		var top *frame
		if len(stack) > 0 {
			top = &stack[len(stack)-1]
		}
		switch {
		case t.is("@") && i+1 < len(toks) && toks[i+1].kind == jIdent && toks[i+1].val == "interface":
			// An annotation type declaration: "@interface Name {".
			i = openType(toks, i+1, "@interface", &pending, func(nt *jtype, body int) {
				nt.name = qualify(stack, func(f frame) string { return f.t.name }, nt.name)
				jf.types = append(jf.types, nt)
				stack = append(stack, frame{t: nt})
			})
			memberStart = i
		case t.is("@") && i+1 < len(toks) && toks[i+1].kind == jIdent:
			a, next := parseAnnotation(toks, i)
			pending = append(pending, a)
			i = next
		case t.kind == jIdent && javaTypeKeywords[t.val] && !(i > 0 && toks[i-1].is(".")) && i+1 < len(toks) && toks[i+1].kind == jIdent:
			kind := t.val
			i = openType(toks, i, kind, &pending, func(nt *jtype, body int) {
				nt.name = qualify(stack, func(f frame) string { return f.t.name }, nt.name)
				jf.types = append(jf.types, nt)
				stack = append(stack, frame{t: nt, enumConst: kind == "enum"})
			})
			memberStart = i
		case t.is("}"):
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			i++
			endMember(i)
		case t.is(";"):
			if top != nil && top.enumConst {
				top.enumConst = false
			}
			i++
			endMember(i)
		case top != nil && top.enumConst && (t.is("(") || t.is("{")):
			i = matchClose(toks, i) + 1
		case t.is("{"):
			// An initializer block, or a body this walk does not enter.
			i = matchClose(toks, i) + 1
			endMember(i)
		case t.is("="):
			// A field initializer: skip to the member's ";", folding a String
			// constant on the way.
			j := i + 1
			for j < len(toks) && !toks[j].is(";") {
				if toks[j].is("(") || toks[j].is("{") || toks[j].is("[") {
					j = matchClose(toks, j)
				}
				j++
			}
			if name, ok := constName(toks[memberStart:i], top != nil && (top.t.kind == "interface" || top.t.kind == "@interface")); ok {
				jf.consts.decls[name] = append(jf.consts.decls[name], toks[i+1:min(j, len(toks))])
			}
			i = j + 1
			endMember(i)
		case t.is("(") && top != nil && i > 0 && toks[i-1].kind == jIdent:
			// A method or constructor declaration (a field initializer's
			// calls were skipped with it).
			name := toks[i-1]
			line := name.line
			if len(pending) > 0 {
				line = pending[0].line
			}
			top.t.methods = append(top.t.methods, jmethod{name: name.val, anns: pending, line: line})
			j := matchClose(toks, i) + 1
			for j < len(toks) && !toks[j].is("{") && !toks[j].is(";") {
				if toks[j].is("(") {
					j = matchClose(toks, j)
				}
				j++
			}
			if j < len(toks) && toks[j].is("{") {
				j = matchClose(toks, j)
			}
			i = j + 1
			endMember(i)
		default:
			i++
		}
	}
	return jf
}

// openType reads a type declaration whose keyword is at toks[i] ("class",
// "interface", "enum", "record"; for "@interface" i is at "interface"),
// hands the new type (with the pending annotations) to push and returns
// the index after its body's "{". A declaration with no body is skipped.
func openType(toks []jtok, i int, kind string, pending *[]jann, push func(*jtype, int)) int {
	if i+1 >= len(toks) || toks[i+1].kind != jIdent {
		return i + 1
	}
	nt := &jtype{name: toks[i+1].val, kind: kind, anns: *pending}
	*pending = nil
	j := i + 2
	for j < len(toks) && !toks[j].is("{") && !toks[j].is(";") {
		if toks[j].is("(") {
			j = matchClose(toks, j)
		}
		j++
	}
	if j >= len(toks) || !toks[j].is("{") {
		return j + 1
	}
	push(nt, j)
	return j + 1
}

// qualify is name under the enclosing types' container chain.
func qualify[F any](stack []F, name func(F) string, leaf string) string {
	if len(stack) == 0 {
		return leaf
	}
	return name(stack[len(stack)-1]) + "." + leaf
}

// parseAnnotation reads "@a.b.Name" and an optional "(args)" at toks[i].
func parseAnnotation(toks []jtok, i int) (jann, int) {
	a := jann{line: toks[i].line}
	j := i + 1
	for j < len(toks) && toks[j].kind == jIdent {
		a.name = toks[j].val
		if j+2 < len(toks) && toks[j+1].is(".") && toks[j+2].kind == jIdent {
			j += 2
			continue
		}
		j++
		break
	}
	if j < len(toks) && toks[j].is("(") {
		end := matchClose(toks, j)
		a.args = toks[j+1 : min(end, len(toks))]
		j = end + 1
	}
	return a, j
}

// constName reports the declared name of a String field whose tokens up to
// its "=" are decl ("[modifiers] String NAME"): final, or any field of an
// interface (implicitly static final).
func constName(decl []jtok, inInterface bool) (string, bool) {
	var idents []string
	final := inInterface
	for _, t := range decl {
		switch {
		case t.kind == jIdent && t.val == "final":
			final = true
		case t.kind == jIdent:
			idents = append(idents, t.val)
		case t.is("@"), t.is("("), t.is(")"):
			return "", false // an annotated field: not worth guessing
		default:
			return "", false
		}
	}
	if !final || len(idents) < 2 || idents[len(idents)-2] != "String" {
		return "", false
	}
	return idents[len(idents)-1], true
}

// foldStrings evaluates a String constant expression: string literals and
// the file's constants (by simple name, Class.NAME included) joined by
// "+", in parentheses or not.
func foldStrings(toks []jtok, consts *jconsts) (string, bool) {
	if len(toks) == 0 {
		return "", false
	}
	var b strings.Builder
	expectOperand := true
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case expectOperand && t.is("("):
			end := matchClose(toks, i)
			if end >= len(toks) {
				return "", false
			}
			v, ok := foldStrings(toks[i+1:end], consts)
			if !ok {
				return "", false
			}
			b.WriteString(v)
			i = end
			expectOperand = false
		case expectOperand && t.kind == jString:
			b.WriteString(t.val)
			expectOperand = false
		case expectOperand && t.kind == jIdent:
			name := t.val
			for i+2 < len(toks) && toks[i+1].is(".") && toks[i+2].kind == jIdent {
				i += 2
				name = toks[i].val
			}
			v, ok := consts.value(name)
			if !ok {
				return "", false
			}
			b.WriteString(v)
			expectOperand = false
		case !expectOperand && t.is("+"):
			expectOperand = true
		default:
			return "", false
		}
	}
	if expectOperand {
		return "", false
	}
	return b.String(), true
}

// annElements splits an annotation's arguments into named elements; a
// single unnamed value is "value".
func (a jann) elements() map[string][]jtok {
	out := map[string][]jtok{}
	if len(a.args) == 0 {
		return out
	}
	var parts [][]jtok
	start := 0
	for i := 0; i < len(a.args); i++ {
		t := a.args[i]
		if t.is("(") || t.is("{") || t.is("[") {
			i = matchClose(a.args, i)
			continue
		}
		if t.is(",") {
			parts = append(parts, a.args[start:i])
			start = i + 1
		}
	}
	parts = append(parts, a.args[start:])
	for _, p := range parts {
		if len(p) >= 2 && p[0].kind == jIdent && p[1].is("=") {
			out[p[0].val] = p[2:]
			continue
		}
		if len(parts) == 1 {
			out["value"] = p
		}
	}
	return out
}

// arrayItems splits "{a, b}" into its items; any other value is one item.
func arrayItems(v []jtok) [][]jtok {
	if len(v) == 0 || !v[0].is("{") {
		return [][]jtok{v}
	}
	end := matchClose(v, 0)
	if end >= len(v) {
		return nil
	}
	inner := v[1:end]
	var out [][]jtok
	start := 0
	for i := 0; i < len(inner); i++ {
		if inner[i].is("(") || inner[i].is("{") || inner[i].is("[") {
			i = matchClose(inner, i)
			continue
		}
		if inner[i].is(",") {
			out = append(out, inner[start:i])
			start = i + 1
		}
	}
	if start < len(inner) {
		out = append(out, inner[start:])
	}
	return out
}

// paths is the annotation's value or path element as strings (none when
// both are absent); ok is false when an item folds to no constant.
func (a jann) paths(consts *jconsts) ([]string, bool) {
	el := a.elements()
	v, has := el["path"]
	if !has {
		v, has = el["value"]
	}
	if !has {
		return nil, true
	}
	var out []string
	for _, item := range arrayItems(v) {
		if len(item) == 0 {
			continue
		}
		s, ok := foldStrings(item, consts)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// idents is the named element's enum constants by simple name
// (RequestMethod.GET or a static-imported GET), a single value or an array.
func (a jann) idents(name string) []string {
	v, ok := a.elements()[name]
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arrayItems(v) {
		if len(item) > 0 && item[len(item)-1].kind == jIdent {
			out = append(out, item[len(item)-1].val)
		}
	}
	return out
}
