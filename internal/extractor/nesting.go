package extractor

// nesting.go -- the recursion guard for the parsers that recurse on nesting.
// Go reports a stack overflow as a fatal error that recover cannot catch, so
// a file nested deep enough inside the size limit ends the caller's process.
// A file nested past maxNesting is refused as incomplete before it reaches
// the parser. What is counted is what the parser recurses on, read from the
// parser's own lexer or from the parser itself, never from a second scan of
// the raw text that can lose sync with it (a quote, an escaped brace, a four-
// quote string end each did):
//
//   - hclsyntax (Terraform, Packer, Nomad, Terragrunt, docker-bake): the lexer's
//     tokens, counted by internal/hclbound, the one HCL guard (brackets, runs of
//     prefix operators, conditionals, splats and open directives as levels, and
//     the melding cost of a template); the cue parser, in package cuelang, uses
//     NestMeter, with the scanner driven as the parser drives it.
//   - nginx and Caddyfile: the parsers carry the block depth and refuse by it.
//   - proto: text/scanner in the parser's own mode.
//   - TOML (go-toml): a scan by TOML's rules that counts brackets and the dots
//     of a key, and bounds the key count (tomlRefusal).
//   - JSON, JSONC, XML and libconfig: bracket scans. encoding/json refuses past
//     its own limit and the XML and libconfig decoders survived the deepest
//     2 MiB shape when measured; the depth is a stated guarantee, so it does
//     not rest on a runtime limit that another toolchain may not have.

import (
	"bytes"
	"fmt"
	"text/scanner"

	"github.com/geoffmilleraz/fathom/internal/hclbound"
)

// MaxNesting is the deepest nesting a recursive parser is handed.
const MaxNesting = 10_000

const maxNesting = MaxNesting

// errTooDeep is the one-line reason for a file refused for its nesting.
func errTooDeep(what string) string { return TooDeepReason(what) }

// TooDeepReason is the one-line reason for a file refused for its nesting, for
// the extractors outside this package that guard a parser of their own.
func TooDeepReason(what string) string {
	return fmt.Sprintf("%s: nesting deeper than %d is not read", what, MaxNesting)
}

// NestMeter measures how deep a recursive-descent parser recurses on a token
// stream, so a guard can refuse a file before the parser overflows the stack
// (a fatal error recover cannot catch). The depth is the open brackets plus
// the operators of the expressions they sit in: an operator whose operand is
// parsed by calling the parser again (a prefix operator, a conditional, a field
// label's colon, a postfix applied to a bracketed operand) is a level exactly
// as a bracket is. A caller feeds it the tokens of the parser's own lexer, so a
// bracket in a comment or a string is never counted and the guard cannot lose
// sync with the parser the way a second reading of the raw text does.
type NestMeter struct {
	levels []nestLevel
	outer  int // operators pending in the expressions the open brackets interrupt
	run    int // operators pending in the expression being read
}

type nestLevel struct {
	run  int  // the interrupted expression's pending operators
	kind byte // the bracket that opened the level
}

func (m *NestMeter) deep() bool { return len(m.levels)+m.outer+m.run > MaxNesting }

// Open enters a bracket of the given kind and reports whether the nesting is
// now past MaxNesting.
func (m *NestMeter) Open(kind byte) bool {
	m.levels = append(m.levels, nestLevel{m.run, kind})
	m.outer += m.run
	m.run = 0
	return m.deep()
}

// Close leaves the innermost bracket and resumes the expression it interrupted,
// when that bracket is of the given kind. A closer that does not match the
// innermost bracket leaves the nesting as it was: a parser never nests less than
// the brackets nothing has matched, so a stray closer cannot hide a level.
func (m *NestMeter) Close(kind byte) {
	if n := len(m.levels); n > 0 && m.levels[n-1].kind == kind {
		m.run = m.levels[n-1].run
		m.outer -= m.run
		m.levels = m.levels[:n-1]
	}
}

// CloseThrough leaves every bracket opened inside the innermost one of the given
// kind, and that one with them, when one is open. A template's end closes the
// directives (`%{if}`) its body left open, since the parser nested them inside it.
func (m *NestMeter) CloseThrough(kind byte) {
	for _, l := range m.levels {
		if l.kind != kind {
			continue
		}
		for m.Innermost() != kind {
			m.Close(m.Innermost())
		}
		m.Close(kind)
		return
	}
}

