package main

// scan.go -- a declaration scanner for Apache Thrift IDL. `thrift --gen json` carries no source positions and
// resolves typedef names away, so the harness places each declaration, entry and type name itself: a comment- and
// string-aware tokenizer and a small recursive-descent reader of the IDL's declarations (the grammar the compiler's
// thrifty.yy accepts, minus the legacy forms). It is independent of the tree-sitter grammar under test. The compiler
// decides which files are read at all and the definitions it reports must equal the scanner's (main.go checks).

import (
	"fmt"
	"strings"
)

type tokKind int

const (
	tIdent tokKind = iota
	tNumber
	tString
	tPunct
)

type token struct {
	kind tokKind
	text string
	line int
}

// lex returns the tokens of src with comments dropped (`//`, `#` and `/* */`). Strings keep their unquoted text.
func lex(src string) []token {
	var out []token
	line := 1
	i := 0
	n := len(src)
	isIdentStart := func(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	isIdent := func(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' || c == '.' }
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#' || c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"' || c == '\'':
			start := line
			i++
			var b strings.Builder
			for i < n && src[i] != c {
				if src[i] == '\\' && i+1 < n {
					i++
				}
				if src[i] == '\n' {
					line++
				}
				b.WriteByte(src[i])
				i++
			}
			i++
			out = append(out, token{tString, b.String(), start})
		case isIdentStart(c):
			j := i
			for j < n && isIdent(src[j]) {
				j++
			}
			out = append(out, token{tIdent, strings.TrimRight(src[i:j], "."), line})
			i = j
		case c >= '0' && c <= '9' || c == '-' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9' || c == '+' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9':
			j := i + 1
			for j < n && (src[j] >= '0' && src[j] <= '9' || src[j] == '.' || src[j] == 'x' || src[j] == 'X' || src[j] == 'e' || src[j] == 'E' ||
				src[j] >= 'a' && src[j] <= 'f' || src[j] >= 'A' && src[j] <= 'F' ||
				(src[j] == '+' || src[j] == '-') && (src[j-1] == 'e' || src[j-1] == 'E') && !strings.HasPrefix(src[i:], "0x")) {
				j++
			}
			out = append(out, token{tNumber, src[i:j], line})
			i = j
		default:
			out = append(out, token{tPunct, string(c), line})
			i++
		}
	}
	return out
}

// Decl is one declaration or member the scanner places.
type Decl struct {
	Kind      string // namespace typedef const enum enum_value struct union exception field service function
	Name      string
	Container string // the enclosing declaration's name, "" at the top
	Line, End int
	Refs      []TypeRef // type names written inside it (a field's type, a function's signature, a typedef's target)
}

// TypeRef is a user type name as written, `Money` or `shared.Money`.
type TypeRef struct {
	Written string
	Line    int
}

// Include is an `include "x.thrift"` statement.
type Include struct {
	Path string
	Line int
}

// Doc is a scanned file.
type Doc struct {
	Decls    []Decl
	Includes []Include
}

type scanner struct {
	toks []token
	pos  int
	doc  *Doc
	err  error
}

var builtinTypes = map[string]bool{
	"bool": true, "byte": true, "i8": true, "i16": true, "i32": true, "i64": true, "double": true, "string": true,
	"binary": true, "uuid": true, "slist": true, "void": true, "map": true, "list": true, "set": true,
}

func (s *scanner) fail(format string, a ...any) {
	if s.err == nil {
		line := 0
		if s.pos < len(s.toks) {
			line = s.toks[s.pos].line
		} else if len(s.toks) > 0 {
			line = s.toks[len(s.toks)-1].line
		}
		s.err = fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, a...))
	}
}

func (s *scanner) eof() bool { return s.pos >= len(s.toks) || s.err != nil }

func (s *scanner) peek() token {
	if s.pos < len(s.toks) {
		return s.toks[s.pos]
	}
	return token{kind: tPunct}
}

func (s *scanner) is(text string) bool { t := s.peek(); return t.kind != tString && t.text == text }

func (s *scanner) next() token {
	t := s.peek()
	if s.pos < len(s.toks) {
		s.pos++
	}
	return t
}

func (s *scanner) lastLine() int {
	if s.pos > 0 {
		return s.toks[s.pos-1].line
	}
	return 0
}

func (s *scanner) expect(text string) {
	if !s.is(text) {
		s.fail("expected %q, found %q", text, s.peek().text)
		return
	}
	s.next()
}

func (s *scanner) ident() token {
	t := s.next()
	if t.kind != tIdent {
		s.fail("expected an identifier, found %q", t.text)
	}
	return t
}

// Scan reads src's declarations.
func Scan(src string) (*Doc, error) {
	s := &scanner{toks: lex(src), doc: &Doc{}}
	for !s.eof() {
		s.statement()
	}
	return s.doc, s.err
}

func (s *scanner) statement() {
	kw := s.next()
	if kw.kind != tIdent {
		s.fail("unexpected %q", kw.text)
		return
	}
	switch kw.text {
	case "include":
		t := s.next()
		if t.kind != tString {
			s.fail("include needs a string")
			return
		}
		s.doc.Includes = append(s.doc.Includes, Include{t.text, kw.line})
	case "cpp_include":
		s.next()
	case "namespace":
		scope := s.next()
		if scope.kind == tIdent || scope.text == "*" {
			s.ident()
			s.annotations()
			s.add(Decl{Kind: "namespace", Name: scope.text, Line: kw.line, End: s.lastLine()})
		} else {
			s.fail("bad namespace")
		}
	case "typedef":
		d := Decl{Kind: "typedef", Line: kw.line}
		d.Refs = s.fieldType()
		d.Name = s.ident().text
		s.annotations()
		s.optSep()
		d.End = s.lastLine()
		s.add(d)
	case "const":
		d := Decl{Kind: "const", Line: kw.line}
		d.Refs = s.fieldType()
		d.Name = s.ident().text
		s.expect("=")
		s.value()
		s.optSep()
		d.End = s.lastLine()
		s.add(d)
	case "enum":
		s.enum(kw)
	case "struct", "union", "exception":
		s.structure(kw)
	case "service":
		s.service(kw)
	default:
		s.fail("unknown statement %q", kw.text)
	}
}

