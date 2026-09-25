package gallery

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"golang.org/x/net/html"
)

func analyticsRouteHandler() http.Handler {
	mux := http.NewServeMux()
	registerAnalytics(mux)
	return mux
}

func analyticsRequest(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func analyticsHTMLLinks(t *testing.T, body string) map[string]bool {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]bool{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					links[attr.Val] = true
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return links
}

func analyticsNodeText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func assertAnalyticsRenderedValues(t *testing.T, body string, j analyticsJourney, selected []analyticsDetail) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	tables, charts := map[string]*html.Node{}, map[string]*html.Node{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		for _, a := range n.Attr {
			var nodes map[string]*html.Node
			if a.Key == "data-value-id" {
				nodes = tables
			} else if a.Key == "data-service-id" {
				nodes = charts
			}
			if nodes != nil {
				if nodes[a.Val] != nil {
					t.Fatalf("duplicate rendered service identity %s", a.Val)
				}
				nodes[a.Val] = n
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if len(tables) != len(selected) || len(charts) != len(selected) {
		t.Fatal("table/chart retained rows outside the selection or omitted rows")
	}
	number := func(v *float64) string {
		if v == nil {
			return "Missing"
		}
		return strconv.FormatFloat(*v, 'f', 0, 64)
	}
	for _, d := range selected {
		table, chart := tables[d.ID], charts[d.ID]
		if table == nil || chart == nil {
			t.Fatalf("table/chart lost %s", d.ID)
		}
		var cells []string
		for child := table.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && (child.Data == "th" || child.Data == "td") {
				cells = append(cells, analyticsNodeText(child))
			}
		}
		count := number(d.Current.Value)
		if d.Current.Value != nil && j.Selection.State == analyticsStatePartial {
			count = "At least " + count
		}
		wantColumns := 3
		if d.Previous != nil {
			wantColumns = 5
		}
		if len(cells) != wantColumns || cells[0] != d.Label || cells[1] != count {
			t.Fatalf("table disagrees with selected current row %s: %v", d.ID, cells)
		}
		if d.Previous != nil {
			if cells[2] != number(d.Previous.Value) {
				t.Fatal("previous period table count differs from export selection")
			}
			if (j.Selection.State == analyticsStateMissing || j.Selection.State == analyticsStatePartial) && cells[3] != "Not comparable" {
				t.Fatal("incomplete observations acquired a complete-period difference")
			}
		}
		var svgCount int
		var svgTexts []string
		var inspect func(*html.Node, bool)
		inspect = func(n *html.Node, inSVG bool) {
			if n.Type == html.ElementNode && n.Data == "svg" {
				svgCount++
				inSVG = true
			}
			if inSVG && n.Type == html.ElementNode && n.Data == "text" {
				svgTexts = append(svgTexts, analyticsNodeText(n))
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				inspect(child, inSVG)
			}
		}
		inspect(chart, false)
		if d.Current.Value == nil {
			if svgCount != 0 || !strings.Contains(cells[len(cells)-1], "Missing") {
				t.Fatal("missing observation became a plotted zero/present state")
			}
			continue
		}
		label := j.Current.Contract.Window.Start.Format("Jan 2006")
		if j.Selection.State == analyticsStatePartial {
			label = "First 20 days"
		}
		if svgCount != 1 || !slices.Contains(svgTexts, label) || !slices.Contains(svgTexts, number(d.Current.Value)) {
			t.Fatalf("chart lost current period/value for %s: %v", d.ID, svgTexts)
		}
		if d.Previous != nil {
			previousLabel := j.Previous.Contract.Window.Start.Format("Jan 2006")
			plotted := slices.Contains(svgTexts, previousLabel)
			if plotted != (j.Selection.State != analyticsStatePartial) {
				t.Fatal("chart compared mismatched full/partial periods")
			}
			if plotted && !slices.Contains(svgTexts, number(d.Previous.Value)) {
				t.Fatal("chart previous-period value differs from table/export")
			}
		}
	}
}

func analyticsCSVRead(t *testing.T, body []byte) (map[string]int, [][]string) {
	t.Helper()
	// Default fixed-width parsing must work for ordinary spreadsheet imports.
	records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatalf("not a rectangular CSV with data: %v", err)
	}
	columns := map[string]int{}
	for i, name := range records[0] {
		if _, exists := columns[name]; exists {
			t.Fatalf("duplicate CSV column %q", name)
		}
		columns[name] = i
	}
	if len(columns) != 33 {
		t.Fatalf("got %d CSV columns, want six selection/series + 17 context + ten data", len(columns))
	}
	return columns, records[1:]
}

func assertAnalyticsCSVAgreement(t *testing.T, body []byte, j analyticsJourney, serviceID string) {
	t.Helper()
	columns, rows := analyticsCSVRead(t, body)
	type section struct {
		name string
		data analytics.Dataset
	}
	sections := []section{{"current", j.Current}}
	if j.Previous != nil {
		sections = append(sections, section{"previous", *j.Previous})
	}
	index := 0
	for _, section := range sections {
		var shared bytes.Buffer
		if err := analytics.WriteCSV(&shared, section.data, j.Now); err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(&shared)
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		context := records[:17]
		dataHeader, dataRows := records[17], records[18:]
		for _, sharedRow := range dataRows {
			if index >= len(rows) {
				t.Fatal("comparison export omitted a service or series")
			}
			got := rows[index]
			for name, want := range map[string]string{
				"series": section.name, "selection_period": string(j.Selection.Period),
				"selection_compare": string(j.Selection.Compare), "selection_category": string(j.Selection.Category),
				"selection_state": string(j.Selection.State), "selection_service": serviceID,
			} {
				if got[columns[name]] != want {
					t.Fatalf("row %d %s=%q; want %q", index, name, got[columns[name]], want)
				}
			}
			for _, pair := range context {
				if got[columns["context_"+pair[0]]] != pair[1] {
					t.Fatalf("row %d lost shared context %s", index, pair[0])
				}
			}
			for i, name := range dataHeader {
				if got[columns[name]] != sharedRow[i] {
					t.Fatalf("row %d changed shared %s cell", index, name)
				}
			}
			index++
		}
	}
	if index != len(rows) {
		t.Fatal("comparison export added rows outside the shared selection")
	}
}

func TestAnalyticsRoutesAll48Selections(t *testing.T) {
	h := analyticsRouteHandler()
	count := 0
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, compare := range []analyticsComparison{analyticsCompareNone, analyticsComparePrevious} {
			for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker} {
				for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
					s := analyticsSelection{period, compare, category, state}
					theme := []string{"", "light", "dark"}[count%3]
					count++
					t.Run(s.Values().Encode(), func(t *testing.T) {
						j, err := buildAnalyticsJourney(s)
						if err != nil {
							t.Fatal(err)
						}
						page := analyticsRequest(h, "GET", analyticsURL("/analytics", s, theme))
						if page.Code != 200 {
							t.Fatalf("overview status %d", page.Code)
						}
						body := page.Body.String()
						if !strings.Contains(body, j.Narrative) {
							t.Fatal("overview lost the selected deterministic explanation")
						}
						assertAnalyticsRenderedValues(t, body, j, j.Details)
						links := analyticsHTMLLinks(t, body)
						exportURL := analyticsURL("/analytics/export.csv", s, theme)
						if !links[exportURL] {
							t.Fatal("overview export link lost selection/theme")
						}
						export := analyticsRequest(h, "GET", exportURL)
						if export.Code != 200 {
							t.Fatalf("CSV status %d", export.Code)
						}
						assertAnalyticsCSVAgreement(t, export.Body.Bytes(), j, "")
						for _, detail := range j.Details {
							detailURL := analyticsURL("/analytics/"+detail.ID, s, theme)
							if !links[detailURL] {
								t.Fatal("overview drill-down link lost selection/theme")
							}
							detailPage := analyticsRequest(h, "GET", detailURL)
							if detailPage.Code != 200 || !strings.Contains(detailPage.Body.String(), detail.Explanation) {
								t.Fatal("detail does not explain the selected service")
							}
							assertAnalyticsRenderedValues(t, detailPage.Body.String(), j, []analyticsDetail{detail})
							detailLinks := analyticsHTMLLinks(t, detailPage.Body.String())
							detailExportURL := analyticsURL("/analytics/"+detail.ID+"/export.csv", s, theme)
							if !detailLinks[analyticsURL("/analytics", s, theme)] || !detailLinks[detailExportURL] {
								t.Fatal("detail back/export link lost its exact service or selection")
							}
							detailCSV := analyticsRequest(h, "GET", detailExportURL)
							if detailCSV.Code != 200 {
								t.Fatalf("detail CSV status %d", detailCSV.Code)
							}
							assertAnalyticsCSVAgreement(t, detailCSV.Body.Bytes(), analyticsServiceExport(j, detail), detail.ID)
						}
					})
				}
			}
		}
	}
	if count != 48 {
		t.Fatalf("selection denominator=%d, want 48", count)
	}
}

