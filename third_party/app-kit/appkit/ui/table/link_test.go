package table

import (
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

var linkTestCols = []Column{{Key: "name", Label: "Name"}}

func linkTestDocument(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func linkTestAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func linkTestHasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func linkTestClassHas(n *html.Node, want string) bool {
	return slices.Contains(strings.Fields(linkTestAttr(n, "class")), want)
}

func linkTestRowAction(t *testing.T, body string) *html.Node {
	t.Helper()
	doc := linkTestDocument(t, body)
	var action *html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" && linkTestAttr(n, "data-table-row") != "" {
			var cells []*html.Node
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && child.Data == "td" {
					cells = append(cells, child)
				}
			}
			if len(cells) > 0 {
				var find func(*html.Node)
				find = func(node *html.Node) {
					if action != nil {
						return
					}
					if node.Type == html.ElementNode && (node.Data == "a" || node.Data == "span") {
						action = node
						return
					}
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						find(child)
					}
				}
				find(cells[len(cells)-1])
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return action
}

func TestTable_RowHrefAcceptsADR0079Destinations(t *testing.T) {
	for _, href := range []string{
		"./details", "../details", "details", "?q=one", "#details", "/référence?q=a&next=2",
		"http://example.test/details", "https://example.test/details?q=a&next=2",
	} {
		t.Run(href, func(t *testing.T) {
			body := render(t, Table(TableParams{
				Columns: linkTestCols,
				Rows:    []Row{{ID: "r1", Cells: []string{`A & <row>`}, Href: href}}, RowHrefs: true,
			}))
			action := linkTestRowAction(t, body)
			if action == nil || action.Data != "a" {
				t.Fatalf("valid row href did not render an action anchor: %s", body)
			}
			if got := linkTestAttr(action, "href"); got != href {
				t.Fatalf("href = %q, want exact %q", got, href)
			}
			for _, class := range []string{"inline-flex", "min-h-11", "min-w-11", "items-center", "justify-center", "focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus", "text-accent"} {
				if !linkTestClassHas(action, class) {
					t.Errorf("valid row link lacks class %q: %s", class, body)
				}
			}
			if !strings.Contains(body, "A &amp; &lt;row&gt;") || strings.Contains(href, "&next=2") && !strings.Contains(body, "&amp;next=2") {
				t.Fatalf("row text or href was not escaped in HTML: %s", body)
			}
		})
	}
}

func TestTable_RowHrefRefusesUnsafeDestinations(t *testing.T) {
	unsafe := []string{
		" javascript:alert(1)", "javascript:alert(1)", "JaVaScRiPt:alert(1)", "java\tscript:alert(1)", "java\nscript:alert(1)",
		"data:text/html,hi", "custom:details", "http:details", "https:///details", "//evil.example/details",
		"/details%ZZ", "/details?q=%ZZ", "/details ", "/details\x7f", "/details\x01", "https://example.test:bad/", `/\\evil`, `\\details`,
	}
	for _, href := range unsafe {
		t.Run(href, func(t *testing.T) {
			body := render(t, Table(TableParams{
				Columns: linkTestCols,
				Rows:    []Row{{ID: "r1", Cells: []string{"A"}, Href: href}}, RowHrefs: true,
			}))
			action := linkTestRowAction(t, body)
			if action == nil || action.Data != "span" || linkTestAttr(action, "role") != "link" || linkTestAttr(action, "aria-disabled") != "true" {
				t.Fatalf("unsafe row href remained actionable: %s", body)
			}
			if !strings.Contains(body, "Open unavailable") || strings.Contains(body, "<button") {
				t.Fatalf("unsafe row fallback lost unavailable action semantics: %s", body)
			}
			for _, attr := range []string{"href", "tabindex", "onclick", "onkeydown", "onkeyup"} {
				if linkTestHasAttr(action, attr) {
					t.Errorf("unsafe fallback emitted %s: %s", attr, body)
				}
			}
			for _, class := range []string{"hover:text-accent-hover", "focus-visible:outline-2", "focus-visible:outline-offset-2", "focus-visible:outline-focus"} {
				if linkTestClassHas(action, class) {
					t.Errorf("unsafe fallback retained interactive class %q: %s", class, body)
				}
			}
		})
	}
}

func TestTable_RowHrefDoesNotChangeEmptyHrefOrRowActionContracts(t *testing.T) {
	plain := render(t, Table(TableParams{Columns: linkTestCols, Rows: []Row{{ID: "r1", Cells: []string{"A"}}}, RowHrefs: true}))
	if strings.Contains(plain, "Open unavailable") || strings.Contains(plain, `href=""`) {
		t.Fatalf("empty Href invented an action: %s", plain)
	}
	custom := render(t, Table(TableParams{
		Columns:   linkTestCols,
		Rows:      []Row{{ID: "r1", Cells: []string{"A"}, Href: "/details"}},
		RowAction: templ.Raw(`<a href="/edit">Edit</a>`), RowHrefs: true,
	}))
	if !strings.Contains(custom, `href="/edit">Edit</a>`) || strings.Contains(custom, "Open unavailable") || strings.Contains(custom, `href="/details"`) {
		t.Fatalf("RowAction was changed or Href leaked around the custom slot: %s", custom)
	}
}
