package gallery

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func emptyAnalyticsQuery(period analyticsPeriod, compare analyticsComparison, state analyticsState, theme string) string {
	q := url.Values{
		"period":   {string(period)},
		"compare":  {string(compare)},
		"category": {"empty"},
		"state":    {string(state)},
	}
	if theme != "" {
		q.Set("theme", theme)
	}
	return q.Encode()
}

func emptyCSVRecords(t *testing.T, body []byte) [][]string {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func emptyRecipeHasPlottedSVG(t *testing.T, body string) bool {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var visit func(*html.Node, bool)
	visit = func(n *html.Node, inFrame bool) {
		if n.Type == html.ElementNode && n.Data == "figure" {
			inFrame = false
			for _, attr := range n.Attr {
				if attr.Key == "class" && slices.Contains(strings.Fields(attr.Val), "ak-vizframe") {
					inFrame = true
				}
			}
		}
		if inFrame && n.Type == html.ElementNode && n.Data == "svg" {
			found = true
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child, inFrame)
		}
	}
	visit(doc, false)
	return found
}

func assertEmptyRendererCSV(t *testing.T, body []byte, period analyticsPeriod, state analyticsState) {
	t.Helper()
	records := emptyCSVRecords(t, body)
	header := -1
	context := map[string]string{}
	for i, record := range records {
		if len(record) == 2 {
			context[record[0]] = record[1]
		}
		if len(record) == 10 && record[0] == "id" {
			header = i
		}
	}
	if header != 17 || len(records) != 18 {
		t.Fatalf("renderer CSV shape = %d records, header=%d; want 18/17 (encoding/csv omits blank lines)", len(records), header)
	}
	if !strings.Contains(context["source"], "category=empty") || !strings.Contains(context["source"], "state="+string(state)) || !strings.Contains(context["window"], string(period)) {
		t.Fatalf("renderer CSV lost empty context: %#v", context)
	}
}

func assertEmptyGETHEAD(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	get := analyticsRequest(h, http.MethodGet, target)
	head := analyticsRequest(h, http.MethodHead, target)
	if get.Code != http.StatusOK || head.Code != get.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Fatalf("GET/HEAD mismatch for %s: get=%d/%d head=%d/%d length=%q", target, get.Code, get.Body.Len(), head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}
	return get
}

func TestAnalyticsEmptyCategoryOverviewAndExports(t *testing.T) {
	h := analyticsRouteHandler()
	periods := []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember}
	compares := []analyticsComparison{analyticsCompareNone, analyticsComparePrevious}
	states := []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial}
	for _, period := range periods {
		for _, compare := range compares {
			for _, state := range states {
				for _, theme := range []string{"light", "dark"} {
					name := strings.Join([]string{string(period), string(compare), string(state), theme}, "/")
					t.Run(name, func(t *testing.T) {
						selection := analyticsSelection{Period: period, Compare: compare, Category: analyticsCategory("empty"), State: state}
						j, err := buildAnalyticsJourney(selection)
						if err != nil {
							t.Fatal(err)
						}
						if len(j.Current.Rows) != 0 || j.Previous == nil != (compare == analyticsCompareNone) || j.Previous != nil && len(j.Previous.Rows) != 0 {
							t.Fatalf("empty datasets retained rows: current=%d previous=%v", len(j.Current.Rows), j.Previous != nil && len(j.Previous.Rows) > 0)
						}
						lowerNarrative := strings.ToLower(j.Narrative)
						if !strings.Contains(j.Narrative, "No matching services") || !strings.Contains(lowerNarrative, "not zero") || strings.Contains(lowerNarrative, "total") || strings.Contains(lowerNarrative, "difference") {
							t.Fatalf("empty narrative invents a result: %q", j.Narrative)
						}

						query := emptyAnalyticsQuery(period, compare, state, theme)
						page := assertEmptyGETHEAD(t, h, "/analytics?"+query)
						body := page.Body.String()
						for _, forbidden := range []string{`id="analytics-value-table"`, `data-service-id=`, "shared scale 0 to"} {
							if strings.Contains(body, forbidden) {
								t.Fatalf("empty overview rendered %q: %s", forbidden, body)
							}
						}
						if !strings.Contains(body, `value="empty"`) || !strings.Contains(body, "No matching services") || !strings.Contains(body, "Show all service groups") || !strings.Contains(body, "Export CSV (headers only)") || !strings.Contains(body, "Export current-period context CSV") || !strings.Contains(strings.ToLower(body), "not zero") {
							t.Fatalf("empty overview lost its explicit empty narrative: %s", body)
						}
						if strings.Contains(body, "Load the complete fixture") {
							t.Fatal("empty category offered a state-only complete-fixture link")
						}
						allHref := analyticsURL("/analytics", analyticsSelection{Period: period, Compare: compare, Category: analyticsCategoryAll, State: state}, theme)
						if !analyticsHTMLLinks(t, body)[allHref] {
							t.Fatalf("empty overview lacks preserving All services recovery link %q", allHref)
						}
						recipeSelection := selection
						recipeSelection.Compare = analyticsCompareNone
						if !analyticsHTMLLinks(t, body)[analyticsURL(analyticsRecipeExportPath, recipeSelection, theme)] {
							t.Fatalf("empty overview lacks current-period context export link")
						}

						detail := analyticsRequest(h, http.MethodGet, "/analytics/gateway-api?"+query)
						if detail.Code != http.StatusNotFound || !analyticsHTMLLinks(t, detail.Body.String())[analyticsURL("/analytics", selection, theme)] {
							t.Fatalf("empty detail did not retain empty-selection recovery: %d %s", detail.Code, detail.Body.String())
						}
						export := analyticsRequest(h, http.MethodGet, "/analytics/export.csv?"+query)
						records := emptyCSVRecords(t, export.Body.Bytes())
						if export.Code != http.StatusOK || len(records) != 1 {
							t.Fatalf("empty overview CSV = status %d, %d records; want 200, header only, 33 columns", export.Code, len(records))
						}
						if len(records[0]) != 33 {
							t.Fatalf("empty overview CSV has %d columns; want 33", len(records[0]))
						}
					})
				}
			}
		}
	}
}

