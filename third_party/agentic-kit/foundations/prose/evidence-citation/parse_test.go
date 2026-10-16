package evidencecitation_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	ec "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/prose/evidence-citation"
)

// TestParse pins the tag grammar: both the "[tag:id]" bracket-tag form and
// Cite's "[ref1; ref2]" form, restricted vs unrestricted vocabularies,
// malformed/nested/adjacent blocks, unicode IDs, and exact byte offsets.
func TestParse(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		text string
		want []ec.Citation
	}{
		{
			name: "single bracket tag",
			tags: []string{"span"},
			text: "See [span:abc], ok.",
			want: []ec.Citation{{Tag: "span", ID: "abc", Start: 5, End: 13, Raw: "span:abc"}},
		},
		{
			name: "throughline vocabulary, two tags",
			tags: []string{"span", "authority"},
			text: "Fact one [span:a] and rule [authority:b-2].",
			want: []ec.Citation{
				{Tag: "span", ID: "a", Start: 10, End: 16, Raw: "span:a"},
				{Tag: "authority", ID: "b-2", Start: 28, End: 41, Raw: "authority:b-2"},
			},
		},
		{
			name: "cite output, bare refs, unrestricted",
			text: "[span-a; span-b]",
			want: []ec.Citation{
				{ID: "span-a", Start: 1, End: 7, Raw: "span-a"},
				{ID: "span-b", Start: 9, End: 15, Raw: "span-b"},
			},
		},
		{
			name: "restricted parser skips unknown tags and bare refs",
			tags: []string{"span"},
			text: "[note:x] [span:y] [plain]",
			want: []ec.Citation{{Tag: "span", ID: "y", Start: 10, End: 16, Raw: "span:y"}},
		},
		{
			name: "unrestricted accepts any tag word and bare refs",
			text: "[note:x] [span:y] [plain]",
			want: []ec.Citation{
				{Tag: "note", ID: "x", Start: 1, End: 7, Raw: "note:x"},
				{Tag: "span", ID: "y", Start: 10, End: 16, Raw: "span:y"},
				{ID: "plain", Start: 19, End: 24, Raw: "plain"},
			},
		},
		{
			name: "multi-ref block with tag filter drops only unmatched refs",
			tags: []string{"span", "authority"},
			text: "[span:a; see note 4; authority:b]",
			want: []ec.Citation{
				{Tag: "span", ID: "a", Start: 1, End: 7, Raw: "span:a"},
				{Tag: "authority", ID: "b", Start: 21, End: 32, Raw: "authority:b"},
			},
		},
		{
			name: "nested bracket restarts at inner block",
			text: "[span:[span:x]]",
			want: []ec.Citation{{Tag: "span", ID: "x", Start: 7, End: 13, Raw: "span:x"}},
		},
		{
			name: "adjacent blocks",
			tags: []string{"span"},
			text: "[span:a][span:b]",
			want: []ec.Citation{
				{Tag: "span", ID: "a", Start: 1, End: 7, Raw: "span:a"},
				{Tag: "span", ID: "b", Start: 9, End: 15, Raw: "span:b"},
			},
		},
		{
			name: "unicode ID, byte offsets",
			tags: []string{"span"},
			text: "[span:héllo-✓]",
			want: []ec.Citation{{Tag: "span", ID: "héllo-✓", Start: 1, End: 16, Raw: "span:héllo-✓"}},
		},
		{
			name: "ID keeps interior colons (split is at first colon)",
			tags: []string{"span"},
			text: "[span:urn:x:y]",
			want: []ec.Citation{{Tag: "span", ID: "urn:x:y", Start: 1, End: 13, Raw: "span:urn:x:y"}},
		},
		{
			name: "ID keeps interior spaces",
			tags: []string{"span"},
			text: "[span:two words]",
			want: []ec.Citation{{Tag: "span", ID: "two words", Start: 1, End: 15, Raw: "span:two words"}},
		},
		{
			name: "refs are trimmed of ASCII spaces, offsets cover trimmed ref",
			text: "[ span:a ; span-b ]",
			want: []ec.Citation{
				{Tag: "span", ID: "a", Start: 2, End: 8, Raw: "span:a"},
				{ID: "span-b", Start: 11, End: 17, Raw: "span-b"},
			},
		},
		{
			name: "non-tag-word prefixes fall back to bare refs",
			text: "[:x] [x:] [1:2] [a b:c]",
			want: []ec.Citation{
				{ID: ":x", Start: 1, End: 3, Raw: ":x"},
				{ID: "x:", Start: 6, End: 8, Raw: "x:"},
				{ID: "1:2", Start: 11, End: 14, Raw: "1:2"},
				{ID: "a b:c", Start: 17, End: 22, Raw: "a b:c"},
			},
		},
		{
			name: "tag match is case-sensitive",
			tags: []string{"span"},
			text: "[Span:a]",
			want: nil,
		},
		{name: "empty text", text: "", want: nil},
		{name: "no brackets", text: "no citations here", want: nil},
		{name: "unclosed block", tags: []string{"span"}, text: "see [span:a", want: nil},
		{name: "empty block", text: "[]", want: nil},
		{name: "space-only block", text: "[ ]", want: nil},
		{name: "separator-only block", text: "[;]", want: nil},
		{
			name: "any empty segment invalidates the whole block",
			text: "[a;; b]",
			want: nil,
		},
		{
			name: "trailing separator invalidates the block",
			text: "[a;]",
			want: nil,
		},
		{
			name: "blocks never span lines",
			tags: []string{"span"},
			text: "[span:\na]",
			want: nil,
		},
		{
			name: "carriage return aborts a block",
			tags: []string{"span"},
			text: "[span:\ra]",
			want: nil,
		},
		{
			name: "empty-string tag names are ignored (unrestricted)",
			tags: []string{"", ""},
			text: "[anything:goes]",
			want: []ec.Citation{{Tag: "anything", ID: "goes", Start: 1, End: 14, Raw: "anything:goes"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := ec.NewParser(tc.tags...)
			got := p.Parse(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q):\n got  %+v\n want %+v", tc.text, got, tc.want)
			}
			// Offsets must always address the raw slice.
			for i, c := range got {
				if c.Start < 0 || c.End > len(tc.text) || c.Start >= c.End {
					t.Errorf("citation %d: offsets [%d,%d) out of bounds for len %d", i, c.Start, c.End, len(tc.text))
					continue
				}
				if tc.text[c.Start:c.End] != c.Raw {
					t.Errorf("citation %d: text[%d:%d] = %q, Raw = %q", i, c.Start, c.End, tc.text[c.Start:c.End], c.Raw)
				}
			}
		})
	}
}