// CloseOperand is Close for a bracket pair that is itself an operand of what
// follows it (an index, a call's result, a splat): the pair counts as one
// operator of the expression around it, since what is applied to it nests the
// tree one deeper. It reports whether the nesting is now past MaxNesting.
func (m *NestMeter) CloseOperand(kind byte) bool {
	if m.Innermost() != kind {
		return false
	}
	m.Close(kind)
	return m.Op()
}

// Unclosed is the number of brackets still open, which a file that ends has none of
// when a parser can read it to the end.
func (m *NestMeter) Unclosed() int { return len(m.levels) }

// Innermost is the kind of the bracket open now, or 0 at the top level.
func (m *NestMeter) Innermost() byte {
	if n := len(m.levels); n > 0 {
		return m.levels[n-1].kind
	}
	return 0
}

// Op counts one operator in the expression being read and reports whether the
// nesting is now past MaxNesting.
func (m *NestMeter) Op() bool {
	m.run++
	return m.deep()
}

// EndExpr ends the expression being read (a comma, or a newline that separates
// statements): its operators no longer nest the next one.
func (m *NestMeter) EndExpr() { m.run = 0 }

// hclRefusal is why src, as HCL, is not handed to the parser, and "" when it is
// read. The guard is internal/hclbound, shared with the stand-alone reads
// (internal/iacfacts): it counts what the parser recurses on (brackets, a run of
// prefix operators, conditionals, splats, open template directives) against
// MaxNesting, and prices the quadratic melding of a template's literal pieces
// and of a function name's `::` segments. It counts the lexer's tokens, which
// know a string from code, so a bracket in a comment or a string is not counted
// and one in an interpolation is. The reason names the bound that fired.
func hclRefusal(src []byte) string {
	if r := hclbound.Check(src, "nesting", hclbound.Bounds{MaxDepth: MaxNesting}); r != nil {
		return "hcl parse: " + r.Reason
	}
	return ""
}

// maxRun is the most tokens a parser is handed in a row that none of its loops
// ends. emicklei/proto builds a dotted name, an option name and a multi-line
// string by concatenation, one token at a time, so a run of n tokens costs n*n/2
// byte moves: 250,000 `+` tokens in an enum option took 12 s, and the same run of
// `.` after a message's `{` took 10 s. A real statement is a few tokens, and the
// longest real run (a long dotted name, a string in a hundred pieces) is well
// under it.
const maxRun = MaxNesting

// TooLongReason is the one-line reason for a file refused for a run of tokens
// no statement or item ends.
func TooLongReason(what string) string {
	return fmt.Sprintf("%s: more than %d tokens in a row without a statement or item boundary are not read", what, maxRun)
}

// TooManyErrorsReason is the one-line reason for a file refused for the
// lexical errors it holds, for a parser that keeps every one of them in a list
// it searches linearly for a repeat (cue: 100,000 illegal tokens took 13 s).
func TooManyErrorsReason(what string) string {
	return fmt.Sprintf("%s: more than %d lexical errors are not read", what, MaxNesting)
}