func TestAnalyticsCSVExactValuesAndTruthStates(t *testing.T) {
	h := analyticsRouteHandler()
	for _, test := range []struct {
		query, current, previous, start, end, coverage string
	}{
		{"", "4,1,0", "6,3,0", "2026-09-01", "2026-10-01", "complete"},
		{"period=2026-08", "6,3,0", "8,5,2", "2026-08-01", "2026-09-01", "complete"},
		{"state=partial", "3,1,0", "6,3,0", "2026-09-01", "2026-10-01", "partial"},
		{"state=missing", ",,", "6,3,0", "2026-09-01", "2026-10-01", "unknown"},
		{"state=stale", "4,1,0", "6,3,0", "2026-09-01", "2026-10-01", "complete"},
	} {
		t.Run(test.query, func(t *testing.T) {
			response := analyticsRequest(h, "GET", "/analytics/export.csv?"+test.query)
			columns, rows := analyticsCSVRead(t, response.Body.Bytes())
			if len(rows) != 6 {
				t.Fatal("full comparison must export exactly six service/series rows")
			}
			for i, want := range append(strings.Split(test.current, ","), strings.Split(test.previous, ",")...) {
				row := rows[i]
				if row[columns["value"]] != want || row[columns["unit"]] != "failed operations" {
					t.Fatalf("row %d value/unit changed", i)
				}
				if i < 3 {
					window := row[columns["context_window"]]
					if !strings.Contains(window, test.start) || !strings.Contains(window, test.end) || row[columns["context_completeness"]] != test.coverage {
						t.Fatal("current period/coverage was lost")
					}
					if (want == "") != (row[columns["state"]] == "missing") || (want == "") != (row[columns["missing_reason"]] != "") {
						t.Fatal("missing data was converted to zero or lost its reason")
					}
				}
			}
		})
	}
}

