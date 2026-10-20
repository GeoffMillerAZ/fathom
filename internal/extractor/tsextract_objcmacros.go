package extractor

import "strings"

// tsextract_objcmacros.go — the Objective-C dialect's pre-parse step (the
// objective-c dialect.json's preprocess key "objc-macros"). Apple's headers
// and every project built on them spell enumerations, availability and
// export decorations as object-like or function-like macros the compiler
// expands and tree-sitter-objc cannot: `typedef NS_ENUM(NSInteger, Mode) {
// ModeA };` parses as an error that swallows the rest of the file, as does a
// bare `NS_ASSUME_NONNULL_BEGIN` line or a `FOUNDATION_EXPORT NSString *f(void);`.
//
// The step finds those macros by where they are written, never by a project's
// list of names (an all-capitals word with an underscore: NS_, UI_, API_, SD_),
// and rewrites them at equal byte length, so every line, column and byte
// offset of the file is unchanged:
//   - `typedef NS_ENUM(T, Name)` (also NS_OPTIONS, NS_CLOSED_ENUM,
//     NS_CLOSED_OPTIONS, NS_ERROR_ENUM(domain, Name), CF_ENUM, CF_OPTIONS,
//     CF_CLOSED_ENUM) reads as `enum Name`;
//   - a macro standing where a declaration starts (the file's start, after
//     `;`, `}` or `@end`, outside any braces), with its argument list when it
//     takes one, followed by a word, `@`, `-` or `+` or standing alone on its
//     line: `NS_ASSUME_NONNULL_BEGIN`, `FOUNDATION_EXPORT NSString *f(void);`,
//     `NS_SWIFT_UI_ACTOR @interface`;
//   - a macro that ends a declaration, with its argument list when it takes
//     one, right before the `;` that closes it: `typedef NSString *Key
//     NS_STRING_ENUM;`, `- (void)f API_DEPRECATED("x", ios(8.0, 9.0));`;
//   - the nullability and ownership keywords the grammar does not know
//     (`__nullable`, `_Nonnull`, `__kindof`, ...).
//
// A second candidate also blanks the conditional directive lines, every branch
// kept, as the C++ step does (cppBlankConditionals): `#if A` / `#else` around
// two heads of one `if` chain parse as the branches in a row. The first text
// that parses with no error is read (tsPreprocessed), so a file that parses as
// written is never rewritten.

// objcEnumMacros are the enumeration macros: the underlying type (or error
// domain), then the enumeration's name.
var objcEnumMacros = map[string]bool{
	"NS_ENUM": true, "NS_OPTIONS": true, "NS_CLOSED_ENUM": true, "NS_CLOSED_OPTIONS": true,
	"NS_ERROR_ENUM": true, "CF_ENUM": true, "CF_OPTIONS": true, "CF_CLOSED_ENUM": true,
}

// objcQualifierWords are keywords the grammar does not read, blanked wherever
// they are written in code.
var objcQualifierWords = map[string]bool{
	"__nullable": true, "__nonnull": true, "_Nullable": true, "_Nonnull": true,
	"_Null_unspecified": true, "__null_unspecified": true, "__kindof": true,
	"__unsafe_unretained": true,
}

// objcNotMacroPrev are words after which a trailing all-capitals word is an
// operand, never a decoration (`return FOO_BAR;`).
var objcNotMacroPrev = map[string]bool{
	"return": true, "case": true, "goto": true, "else": true, "do": true, "sizeof": true,
	"break": true, "continue": true, "typeof": true,
}

// objcMacroTexts are the texts an Objective-C file that does not parse as
// written is parsed as instead (tsPreprocessors["objc-macros"]), in order:
// its macros rewritten, then the same with its conditional directive lines
// blanked; nil when neither changes anything.
func objcMacroTexts(src []byte) [][]byte {
	toks, dirs := cppLex(src)
	text, changed := objcBlankMacros(src, toks)
	if !changed {
		text = src
	}
	var out [][]byte
	if changed {
		out = append(out, text)
	}
	if cond, ok := cppBlankConditionals(text, dirs); ok {
		out = append(out, cond)
	}
	return out
}

