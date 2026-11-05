package extractor

// nesting.go -- the recursion guard for the parsers that recurse on nesting.
// Go reports a stack overflow as a fatal error that recover cannot catch, so
// a file nested deep enough inside the size limit ends the caller's process:
// hclsyntax (a Terraform, Packer, Nomad or Terragrunt file), go-toml (Cargo.toml),
// the nginx walk and the Caddyfile parser died between 150 KB and 1.8 MB of
// nesting. A file nested past maxNesting is refused as incomplete before it
// reaches the parser. The guard covers JSON, JSONC, XML and libconfig too,
// though the pinned toolchain's encoding/json refuses past its own limit and
// the XML and libconfig decoders survived the deepest 2 MiB shape when
// measured: the depth is a stated guarantee, so it does not rest on a runtime
// limit that another toolchain may not have.

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// maxNesting is the deepest bracket nesting a recursive parser is handed.
const maxNesting = 10_000

// errTooDeep is the one-line reason for a file refused for its nesting.
func errTooDeep(what string) string {
	return fmt.Sprintf("%s: nesting deeper than %d is not read", what, maxNesting)
}

// hclTooDeep reports whether src, as HCL, nests brackets, quotes, heredocs or
// template sequences deeper than maxNesting. It counts the lexer's tokens, which
// are iterative and know a string from code, so a bracket in a comment or a
// string is not counted and one in an interpolation is.
func hclTooDeep(src []byte) bool {
	toks, _ := hclsyntax.LexConfig(src, "nesting", hcl.Pos{Line: 1, Column: 1})
	depth := 0
	for _, t := range toks {
		switch t.Type {
		case hclsyntax.TokenOBrace, hclsyntax.TokenOBrack, hclsyntax.TokenOParen,
			hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl,
			hclsyntax.TokenOQuote, hclsyntax.TokenOHeredoc:
			if depth++; depth > maxNesting {
				return true
			}
		case hclsyntax.TokenCBrace, hclsyntax.TokenCBrack, hclsyntax.TokenCParen,
			hclsyntax.TokenTemplateSeqEnd, hclsyntax.TokenCQuote, hclsyntax.TokenCHeredoc:
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}

// textTooDeep reports whether src nests the bracket pairs in opens/closes
// (parallel strings) deeper than maxNesting. A comment to the end of the line
// and a quoted string are skipped, so a bracket in either is not counted. The
// two syntaxes it serves differ: TOML (nginx false) has `#` comments anywhere,
// single-line strings and triple-quoted multi-line ones; nginx has them only at
// the start of a word, its strings run across lines, and a quote or `#` inside
// a word is part of the word.
func textTooDeep(src []byte, opens, closes string, nginx bool) bool {
	depth := 0
	for i := 0; i < len(src); i++ {
		c := src[i]
		wordStart := !nginx || i == 0 || indexByte(" \t\r\n;{}", src[i-1]) >= 0
		switch {
		case c == '#' && wordStart:
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case (c == '"' || c == '\'') && wordStart:
			i = skipQuoted(src, i, !nginx)
		default:
			if indexByte(opens, c) >= 0 {
				if depth++; depth > maxNesting {
					return true
				}
			} else if indexByte(closes, c) >= 0 && depth > 0 {
				depth--
			}
		}
	}
	return false
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
			i = skipQuoted(src, i, true)
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

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// skipQuoted returns the index of the last byte of the string that starts at
// src[i]. toml selects TOML's rules (a triple quote opens a multi-line string, a
// single-quoted string takes no escapes, no other string crosses a line);
// otherwise a string runs to its closing quote across lines, backslash escapes
// honoured, as nginx reads one.
func skipQuoted(src []byte, i int, toml bool) int {
	q := src[i]
	if toml && i+2 < len(src) && src[i+1] == q && src[i+2] == q {
		for j := i + 3; j+2 < len(src); j++ {
			if q == '"' && src[j] == '\\' {
				j++
				continue
			}
			if src[j] == q && src[j+1] == q && src[j+2] == q {
				return j + 2
			}
		}
		return len(src)
	}
	for j := i + 1; j < len(src); j++ {
		switch {
		case src[j] == '\\' && (q == '"' || !toml):
			j++
		case src[j] == '\n' && toml:
			return j - 1
		case src[j] == q:
			return j
		}
	}
	return len(src)
}
