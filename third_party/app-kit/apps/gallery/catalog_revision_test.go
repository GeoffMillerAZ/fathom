package gallery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"golang.org/x/net/html"
)

func TestCatalogRevisionGeneratedDetailLinksAreGuarded(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	page := catalogHTML(t, New(), "/catalog?q=navigation&profile=go-web&limit=1&theme=dark")
	href := catalogAttribute(catalogArticleLink(page), "href")
	u, err := url.Parse(href)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("expected_revision") != current.Revision {
		t.Fatalf("generated detail link revision = %q, want %q: %s", u.Query().Get("expected_revision"), current.Revision, href)
	}
}

func TestCatalogRevisionCurrentAndLegacyExactResponsesMatch(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := New()
	_, legacyJSON := get(t, h, "/api/catalog/shell.layout")
	for _, suffix := range []string{"?expected_revision=", "?expected_revision=" + current.Revision} {
		res, guarded := get(t, h, "/api/catalog/shell.layout"+suffix)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/catalog/shell.layout%s = %d: %s", suffix, res.StatusCode, guarded)
		}
		if guarded != legacyJSON {
			t.Errorf("GET /api/catalog/shell.layout%s changed exact JSON bytes", suffix)
		}
	}

	_, legacyHTML := get(t, h, "/catalog/shell.layout")
	wantMain := catalogRevisionMain(t, legacyHTML)
	for _, suffix := range []string{"?expected_revision=", "?expected_revision=" + current.Revision} {
		res, guarded := get(t, h, "/catalog/shell.layout"+suffix)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /catalog/shell.layout%s = %d: %s", suffix, res.StatusCode, guarded)
		}
		if got := catalogRevisionMain(t, guarded); got != wantMain {
			t.Errorf("GET /catalog/shell.layout%s changed capability <main> bytes", suffix)
		}
	}

	guardedPath := "/catalog/shell.layout?expected_revision=" + current.Revision + "&profile=go-web&q=navigation"
	guarded := catalogHTML(t, h, guardedPath)
	themeGroup := catalogFind(guarded, func(node *html.Node) bool {
		return node.Data == "div" && catalogAttribute(node, "role") == "group" && catalogAttribute(node, "aria-label") == "Theme"
	})
	links := catalogRevisionLinks(themeGroup)
	if len(links) != 3 {
		t.Fatalf("native theme handoff has %d links, want 3", len(links))
	}
	wantTheme := map[string]string{"light": "light", "dark": "dark", "OS": ""}
	for _, link := range links {
		u, err := url.Parse(catalogAttribute(link, "href"))
		if err != nil {
			t.Fatal(err)
		}
		label := strings.TrimSpace(catalogNodeText(link))
		theme, known := wantTheme[label]
		if !known || u.Path != "/catalog/shell.layout" || u.Query().Get("theme") != theme || u.Query().Get("expected_revision") != current.Revision || u.Query().Get("profile") != "go-web" || u.Query().Get("q") != "navigation" {
			t.Errorf("theme %q lost exact guarded native handoff: %s", label, u.String())
		}
	}
	for _, path := range []string{"/catalog/shell.layout", "/catalog/shell.layout?expected_revision="} {
		links := catalogRevisionLinks(catalogFind(catalogHTML(t, h, path), func(node *html.Node) bool {
			return node.Data == "div" && catalogAttribute(node, "role") == "group" && catalogAttribute(node, "aria-label") == "Theme"
		}))
		if len(links) != 3 {
			t.Fatalf("latest-mode native theme handoff has %d links, want 3", len(links))
		}
		for _, link := range links {
			u, err := url.Parse(catalogAttribute(link, "href"))
			if err != nil {
				t.Fatal(err)
			}
			if u.Query().Get("expected_revision") != "" {
				t.Errorf("latest-mode theme handoff unexpectedly acquired a revision: %s", u.String())
			}
		}
	}
}

func catalogRevisionMain(t *testing.T, body string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	main := catalogFind(doc, func(node *html.Node) bool { return node.Data == "main" })
	if main == nil {
		t.Fatal("catalog response has no main landmark")
	}
	var out strings.Builder
	if err := html.Render(&out, main); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func catalogRevisionLinks(root *html.Node) []*html.Node {
	var links []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			links = append(links, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return links
}

func TestCatalogRevisionStaleGuardRefusesCapabilityContent(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := current.Get("analytics.comparison")
	if !ok {
		t.Fatal("fixture capability missing")
	}
	stale := strings.Repeat("0", 64)
	if stale == current.Revision {
		stale = strings.Repeat("1", 64)
	}
	for _, base := range []string{"/catalog/analytics.comparison", "/api/catalog/analytics.comparison"} {
		res, body := get(t, New(), base+"?expected_revision="+stale)
		if res.StatusCode != http.StatusConflict {
			t.Fatalf("GET %s = %d, want 409: %s", base, res.StatusCode, body)
		}
		for _, forbidden := range []string{entry.Name, entry.Source.Path, `"entry"`, `"capability_revision"`} {
			if forbidden != "" && strings.Contains(body, forbidden) {
				t.Errorf("stale %s leaked capability content %q", base, forbidden)
			}
		}
	}
}

func TestCatalogRevisionInvalidGuardsAreBadRequests(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	bad := []string{
		"expected_revision=bad",
		"expected_revision=" + strings.ToUpper(current.Revision),
		"expected_revision=" + strings.Repeat("g", 64),
		"expected_revision=" + current.Revision + "&expected_revision=" + current.Revision,
	}
	for _, base := range []string{"/catalog/shell.layout", "/api/catalog/shell.layout"} {
		for _, query := range bad {
			res, body := get(t, New(), base+"?"+query)
			if res.StatusCode != http.StatusBadRequest {
				t.Errorf("GET %s?%s = %d, want 400: %s", base, query, res.StatusCode, body)
			}
		}

		req := httptest.NewRequest(http.MethodGet, base, nil)
		req.URL.RawQuery = "expected_revision=%zz"
		rec := httptest.NewRecorder()
		New().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s?expected_revision=%%zz = %d, want 400: %s", base, rec.Code, rec.Body.String())
		}
	}
}

func TestCatalogRevisionExactRouteStatusBoundaries(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := New()
	for _, base := range []string{"/catalog/notreal", "/api/catalog/notreal"} {
		res, _ := get(t, h, base+"?expected_revision="+current.Revision)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s with valid guard = %d, want 404", base, res.StatusCode)
		}
	}
	for _, path := range []string{"/catalog/shell.layout", "/api/catalog/shell.layout"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", path, rec.Code)
		}
	}
}