// TestParse_DocumentOrder pins the documented ordering: citations are
// returned in document order (ascending byte offset), never re-sorted.
func TestParse_DocumentOrder(t *testing.T) {
	text := "[span:z] then [span:a] then [span:m]"
	got := ec.NewParser("span").Parse(text)
	if len(got) != 3 {
		t.Fatalf("expected 3 citations, got %d", len(got))
	}
	wantIDs := []string{"z", "a", "m"}
	for i, c := range got {
		if c.ID != wantIDs[i] {
			t.Errorf("citation %d: ID %q, want %q (document order)", i, c.ID, wantIDs[i])
		}
		if i > 0 && c.Start < got[i-1].End {
			t.Errorf("citation %d overlaps previous: start %d < previous end %d", i, c.Start, got[i-1].End)
		}
	}
}

// TestParse_HugeInputs ensures pathological inputs complete (linear scan)
// and still find the well-formed citation.
func TestParse_HugeInputs(t *testing.T) {
	p := ec.NewParser("span")

	open := strings.Repeat("[", 1<<16) + "span:a]"
	got := p.Parse(open)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("repeated-open input: got %+v, want one span:a", got)
	}

	if got := p.Parse(strings.Repeat("a]", 1<<15)); got != nil {
		t.Errorf("close-only input: got %+v, want nil", got)
	}

	many := strings.Repeat("[span:x] ", 1000)
	cites := p.Parse(many)
	if len(cites) != 1000 {
		t.Errorf("expected 1000 citations, got %d", len(cites))
	}
}

