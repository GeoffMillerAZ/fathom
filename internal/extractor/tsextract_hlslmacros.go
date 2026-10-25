package extractor

import "strings"

// tsextract_hlslmacros.go — the HLSL dialect's pre-parse step (preprocess key
// "hlsl-macros", LB-Q-hlsl). No dialect names it yet: HLSL stays refused at
// the parse-error gate with it (LB-Q-hlsl-rest), and the hlsl dialect.json
// declares it when that row's trigger restores the dialect. tree-sitter-hlsl's
// cbuffer_specifier takes a name, an optional base class clause and the body,
// so the register clause ordinary HLSL writes in a constant buffer's header
// (`cbuffer Frame : register(b0) {`) does not fit: the parser inserts a
// MISSING `;` after the header and the file holds an error.
//
// The step finds what it rewrites by where it is written, never by a
// project's list of names, and blanks it to spaces at equal byte length, so
// every line, column and byte offset of the file is unchanged:
//   - the register clause of a `cbuffer` or `tbuffer` header, `: register(...)`
//     with any space-qualified form (`register(b0, space1)`), between the name
//     and the `{`; a register on a resource declaration is the grammar's own
//     and stays;
//   - a function-like macro invocation standing where a declaration starts
//     (the file's start, after `;` or `}`, outside any braces) and alone on
//     its line with no `;`: an all-capitals word with an underscore, then its
//     argument list (`DEFINE_LIGHT_BINDINGS(4, 5)`, a macro that expands to
//     resource declarations the grammar cannot see through).
//
// A second candidate also blanks the conditional directive lines, every branch
// kept, as the C++ and Objective-C steps do (cppBlankConditionals): `#if A` /
// `#else` around two heads of one statement or expression parse as the
// branches in a row. It is tried only after the rewritten text still fails to
// parse. The rewritten texts are tried only when the file does not parse as
// written, and read only when one parses clean (tsPreprocessed). The trigger to retire
// the register rewrite is a tree-sitter-hlsl release whose cbuffer and
// tbuffer headers take a register clause (LB-Q-hlsl-rest; the dialect's
// PROVENANCE.md carries it once the dialect is restored).

// hlslMacroTexts are the texts an HLSL file that does not parse as written is
// parsed as instead (tsPreprocessors["hlsl-macros"]), in order: every rewrite
// applied, then the same with its conditional directive lines blanked; nil
// when neither changes anything.
func hlslMacroTexts(src []byte) [][]byte {
	toks, dirs := cppLex(src)
	text, changed := hlslBlankMacros(src, toks)
	var out [][]byte
	if changed {
		out = append(out, text)
	}
	if cond, ok := cppBlankConditionals(text, dirs); ok {
		out = append(out, cond)
	}
	return out
}

// hlslBlankMacros applies rewrites (a) and (b) as the step's doc says;
// changed is false when none was found (the returned slice is then src).
func hlslBlankMacros(src []byte, toks []cppTok) ([]byte, bool) {
	out := append([]byte(nil), src...)
	changed := false
	word := func(i int) string {
		if i < 0 || i >= len(toks) || toks[i].kind != 'w' {
			return ""
		}
		return string(src[toks[i].start:toks[i].end])
	}
	punct := func(i int, c byte) bool {
		return i >= 0 && i < len(toks) && toks[i].kind == 'p' && src[toks[i].start] == c
	}
	closeParen := func(open int) int {
		depth := 0
		for j := open; j < len(toks); j++ {
			switch {
			case punct(j, '('):
				depth++
			case punct(j, ')'):
				depth--
				if depth == 0 {
					return j
				}
			case punct(j, ';') || punct(j, '{') || punct(j, '}'):
				return -1
			}
		}
		return -1
	}
	braces := 0
	atStart := true // the file's start
	for i := 0; i < len(toks); i++ {
		start := atStart && braces == 0
		atStart = false
		switch {
		case punct(i, '{'):
			braces++
			continue
		case punct(i, '}'):
			if braces > 0 {
				braces--
			}
			atStart = braces == 0
			continue
		case punct(i, ';'):
			atStart = braces == 0
			continue
		}
		// a declaration-position macro alone on its line
		if w := word(i); start && cppAllCaps([]byte(w)) && strings.Contains(w, "_") && punct(i+1, '(') {
			cp := closeParen(i + 1)
			if cp > 0 && (cp+1 == len(toks) || (!punct(cp+1, ';') &&
				strings.Contains(string(src[toks[cp].end:toks[cp+1].start]), "\n"))) {
				blankKeepNewlines(out, toks[i].start, toks[cp].end)
				changed = true
				i = cp
				atStart = true
				continue
			}
		}
		// cbuffer Name : register ( ... ) {
		if w := word(i); (w == "cbuffer" || w == "tbuffer") && word(i+1) != "" && punct(i+2, ':') &&
			word(i+3) == "register" && punct(i+4, '(') {
			cp := -1
			for j := i + 5; j < len(toks); j++ {
				if punct(j, ')') {
					cp = j
					break
				}
				if punct(j, '(') || punct(j, ';') || punct(j, '{') || punct(j, '}') {
					break
				}
			}
			if cp < 0 || !punct(cp+1, '{') {
				continue
			}
			blankKeepNewlines(out, toks[i+2].start, toks[cp].end)
			changed = true
			i = cp
		}
	}
	if !changed {
		return src, false
	}
	return out, true
}