func TestCatalogRevisionStaleRecoveryAndNormalBack(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("0", 64)
	if stale == current.Revision {
		stale = strings.Repeat("1", 64)
	}
	h := New()
	recoveryValues := url.Values{
		"kind":     {"primitive"},
		"layer":    {"L3b"},
		"limit":    {"1"},
		"maturity": {"available"},
		"profile":  {"go-web"},
		"q":        {"navigation"},
		"theme":    {"dark"},
	}
	staleValues := url.Values{}
	for key, values := range recoveryValues {
		staleValues[key] = append([]string(nil), values...)
	}
	staleValues.Set("cursor", "obsolete")
	staleValues.Set("expected_revision", stale)
	res, staleBody := get(t, h, "/catalog/shell.layout?"+staleValues.Encode())
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale detail = %d, want 409: %s", res.StatusCode, staleBody)
	}
	wantRecovery := "/catalog?" + recoveryValues.Encode()
	if !strings.Contains(staleBody, `href="`+strings.ReplaceAll(wantRecovery, "&", "&amp;")+`"`) {
		t.Fatalf("stale recovery did not preserve filters/theme and drop cursor/guard: %s", staleBody)
	}

	list := catalogHTML(t, h, wantRecovery)
	detailHref := catalogAttribute(catalogArticleLink(list), "href")
	detailURL, err := url.Parse(detailHref)
	if err != nil {
		t.Fatal(err)
	}
	if detailURL.Query().Get("expected_revision") != current.Revision {
		t.Fatalf("recovered list detail is not current-revision guarded: %s", detailHref)
	}
	detailRes, _ := get(t, h, detailHref)
	if detailRes.StatusCode != http.StatusOK {
		t.Fatalf("recovered guarded detail = %d", detailRes.StatusCode)
	}

	page, err := current.Search(catalog.Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal("catalog fixture must provide a valid pagination cursor")
	}
	detailValues := url.Values{"cursor": {page.NextCursor}, "limit": {"1"}, "theme": {"dark"}, "expected_revision": {current.Revision}}
	detail := catalogHTML(t, h, "/catalog/"+url.PathEscape(page.Items[0].ID)+"?"+detailValues.Encode())
	back := catalogAttribute(catalogTextElement(detail, "a", "← Capability catalog"), "href")
	wantBack := "/catalog?" + url.Values{"cursor": {page.NextCursor}, "limit": {"1"}, "theme": {"dark"}}.Encode()
	if back != wantBack {
		t.Errorf("normal Back = %q, want %q", back, wantBack)
	}
}

func TestCatalogRevisionInvalidContextRecoversToCurrentCatalog(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("0", 64)
	if stale == current.Revision {
		stale = strings.Repeat("1", 64)
	}
	h := New()
	for _, bad := range []struct{ key, value string }{
		{"profile", "not-a-profile"}, {"layer", "not-a-layer"},
		{"kind", "not-a-kind"}, {"maturity", "not-a-maturity"},
		{"limit", "0"}, {"limit", "51"}, {"limit", "abc"},
	} {
		t.Run(bad.key+"="+bad.value, func(t *testing.T) {
			q := url.Values{"q": {"analytics"}, "theme": {"dark"}, "expected_revision": {stale}, "cursor": {"obsolete"}}
			q.Set(bad.key, bad.value)
			res, body := get(t, h, "/catalog/analytics.comparison?"+q.Encode())
			if res.StatusCode != http.StatusConflict {
				t.Fatalf("stale selection status = %d", res.StatusCode)
			}
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			link := catalogTextElement(doc, "a", "Review current catalog results")
			href := catalogAttribute(link, "href")
			if href != "/catalog?theme=dark" {
				t.Errorf("invalid selection must reset filters together: %s", href)
			}
			if !strings.Contains(body, "Filters have been cleared") {
				t.Error("recovery must explain the filter reset")
			}
			recovered, _ := get(t, h, href)
			if recovered.StatusCode != http.StatusOK {
				t.Errorf("recovery = %d, want 200", recovered.StatusCode)
			}
		})
	}
}