// objcBlankMacros rewrites src's macros as the step's doc says; changed is
// false when none was found (the returned slice is then src).
func objcBlankMacros(src []byte, toks []cppTok) ([]byte, bool) {
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
	// close returns the index of the ')' matching the '(' at open, or -1.
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
	blank := func(from, to int) {
		blankKeepNewlines(out, from, to)
		changed = true
	}
	braces := 0
	// atBoundary: toks[i] starts a declaration (outside braces).
	prevBoundary := true // the file's start
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		w := word(i)
		atStart := prevBoundary && braces == 0
		prevBoundary = false
		switch {
		case punct(i, '{'):
			braces++
		case punct(i, '}'):
			if braces > 0 {
				braces--
			}
			prevBoundary = braces == 0
		case punct(i, ';'):
			prevBoundary = braces == 0
		case w == "end" && punct(i-1, '@'):
			prevBoundary = braces == 0
		case w == "":
		case objcQualifierWords[w]:
			blank(t.start, t.end)
		case objcEnumMacros[w] && punct(i+1, '('):
			cp := closeParen(i + 1)
			if cp < 0 {
				continue
			}
			// the name is the last word before the ')' (NS_ERROR_ENUM puts the
			// domain first, the others the type).
			name := ""
			for j := cp - 1; j > i+1; j-- {
				if toks[j].kind == 'w' && punct(j-1, ',') {
					name = word(j)
					break
				}
			}
			if name == "" {
				continue
			}
			from := t.start
			if i > 0 && word(i-1) == "typedef" {
				from = toks[i-1].start
			}
			end := toks[cp].end
			blank(from, end)
			head := end - (len("enum ") + len(name))
			if head < from {
				// the macro text is always longer than "enum Name"
				head = from
			}
			copy(out[head:], "enum ")
			copy(out[head+len("enum "):], name)
			i = cp
		case cppAllCaps([]byte(w)) && strings.Contains(w, "_"):
			end := i
			if punct(i+1, '(') {
				cp := closeParen(i + 1)
				if cp < 0 {
					continue
				}
				end = cp
			}
			next := end + 1
			// a decoration standing where a declaration starts
			if atStart {
				if next >= len(toks) {
					blank(t.start, toks[end].end)
					i = end
					continue
				}
				aloneOnLine := strings.Contains(string(src[toks[end].end:toks[next].start]), "\n")
				if word(next) != "" || punct(next, '@') || punct(next, '-') || punct(next, '+') || aloneOnLine {
					blank(t.start, toks[end].end)
					i = end
					prevBoundary = true
					continue
				}
			}
			// a decoration after an enumerator (`Name API_DEPRECATED(..) = 1 << 4,`)
			if braces > 0 && (punct(next, '=') || punct(next, ',') || punct(next, '}')) && i > 0 && word(i-1) != "" && !objcNotMacroPrev[word(i-1)] && end > i {
				blank(t.start, toks[end].end)
				i = end
				continue
			}
			// a decoration before a method body (`- (void)f:(int)a API_AVAILABLE(ios(10)) {`)
			if braces == 0 && punct(next, '{') && punct(i-1, ')') && end > i {
				blank(t.start, toks[end].end)
				i = end
				continue
			}
			// a decoration ending a declaration
			if punct(next, ';') && i > 0 {
				pw := word(i - 1)
				if (pw != "" && !objcNotMacroPrev[pw]) || punct(i-1, ')') || punct(i-1, ']') || punct(i-1, '*') {
					blank(t.start, toks[end].end)
					i = end
				}
			}
		}
	}
	if !changed {
		return src, false
	}
	return out, true
}

// blankKeepNewlines turns b[from:to] into spaces, keeping every line break.
func blankKeepNewlines(b []byte, from, to int) {
	for i := from; i < to; i++ {
		if b[i] != '\n' && b[i] != '\r' {
			b[i] = ' '
		}
	}
}