// protoRefusal is why src is not handed to the proto parser: it nests { [ ( <
// deeper than maxNesting, or holds a run of tokens that no boundary ends (maxRun).
// It is "" for a file that is read. It counts the tokens of the scanner the
// parser reads through (the same text/scanner mode, and the single-quoted string
// the parser re-reads with comments off), so a bracket in a comment or a string
// is not counted and the guard cannot lose sync with the parser.
//
// The parser also recurses where no bracket is: Literal.parse calls itself for a
// `-` and for each comment where a literal is expected (after `=`, `:`, `,`,
// `[` or another `-`), and nextInteger for a `-`. Those count as levels, as a
// bracket does.
func protoRefusal(src []byte) string {
	const base = scanner.ScanIdents | scanner.ScanFloats | scanner.ScanStrings | scanner.ScanRawStrings
	var s scanner.Scanner
	s.Init(bytes.NewReader(src))
	s.Mode = base | scanner.ScanComments
	s.Error = func(*scanner.Scanner, string) {}
	var m NestMeter
	run := 0         // tokens since a boundary
	literal := false // the last token before a comment expects a literal after it
	deep := func() string { return errTooDeep("proto parse") }
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if tok == scanner.Comment {
			if literal && m.Op() {
				return deep()
			}
			continue
		}
		if run++; run > maxRun {
			return TooLongReason("proto parse")
		}
		literal = false
		switch tok {
		case '-':
			literal = true
			if m.Op() {
				return deep()
			}
			continue
		case '{', '[', '(', '<':
			run = 0
			literal = tok == '['
			if m.Open(byte(tok)) {
				return deep()
			}
		case '}':
			run = 0
			m.Close('{')
		case ']':
			run = 0
			m.Close('[')
		case ')':
			run = 0
			m.Close('(')
		case '>':
			run = 0
			m.Close('<')
		case '\'':
			s.Mode = base
			for t := s.Scan(); t != scanner.EOF && s.TokenText() != "'"; t = s.Scan() {
			}
			s.Mode = base | scanner.ScanComments
		case ';', ',', '=', ':':
			run = 0
			literal = tok != ';'
		}
		m.EndExpr()
	}
	if m.Unclosed() > 0 {
		// emicklei/proto's service and message loops have no exit at the end of the
		// input: `service s{ rpc a(b) returns (c) {` never returns, and a real file
		// cut at a bracket reads the same. A file with a bracket open at its end is
		// no proto file the parser can finish, and it is refused here.
		return UnclosedReason("proto parse")
	}
	return ""
}

// UnclosedReason is the one-line reason for a file refused for a bracket still
// open at its end.
func UnclosedReason(what string) string {
	return fmt.Sprintf("%s: a bracket is still open at the end of the file, which the parser cannot read to its end", what)
}

// jsonTooDeep reports whether src nests [ and { deeper than maxNesting. A
// comment to the end of the line and a quoted string are skipped, so a bracket
// in either is not counted. (nginx and Caddyfile have no text guard: their
// parsers carry the block depth and refuse by it, and TOML has tomlRefusal.)
func jsonTooDeep(src []byte) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '"' || c == '\'':
			i = skipQuoted(src, i)
		case c == '[' || c == '{':
			if depth++; depth > maxNesting {
				return true
			}
		case (c == ']' || c == '}') && depth > 0:
			depth--
		}
	}
	return false
}

// maxTOMLKeys is the most keys and tables a TOML document may hold. go-toml
// keeps every key it has seen in a sorted list to find a repeat, so a document of
// n keys costs n*n/2 moves: 60,000 keys took 4 s and 180,000 took 27 s, inside a
// 2 MiB file. A real document (a lock file, a manifest, a catalogue of strings)
// holds thousands.
const maxTOMLKeys = 40_000

// maxTOMLSegments is the most segments a table header may have. The locator that
// gives each key its line hashes every dotted prefix of the key's full name, so a
// header of n segments costs each key under it n*n bytes: 9,000 dots and 8,000
// keys (81 KB) took 27 s and 2.1 GB. A real header has a handful.
const maxTOMLSegments = 64

// tomlRefusal is why src is not handed to the TOML decoder: it nests deeper than
// maxNesting in any way the decoder recurses on (an array or an inline table, and
// a dotted key, which the decoder reads as that many nested tables: `a.a.a.a = 1`,
// `[a.a.a.a]`, `[[a.a.a.a]]`, `{a.a.a = 1}`), or it holds more than maxTOMLKeys
// keys and tables. It is "" for a document that is read. Only the dots of a key
// count: a float's dot does not. Comments and strings are skipped by TOML's own
// rules (skipQuoted).
func tomlRefusal(src []byte) string {
	var open []byte // the brackets open now
	dots := 0       // dots in the key being read
	keys := 0       // keys and tables seen
	inKey := true   // a key (or a table header) is being read, not a value
	header := false // the key being read is a table header
	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '#':
			for i+1 < len(src) && src[i+1] != '\n' {
				i++
			}
		case '"', '\'':
			i = skipQuoted(src, i)
		case '\n':
			dots, inKey, header = 0, len(open) == 0 || open[len(open)-1] == '{', false
		case '=':
			if inKey {
				keys++
			}
			dots, inKey = 0, false
		case ',':
			dots, inKey = 0, len(open) > 0 && open[len(open)-1] == '{'
		case '.':
			if inKey {
				dots++
			}
		case '{':
			open = append(open, c)
			dots, inKey = 0, true
		case '[':
			if len(open) == 0 && inKey {
				keys++ // a table header
				header = true
			}
			open = append(open, c)
		case ']', '}':
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		}
		if len(open)+dots > maxNesting {
			return errTooDeep("toml decode")
		}
		if keys > maxTOMLKeys {
			return fmt.Sprintf("toml decode: more than %d keys is not read", maxTOMLKeys)
		}
		if header && dots >= maxTOMLSegments {
			return fmt.Sprintf("toml decode: a table header of more than %d segments is not read", maxTOMLSegments)
		}
	}
	return ""
}

