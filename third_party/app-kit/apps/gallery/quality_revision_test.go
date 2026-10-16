package gallery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/qualityplan"
	"golang.org/x/net/html"
)

func TestQualityCatalogRevisionCurrentAndLegacyJSONMatch(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	list, err := plan.List("analytics.comparison", "all")
	if err != nil {
		t.Fatal(err)
	}

	h := New()
	for _, path := range []string{
		"/api/quality?scope=all&capability_id=analytics.comparison",
		"/api/quality/core.analytics?scope=all&capability_id=analytics.comparison&expected_revision=" + list.Revision,
	} {
		res, legacy := get(t, h, path)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("legacy GET %s = %d: %s", path, res.StatusCode, legacy)
		}
		separator := "&"
		for _, guard := range []string{"", current.Revision} {
			res, guarded := get(t, h, path+separator+"expected_catalog_revision="+guard)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("guarded GET %s = %d: %s", path, res.StatusCode, guarded)
			}
			if guarded != legacy {
				t.Errorf("expected_catalog_revision=%q changed exact JSON bytes for %s", guard, path)
			}
		}
	}
}

func TestQualityCatalogRevisionRefusesStaleWithoutPayload(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	list, err := plan.List("analytics.comparison", "all")
	if err != nil {
		t.Fatal(err)
	}
	item, err := plan.Get("core.analytics", list.Revision)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("0", 64)
	if stale == current.Revision {
		stale = strings.Repeat("1", 64)
	}

	h := New()
	for _, base := range []string{"/quality", "/api/quality"} {
		res, body := get(t, h, base+"?scope=all&capability_id=analytics.comparison&expected_catalog_revision="+stale)
		qualityRevisionError(t, res, body, http.StatusConflict, item.Item.Task)
	}
	for _, base := range []string{"/quality/core.analytics", "/api/quality/core.analytics"} {
		q := url.Values{
			"scope":                     {"all"},
			"capability_id":             {"analytics.comparison"},
			"expected_revision":         {list.Revision},
			"expected_catalog_revision": {stale},
		}
		res, body := get(t, h, base+"?"+q.Encode())
		qualityRevisionError(t, res, body, http.StatusConflict, item.Item.Task)
	}
}

func TestQualityCatalogRevisionInvalidGuardsFailBeforeRecords(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	bad := []string{
		"expected_catalog_revision=bad",
		"expected_catalog_revision=" + strings.ToUpper(current.Revision),
		"expected_catalog_revision=" + strings.Repeat("g", 64),
		"expected_catalog_revision=" + current.Revision + "&expected_catalog_revision=" + current.Revision,
	}
	h := New()
	for _, base := range []string{"/quality", "/api/quality", "/quality/core.analytics", "/api/quality/core.analytics"} {
		for _, query := range bad {
			res, body := get(t, h, base+"?"+query)
			qualityRevisionError(t, res, body, http.StatusBadRequest, "Seven enhancement standards")
		}

		req := httptest.NewRequest(http.MethodGet, base, nil)
		req.URL.RawQuery = "expected_catalog_revision=%zz"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		qualityRevisionError(t, rec.Result(), rec.Body.String(), http.StatusBadRequest, "Seven enhancement standards")
	}
}

