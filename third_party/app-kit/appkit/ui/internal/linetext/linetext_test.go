package linetext

import (
	"reflect"
	"testing"
	"unicode"
)

func TestSplitKeepsFinalNewlineSeparateAndCarriageReturnAsContent(t *testing.T) {
	cases := []struct {
		in    string
		lines []string
		final bool
	}{
		{"", nil, false},
		{"\n", []string{""}, true},
		{"a", []string{"a"}, false},
		{"a\n", []string{"a"}, true},
		{"a\n\nb", []string{"a", "", "b"}, false},
		{"a\r\nb\r\n", []string{"a\r", "b\r"}, true},
	}
	for _, c := range cases {
		lines, final := Split(c.in)
		if !reflect.DeepEqual(lines, c.lines) || final != c.final {
			t.Errorf("Split(%q) = %q, %v; want %q, %v", c.in, lines, final, c.lines, c.final)
		}
	}
}

func TestSegmentsRevealHiddenAndReorderingCharacters(t *testing.T) {
	got := Segments("if a\u202e { \tx\u200b\x00\r" + string([]byte{0xff}) + "é")
	want := []Segment{
		{Text: "if a"}, {Text: "U+202E", Revealed: true}, {Text: " { \tx"},
		{Text: "U+200B", Revealed: true}, {Text: "U+0000", Revealed: true},
		{Text: "U+000D", Revealed: true}, {Text: `\xff`, Revealed: true}, {Text: "é"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Segments = %#v\nwant %#v", got, want)
	}
	if got := Segments("plain\ttext"); !reflect.DeepEqual(got, []Segment{{Text: "plain\ttext"}}) {
		t.Fatalf("ordinary text split: %#v", got)
	}
	if got := Segments(""); got != nil {
		t.Fatalf("empty line produced segments: %#v", got)
	}
	for _, r := range []rune{0x061c, 0x200e, 0x200f, 0x202a, 0x2066, 0x2069, 0xfeff, 0x85, 0x7f} {
		if !Hidden(r) {
			t.Errorf("U+%04X not revealed", r)
		}
	}
	for _, r := range []rune{'\t', ' ', 'a', 0xa0, 0x2026, 'é', '中', 0x3000, 0x1f600} {
		if Hidden(r) {
			t.Errorf("U+%04X revealed but has a visible rendering", r)
		}
	}
}

// TestHiddenCoversEveryInvisibleClass enumerates the Unicode classes that draw
// nothing or change how the surrounding text is parsed or ordered, so a new
// hand-picked list cannot silently miss one again. U+2028 is drawn as a space
// but ends a JavaScript line; tag characters (ASCII smuggling) have no width.
func TestHiddenCoversEveryInvisibleClass(t *testing.T) {
	classes := map[string]*unicode.RangeTable{
		"Cc (controls)":            unicode.Cc,
		"Cf (format)":              unicode.Cf,
		"Zl (line separator)":      unicode.Zl,
		"Zp (paragraph separator)": unicode.Zp,
		"default ignorable":        unicode.Other_Default_Ignorable_Code_Point,
		"variation selectors":      unicode.Variation_Selector,
	}
	for name, table := range classes {
		missed := 0
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if r != '\t' && unicode.Is(table, r) && !Hidden(r) {
				if missed < 5 {
					t.Errorf("%s: U+%04X is not revealed", name, r)
				}
				missed++
			}
		}
		if missed > 0 {
			t.Errorf("%s: %d code points not revealed", name, missed)
		}
	}
	for _, r := range []rune{
		0x2028, 0x2029, // line and paragraph separators
		0xe0001, 0xe0020, 0xe0061, 0xe007f, // language tag and tag characters
		0x00ad, 0x180e, 0x2061, 0x2064, 0x206a, 0x206f, 0xfff9, 0xfffb, // format characters
		0x3164, 0x115f, 0x1160, 0xffa0, 0x034f, // fillers and the grapheme joiner
		0xfe0f, 0xe0100, // variation selectors
	} {
		if !Hidden(r) {
			t.Errorf("U+%04X not revealed", r)
		}
	}
	got := Segments("x = 1; // off\u2028run()\U000E0061")
	want := []Segment{{Text: "x = 1; // off"}, {Text: "U+2028", Revealed: true}, {Text: "run()"}, {Text: "U+E0061", Revealed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Segments = %#v\nwant %#v", got, want)
	}
}

func TestRevealReplacesHiddenCharactersInPlainText(t *testing.T) {
	if got := Reveal("admin\u202egpj.exe\u2028" + string([]byte{0xff})); got != `adminU+202Egpj.exeU+2028\xff` {
		t.Fatalf("Reveal = %q", got)
	}
	if got := Reveal("main\tbranch"); got != "main\tbranch" {
		t.Fatalf("Reveal changed ordinary text: %q", got)
	}
}