func TestAnalyticsRouteThemesHeadAndRecovery(t *testing.T) {
	h := analyticsRouteHandler()
	s := analyticsSelection{analyticsPeriodAugust, analyticsComparePrevious, analyticsCategoryWorker, analyticsStatePartial}
	for _, theme := range []string{"", "light", "dark"} {
		for _, path := range []string{"/analytics", "/analytics/report-worker", "/analytics/export.csv", "/analytics/report-worker/export.csv"} {
			target := analyticsURL(path, s, theme)
			get := analyticsRequest(h, "GET", target)
			head := analyticsRequest(h, "HEAD", target)
			if get.Code != 200 || head.Code != get.Code || head.Body.Len() != 0 {
				t.Fatalf("GET/HEAD mismatch for %s: %d/%d", target, get.Code, head.Code)
			}
			for _, key := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Content-Security-Policy", "X-Content-Type-Options"} {
				if head.Header().Get(key) != get.Header().Get(key) {
					t.Fatalf("HEAD changed %s", key)
				}
			}
			if get.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(get.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Fatal("analytics response lost conservative browser headers")
			}
			if strings.HasSuffix(path, ".csv") {
				continue
			}
			links := analyticsHTMLLinks(t, get.Body.String())
			for _, choice := range []string{"", "light", "dark"} {
				if !links[analyticsURL(path, s, choice)] {
					t.Fatalf("theme link lost detail/selection context: %s", path)
				}
			}
			if theme != "" && !strings.Contains(get.Body.String(), "data-theme=\""+theme+"\"") {
				t.Fatal("selected theme not applied to the shared shell")
			}
		}
	}
	for _, path := range []string{"/analytics/unknown", "/analytics/gateway-api", "/analytics/gateway-api/export.csv"} {
		target := analyticsURL(path, s, "dark")
		get, head := analyticsRequest(h, "GET", target), analyticsRequest(h, "HEAD", target)
		if get.Code != 404 || head.Code != 404 || head.Body.Len() != 0 {
			t.Fatal("unknown/filtered detail must be 404, including HEAD/export")
		}
		back := analyticsURL("/analytics", s, "dark")
		if !analyticsHTMLLinks(t, get.Body.String())[back] || analyticsRequest(h, "GET", back).Code != 200 {
			t.Fatal("not-found recovery lost valid selection/theme")
		}
	}
}