func TestQualityCatalogRevisionLinksPreserveBothIdentities(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := New()

	entry := qualityRevisionHTML(t, h, "/catalog/analytics.comparison?expected_revision="+current.Revision+"&theme=dark", http.StatusOK)
	qualityLink := catalogTextElement(entry, "a", "Browse recorded quality work for this capability")
	qualityURL := qualityRevisionURL(t, catalogAttribute(qualityLink, "href"))
	if qualityURL.Path != "/quality" || qualityURL.Query().Get("capability_id") != "analytics.comparison" || qualityURL.Query().Get("theme") != "dark" || qualityURL.Query().Get("expected_catalog_revision") != current.Revision {
		t.Fatalf("capability handoff lost catalog identity: %s", qualityURL.String())
	}

	list := qualityRevisionHTML(t, h, qualityURL.String()+"&scope=all", http.StatusOK)
	if text := catalogNodeText(list); !strings.Contains(text, "Inventory revision: ") || !strings.Contains(text, "Catalog revision: "+current.Revision) {
		t.Error("guarded list does not expose both inventory and catalog identities")
	}
	form := catalogElement(list, "form", "action", "/quality")
	hidden := catalogFind(form, func(node *html.Node) bool {
		return node.Data == "input" && catalogAttribute(node, "type") == "hidden" && catalogAttribute(node, "name") == "expected_catalog_revision"
	})
	if got := catalogAttribute(hidden, "value"); got != current.Revision {
		t.Errorf("filter form catalog revision = %q, want %q", got, current.Revision)
	}
	detailURL := qualityRevisionURL(t, catalogAttribute(catalogArticleLink(list), "href"))
	if detailURL.Query().Get("expected_revision") == "" || detailURL.Query().Get("expected_catalog_revision") != current.Revision || detailURL.Query().Get("capability_id") != "analytics.comparison" || detailURL.Query().Get("scope") != "all" || detailURL.Query().Get("theme") != "dark" {
		t.Fatalf("generated detail link lost guarded context: %s", detailURL.String())
	}
	resetURL := qualityRevisionURL(t, catalogAttribute(catalogTextElement(list, "a", "Reset filters"), "href"))
	if resetURL.Path != "/quality" || resetURL.Query().Get("expected_catalog_revision") != current.Revision || resetURL.Query().Get("theme") != "dark" || resetURL.Query().Get("scope") != "" || resetURL.Query().Get("capability_id") != "" {
		t.Errorf("deliberate reset did not keep only theme/catalog identity: %s", resetURL.String())
	}
	qualityRevisionThemesKeep(t, list, current.Revision, "")

	detail := qualityRevisionHTML(t, h, detailURL.String(), http.StatusOK)
	if text := catalogNodeText(detail); !strings.Contains(text, "Inventory revision: "+detailURL.Query().Get("expected_revision")) || !strings.Contains(text, "Catalog revision: "+current.Revision) {
		t.Error("guarded detail does not expose both inventory and catalog identities")
	}
	backURL := qualityRevisionURL(t, catalogAttribute(catalogTextElement(detail, "a", "← Quality work"), "href"))
	if backURL.Query().Get("expected_catalog_revision") != current.Revision || backURL.Query().Get("expected_revision") != "" || backURL.Query().Get("capability_id") != "analytics.comparison" || backURL.Query().Get("scope") != "all" {
		t.Errorf("Back link retained the wrong identities: %s", backURL.String())
	}
	qualityRevisionThemesKeep(t, detail, current.Revision, detailURL.Query().Get("expected_revision"))
}

