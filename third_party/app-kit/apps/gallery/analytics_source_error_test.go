package gallery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const analyticsSourceErrorState analyticsState = "error"

func analyticsSourceErrorQuery(category analyticsCategory) string {
	return url.Values{
		"period":   {string(analyticsPeriodSeptember)},
		"compare":  {string(analyticsComparePrevious)},
		"category": {string(category)},
		"state":    {string(analyticsSourceErrorState)},
		"theme":    {"dark"},
		"probe":    {`raw-query-canary<script>`},
	}.Encode()
}

func analyticsSourceErrorSection(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var section *html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "section" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "analytics-source-error" {
					section = n
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if section == nil {
		t.Fatalf("missing #analytics-source-error: %s", body)
	}
	return section
}

func assertAnalyticsSourceErrorResponse(t *testing.T, get, head *httptest.ResponseRecorder, category analyticsCategory, comparison, recovery string) {
	t.Helper()
	if get.Code != http.StatusServiceUnavailable || head.Code != get.Code || head.Body.Len() != 0 {
		t.Fatalf("source error GET/HEAD = %d/%d body=%d", get.Code, head.Code, head.Body.Len())
	}
	for name, want := range map[string]string{
		"Content-Type":           "text/html; charset=utf-8",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Content-Disposition":    "",
	} {
		if got := get.Header().Get(name); got != want || head.Header().Get(name) != got {
			t.Errorf("%s GET/HEAD = %q/%q, want %q", name, got, head.Header().Get(name), want)
		}
	}
	if csp := get.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || head.Header().Get("Content-Security-Policy") != csp {
		t.Errorf("source error lost CSP: %q", csp)
	}
	if get.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Fatal("source error Content-Length disagrees with GET body")
	}
	body := get.Body.String()
	section := analyticsSourceErrorSection(t, body)
	sectionText := analyticsNodeText(section)
	var heading string
	var findHeading func(*html.Node)
	findHeading = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h1" {
			heading = analyticsNodeText(n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			findHeading(child)
		}
	}
	findHeading(section)
	lower := strings.ToLower(sectionText)
	if heading != "The data source could not be read" ||
		!strings.Contains(lower, "synthetic source failure") ||
		!strings.Contains(lower, "not zero") ||
		!strings.Contains(lower, "not an empty result") ||
		!strings.Contains(sectionText, "September 2026") ||
		!strings.Contains(lower, strings.ToLower(analyticsCategoryLabel(category))) ||
		!strings.Contains(sectionText, comparison) ||
		!strings.Contains(sectionText, "Load the complete fixture") {
		t.Fatalf("source error explanation/context incomplete: %q", sectionText)
	}
	links := analyticsHTMLLinks(t, body)
	if !links[recovery] {
		t.Fatalf("source error lacks exact recovery %q: %v", recovery, links)
	}
	if !strings.Contains(body, `data-theme="dark"`) {
		t.Fatal("source error did not apply the preserved dark theme")
	}
	if emptyRecipeHasPlottedSVG(t, body) {
		t.Fatal("source error rendered an analytics figure")
	}
	for href := range links {
		if strings.Contains(href, "export.csv") || strings.HasPrefix(href, "/analytics/gateway-api?") {
			t.Fatalf("source error exposed data/detail link %q", href)
		}
	}
	for _, forbidden := range []string{
		"raw-query-canary", `id="analytics-value-table"`, "data-service-id=", "data-value-id=",
		"No matching services", "headers only", "Export CSV", "Export service CSV", "observed failed operations",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("source error rendered forbidden %q", forbidden)
		}
	}
}

func TestAnalyticsSourceErrorSelectionRefusesJourneyValues(t *testing.T) {
	selection, err := parseAnalyticsSelection(url.Values{
		"period": {"2026-09"}, "compare": {"previous"}, "category": {"all"}, "state": {"error"},
	})
	if err != nil || selection.State != analyticsSourceErrorState {
		t.Fatalf("source-error selection = %+v, %v", selection, err)
	}
	journey, err := buildAnalyticsJourney(selection)
	if err == nil {
		t.Fatal("source-error selection built values")
	}
	if journey.Selection != (analyticsSelection{}) || journey.Previous != nil || !journey.Now.IsZero() ||
		journey.Narrative != "" || len(journey.Current.Rows) != 0 || len(journey.Details) != 0 || journey.Current.Contract.ID != "" {
		t.Fatalf("refused source-error journey leaked state or values: %+v", journey)
	}
}