// TestCiteParseRoundTrip pins the lossless round trip: an unrestricted
// Parse of Cite output recovers exactly the deduplicated, sorted refs.
func TestCiteParseRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		refs []string
		want []string // nil means expect no citations
	}{
		{name: "bare refs sorted", refs: []string{"span-b", "span-a"}, want: []string{"span-a", "span-b"}},
		{name: "tagged refs deduped", refs: []string{"span:a", "authority:b", "span:a"}, want: []string{"authority:b", "span:a"}},
		{name: "unicode and non-ASCII colon prefix", refs: []string{"héllo-✓", "α:β"}, want: []string{"héllo-✓", "α:β"}},
		{name: "interior colons survive", refs: []string{"urn:x:y"}, want: []string{"urn:x:y"}},
		{name: "empty refs filtered by Cite", refs: []string{"", "a", ""}, want: []string{"a"}},
		{name: "no refs", refs: nil, want: nil},
	}
	p := ec.NewParser()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := ec.Cite(tc.refs)
			got := p.Parse(rendered)
			var gotRefs []string
			for _, c := range got {
				gotRefs = append(gotRefs, c.Ref())
				if rendered[c.Start:c.End] != c.Raw {
					t.Errorf("offsets do not address raw slice: %+v in %q", c, rendered)
				}
			}
			if !reflect.DeepEqual(gotRefs, tc.want) {
				t.Errorf("round trip of %v via %q: got %v, want %v", tc.refs, rendered, gotRefs, tc.want)
			}
		})
	}
}

// TestValidate pins the anti-fabrication seam: citations the resolver does
// not recognize come back in document order; a nil resolver fails closed.
func TestValidate(t *testing.T) {
	cites := ec.NewParser("span").Parse("[span:real-1] [span:fake-9] [span:real-2]")
	if len(cites) != 3 {
		t.Fatalf("setup: expected 3 citations, got %d", len(cites))
	}

	known := func(tag, id string) bool {
		return tag == "span" && strings.HasPrefix(id, "real-")
	}

	unknown := ec.Validate(cites, known)
	if len(unknown) != 1 || unknown[0].ID != "fake-9" {
		t.Errorf("Validate: got %+v, want the single fake-9 citation", unknown)
	}

	if got := ec.Validate(cites, func(string, string) bool { return true }); got != nil {
		t.Errorf("all-known resolver: got %+v, want nil", got)
	}

	// Fail closed: nil resolver treats everything as unresolvable.
	all := ec.Validate(cites, nil)
	if !reflect.DeepEqual(all, cites) {
		t.Errorf("nil resolver: got %+v, want all citations back", all)
	}
	if got := ec.Validate(nil, nil); got != nil {
		t.Errorf("nil citations, nil resolver: got %+v, want nil", got)
	}
}

