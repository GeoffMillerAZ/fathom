package gallery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"golang.org/x/net/html"
)

func TestHeaderNavigationRetainsRouteParityAndNativeDisclosure(t *testing.T) {
	w := httptest.NewRecorder()
	NewWithOptions(Options{}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/analytics?theme=dark", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("analytics page returned %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	doc := headerNavigationDocument(t, body)
	header := headerNavigationFind(doc, func(n *html.Node) bool { return n.Data == "header" })
	if header == nil {
		t.Fatal("missing Gallery header")
	}

	desktop := headerNavigationFind(header, func(n *html.Node) bool {
		return n.Data == "nav" && headerNavigationStyled(t, n, "hidden") && headerNavigationStyled(t, n, "lg:flex")
	})
	disclosure := headerNavigationFind(header, func(n *html.Node) bool {
		return n.Data == "details" && headerNavigationAttribute(n, "id") == "gallery-navigation"
	})
	if desktop == nil || disclosure == nil {
		t.Fatal("header must retain desktop navigation and #gallery-navigation disclosure")
	}
	if headerNavigationAttribute(disclosure, "open") != "" || headerNavigationHasAttribute(disclosure, "open") {
		t.Fatal("native navigation disclosure must be initially closed without script")
	}
	mobile := headerNavigationFind(disclosure, func(n *html.Node) bool {
		return n.Data == "nav" && headerNavigationAttribute(n, "aria-label") == "Gallery sections"
	})
	if mobile == nil {
		t.Fatal("disclosure missing its native navigation landmark")
	}

	desktopLinks := headerNavigationLinks(desktop)
	if len(desktopLinks) != len(nav) {
		t.Fatalf("desktop route inventory = %d links, want %d", len(desktopLinks), len(nav))
	}
	items := make([]dashboard.NavItem, 0, len(nav))
	currentPath := ""
	for i, configured := range nav {
		got := desktopLinks[i]
		if got.label != configured.Label || got.current != (configured.Key == "analytics") {
			t.Errorf("desktop route %d = %#v, want label %q current=%t", i, got, configured.Label, configured.Key == "analytics")
		}
		if !strings.Contains(got.href, "theme=dark") {
			t.Errorf("desktop route %q lost dark theme: %q", got.label, got.href)
		}
		if got.current {
			currentPath = got.href
		}
		items = append(items, dashboard.NavItem{Href: got.href, Label: got.label})
	}
	if currentPath == "" {
		t.Fatal("desktop navigation lost its current route")
	}
	analyticsURL, err := url.Parse(currentPath)
	if err != nil || analyticsURL.Path != "/analytics" || analyticsURL.Query().Get("theme") != "dark" || analyticsURL.Query().Get("period") != "2026-09" || analyticsURL.Query().Get("compare") != "previous" || analyticsURL.Query().Get("category") != "all" || analyticsURL.Query().Get("state") != "complete" {
		t.Fatalf("analytics navigation did not preserve its selection query: %q", currentPath)
	}
	if got := headerNavigationLinks(mobile); !headerNavigationSameLinks(got, desktopLinks) {
		t.Fatalf("mobile route inventory/current state = %#v, want desktop %#v", got, desktopLinks)
	}
	shared := dashboard.RenderNav([]dashboard.NavGroup{{Items: items}}, currentPath)
	if !strings.Contains(body, shared) {
		t.Fatal("mobile disclosure must embed the shared dashboard.RenderNav output unchanged")
	}

	if brand := headerNavigationTextElement(header, "a", "app-kit Gallery"); brand == nil || headerNavigationAttribute(brand, "href") != "/?theme=dark" {
		t.Fatal("brand home link must retain the sanitized active theme")
	}
	for _, theme := range []struct {
		label, current string
	}{
		{"light", ""}, {"dark", "true"}, {"OS", ""},
	} {
		node := headerNavigationTextElement(header, "a", theme.label)
		if node == nil || headerNavigationAttribute(node, "aria-current") != theme.current {
			t.Errorf("theme %q aria-current = %q, want %q", theme.label, headerNavigationAttribute(node, "aria-current"), theme.current)
		}
		if theme.current == "true" && (!headerNavigationStyled(t, node, "bg-surface-sunken") || !headerNavigationStyled(t, node, "font-semibold")) {
			t.Error("selected theme needs a visible, non-color-only distinction")
		}
	}
	controls := headerNavigationControls(header)
	for i, label := range []string{"app-kit Gallery", "light", "dark", "OS"} {
		if len(controls) <= i || headerNavigationNodeText(controls[i]) != label {
			t.Fatalf("header DOM order must match visual brand/theme/navigation order at %d", i)
		}
	}

	for _, node := range headerNavigationControls(header) {
		if !headerNavigationStyled(t, node, "min-h-[44px]") || !headerNavigationStyled(t, node, "focus-visible:outline-2") || !headerNavigationStyled(t, node, "focus-visible:outline-focus") {
			t.Errorf("header %s %q lacks declared minimum/focus utilities: %q", node.Data, headerNavigationNodeText(node), headerNavigationAttribute(node, "class"))
		}
	}
	if skip := headerNavigationFindAll(doc, func(n *html.Node) bool {
		return n.Data == "a" && headerNavigationAttribute(n, "href") == "#gallery-main"
	}); len(skip) != 1 {
		t.Fatalf("skip links to main = %d, want 1", len(skip))
	}
	main := headerNavigationFind(doc, func(n *html.Node) bool {
		return n.Data == "main" && headerNavigationAttribute(n, "id") == "gallery-main"
	})
	if main == nil || headerNavigationAttribute(main, "tabindex") != "-1" {
		t.Fatal("skip target must be the focusable Gallery main landmark")
	}
}

// Gallery-only aliases move the original utilities to the compiled stylesheet.
// A class name alone is not proof: verify its exact @apply binding as well.
// Browser parity separately tests the resulting cascade, dimensions and focus.
func headerNavigationStyled(t *testing.T, node *html.Node, utility string) bool {
	t.Helper()
	if headerNavigationClass(node, utility) {
		return true
	}
	css, err := os.ReadFile("ui/input.css")
	if err != nil {
		t.Fatal(err)
	}
	var selectors []string
	for _, class := range []string{"g-nav", "g-theme", "g-home", "g-summary", "g-header"} {
		if headerNavigationClass(node, class) {
			selectors = append(selectors, "."+class)
		}
	}
	if node != nil && node.Data == "a" && node.Parent != nil {
		for _, class := range []string{"g-nav", "g-theme"} {
			if !headerNavigationClass(node.Parent, class) {
				continue
			}
			selector := "." + class + " > a"
			selectors = append(selectors, selector)
			if current := headerNavigationAttribute(node, "aria-current"); current != "" {
				selectors = append(selectors, selector+`[aria-current="`+current+`"]`)
			} else {
				selectors = append(selectors, selector+":not([aria-current])")
			}
		}
	}
	for _, selector := range selectors {
		if headerAliasDeclares(string(css), selector, utility) {
			return true
		}
	}
	return false
}

func headerAliasDeclares(css, selector, utility string) bool {
	rule := regexp.MustCompile(`(?m)^[\t ]*` + regexp.QuoteMeta(selector) + `\s*\{\s*@apply\s+([^;]+);`)
	match := rule.FindStringSubmatch(css)
	if len(match) != 2 {
		return false
	}
	for _, token := range strings.Fields(match[1]) {
		if token == utility {
			return true
		}
	}
	return false
}

func TestHeaderAliasRequiresItsOwnDeclaredUtility(t *testing.T) {
	css := `.g-home { @apply min-h-[44px] focus-visible:outline-2; }
.g-summary { @apply focus-visible:outline-focus; }`
	if !headerAliasDeclares(css, ".g-home", "focus-visible:outline-2") ||
		headerAliasDeclares(".wrong "+css, ".g-home", "focus-visible:outline-2") ||
		headerAliasDeclares(css, ".g-home", "focus-visible:outline-focus") ||
		headerAliasDeclares(css, ".absent", "focus-visible:outline-2") ||
		headerAliasDeclares(strings.ReplaceAll(css, "focus-visible:outline-2", ""), ".g-home", "focus-visible:outline-2") {
		t.Fatal("alias declaration check accepted missing or another selector's utility")
	}
}

func TestHeaderNavigationUsesSharedRendererAndSafelyDegradesHostileData(t *testing.T) {
	longLabel := strings.Repeat("wide navigation label ", 24) + "<em>not markup</em>"
	unsafeLabel := `unsafe </span><script>not-executed()</script>`
	links := []navLink{
		{Key: "safe", Label: longLabel, Href: "/safe?theme=dark", Active: true},
		{Key: "unsafe", Label: unsafeLabel, Href: "javascript:alert(1)"},
	}
	body, err := render(pageData{Title: "header test", Nav: links, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	items := []dashboard.NavItem{{Href: links[0].Href, Label: links[0].Label}, {Href: links[1].Href, Label: links[1].Label}}
	shared := dashboard.RenderNav([]dashboard.NavGroup{{Items: items}}, links[0].Href)
	if !strings.Contains(string(body), shared) {
		t.Fatal("mobile contents diverged from dashboard.RenderNav")
	}

	doc := headerNavigationDocument(t, string(body))
	if headerNavigationFind(doc, func(n *html.Node) bool { return n.Data == "script" }) != nil {
		t.Fatal("hostile navigation labels became executable markup")
	}
	disclosure := headerNavigationFind(doc, func(n *html.Node) bool {
		return n.Data == "details" && headerNavigationAttribute(n, "id") == "gallery-navigation"
	})
	mobile := headerNavigationFind(disclosure, func(n *html.Node) bool { return n.Data == "nav" })
	if got := headerNavigationTextElement(mobile, "a", longLabel); got == nil || headerNavigationAttribute(got, "href") != links[0].Href {
		t.Fatal("long safe label was not retained as an escaped mobile link")
	}
	disabled := headerNavigationFind(mobile, func(n *html.Node) bool {
		return n.Data == "span" && headerNavigationAttribute(n, "aria-disabled") == "true"
	})
	if disabled == nil || headerNavigationNodeText(disabled) != unsafeLabel {
		t.Fatal("unsafe mobile navigation item must remain visible as disabled escaped text")
	}
	if headerNavigationFind(mobile, func(n *html.Node) bool {
		return n.Data == "a" && strings.Contains(headerNavigationAttribute(n, "href"), "javascript:")
	}) != nil {
		t.Fatal("unsafe mobile href became interactive")
	}
}

type headerNavigationLink struct {
	href, label string
	current     bool
}

func headerNavigationDocument(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func headerNavigationLinks(root *html.Node) []headerNavigationLink {
	var links []headerNavigationLink
	for _, node := range headerNavigationFindAll(root, func(n *html.Node) bool { return n.Data == "a" }) {
		links = append(links, headerNavigationLink{
			href:    headerNavigationAttribute(node, "href"),
			label:   headerNavigationNodeText(node),
			current: headerNavigationAttribute(node, "aria-current") == "page",
		})
	}
	return links
}

func headerNavigationSameLinks(got, want []headerNavigationLink) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func headerNavigationControls(root *html.Node) []*html.Node {
	return headerNavigationFindAll(root, func(n *html.Node) bool { return n.Data == "a" || n.Data == "summary" })
}

func headerNavigationTextElement(root *html.Node, tag, want string) *html.Node {
	return headerNavigationFind(root, func(n *html.Node) bool {
		return n.Data == tag && headerNavigationNodeText(n) == want
	})
}

func headerNavigationFind(root *html.Node, match func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && match(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := headerNavigationFind(child, match); found != nil {
			return found
		}
	}
	return nil
}

func headerNavigationFindAll(root *html.Node, match func(*html.Node) bool) []*html.Node {
	if root == nil {
		return nil
	}
	var found []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && match(node) {
			found = append(found, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return found
}

func headerNavigationAttribute(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func headerNavigationHasAttribute(node *html.Node, key string) bool {
	if node != nil {
		for _, attribute := range node.Attr {
			if attribute.Key == key {
				return true
			}
		}
	}
	return false
}

func headerNavigationClass(node *html.Node, want string) bool {
	return strings.Contains(" "+headerNavigationAttribute(node, "class")+" ", " "+want+" ")
}

func headerNavigationNodeText(node *html.Node) string {
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			text.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return strings.Join(strings.Fields(text.String()), " ")
}
