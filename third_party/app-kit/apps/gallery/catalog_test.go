package gallery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"golang.org/x/net/html"
)

func TestCatalogPagesAndRecovery(t *testing.T) {
	h := New()
	for _, tc := range []struct {
		path   string
		status int
		wants  []string
	}{
		{"/", 200, []string{"What are you building?", "Task or capability", "Runtime profile", "Adoption layer", "Choose the design reference", "private reference library"}},
		{"/catalog?q=navigation&profile=go-web&theme=dark", 200, []string{`data-theme="dark"`, `value="navigation"`, `name="theme" value="dark"`, "q=navigation", `/catalog?theme=dark`}},
		{"/catalog?q=definitelynotpresent", 200, []string{"No compatible capability found", "Clear filters and explore"}},
		{"/catalog?profile=cli&layer=L4", 200, []string{"0 matching capabilities", "No compatible capability found"}},
		{"/catalog?profile=not-a-profile", 400, []string{"This catalog request needs a reset", "Clear filters and return"}},
		{"/catalog?cursor=bad", 400, []string{"This catalog request needs a reset", "cursor"}},
		{"/catalog?limit=0", 400, []string{"limit must be an integer"}},
		{"/catalog/shell.layout?q=navigation&profile=go-web&theme=dark", 200, []string{`data-theme="dark"`, "Your application owns", "Known"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, body := get(t, h, tc.path)
			if res.StatusCode != tc.status {
				t.Fatalf("status %d: %s", res.StatusCode, body)
			}
			for _, want := range tc.wants {
				if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Count(body, "<main ") != 1 {
				t.Error("catalog must have one main landmark")
			}
			if strings.Contains(body, "#ZgotmplZ") {
				t.Error("template rejected a navigation URL")
			}
		})
	}
	_, detail := get(t, h, "/catalog/shell.layout?q=navigation&profile=go-web&theme=dark")
	for _, want := range []string{`href="/catalog?profile=go-web&amp;q=navigation&amp;theme=dark"`, `href="/dashboard?theme=dark"`} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail loses navigation context: %q", want)
		}
	}
	for _, path := range []string{"/catalog/notreal", "/api/catalog/notreal", "/forms/notreal"} {
		res, _ := get(t, h, path)
		if res.StatusCode != 404 {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
}

func TestCatalogJSONProtocol(t *testing.T) {
	h := New()
	want, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	res, body := get(t, h, "/api/catalog")
	var got catalog.Catalog
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Revision != want.Revision || len(got.Entries) != len(want.Entries) {
		t.Fatal("JSON is not the shared catalog")
	}
	if res.Header.Get("ETag") != `"`+want.Revision+`"` {
		t.Fatal("missing catalog ETag")
	}
	req := httptest.NewRequest("GET", "/api/catalog", nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 304 || rec.Body.Len() != 0 {
		t.Fatal("conditional inventory must have no duplicate body")
	}
	_, pageBody := get(t, h, "/api/catalog/search?limit=1")
	var page catalog.Page
	if err := json.Unmarshal([]byte(pageBody), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal("bounded search must paginate")
	}
	_, nextBody := get(t, h, "/api/catalog/search?limit=1&cursor="+url.QueryEscape(page.NextCursor))
	var next catalog.Page
	if err := json.Unmarshal([]byte(nextBody), &next); err != nil {
		t.Fatal(err)
	}
	if next.Items[0].ID == page.Items[0].ID {
		t.Fatal("pagination repeats first entry")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/catalog", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("catalog is read-only, got POST %d", rec.Code)
	}
}

func TestCatalogEscapesSearchInput(t *testing.T) {
	_, body := get(t, New(), "/catalog?q="+url.QueryEscape(`<script>alert("test")</script>`))
	if strings.Contains(body, "<script>") {
		t.Fatal("search input became active markup")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("escaped search input must remain editable")
	}
}

func TestCatalogRenderedControlTargets(t *testing.T) {
	h := New()
	page := catalogHTML(t, h, "/catalog?limit=1")
	for _, target := range []struct {
		name  string
		node  *html.Node
		class []string
	}{
		{"query", catalogElement(page, "input", "id", "catalog-query"), []string{"min-h-11"}},
		{"profile", catalogElement(page, "select", "id", "catalog-profile"), []string{"min-h-11"}},
		{"layer", catalogElement(page, "select", "id", "catalog-layer"), []string{"min-h-11"}},
		{"find", catalogTextElement(page, "button", "Find capabilities"), []string{"min-h-11"}},
		{"reset", catalogTextElement(page, "a", "Reset filters"), []string{"inline-flex", "min-h-11", "items-center"}},
		{"result", catalogArticleLink(page), []string{"inline-flex", "min-h-11", "items-center"}},
		{"next", catalogTextElement(page, "a", "Next page →"), []string{"inline-flex", "min-h-11", "items-center"}},
		{"summary", catalogTextElement(page, "summary", "Catalog revision and machine-readable inventory"), []string{"min-h-11", "py-3"}},
		{"snapshot", catalogTextElement(page, "a", "Read the shared JSON snapshot"), []string{"inline-flex", "min-h-11", "items-center"}},
	} {
		catalogTargetClasses(t, target.name, target.node, target.class...)
	}
	detail := catalogHTML(t, h, "/catalog/shell.layout")
	catalogTargetClasses(t, "provenance summary", catalogTextElement(detail, "summary", "Provenance and revision"), "min-h-11", "py-3")
	for _, summary := range []struct {
		name string
		node *html.Node
	}{
		{"catalog summary", catalogTextElement(page, "summary", "Catalog revision and machine-readable inventory")},
		{"provenance summary", catalogTextElement(detail, "summary", "Provenance and revision")},
	} {
		if summary.node != nil && (catalogClass(summary.node, "inline-flex") || catalogClass(summary.node, "flex")) {
			t.Errorf("%s must retain its native marker/list-item layout", summary.name)
		}
	}

	for _, tc := range []struct {
		path, label, text string
	}{
		{"/catalog?q=definitelynotpresent", "empty reset", "Clear filters and explore the catalog"},
		{"/catalog?profile=not-a-profile", "error reset", "Clear filters and return to the catalog"},
		{"/catalog/shell.layout", "back", "← Capability catalog"},
		{"/catalog/shell.layout", "preview", "Open live specimen →"},
	} {
		catalogTargetClasses(t, tc.label, catalogTextElement(catalogHTML(t, h, tc.path), "a", tc.text), "inline-flex", "min-h-11", "items-center")
	}
}

func catalogHTML(t *testing.T, h http.Handler, path string) *html.Node {
	t.Helper()
	res, body := get(t, h, path)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusBadRequest {
		t.Fatalf("%s: status %d", path, res.StatusCode)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func catalogElement(root *html.Node, tag, key, value string) *html.Node {
	return catalogFind(root, func(node *html.Node) bool { return node.Data == tag && catalogAttribute(node, key) == value })
}

func catalogTextElement(root *html.Node, tag, want string) *html.Node {
	return catalogFind(root, func(node *html.Node) bool {
		return node.Data == tag && strings.TrimSpace(catalogNodeText(node)) == want
	})
}

func catalogArticleLink(root *html.Node) *html.Node {
	article := catalogFind(root, func(node *html.Node) bool { return node.Data == "article" })
	if article == nil {
		return nil
	}
	return catalogFind(article, func(node *html.Node) bool { return node.Data == "a" })
}

func catalogFind(root *html.Node, match func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && match(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := catalogFind(child, match); found != nil {
			return found
		}
	}
	return nil
}

func catalogAttribute(node *html.Node, key string) string {
	if node != nil {
		for _, attribute := range node.Attr {
			if attribute.Key == key {
				return attribute.Val
			}
		}
	}
	return ""
}

func catalogNodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
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
	return text.String()
}

func catalogTargetClasses(t *testing.T, name string, node *html.Node, classes ...string) {
	t.Helper()
	if node == nil {
		t.Errorf("%s control is absent", name)
		return
	}
	for _, class := range classes {
		if !catalogClass(node, class) {
			t.Errorf("%s control lacks %q", name, class)
		}
	}
}

func catalogClass(node *html.Node, want string) bool {
	return strings.Contains(" "+catalogAttribute(node, "class")+" ", " "+want+" ")
}
