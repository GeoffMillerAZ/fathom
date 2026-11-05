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
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/scanner"
	"cuelang.org/go/cue/token"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// cueTooDeepReason is the one-line reason for a file refused for its nesting.
func cueTooDeepReason() string { return extractor.TooDeepReason("cue parse") }

// parseCUE parses src as parser.ParseFile does, and refuses a file nested past
// extractor.MaxNesting: it returns a nil file and the reason.
func parseCUE(relPath string, src []byte) (f *ast.File, tooDeep string, err error) {
	if cueTooDeep(src) {
		return nil, cueTooDeepReason(), nil
	}
	f, err = parser.ParseFile(relPath, src)
	return f, "", err
}

// cueTooDeep reports whether src nests deeper than extractor.MaxNesting. It
// counts the tokens of the scanner the parser reads through, driven as the parser
// drives it (a string interpolation is resumed at the parenthesis that closes it),
// so a bracket in a comment or a string is never counted. A run of operators is
// the expression it sits in: it ends at a comma (the scanner inserts one at a
// newline after a value), and a bracket pair, once closed, is one more level of
// the expression around it, because a selector, call or index applied to it nests
// the tree one deeper. A closer that does not match the bracket open now leaves
// the nesting as it was, and one that cannot be matched inside a string
// interpolation refuses the file, since the parser's recovery there is not one
// the guard can follow.
func cueTooDeep(src []byte) (deep bool) {
	defer func() {
		if recover() != nil {
			deep = true // a guard that cannot follow the scanner does not let the file through
		}
	}()
	var s scanner.Scanner
	s.Init(token.NewFile("nesting", -1, len(src)), src, func(token.Pos, string, []interface{}) {}, 0)
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
		_, tok, lit := s.Scan()
		switch tok {
		case token.EOF:
			return false
		case token.INTERPOLATION:
			pending = strings.HasSuffix(lit, "(")
		case token.LPAREN:
			kind := byte('(')
			if pending {
				kind, pending = 'I', false
				interps++
			}
			if m.Open(kind) {
				return true
			}
		case token.LBRACK:
			if m.Open('[') {
				return true
			}
		case token.LBRACE:
			if m.Open('{') {
				return true
			}
		case token.RPAREN:
			if m.Innermost() == 'I' {
				m.Close('I')
				interps--
				if strings.HasSuffix(s.ResumeInterpolation(), "(") {
					pending = true
				}
				if m.Op() {
					return true
				}
			} else if closer('(') {
				return true
			}
		case token.RBRACK:
			if closer('[') {
				return true
			}
		case token.RBRACE:
			if closer('{') {
				return true
			}
		case token.COMMA, token.SEMICOLON:
			m.EndExpr()
		case token.IF, token.ELSE, token.FOR, token.IN, token.LET, token.TRY,
			token.FALLBACK, token.OTHERWISE, token.FUNC:
			if m.Op() {
				return true
			}
		default:
			if tok.IsOperator() && m.Op() {
				return true
			}
		}
	}
}
