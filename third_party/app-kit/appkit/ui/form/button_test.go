package form

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseButtonHTML(t *testing.T, body string) *html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func buttonElements(root *html.Node, name string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == name {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func buttonAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func buttonHasAttr(n *html.Node, name string) bool {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}

func buttonClassHas(n *html.Node, want string) bool {
	for _, class := range strings.Fields(buttonAttr(n, "class")) {
		if class == want {
			return true
		}
	}
	return false
}

func TestRenderButtonAcceptsDocumentAndHTTPLinks(t *testing.T) {
	for _, href := range []string{
		"./settings", "../settings", "settings", "?tab=advanced", "#details",
		"/settings?tab=advanced#details", "http://example.test/settings", "https://example.test/settings",
		"HTTP://example.test/settings", "/search?q=%22x%22&next=%26ok", "#", "?",
	} {
		t.Run(href, func(t *testing.T) {
			body := RenderButton(Button{Label: `Go <now> & see`, Variant: ButtonGhost, Full: true, Href: href})
			anchors := buttonElements(parseButtonHTML(t, body), "a")
			if len(anchors) != 1 || len(buttonElements(parseButtonHTML(t, body), "button")) != 0 {
				t.Fatalf("valid href did not render as an anchor: %s", body)
			}
			anchor := anchors[0]
			if got := buttonAttr(anchor, "href"); got != href {
				t.Fatalf("href = %q, want %q", got, href)
			}
			if !buttonClassHas(anchor, "w-full") || !buttonClassHas(anchor, "border") || strings.Contains(body, "<script") {
				t.Fatalf("valid anchor lost styling or escaping: %s", body)
			}
			if !strings.Contains(body, "Go &lt;now&gt; &amp; see") {
				t.Fatalf("label was not escaped: %s", body)
			}
		})
	}
}

func TestRenderButtonEnabledAnchorBytesRemainStable(t *testing.T) {
	class := "inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium focus:outline-none focus:ring-2 focus:ring-focus disabled:opacity-50 border border-line bg-surface text-ink hover:bg-surface-sunken"
	want := `<a class="` + class + `" href="#">Back</a>`
	if got := RenderButton(Button{Label: "Back", Variant: ButtonGhost, Href: "#"}); got != want {
		t.Fatalf("enabled anchor changed:\n got %s\nwant %s", got, want)
	}
}

func TestRenderButtonRejectsUnsafeLinksAsDisabledNonInteractiveLinks(t *testing.T) {
	unsafe := []struct {
		name string
		href string
	}{
		{"leading whitespace", " /settings"}, {"trailing whitespace", "/settings "},
		{"control", "/settings\x01"}, {"delete", "/settings\x7f"}, {"backslash", `\\settings`},
		{"slash backslash host", `/\\host`}, {"protocol relative", "//evil.example/settings"},
		{"invalid escape", "/settings%ZZ"}, {"invalid query escape", "/settings?q=%ZZ"},
		{"javascript", "javascript:alert(1)"}, {"mixed javascript", "JaVaScRiPt:alert(1)"},
		{"tab scheme obfuscation", "java\tscript:alert(1)"}, {"newline scheme obfuscation", "java\nscript:alert(1)"},
		{"NUL scheme obfuscation", "java\x00script:alert(1)"}, {"data", "data:text/html,hi"},
		{"custom scheme", "custom:settings"}, {"opaque HTTP", "http:settings"},
		{"missing HTTP host", "https:///settings"}, {"empty HTTP host", "https://:443/"},
		{"invalid port", "https://example.test:bad/"},
	}
	for _, tt := range unsafe {
		t.Run(tt.name, func(t *testing.T) {
			body := RenderButton(Button{Label: `Delete <now>`, Variant: ButtonDanger, Full: true, Href: tt.href, Type: "button", Name: "action", Value: "delete"})
			root := parseButtonHTML(t, body)
			spans, anchors, buttons := buttonElements(root, "span"), buttonElements(root, "a"), buttonElements(root, "button")
			if len(spans) != 1 || len(anchors) != 0 || len(buttons) != 0 {
				t.Fatalf("unsafe href remained interactive: %s", body)
			}
			span := spans[0]
			if buttonAttr(span, "role") != "link" || buttonAttr(span, "aria-disabled") != "true" || !buttonClassHas(span, "opacity-50") || !buttonClassHas(span, "w-full") {
				t.Fatalf("unsafe link missing disabled presentation: %s", body)
			}
			for _, attr := range []string{"href", "tabindex", "name", "value", "type"} {
				if buttonHasAttr(span, attr) {
					t.Errorf("unsafe link emitted %s: %s", attr, body)
				}
			}
			for _, class := range []string{"hover:bg-surface-sunken", "hover:opacity-90", "hover:bg-accent-hover", "focus:outline-none", "focus:ring-2", "disabled:opacity-50"} {
				if buttonClassHas(span, class) {
					t.Errorf("inert link retained interactive class %q: %s", class, body)
				}
			}
			if !strings.Contains(body, "Delete &lt;now&gt;") || strings.Contains(body, "href=") {
				t.Fatalf("unsafe link label or href escaping failed: %s", body)
			}
		})
	}

	body := RenderButton(Button{Label: "Disabled", Href: "/settings", Disabled: true, Type: "button", Name: "action", Value: "delete"})
	root := parseButtonHTML(t, body)
	spans := buttonElements(root, "span")
	if len(spans) != 1 || len(buttonElements(root, "a")) != 0 || buttonAttr(spans[0], "role") != "link" || buttonAttr(spans[0], "aria-disabled") != "true" {
		t.Fatalf("disabled valid link remained interactive: %s", body)
	}
	for _, attr := range []string{"href", "tabindex", "name", "value", "type"} {
		if buttonHasAttr(spans[0], attr) {
			t.Errorf("disabled link emitted %s: %s", attr, body)
		}
	}
	for _, class := range []string{"hover:bg-surface-sunken", "hover:opacity-90", "hover:bg-accent-hover", "focus:outline-none", "focus:ring-2", "disabled:opacity-50"} {
		if buttonClassHas(spans[0], class) {
			t.Errorf("inert link retained interactive class %q: %s", class, body)
		}
	}
}

func TestRenderButtonNoHrefNativeOutputRemainsStable(t *testing.T) {
	want := `<button type="submit" class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium focus:outline-none focus:ring-2 focus:ring-focus disabled:opacity-50 bg-accent text-accent-ink hover:bg-accent-hover">Save</button>`
	if got := RenderButton(Button{Label: "Save"}); got != want {
		t.Fatalf("native button changed:\n got %s\nwant %s", got, want)
	}
	class := "inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium focus:outline-none focus:ring-2 focus:ring-focus disabled:opacity-50 bg-accent text-accent-ink hover:bg-accent-hover w-full"
	want = `<button type="button" class="` + class + `" name="act&#34;" value="v&amp;" disabled>Save &amp; &lt;now&gt;</button>`
	if got := RenderButton(Button{Label: "Save & <now>", Type: "button", Name: `act"`, Value: "v&", Full: true, Disabled: true}); got != want {
		t.Fatalf("native button attributes or escaping changed:\n got %s\nwant %s", got, want)
	}
}