func TestAnalyticsRouteInvalidQueryMethodsAndPaths(t *testing.T) {
	h := analyticsRouteHandler()
	queries := []string{"period=", "period=2026-07", "compare=yes", "category=API", "state=zero", "theme=system", "theme=LIGHT", "period=%zz", "period=x;y=z", "unused=%ff", "unused=%00", "state=%3Cscript%3Ereview-canary%3C/script%3E"}
	for field, valid := range map[string]string{"period": "2026-09", "compare": "previous", "category": "all", "state": "complete", "theme": "dark"} {
		queries = append(queries, field+"="+valid+"&"+field+"="+valid)
	}
	queries = append(queries, "theme=&theme=", "period=2026-09&%70eriod=2026-09")
	for _, query := range queries {
		for _, path := range []string{"/analytics", "/analytics/gateway-api", "/analytics/export.csv", "/analytics/gateway-api/export.csv"} {
			get, head := analyticsRequest(h, "GET", path+"?"+query), analyticsRequest(h, "HEAD", path+"?"+query)
			if get.Code != 400 || head.Code != 400 || head.Body.Len() != 0 || strings.Contains(get.Body.String(), "review-canary") {
				t.Fatalf("invalid query %q was accepted/reflected: %d/%d", query, get.Code, head.Code)
			}
			defaults, _ := parseAnalyticsSelection(nil)
			if !analyticsHTMLLinks(t, get.Body.String())[analyticsURL("/analytics", defaults, "")] {
				t.Fatal("invalid selection has no safe reset link")
			}
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		for _, path := range []string{"/analytics", "/analytics/gateway-api", "/analytics/export.csv", "/analytics/gateway-api/export.csv"} {
			w := analyticsRequest(h, method, path)
			if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("%s unexpectedly accepted by %s", method, path)
			}
		}
	}
	for _, path := range []string{"/analytics/", "/analytics/gateway-api/extra", "/analytics/gateway-api/", "/analytics/%2e%2e", "/analytics/%2fetc", "/analytics/gateway%2dapi", "/analytics/gateway-api%2fexport.csv", "/analytics/%5csecret", "/analytics/Gateway-API"} {
		if w := analyticsRequest(h, "GET", path); w.Code != 404 {
			t.Fatalf("unsafe/non-exact detail %s returned %d", path, w.Code)
		}
	}
	for path, canonical := range map[string]string{"/analytics//gateway-api": "/analytics/gateway-api", "/analytics/./gateway-api": "/analytics/gateway-api", "/analytics/../tokens": "/tokens"} {
		w := analyticsRequest(h, "GET", path)
		if (w.Code != http.StatusMovedPermanently && w.Code != http.StatusTemporaryRedirect) || w.Header().Get("Location") != canonical {
			t.Fatalf("ServeMux canonical redirect changed: %s = %d %s", path, w.Code, w.Header().Get("Location"))
		}
	}
	base := analyticsRequest(h, "GET", "/analytics")
	for _, query := range []string{"theme=", "ignored=one&ignored=two"} {
		if w := analyticsRequest(h, "GET", "/analytics?"+query); w.Code != 200 || !bytes.Equal(base.Body.Bytes(), w.Body.Bytes()) {
			t.Fatal("empty system theme or unowned query changed canonical output")
		}
	}
	atLimit := "ignored=" + strings.Repeat("x", analyticsMaxQueryBytes-len("ignored="))
	accepted := analyticsRequest(h, "GET", "/analytics?"+atLimit)
	if accepted.Code != 200 || !bytes.Equal(base.Body.Bytes(), accepted.Body.Bytes()) {
		t.Fatal("bounded ignored parameters leaked into rendered links or content")
	}
	if w := analyticsRequest(h, "GET", "/analytics?"+atLimit+"x"); w.Code != 400 {
		t.Fatal("query above 16 KiB was not rejected")
	}
}