func TestAnalyticsEmptyCategoryRecipeAndCSVContracts(t *testing.T) {
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, compare := range []analyticsComparison{analyticsCompareNone, analyticsComparePrevious} {
			for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
				for _, theme := range []string{"light", "dark"} {
					name := strings.Join([]string{string(period), string(compare), string(state), theme}, "/")
					t.Run(name, func(t *testing.T) {
						query := emptyAnalyticsQuery(period, compare, state, theme)
						page := analyticsRecipeRequest(http.MethodGet, analyticsRecipePath+"?"+query)
						head := analyticsRecipeRequest(http.MethodHead, analyticsRecipePath+"?"+query)
						if page.Code != http.StatusOK || head.Code != page.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(page.Body.Len()) {
							t.Fatalf("recipe GET/HEAD mismatch: %d/%d, %d/%d", page.Code, page.Body.Len(), head.Code, head.Body.Len())
						}
						body := page.Body.String()
						for _, want := range []string{`value="empty"`, `name="category"`, `compare=none`, "No measured values in this selection", "No matching services", "Show all service groups"} {
							if !strings.Contains(body, want) {
								t.Fatalf("empty recipe missing %q: %s", want, body)
							}
						}
						if emptyRecipeHasPlottedSVG(t, body) || strings.Contains(body, "data-value-id=") {
							t.Fatalf("empty recipe rendered measured values: %s", body)
						}
						recipeSelection := analyticsSelection{Period: period, Compare: analyticsCompareNone, Category: analyticsCategory("empty"), State: state}
						links := analyticsHTMLLinks(t, body)
						if !links[analyticsURL(analyticsRecipeExportPath, recipeSelection, theme)] || !links[analyticsURL("/analytics", recipeSelection, theme)] {
							t.Fatalf("empty recipe lost canonical context links: %v", links)
						}

						renderer := analyticsRecipeRequest(http.MethodGet, analyticsRecipeExportPath+"?"+query)
						if renderer.Code != http.StatusOK {
							t.Fatalf("renderer CSV status=%d", renderer.Code)
						}
						assertEmptyRendererCSV(t, renderer.Body.Bytes(), period, state)
						table := analyticsRecipeRequest(http.MethodGet, analyticsRecipeTableExportPath+"?"+query)
						tableRecords := emptyCSVRecords(t, table.Body.Bytes())
						if table.Code != http.StatusOK || len(tableRecords) != 1 {
							t.Fatalf("spreadsheet CSV = status %d, %d records; want 200, header only, 27 columns", table.Code, len(tableRecords))
						}
						if len(tableRecords[0]) != 27 {
							t.Fatalf("spreadsheet CSV has %d columns; want 27", len(tableRecords[0]))
						}
					})
				}
			}
		}
	}
}