// tooDeepErr is the error a decoder returns for a file refused for its nesting.
func tooDeepErr() error {
	return fmt.Errorf("nesting deeper than %d is not read", maxNesting)
}

// xmlTooDeep reports whether src nests XML elements deeper than maxNesting.
// Comments, processing instructions, CDATA, a doctype and the quoted values in
// a tag are skipped, and a self-closing tag is not a level.
func xmlTooDeep(src []byte) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		if src[i] != '<' {
			continue
		}
		rest := src[i+1:]
		switch {
		case bytes.HasPrefix(rest, []byte("!--")):
			i = skipPast(src, i, "-->")
		case bytes.HasPrefix(rest, []byte("![CDATA[")):
			i = skipPast(src, i, "]]>")
		case bytes.HasPrefix(rest, []byte("?")):
			i = skipPast(src, i, "?>")
		case bytes.HasPrefix(rest, []byte("!")):
			i = skipPast(src, i, ">")
		case bytes.HasPrefix(rest, []byte("/")):
			i = skipPast(src, i, ">")
			if depth > 0 {
				depth--
			}
		default:
			j, selfClose := skipTag(src, i)
			i = j
			if !selfClose {
				if depth++; depth > maxNesting {
					return true
				}
			}
		}
	}
	return false
}

// skipPast returns the index of the last byte of the first end after src[i],
// or the end of src.
func skipPast(src []byte, i int, end string) int {
	if k := bytes.Index(src[i:], []byte(end)); k >= 0 {
		return i + k + len(end) - 1
	}
	return len(src)
}

// skipTag returns the index of the > that ends the tag opened at src[i] (quoted
// attribute values skipped) and whether the tag is self-closing.
func skipTag(src []byte, i int) (int, bool) {
	var q byte
	for j := i + 1; j < len(src); j++ {
		switch c := src[j]; {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
		case c == '>':
			return j, src[j-1] == '/'
		}
	}
	return len(src), false
}

// libconfigTooDeep reports whether src nests ( [ { deeper than maxNesting.
// Comments (#, //, /* */) and strings are skipped.
func libconfigTooDeep(src []byte) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '#' || (c == '/' && i+1 < len(src) && src[i+1] == '/'):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i = skipPast(src, i, "*/")
		case c == '"':
			i = skipQuoted(src, i)
		case c == '(' || c == '[' || c == '{':
			if depth++; depth > maxNesting {
				return true
			}
		case (c == ')' || c == ']' || c == '}') && depth > 0:
			depth--
		}
	}
	return false
}

// skipQuoted returns the index of the last byte of the string that starts at
// src[i], by TOML's rules (which JSON and libconfig strings are a subset of): a
// triple quote opens a multi-line string that a run of three to five quotes
// ends (the last three close it, the quotes before them are its text), a
// single-quoted string takes no escapes, and no other string crosses a line.
func skipQuoted(src []byte, i int) int {
	q := src[i]
	if i+2 < len(src) && src[i+1] == q && src[i+2] == q {
		for j := i + 3; j+2 < len(src); j++ {
			if q == '"' && src[j] == '\\' {
				j++
				continue
			}
			if src[j] == q && src[j+1] == q && src[j+2] == q {
				run := 3
				for run < 5 && j+run < len(src) && src[j+run] == q {
					run++
				}
				return j + run - 1
			}
		}
		return len(src)
	}
	for j := i + 1; j < len(src); j++ {
		switch {
		case src[j] == '\\' && q == '"':
			j++
		case src[j] == '\n':
			return j - 1
		case src[j] == q:
			return j
		}
	}
	return len(src)
}