// TestStrip pins block-granular removal with the one-preceding-space rule.
func TestStrip(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		text string
		want string
	}{
		{
			name: "sentence-final citation leaves clean punctuation",
			tags: []string{"span"},
			text: "The lease was signed [span:doc-3].",
			want: "The lease was signed.",
		},
		{
			name: "mid-sentence citation",
			tags: []string{"span"},
			text: "a [span:x] b",
			want: "a b",
		},
		{
			name: "block at start may leave a leading space (documented)",
			tags: []string{"span"},
			text: "[span:x] leading",
			want: " leading",
		},
		{
			name: "adjacent blocks",
			tags: []string{"span"},
			text: "a [span:x][span:y] b",
			want: "a b",
		},
		{
			name: "unmatched footnote block is untouched",
			tags: []string{"span"},
			text: "see [1] and [span:x]",
			want: "see [1] and",
		},
		{
			name: "block with only filtered refs is untouched",
			tags: []string{"span"},
			text: "x [note:y]",
			want: "x [note:y]",
		},
		{
			name: "whole block removed even when some refs were filtered",
			tags: []string{"span"},
			text: "fact [span:a; see note] end",
			want: "fact end",
		},
		{
			name: "cite-format block, unrestricted",
			text: "[span-a; span-b] done",
			want: " done",
		},
		{
			name: "no citations returns text unchanged",
			tags: []string{"span"},
			text: "no citations",
			want: "no citations",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ec.NewParser(tc.tags...).Strip(tc.text)
			if got != tc.want {
				t.Errorf("Strip(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestReplace pins offset-safety (identity replacement reproduces the
// input byte-for-byte) and anchor rewriting.
func TestReplace(t *testing.T) {
	p := ec.NewParser("span", "authority")
	text := "Fact [span:a; span:b] and [authority:c]. Footnote [1] stays."

	t.Run("identity replacement is byte-identical", func(t *testing.T) {
		got := p.Replace(text, func(raw string, _ []ec.Citation) string { return raw })
		if got != text {
			t.Errorf("identity Replace changed text:\n got  %q\n want %q", got, text)
		}
	})

	t.Run("anchor rendering", func(t *testing.T) {
		got := p.Replace(text, func(raw string, cites []ec.Citation) string {
			ids := make([]string, len(cites))
			for i, c := range cites {
				// Offsets must address the original text.
				if text[c.Start:c.End] != c.Raw {
					t.Errorf("citation offsets do not address original text: %+v", c)
				}
				ids[i] = c.ID
			}
			return "(" + strings.Join(ids, ",") + ")"
		})
		want := "Fact (a,b) and (c). Footnote [1] stays."
		if got != want {
			t.Errorf("Replace = %q, want %q", got, want)
		}
	})

	t.Run("nil repl returns text unchanged", func(t *testing.T) {
		if got := p.Replace(text, nil); got != text {
			t.Errorf("nil repl changed text: %q", got)
		}
	})

	t.Run("no recognized blocks returns text unchanged", func(t *testing.T) {
		plain := "nothing [here;] to do"
		if got := p.Replace(plain, func(string, []ec.Citation) string { return "X" }); got != plain {
			t.Errorf("Replace on citation-free text = %q, want unchanged", got)
		}
	})
}

// TestParseable pins the round-trip precondition predicate.
func TestParseable(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"", false},
		{" a", false},
		{"a ", false},
		{"a;b", false},
		{"a[b", false},
		{"a]b", false},
		{"a\nb", false},
		{"a\rb", false},
		{"span:a", true},
		{"span-001", true},
		{"héllo-✓", true},
		{"two words", true},
		{"urn:x:y", true},
	}
	for _, tc := range cases {
		if got := ec.Parseable(tc.ref); got != tc.want {
			t.Errorf("Parseable(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

// TestRoundTrip_SortedRecovery cross-checks the documented contract with a
// sort-based oracle over a larger ref set.
func TestRoundTrip_SortedRecovery(t *testing.T) {
	refs := []string{"span:z", "span:a", "plain-9", "authority:m", "span:a", "α:β"}
	want := uniqueSorted(refs)
	got := ec.NewParser().Parse(ec.Cite(refs))
	if len(got) != len(want) {
		t.Fatalf("got %d citations, want %d", len(got), len(want))
	}
	for i, c := range got {
		if c.Ref() != want[i] {
			t.Errorf("citation %d: Ref() = %q, want %q", i, c.Ref(), want[i])
		}
	}
}

func uniqueSorted(refs []string) []string {
	seen := make(map[string]struct{}, len(refs))
	var out []string
	for _, r := range refs {
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
