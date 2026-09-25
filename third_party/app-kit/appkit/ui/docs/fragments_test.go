package docs

import (
	"html"
	"net/url"
	"strings"
	"testing"
)

func TestHeadingFragmentLinksPreserveCallerOwnedIDs(t *testing.T) {
	t.Parallel()

	renderers := map[string]func(Heading) string{
		"boxed": func(h Heading) string { return RenderToC([]Heading{h}) },
		"panel": func(h Heading) string { return RenderToCPanel([]Heading{h}, Options{}) },
		"rail":  func(h Heading) string { return RenderToCRail([]Heading{h}, Options{}) },
		"permalink": func(h Heading) string {
			return RenderPermalink(h.ID)
		},
	}
	ids := []string{
		"percent%",
		"encoded%20literal",
		"hash#literal",
		"question?literal",
		"日本語の見出し",
		`quotes"-ampersand&-less<`,
		"safe-ascii_123",
	}

	for name, render := range renderers {
		for _, id := range ids {
			t.Run(name+"/"+id, func(t *testing.T) {
				got := firstHref(t, render(Heading{Level: 2, Text: `Text " & <`, ID: id}))
				want := (&url.URL{Fragment: id}).String()
				if got != want {
					t.Fatalf("href = %q, want %q", got, want)
				}
				parsed, err := url.Parse(got)
				if err != nil {
					t.Fatalf("parse href %q: %v", got, err)
				}
				if parsed.Fragment != id {
					t.Fatalf("fragment = %q, want literal caller ID %q", parsed.Fragment, id)
				}
				if id == "safe-ascii_123" && got != "#safe-ascii_123" {
					t.Fatalf("safe ASCII href changed: %q", got)
				}
			})
		}
	}
}

func TestHeadingFragmentRenderingPreservesExistingEmptyAndTextEscaping(t *testing.T) {
	t.Parallel()

	if got := RenderPermalink(""); got != "" {
		t.Fatalf("empty permalink = %q, want empty", got)
	}
	for name, render := range map[string]func([]Heading) string{
		"boxed": RenderToC,
		"panel": func(h []Heading) string { return RenderToCPanel(h, Options{}) },
		"rail":  func(h []Heading) string { return RenderToCRail(h, Options{}) },
	} {
		if got := firstHref(t, render([]Heading{{Level: 2, Text: "Empty ID"}})); got != "#" {
			t.Errorf("%s empty-ID href = %q, want #", name, got)
		}
	}
	got := RenderToC([]Heading{{Level: 2, Text: `Text " & <`, ID: "safe"}})
	if !strings.Contains(got, `>Text &#34; &amp; &lt;</a>`) {
		t.Fatalf("heading text was not escaped: %q", got)
	}
}

func firstHref(t *testing.T, rendered string) string {
	t.Helper()
	const prefix = `href="`
	start := strings.Index(rendered, prefix)
	if start < 0 {
		t.Fatalf("rendered output has no href: %q", rendered)
	}
	rest := rendered[start+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		t.Fatalf("rendered href is unterminated: %q", rendered)
	}
	return html.UnescapeString(rest[:end])
}
