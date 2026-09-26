// Package linetext prepares untrusted source text for the kit's line-oriented
// renderers (ui/code excerpts and ui/hitl unified diffs). It splits text into
// lines and marks characters that would otherwise be invisible, would change
// how the surrounding text is parsed, or would silently reorder what a
// reviewer sees. The same reveal applies to the names a renderer shows beside
// the text (paths, labels, side names), not only to the lines.
//
// It emits no markup and no class names: each renderer wraps the segments in
// its own scanned utilities, so a consumer's Tailwind @source of the rendering
// package stays sufficient.
package linetext

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Split breaks text on "\n". A final newline terminates the last line rather
// than starting an empty one, and is reported separately so a diff can tell
// "a\n" from "a". "\r" is content, never a terminator: a CRLF/LF change stays a
// visible difference instead of silently comparing equal. Empty text has no
// lines.
func Split(text string) (lines []string, finalNewline bool) {
	if text == "" {
		return nil, false
	}
	finalNewline = strings.HasSuffix(text, "\n")
	if finalNewline {
		text = text[:len(text)-1]
	}
	return strings.Split(text, "\n"), finalNewline
}

// Segment is a run of line text. Revealed segments replace one hidden
// character (or one invalid byte) with a visible notation such as "U+202E" or
// "\xff"; ordinary segments carry the original text unchanged.
type Segment struct {
	Text     string
	Revealed bool
}

// Segments splits one line into ordinary text and revealed notations. Tabs
// are ordinary: renderers keep them as whitespace. Every character Hidden
// reports is revealed as its code point ("U+2028", "U+E0061"). Invalid UTF-8
// bytes are revealed as \xNN rather than replaced, so two lines that differ
// only in a bad byte never look identical.
func Segments(line string) []Segment {
	var out []Segment
	start := 0
	flush := func(end int) {
		if end > start {
			out = append(out, Segment{Text: line[start:end]})
		}
	}
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		var notation string
		switch {
		case r == utf8.RuneError && size == 1:
			notation = fmt.Sprintf(`\x%02x`, line[i])
		case Hidden(r):
			notation = fmt.Sprintf("U+%04X", r)
		}
		if notation == "" {
			i += size
			continue
		}
		flush(i)
		out = append(out, Segment{Text: notation, Revealed: true})
		i += size
		start = i
	}
	flush(len(line))
	return out
}

// Reveal returns s with every revealed segment's notation in place of the
// hidden character, for plain-text contexts such as an attribute value.
func Reveal(s string) string {
	var b strings.Builder
	for _, segment := range Segments(s) {
		b.WriteString(segment.Text)
	}
	return b.String()
}

// Hidden reports whether r draws no visible glyph of its own, changes how the
// surrounding text is parsed, or reorders its display. Tab is not hidden.
// Membership is by Unicode class rather than a hand-picked list:
//
//   - Cc: C0 and C1 controls and DEL (a carriage return included);
//   - Cf: every format character — bidirectional embeddings, overrides,
//     isolates and marks (the "Trojan Source" class), zero-width characters,
//     the byte-order mark, the soft hyphen, invisible operators, interlinear
//     annotation marks and the tag characters used to smuggle invisible
//     ASCII (U+E0001, U+E0020–U+E007F);
//   - Zl and Zp: U+2028 and U+2029, drawn as a space by browsers but line
//     terminators to JavaScript, so text after a "//" can run as code;
//   - the rest of Unicode's default-ignorable set: Hangul and halfwidth
//     fillers (U+115F, U+1160, U+3164, U+FFA0), the combining grapheme joiner
//     and reserved ignorable ranges, and variation selectors, which can carry
//     invisible data. An emoji's presentation selector (U+FE0F) is therefore
//     shown too: a review surface prefers a visible code point to a hidden
//     payload.
//
// A few format characters do draw a glyph (the Arabic number signs); showing
// their code point costs a reviewer nothing. Ordinary spaces of other widths
// (for example U+00A0 and U+3000) stay text: they draw visible whitespace.
// Classification follows the unicode tables of the Go release that builds the
// kit.
func Hidden(r rune) bool {
	if r < utf8.RuneSelf {
		return (r < 0x20 && r != '\t') || r == 0x7f
	}
	return unicode.In(r, hiddenClasses...)
}

var hiddenClasses = []*unicode.RangeTable{
	unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp,
	unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector,
}
