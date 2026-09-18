package dashboard

import (
	"html/template"
	"regexp"
	"strings"
	"testing"
)

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name, href, current string
		match               NavMatch
		want                bool
	}{
		{"exact default", "/board", "/board", MatchExact, true},
		{"exact excludes descendants", "/board", "/board/1", MatchExact, false},
		{"subtree descendant", "/board", "/board/1", MatchSubtree, true},
		{"subtree self", "/board", "/board", MatchSubtree, true},
		{"prefix boundary", "/board", "/boardwalk", MatchSubtree, false},
		{"root exact", "/", "/", MatchExact, true},
		{"root subtree is exact", "/", "/board", MatchSubtree, false},
		{"root query", "/?view=all", "/#today", MatchExact, true},
		{"trailing slash", "/board/", "/board", MatchExact, true},
		{"current trailing slash", "/board", "/board/", MatchExact, true},
		{"trailing slash subtree", "/board/", "/board/1/", MatchSubtree, true},
		{"queries and fragments", "/board?view=all#top", "/board?view=mine#bottom", MatchExact, true},
		{"encoded separator", "/board", "/board%2F1", MatchSubtree, false},
		{"empty target", "", "/", MatchExact, false},
		{"empty current", "/", "", MatchExact, false},
		{"unknown match", "/board", "/board", NavMatch(99), false},
		{"external target", "https://example.com/board", "/board", MatchExact, false},
		{"external current", "/board", "https://example.com/board", MatchExact, false},
		{"protocol relative", "//example.com/board", "/board", MatchExact, false},
		{"javascript", "javascript:alert(1)", "/", MatchExact, false},
		{"bad escape", "/board%zz", "/board", MatchExact, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchPath(tt.href, tt.current, tt.match); got != tt.want {
				t.Errorf("MatchPath(%q, %q, %d) = %v, want %v", tt.href, tt.current, tt.match, got, tt.want)
			}
		})
	}
}

func TestRenderNavCurrentSelection(t *testing.T) {
	groups := []NavGroup{
		{Items: []NavItem{
			{Href: "/", Label: "Home", Match: MatchSubtree},
			{Href: "/grants", Label: "Grants", Match: MatchSubtree},
			{Href: "/grants/board", Label: "Board subtree", Match: MatchSubtree},
		}},
		{Heading: "More", Items: []NavItem{
			{Href: "/grants/board", Label: "Board exact"},
			{Href: "/grants/board/", Label: "Duplicate exact"},
		}},
	}
	for _, tt := range []struct{ path, label string }{
		{"/", "Home"},
		{"/grants/new", "Grants"},
		{"/grants/board/1", "Board subtree"},
		{"/grants/board?view=all#top", "Board exact"},
		{"/grantsmanship", ""},
	} {
		t.Run(tt.path, func(t *testing.T) {
			got := RenderNav(groups, tt.path)
			selected := regexp.MustCompile(`<a\b[^>]*aria-current="page"[^>]*>(.*?)</a>`).FindAllStringSubmatch(got, -1)
			if tt.label == "" {
				if len(selected) != 0 {
					t.Fatalf("unexpected current link: %v", selected)
				}
				return
			}
			if len(selected) != 1 || !strings.Contains(selected[0][1], ">"+tt.label+"</span>") {
				t.Fatalf("want only %q selected, got %v", tt.label, selected)
			}
		})
	}
}

func TestRenderNavGroupsAndAccessibleLabels(t *testing.T) {
	if got := RenderNav([]NavGroup{{Heading: "Empty"}}, "/"); got != "" {
		t.Fatalf("empty groups rendered markup: %s", got)
	}
	long := strings.Repeat("LongLabel", 50)
	icon := template.HTML(`<svg viewBox="0 0 8 8"><path d="M0 0h8"/></svg>`)
	got := RenderNav([]NavGroup{
		{Heading: "Empty"},
		{Heading: "Money & growth", Items: []NavItem{{Href: "/money", Label: long, Icon: icon}}},
		{Items: []NavItem{{Href: "/help", Label: "Help"}}},
	}, "/money")
	for _, want := range []string{
		`<h2`, "Money &amp; growth", `aria-label="Money &amp; growth"`,
		`aria-hidden="true"`, string(icon), long, `whitespace-normal`, `[overflow-wrap:anywhere]`,
		`min-h-[44px]`, `focus-visible:outline-focus`, `aria-current="page"`, `bg-accent-soft font-semibold text-ink`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(got, "Empty") || strings.Contains(got, " id=") || strings.Contains(got, "truncate") {
		t.Errorf("empty group, duplicated ID risk, or truncation in markup: %s", got)
	}
	if strings.Contains(got, `bg-accent-soft font-semibold text-accent`) {
		t.Errorf("active navigation retained the low-contrast accent text role: %s", got)
	}
	if gotCount := strings.Count(got, `min-h-[44px]`); gotCount != 2 {
		t.Errorf("navigation minimum-height count = %d, want one per item: %s", gotCount, got)
	}
	if strings.Count(got, "<ul ") != 2 || strings.Count(got, "<li ") != 2 {
		t.Errorf("navigation groups are not semantic lists: %s", got)
	}
}

func TestRenderNavEscapesTextAndUnsafeURLs(t *testing.T) {
	unsafe := []string{
		"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<script>alert(1)</script>",
		"//evil.example/", `/\evil.example/`, "https:\\evil.example/", "https:///missing-host",
		"\njavascript:alert(1)", "/ok\nnot-ok", " /padded", "relative/path", "#fragment", "/bad%zz", "",
	}
	items := []NavItem{
		{Href: `/?q="'><script>alert(1)</script>&a=b`, Label: `<script>alert("label")</script>`},
		{Href: "https://example.com/help?x=1&y=2", Label: "External help"},
		{Href: "http://example.com/help", Label: "HTTP help"},
	}
	for _, href := range unsafe {
		items = append(items, NavItem{Href: href, Label: "Unsafe destination"})
	}
	got := RenderNav([]NavGroup{{Heading: `<img src=x onerror="alert(1)">`, Items: items}}, "/")
	if strings.Contains(got, "<script>") || strings.Contains(got, "<img") {
		t.Fatalf("untrusted text rendered as markup: %s", got)
	}
	for _, want := range []string{
		`&lt;script&gt;alert(&#34;label&#34;)&lt;/script&gt;`,
		`href="/?q=&#34;&#39;&gt;&lt;script&gt;alert(1)&lt;/script&gt;&amp;a=b"`,
		`href="https://example.com/help?x=1&amp;y=2"`, `href="http://example.com/help"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing escaped or safe value %q", want)
		}
	}
	if count := strings.Count(got, "<a "); count != 3 {
		t.Errorf("want 3 safe links, got %d", count)
	}
	if count := strings.Count(got, `aria-disabled="true"`); count != len(unsafe) {
		t.Errorf("want %d noninteractive unsafe destinations, got %d", len(unsafe), count)
	}
	if count := strings.Count(got, `aria-current="page"`); count != 1 {
		t.Errorf("external or unsafe destinations affected current selection: %d selected", count)
	}
}
