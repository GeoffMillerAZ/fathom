package cuelang

// cuenesting.go -- the recursion guard for the CUE parser. cue/parser is a
// recursive-descent parser and Go reports a stack overflow as a fatal error that
// recover cannot catch, so a .cue file nested deep enough inside the size limit
// ends the caller's process: brackets, but also a run of prefix operators or of
// field labels (`a: a: a: ...`, one level each), and a chain of selectors, calls,
// indexes or binary operators, whose left-nested tree the extractors then walk
// by recursion. A file past extractor.MaxNesting is refused as incomplete before
// it reaches the parser.

import (
	"bytes"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/scanner"
	"cuelang.org/go/cue/token"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// parseCUE parses src as parser.ParseFile does, and refuses a file nested past
// extractor.MaxNesting, or one that holds more lexical errors than the parser's
// error list takes in linear time: it returns a nil file and the reason.
func parseCUE(relPath string, src []byte) (f *ast.File, refused string, err error) {
	if why := cueRefusal(src); why != "" {
		return nil, why, nil
	}
	f, err = parser.ParseFile(relPath, src)
	return f, "", err
}

// cueRefusal is why src is not handed to the parser, or "" when it is. It refuses
// a file that nests deeper than extractor.MaxNesting, or that holds more than
// extractor.MaxNesting lexical errors (the parser keeps every scanner error in a
// list it searches for a repeat, so n of them cost n*n/2 comparisons: 100,000
// `^` took 13 s).
//
// The guard scans a copy of src with every `@` turned into `&`, because the
// scanner reads an attribute by recursion (`@a(` calls itself once per bracket
// inside the attribute, and once per `@` that follows another), which no guard
// that drives that scanner can survive: 2 MiB of `@a(` ends the process in the
// guard itself. As an `&` the same bytes are an operator and brackets the meter
// counts without recursing, and every other token is the one the parser reads,
// so the depth it measures is at least the recursion's.
//
// It counts the tokens of the scanner the parser reads through, driven as the parser
// drives it (a string interpolation is resumed at the parenthesis that closes it),
// so a bracket in a comment or a string is never counted. A run of operators is
// the expression it sits in: it ends at a comma (the scanner inserts one at a
// newline after a value), and a bracket pair, once closed, is one more level of
// the expression around it, because a selector, call or index applied to it nests
// the tree one deeper. A closer that does not match the bracket open now leaves
// the nesting as it was, and one that cannot be matched inside a string
// interpolation refuses the file, since the parser's recovery there is not one
// the guard can follow.
func cueRefusal(src []byte) (why string) {
	deep := extractor.TooDeepReason("cue parse")
	defer func() {
		if recover() != nil {
			why = deep // a guard that cannot follow the scanner does not let the file through
		}
	}()
	scan := src
	if bytes.IndexByte(src, '@') >= 0 {
		scan = bytes.ReplaceAll(src, []byte("@"), []byte("&"))
	}
	var s scanner.Scanner
	errs := 0
	file := token.NewFile("nesting", -1, len(scan))
	s.Init(file, scan, func(token.Pos, string, []interface{}) { errs++ }, 0)
	head := 0 // 1 right after an attribute's @, 2 after its name: what the scanner needs next is a (
	var m extractor.NestMeter
	interps := 0                     // string interpolations open now
	pending := false                 // an interpolation segment ended in "(": the next ( opens it
	closer := func(kind byte) bool { // reports a closer that cannot be followed
		if m.Innermost() == kind {
			return m.CloseOperand(kind)
		}
		return interps > 0
	}
	for {
		pos, tok, lit := s.Scan()
		switch {
		case head == 1 && tok == token.IDENT:
			head = 2
		case head > 0 && tok != token.LPAREN:
			errs++ // `@a` with no ( is an error the scanner reports once per attribute: a list of them is as slow as one of illegal tokens
			head = 0
		default:
			head = 0
		}
		if tok == token.AND && src[file.Offset(pos)] == '@' {
			head = 1
		}
		if errs > extractor.MaxNesting {
			return extractor.TooManyErrorsReason("cue parse")
		}
		switch tok {
		case token.EOF:
			return ""
		case token.INTERPOLATION:
			pending = strings.HasSuffix(lit, "(")
		case token.LPAREN:
			kind := byte('(')
			if pending {
				kind, pending = 'I', false
				interps++
			}
			if m.Open(kind) {
				return deep
			}
		case token.LBRACK:
			if m.Open('[') {
				return deep
			}
		case token.LBRACE:
			if m.Open('{') {
				return deep
			}
		case token.RPAREN:
			if m.Innermost() == 'I' {
				m.Close('I')
				interps--
				if strings.HasSuffix(s.ResumeInterpolation(), "(") {
					pending = true
				}
				if m.Op() {
					return deep
				}
			} else if closer('(') {
				return deep
			}
		case token.RBRACK:
			if closer('[') {
				return deep
			}
		case token.RBRACE:
			if closer('{') {
				return deep
			}
		case token.COMMA, token.SEMICOLON:
			m.EndExpr()
		case token.IF, token.ELSE, token.FOR, token.IN, token.LET, token.TRY,
			token.FALLBACK, token.OTHERWISE, token.FUNC:
			if m.Op() {
				return deep
			}
		default:
			if tok.IsOperator() && m.Op() {
				return deep
			}
		}
	}
}