func (s *scanner) add(d Decl) { s.doc.Decls = append(s.doc.Decls, d) }

func (s *scanner) optSep() {
	if s.is(",") || s.is(";") {
		s.next()
	}
}

// annotations skips `( k = "v", k2 )`.
func (s *scanner) annotations() {
	if s.is("(") {
		s.skipBalanced("(", ")")
	}
}

func (s *scanner) skipBalanced(open, close string) {
	depth := 0
	for !s.eof() {
		t := s.next()
		if t.kind == tPunct && t.text == open {
			depth++
		} else if t.kind == tPunct && t.text == close {
			depth--
			if depth == 0 {
				return
			}
		}
	}
	s.fail("unbalanced %s", open)
}

// value skips a constant value: a scalar, a name, a list or a map literal.
func (s *scanner) value() {
	switch {
	case s.is("["):
		s.skipBalanced("[", "]")
	case s.is("{"):
		s.skipBalanced("{", "}")
	default:
		s.next()
	}
}

// fieldType reads a type and returns its user type names.
func (s *scanner) fieldType() []TypeRef {
	var refs []TypeRef
	t := s.ident()
	switch t.text {
	case "map":
		s.expect("<")
		if s.is("cpp_type") { // `map<cpp_type "x", K, V>`
			s.next()
			s.next()
		}
		refs = append(refs, s.fieldType()...)
		s.expect(",")
		refs = append(refs, s.fieldType()...)
		s.expect(">")
	case "list", "set":
		s.expect("<")
		if s.is("cpp_type") {
			s.next()
			s.next()
		}
		refs = append(refs, s.fieldType()...)
		s.expect(">")
	default:
		if !builtinTypes[t.text] {
			refs = append(refs, TypeRef{t.text, t.line})
		}
	}
	if s.is("&") { // a reference type (`RecList & next`), the Apache compiler's recursive-type extension
		s.next()
	}
	s.annotations()
	return refs
}

func (s *scanner) enum(kw token) {
	name := s.ident()
	s.expect("{")
	s.add(Decl{Kind: "enum", Name: name.text, Line: kw.line})
	at := len(s.doc.Decls) - 1
	for !s.eof() && !s.is("}") {
		v := s.ident()
		if s.is("=") {
			s.next()
			s.next()
		}
		s.annotations()
		s.optSep()
		s.add(Decl{Kind: "enum_value", Name: v.text, Container: name.text, Line: v.line, End: v.line})
	}
	s.expect("}")
	s.annotations()
	s.doc.Decls[at].End = s.lastLine()
}

func (s *scanner) structure(kw token) {
	name := s.ident()
	if s.is("xsd_all") {
		s.next()
	}
	s.expect("{")
	s.add(Decl{Kind: kw.text, Name: name.text, Line: kw.line})
	at := len(s.doc.Decls) - 1
	for !s.eof() && !s.is("}") {
		s.field(name.text, "field")
	}
	s.expect("}")
	s.annotations()
	s.doc.Decls[at].End = s.lastLine()
}

// field reads one field (or a function argument) and adds it as kind; kind "" reads without adding.
func (s *scanner) field(container, kind string) (refs []TypeRef) {
	start := s.peek().line
	if t := s.peek(); t.kind == tNumber {
		s.next()
		s.expect(":")
	}
	if s.is("required") || s.is("optional") {
		s.next()
	}
	refs = s.fieldType()
	name := s.ident()
	if s.is("=") {
		s.next()
		s.value()
	}
	s.annotations()
	s.optSep()
	if kind != "" {
		s.add(Decl{Kind: kind, Name: name.text, Container: container, Line: start, End: s.lastLine(), Refs: refs})
	}
	return refs
}

func (s *scanner) service(kw token) {
	name := s.ident()
	s.add(Decl{Kind: "service", Name: name.text, Line: kw.line})
	at := len(s.doc.Decls) - 1
	if s.is("extends") {
		s.next()
		t := s.ident()
		s.doc.Decls[at].Refs = []TypeRef{{t.text, t.line}}
	}
	s.expect("{")
	for !s.eof() && !s.is("}") {
		s.function(name.text)
	}
	s.expect("}")
	s.annotations()
	s.doc.Decls[at].End = s.lastLine()
}

func (s *scanner) function(service string) {
	start := s.peek().line
	if s.is("oneway") {
		s.next()
	}
	refs := s.fieldType() // `void` is a builtin
	name := s.ident()
	s.expect("(")
	for !s.eof() && !s.is(")") {
		refs = append(refs, s.field("", "")...)
	}
	s.expect(")")
	if s.is("throws") {
		s.next()
		s.expect("(")
		for !s.eof() && !s.is(")") {
			refs = append(refs, s.field("", "")...)
		}
		s.expect(")")
	}
	s.annotations()
	s.optSep()
	s.add(Decl{Kind: "function", Name: name.text, Container: service, Line: start, End: s.lastLine(), Refs: refs})
}
