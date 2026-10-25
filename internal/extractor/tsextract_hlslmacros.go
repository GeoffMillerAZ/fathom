package extractor

// tsextract_hlslmacros.go — the HLSL dialect's pre-parse step (the hlsl
// dialect.json's preprocess key "hlsl-macros", LB-Q-hlsl). tree-sitter-hlsl's
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
//     and stays.
//
// The rewritten text is tried only when the file does not parse as written,
// and read only when it parses clean (tsPreprocessed). The trigger to retire
// the register rewrite is a tree-sitter-hlsl release whose cbuffer and
// tbuffer headers take a register clause (dialects/hlsl/PROVENANCE.md).

// hlslMacroTexts are the texts an HLSL file that does not parse as written is
// parsed as instead (tsPreprocessors["hlsl-macros"]): one, every rewrite
// applied; nil when no rewrite changes anything.
func hlslMacroTexts(src []byte) [][]byte {
	toks, _ := cppLex(src)
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
	for i := 0; i < len(toks); i++ {
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
		return nil
	}
	return [][]byte{out}
}