func TestAnalyticsSourceErrorRoutesFailClosedWithRecoveryContext(t *testing.T) {
	h := analyticsRouteHandler()
	routes := []struct {
		name, path, recovery string
		recipe               bool
	}{
		{"overview", "/analytics", "/analytics", false},
		{"detail", "/analytics/gateway-api", "/analytics", false},
		{"overview export", "/analytics/export.csv", "/analytics", false},
		{"detail export", "/analytics/gateway-api/export.csv", "/analytics", false},
		{"recipe", analyticsRecipePath, analyticsRecipePath, true},
		{"recipe export", analyticsRecipeExportPath, analyticsRecipePath, true},
		{"table export", analyticsRecipeTableExportPath, analyticsRecipePath, true},
	}
	for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker, analyticsCategoryEmpty} {
		for _, route := range routes {
			t.Run(string(category)+"/"+route.name, func(t *testing.T) {
				query := analyticsSourceErrorQuery(category)
				get := analyticsRequest(h, http.MethodGet, route.path+"?"+query)
				head := analyticsRequest(h, http.MethodHead, route.path+"?"+query)
				recoverySelection := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, category, analyticsStateComplete}
				comparison := "Previous calendar month"
				if route.recipe {
					recoverySelection.Compare = analyticsCompareNone
					comparison = "No comparison"
				}
				recovery := analyticsURL(route.recovery, recoverySelection, "dark")
				assertAnalyticsSourceErrorResponse(t, get, head, category, comparison, recovery)
				if recovered := analyticsRequest(h, http.MethodGet, recovery); recovered.Code != http.StatusOK {
					t.Fatalf("complete-fixture recovery returned %d", recovered.Code)
				}
			})
		}
	}
}

func TestAnalyticsRecipeSourceErrorDoesNotCallBuilder(t *testing.T) {
	valid, err := buildAnalyticsJourney(analyticsSelection{analyticsPeriodSeptember, analyticsCompareNone, analyticsCategoryAll, analyticsStateComplete})
	if err != nil || len(valid.Current.Rows) == 0 {
		t.Fatal("control fixture is not populated")
	}
	for _, path := range []string{analyticsRecipePath, analyticsRecipeExportPath, analyticsRecipeTableExportPath} {
		called := false
		builder := func(analyticsSelection) (analyticsJourney, error) {
			called = true
			return valid, nil
		}
		response := analyticsRecipeSecurityRequest(http.MethodGet, path+"?"+analyticsSourceErrorQuery(analyticsCategoryAll), builder)
		if called || response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s called poisoned builder=%v or returned %d", path, called, response.Code)
		}
	}
}

func TestAnalyticsSourceErrorKeepsRequestPrecedenceAndControls(t *testing.T) {
	h := analyticsRouteHandler()
	if got := analyticsRequest(h, http.MethodGet, "/analytics?state=bogus"); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid query status=%d", got.Code)
	}
	if got := analyticsRequest(h, http.MethodGet, "/analytics/Gateway-API?"+analyticsSourceErrorQuery(analyticsCategoryAll)); got.Code != http.StatusNotFound {
		t.Fatalf("malformed path status=%d", got.Code)
	}
	if got := analyticsRequest(h, http.MethodPost, "/analytics?"+analyticsSourceErrorQuery(analyticsCategoryAll)); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("invalid method status=%d", got.Code)
	}
	zero := analyticsSelection{analyticsPeriodSeptember, analyticsCompareNone, analyticsCategoryWorker, analyticsStateComplete}
	zeroPage := analyticsRequest(h, http.MethodGet, analyticsURL("/analytics", zero, ""))
	if zeroPage.Code != http.StatusOK || !strings.Contains(zeroPage.Body.String(), ">0</td>") || !strings.Contains(zeroPage.Body.String(), "<svg") {
		t.Fatal("measured worker zero control changed")
	}
	empty := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryEmpty, analyticsStateComplete}
	emptyCSV := analyticsRequest(h, http.MethodGet, analyticsURL("/analytics/export.csv", empty, ""))
	if emptyCSV.Code != http.StatusOK || emptyCSV.Header().Get("Content-Disposition") == "" || len(emptyCSVRecords(t, emptyCSV.Body.Bytes())) != 1 {
		t.Fatal("valid empty-selection header-only control changed")
	}
}