func TestQualityCatalogRevisionRecoveryAndExistingBoundaries(t *testing.T) {
	current, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	list, err := plan.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("0", 64)
	if stale == current.Revision {
		stale = strings.Repeat("1", 64)
	}
	h := New()

	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/quality?capability_id=analytics.comparison&theme=dark&expected_catalog_revision=" + stale, http.StatusConflict},
		{"/quality/core.analytics?capability_id=analytics.comparison&theme=dark&expected_revision=" + list.Revision + "&expected_catalog_revision=" + stale, http.StatusConflict},
		{"/quality?capability_id=analytics.comparison&theme=dark&expected_catalog_revision=bad", http.StatusBadRequest},
		{"/quality?capability_id=analytics.comparison&theme=dark&expected_catalog_revision=" + current.Revision + "&expected_catalog_revision=" + current.Revision, http.StatusBadRequest},
	} {
		doc := qualityRevisionHTML(t, h, tc.path, tc.status)
		text := catalogNodeText(doc)
		if !strings.Contains(text, "This quality-work request needs a reset") || !strings.Contains(text, "application") || !strings.Contains(text, "pins") || !strings.Contains(text, "not changed") {
			t.Error("catalog guard error omits the unchanged application/pins boundary")
		}
		recovery := qualityRevisionURL(t, catalogAttribute(catalogTextElement(doc, "a", "Review current capability catalog"), "href"))
		if recovery.Path != "/catalog" || recovery.Query().Get("q") != "analytics.comparison" || recovery.Query().Get("theme") != "dark" || recovery.Query().Get("expected_catalog_revision") != "" {
			t.Errorf("guard failure bypassed fresh capability review: %s", recovery.String())
		}
	}

	badEncoding := httptest.NewRequest(http.MethodGet, "/quality", nil)
	badEncoding.URL.RawQuery = "capability_id=analytics.comparison&theme=dark&expected_catalog_revision=%zz"
	badEncodingRec := httptest.NewRecorder()
	h.ServeHTTP(badEncodingRec, badEncoding)
	qualityRevisionError(t, badEncodingRec.Result(), badEncodingRec.Body.String(), http.StatusBadRequest, "Seven enhancement standards")
	badEncodingDoc, err := html.Parse(strings.NewReader(badEncodingRec.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogAttribute(catalogTextElement(badEncodingDoc, "a", "Review current capability catalog"), "href"); got != "/catalog?q=analytics.comparison&theme=dark" {
		t.Errorf("malformed guard recovery = %q, want /catalog?q=analytics.comparison&theme=dark", got)
	}

	for _, tc := range []struct {
		name, suffix, reset string
	}{
		{"omitted", "", "/quality?theme=dark"},
		{"empty", "&expected_catalog_revision=", "/quality?theme=dark"},
		{"current", "&expected_catalog_revision=" + current.Revision, "/quality?expected_catalog_revision=" + current.Revision + "&theme=dark"},
	} {
		t.Run("generic-invalid-"+tc.name, func(t *testing.T) {
			query := "scope=missing&theme=dark" + tc.suffix
			res, body := get(t, h, "/quality?"+query)
			qualityRevisionError(t, res, body, http.StatusBadRequest, "Seven enhancement standards")
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			if got := catalogAttribute(catalogTextElement(doc, "a", "Clear filters and return to quality work"), "href"); got != tc.reset {
				t.Errorf("generic invalid-request reset = %q, want %q", got, tc.reset)
			}
			if strings.Contains(catalogNodeText(doc), "application and adoption pins have not changed") || catalogTextElement(doc, "a", "Review current capability catalog") != nil {
				t.Error("generic invalid request incorrectly requires capability re-review")
			}

			apiRes, apiBody := get(t, h, "/api/quality?"+query)
			qualityRevisionError(t, apiRes, apiBody, http.StatusBadRequest, "Seven enhancement standards")
		})
	}

	res, body := get(t, h, "/quality/core.analytics?expected_revision="+strings.Repeat("0", 64)+"&expected_catalog_revision="+current.Revision)
	qualityRevisionError(t, res, body, http.StatusBadRequest, "Seven enhancement standards")
	for _, base := range []string{"/quality/not-real", "/api/quality/not-real"} {
		res, _ := get(t, h, base+"?expected_catalog_revision="+current.Revision)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("guarded unknown GET %s = %d, want 404", base, res.StatusCode)
		}
	}
	for _, path := range []string{"/quality", "/api/quality", "/quality/core.analytics", "/api/quality/core.analytics"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", path, rec.Code)
		}
		if rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("POST %s Allow = %q, want GET, HEAD", path, rec.Header().Get("Allow"))
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("POST %s Cache-Control = %q, want no-store", path, rec.Header().Get("Cache-Control"))
		}
	}
}

func qualityRevisionError(t *testing.T, res *http.Response, body string, status int, forbidden string) {
	t.Helper()
	if res.StatusCode != status {
		t.Errorf("status = %d, want %d: %s", res.StatusCode, status, body)
	}
	if !strings.Contains(res.Header.Get("Cache-Control"), "no-store") {
		t.Errorf("error Cache-Control = %q, want no-store", res.Header.Get("Cache-Control"))
	}
	for _, text := range []string{forbidden, `"items"`, `"item"`, `"standards"`} {
		if text != "" && strings.Contains(body, text) {
			t.Errorf("error response leaked quality payload %q", text)
		}
	}
}

func qualityRevisionHTML(t *testing.T, h http.Handler, path string, status int) *html.Node {
	t.Helper()
	res, body := get(t, h, path)
	if res.StatusCode != status {
		t.Fatalf("GET %s = %d, want %d: %s", path, res.StatusCode, status, body)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func qualityRevisionURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	if raw == "" {
		t.Fatal("expected link is missing")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func qualityRevisionThemesKeep(t *testing.T, doc *html.Node, catalogRevision, inventoryRevision string) {
	t.Helper()
	group := catalogFind(doc, func(node *html.Node) bool {
		return node.Data == "div" && catalogAttribute(node, "role") == "group" && catalogAttribute(node, "aria-label") == "Theme"
	})
	links := catalogRevisionLinks(group)
	if len(links) != 3 {
		t.Fatalf("theme group has %d links, want 3", len(links))
	}
	for _, link := range links {
		u := qualityRevisionURL(t, catalogAttribute(link, "href"))
		if u.Query().Get("expected_catalog_revision") != catalogRevision || u.Query().Get("expected_revision") != inventoryRevision {
			t.Errorf("theme link lost supplied guards: %s", u.String())
		}
	}
}