func TestAnalyticsCSVProjectionSafetyDriftAndNoMutation(t *testing.T) {
	j := analyticsTestJourney(t, "")
	before, _ := json.Marshal(j)
	narrow := analyticsServiceExport(j, j.Details[0])
	after, _ := json.Marshal(j)
	if !bytes.Equal(before, after) || len(narrow.Current.Rows) != 1 || narrow.Previous == nil || len(narrow.Previous.Rows) != 1 {
		t.Fatal("single-service export changed the original journey or retained other rows")
	}
	if !strings.Contains(narrow.Current.Contract.Source.Text, "Detail service=gateway-api") || !strings.HasSuffix(narrow.Current.Contract.ID, "-gateway-api") {
		t.Fatal("detail export lost its narrowed source identity")
	}
	j.Current.Rows[0].Label = "=formula-canary"
	j.Current.Contract.Source.Text = "@source-canary"
	body, err := buildAnalyticsCSV(j, "")
	if err != nil {
		t.Fatal(err)
	}
	columns, rows := analyticsCSVRead(t, body)
	if rows[0][columns["label"]] != "'=formula-canary" || rows[0][columns["context_source"]] != "'@source-canary" {
		t.Fatal("flattening removed the shared spreadsheet formula protection")
	}
	var shared bytes.Buffer
	if err := analytics.WriteCSV(&shared, j.Current, j.Now); err != nil {
		t.Fatal(err)
	}
	for name, invalid := range map[string][]byte{
		"context key":   bytes.Replace(shared.Bytes(), []byte("chart_id,"), []byte("renamed_id,"), 1),
		"data header":   bytes.Replace(shared.Bytes(), []byte("id,label,value,unit,state,missing_reason,lower,upper"), []byte("id,label,renamed,unit,state,missing_reason,lower,upper"), 1),
		"extra row":     append(slices.Clone(shared.Bytes()), []byte("extra,row\n")...),
		"row width":     bytes.Replace(shared.Bytes(), []byte("present,,,,accent,API services\n"), []byte("present,,,accent,API services\n"), 1),
		"malformed CSV": []byte("\"unterminated"),
	} {
		if _, _, err := analyticsCSVSection(invalid, len(j.Current.Rows)); err == nil {
			t.Errorf("silently accepted shared CSV drift: %s", name)
		}
	}
	for _, theme := range []string{"", "light", "dark", "\"><script>"} {
		u, err := url.Parse(analyticsURL("/analytics", j.Selection, theme))
		if err != nil || len(u.Query()) < 4 || len(u.Query()) > 5 || u.Query().Get("theme") != sanitizeTheme(theme) {
			t.Fatalf("unsafe/noncanonical helper URL: %v", u)
		}
	}
}

func TestAnalyticsGalleryRegistrationAndActiveNav(t *testing.T) {
	h := New() // No licensed source, external service, JS or alternate runtime.
	s := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAPI, analyticsStateStale}
	target := analyticsURL("/analytics/search-api", s, "dark")
	w := analyticsRequest(h, "GET", target)
	if w.Code != 200 {
		t.Fatalf("Gallery did not register analytics: %d", w.Code)
	}
	doc, err := html.Parse(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	navCount := 0
	for _, navigation := range analyticsTargetNodes(doc, "nav") {
		if analyticsTargetAttr(navigation, "aria-label") != "Gallery sections" {
			continue
		}
		navCount++
		var active []string
		for _, link := range analyticsTargetNodes(navigation, "a") {
			if analyticsTargetAttr(link, "aria-current") == "page" {
				active = append(active, analyticsTargetAttr(link, "href"))
			}
		}
		if !slices.Equal(active, []string{analyticsURL("/analytics", s, "dark")}) {
			t.Fatalf("navigation %d lost the report selection: %v", navCount, active)
		}
	}
	if navCount != 2 {
		t.Fatalf("desktop/mobile navigation count = %d, want 2", navCount)
	}
	if !analyticsHTMLLinks(t, w.Body.String())["/catalog?theme=dark"] {
		t.Fatal("leaving analytics lost the theme or leaked report filters")
	}
}
