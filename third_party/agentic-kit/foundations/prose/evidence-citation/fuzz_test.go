package evidencecitation_test

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	ec "github.com/geoffmilleraz/agentic-kit/foundations/prose/evidence-citation"
)

// FuzzParse exercises Parse/Strip/Replace against arbitrary, hostile, and
// invalid-UTF-8 input.
//
// SCOPE: robustness only — it asserts structural invariants for any input;
// it says nothing about whether a given bracketed string SHOULD be a
// citation (the table tests in parse_test.go pin the grammar).
//
// Invariants asserted for every input, for both an unrestricted parser and
// a restricted ("span", "authority") one:
//   - Parse never panics and is deterministic.
//   - 0 <= Start < End <= len(text), Raw == text[Start:End], ID non-empty,
//     and Ref() == Raw (reconstruction is exact).
//   - Citations are in document order and never overlap.
//   - Every restricted citation appears identically in the unrestricted
//     parse (the tag filter only removes, never invents).
//   - Replace with an identity function is byte-identical to the input
//     (offset-safety).
//   - Strip never grows the text and preserves UTF-8 validity (it cuts
//     only at ASCII block boundaries).
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"no citations at all",
		"[span:a]",
		"[span-a; span-b]",
		ec.Cite([]string{"span:z", "plain", "authority:m"}),
		"[span:[span:x]]",
		"[span:a][authority:b]",
		"see [1] and [span:x].",
		"[:x][x:][1:2]",
		"[span:héllo-✓]",
		"[a;; b]",
		"[span:\na]",
		"[ span:a ; span-b ]",
		strings.Repeat("[", 4096) + "span:a]",
		strings.Repeat("[span:x] ", 256),
		"\xff\xfe[span:a]\xfd",
		"[span:\xc3\x28]",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	unrestricted := ec.NewParser()
	restricted := ec.NewParser("span", "authority")

	f.Fuzz(func(t *testing.T, text string) {
		inAny := make(map[ec.Citation]bool)
		for _, c := range unrestricted.Parse(text) {
			inAny[c] = true
		}

		parsers := []struct {
			name string
			p    *ec.Parser
		}{{"unrestricted", unrestricted}, {"restricted", restricted}}
		for _, pp := range parsers {
			name, p := pp.name, pp.p
			cites := p.Parse(text)

			prevEnd := 0
			for i, c := range cites {
				if c.Start < 0 || c.End > len(text) || c.Start >= c.End {
					t.Fatalf("%s: citation %d offsets [%d,%d) out of bounds for len %d", name, i, c.Start, c.End, len(text))
				}
				if text[c.Start:c.End] != c.Raw {
					t.Errorf("%s: citation %d Raw %q != text[%d:%d] %q", name, i, c.Raw, c.Start, c.End, text[c.Start:c.End])
				}
				if c.ID == "" {
					t.Errorf("%s: citation %d has empty ID: %+v", name, i, c)
				}
				if c.Ref() != c.Raw {
					t.Errorf("%s: citation %d Ref() %q != Raw %q", name, i, c.Ref(), c.Raw)
				}
				if c.Start < prevEnd {
					t.Errorf("%s: citation %d overlaps or precedes previous (start %d < prev end %d)", name, i, c.Start, prevEnd)
				}
				prevEnd = c.End

				if name == "restricted" {
					if c.Tag != "span" && c.Tag != "authority" {
						t.Errorf("restricted: citation %d has tag %q outside vocabulary", i, c.Tag)
					}
					if !inAny[c] {
						t.Errorf("restricted citation %+v missing from unrestricted parse", c)
					}
				}
			}

			if again := p.Parse(text); !reflect.DeepEqual(cites, again) {
				t.Errorf("%s: Parse is not deterministic", name)
			}

			if got := p.Replace(text, func(raw string, _ []ec.Citation) string { return raw }); got != text {
				t.Errorf("%s: identity Replace not byte-identical:\n got  %q\n text %q", name, got, text)
			}

			stripped := p.Strip(text)
			if len(stripped) > len(text) {
				t.Errorf("%s: Strip grew the text: %d > %d", name, len(stripped), len(text))
			}
			if utf8.ValidString(text) && !utf8.ValidString(stripped) {
				t.Errorf("%s: Strip broke UTF-8 validity: %q -> %q", name, text, stripped)
			}
		}
	})
}

// FuzzCiteParseRoundTrip pins the lossless round trip on arbitrary refs:
// for any refs satisfying Parseable, an unrestricted Parse of Cite output
// recovers exactly the deduplicated, sorted refs.
func FuzzCiteParseRoundTrip(f *testing.F) {
	f.Add("span:a", "span:b", "authority:c")
	f.Add("span-a", "span-b", "span-a")
	f.Add("héllo-✓", "α:β", "urn:x:y")
	f.Add("", " a", "a ")
	f.Add("two words", "x:", "1:2")
	f.Add("a;b", "[x]", "ok")

	p := ec.NewParser()

	f.Fuzz(func(t *testing.T, a, b, c string) {
		var refs []string
		for _, r := range []string{a, b, c} {
			if ec.Parseable(r) {
				refs = append(refs, r)
			}
		}
		want := uniqueSorted(refs)

		rendered := ec.Cite(refs)
		got := p.Parse(rendered)
		if len(got) != len(want) {
			t.Fatalf("Parse(Cite(%q)) = %+v: got %d citations, want %d", refs, got, len(got), len(want))
		}
		for i, cit := range got {
			if cit.Ref() != want[i] {
				t.Errorf("citation %d: Ref() = %q, want %q (rendered %q)", i, cit.Ref(), want[i], rendered)
			}
			if rendered[cit.Start:cit.End] != cit.Raw {
				t.Errorf("citation %d: offsets do not address rendered text: %+v", i, cit)
			}
		}
	})
}
