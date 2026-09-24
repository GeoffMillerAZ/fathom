package integrity

// normalize.go — CV-8 (ADR-044 §1.3): NormalizedBodyHash, a rename-
// insensitive hash of a Go function's body.
//
// Design: lex the body's exact source range with go/scanner (the real Go
// tokenizer — the SAME classification the compiler uses for IDENT vs.
// literal vs. keyword vs. operator, so this never has to reinvent "what
// counts as an identifier" with a hand-rolled regexp). Every token.IDENT
// is replaced by a placeholder assigned in order of FIRST APPEARANCE
// within the body ("ID0", "ID1", …) — the SAME original name always maps
// to the SAME placeholder wherever it recurs, so a rename that
// substitutes one name for another EVERYWHERE it appears (the definition
// of a pure rename, including the identifier `t.Error` becomes `t.Fatal`
// is NOT such a substitution — Error and Fatal are different names, so
// they get different placeholders, and an inverted assertion changes an
// operator token, which is never touched) leaves every placeholder
// assignment — and therefore the hash — unchanged. Every literal
// (INT/FLOAT/IMAG/CHAR/STRING) is kept verbatim, exactly as design §1.3
// specifies ("literals kept"). Keywords, operators and punctuation pass
// through as their own fixed token strings.
//
// This is lexical, not a full go/ast walk: go/scanner already resolves the
// IDENT/literal/keyword/operator classification correctly for every Go
// construct a test body can contain (composite literals, closures, type
// switches, generics) without this package maintaining its own ast.Node
// type-switch across every syntax form — a smaller, more robust surface
// than re-deriving the same classification via go/ast, at the cost of not
// being sensitive to which grammatical ROLE an identifier plays (a rename
// of a type all the way to becoming a different, coincidentally
// same-named-after-substitution identifier is exactly as detectable either
// way, since both approaches key on first-appearance order of the exact
// token text).

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/scanner"
	"go/token"
	"strings"
)

// NormalizedBodyHash parses src as one Go source file, locates the
// top-level (non-method) function literally named fn, and returns the hex
// SHA-256 digest of its body under the normalization this file's doc
// comment describes. ok is false when fn is not found as a top-level func
// with a body, or src fails to parse — never a fabricated hash for
// something that was not actually there.
func NormalizedBodyHash(src []byte, fn string) (hash string, ok bool) {
	tests, _, parsed := scanGoFile("<src>", src)
	if !parsed {
		return "", false
	}
	for _, t := range tests {
		if t.Name == fn {
			return normalizedBodyHashOfDecl(src, t.Decl), true
		}
	}
	// fn need not start with "Test" to be hashable in principle, but
	// scanGoFile only collects Test* funcs (every caller of
	// NormalizedBodyHash in this package hashes a guarded test, always
	// Test*-prefixed) — a non-Test* fn is reported not-found rather than
	// silently re-parsing with a laxer filter, keeping one scan path for
	// the whole package.
	return "", false
}

// normalizedBodyHashOfDecl hashes fn.Body's exact source range within src,
// reusing an *ast.FuncDecl the caller (scanGoFile via NormalizedBodyHash,
// or Check directly) already parsed — so a file already scanned once for
// its Test* functions is never re-parsed just to hash one of them.
func normalizedBodyHashOfDecl(src []byte, fn *ast.FuncDecl) string {
	body := fn.Body
	start, end := int(body.Pos())-1, int(body.End())-1 // ast positions are 1-based relative to a token.FileSet base of 1
	if start < 0 || end > len(src) || start > end {
		// Defensive only: a *ast.FuncDecl handed back by scanGoFile's own
		// parse of src always has valid, in-range positions. Never reached
		// in practice; hashing an empty slice rather than panicking keeps
		// this function total.
		start, end = 0, 0
	}
	return hex.EncodeToString(shaSum(normalizeTokens(src[start:end])))
}

func shaSum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// normalizeTokens lexes body (a standalone byte range, not a full file —
// go/scanner tokenizes any byte slice independent of surrounding context)
// and returns the normalized token stream this file's doc comment
// describes, ready to hash.
func normalizeTokens(body []byte) []byte {
	fset := token.NewFileSet()
	file := fset.AddFile("<body>", fset.Base(), len(body))

	var s scanner.Scanner
	// scanner.Scanner never invokes this on well-formed Go source sliced
	// at statement/brace boundaries the way an *ast.BlockStmt's Pos()/
	// End() always are; kept only so a malformed slice degrades to a
	// silently-ignored token rather than a panic (ErrorCount is not
	// consulted — ADR-044 §8 refuses a transformation engine's level of
	// input-repair machinery, and a body that fails to lex at all simply
	// normalizes to whatever tokens the scanner DID manage to emit before
	// giving up, which is still deterministic given the same bad input).
	s.Init(file, body, func(pos token.Position, msg string) {}, 0)

	placeholders := map[string]string{}
	var out strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if out.Len() > 0 {
			out.WriteByte('\x1f') // unit separator: guarantees no cross-token ambiguity regardless of token content
		}
		switch tok {
		case token.IDENT:
			ph, seen := placeholders[lit]
			if !seen {
				ph = "ID" + itoaSmall(len(placeholders))
				placeholders[lit] = ph
			}
			out.WriteString(ph)
		case token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING:
			// Literals kept verbatim (design §1.3).
			out.WriteString(lit)
		default:
			out.WriteString(tok.String())
		}
	}
	return []byte(out.String())
}

// itoaSmall avoids importing strconv solely for base-10 formatting of a
// small non-negative counter.
func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
