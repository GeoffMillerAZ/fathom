package gallery

import (
	"encoding/csv"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
	"golang.org/x/net/html"
)

const analyticsRecipeCSP = "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

type analyticsRecipeSecurityFixture struct {
	journey analyticsJourney
	texts   []string
}

func newAnalyticsRecipeSecurityFixture(t *testing.T, href string, long bool) analyticsRecipeSecurityFixture {
	t.Helper()
	selection := analyticsSelection{
		Period:   analyticsPeriodSeptember,
		Compare:  analyticsCompareNone,
		Category: analyticsCategoryAll,
		State:    analyticsStatePartial,
	}
	j, err := buildAnalyticsJourney(selection)
	if err != nil {
		t.Fatal(err)
	}

	title := `=T<script>x</script>`
	unit := `+u<img>`
	grain := `-g</p>`
	source := `@S<svg onload=x>`
	zeroLabel := `=Z<b>x</b>`
	presentLabel := `@P<i>x</i>`
	missingLabel := `+M<script>x</script>`
	missingReason := `-R</td><script>x</script>`
	accentMeaning := `@A<img src=x>`
	infoMeaning := `=I<script>x</script>`
	if long {
		presentLabel = `@P` + strings.Repeat(" long-label-名称", 24) + `<i>x</i>`
	}
	zero, present := 0.0, 7.0
	j.Current.Contract.Title = title
	j.Current.Contract.Unit = unit
	j.Current.Contract.Grain = grain
	j.Current.Contract.Source = vizframe.Derived(source).Linked(href)
	j.Current.Contract.Completeness = analytics.Partial
	j.Current.Contract.CoverageNote = "Synthetic partial fixture for the composition boundary."
	j.Current.Rows = []analytics.Row{
		{ID: "measured-zero", Label: zeroLabel, Role: "accent", Value: &zero},
		{ID: "measured-present", Label: presentLabel, Role: "info", Value: &present},
		{ID: "missing-value", Label: missingLabel, Role: "info", MissingReason: missingReason},
	}
	j.Current.Legend = []analytics.LegendItem{
		{Role: "accent", Label: accentMeaning},
		{Role: "info", Label: infoMeaning},
	}
	if err := j.Current.Validate(j.Now); err != nil {
		t.Fatal(err)
	}
	return analyticsRecipeSecurityFixture{
		journey: j,
		texts: []string{
			title, unit, grain, source, zeroLabel, presentLabel, missingLabel,
			missingReason, accentMeaning, infoMeaning,
		},
	}
}

func TestAnalyticsRecipeHostileTextAtCompleteHTMLBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, href string
		long       bool
		linked     bool
	}{
		{name: "fitting allowed HTTPS source", href: "https://example.org/report?q=1&view=full", linked: true},
		{name: "fitting rejected javascript source", href: "javascript:alert(1)"},
		{name: "table-only rejected data source", href: "data:text/html,<script>x</script>", long: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAnalyticsRecipeSecurityFixture(t, tc.href, tc.long)
			builder := func(analyticsSelection) (analyticsJourney, error) { return fixture.journey, nil }
			response := analyticsRecipeSecurityRequest(http.MethodGet, analyticsRecipePath, builder)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			assertAnalyticsRecipeResponseHeaders(t, response, "text/html; charset=utf-8")
			doc, err := html.Parse(strings.NewReader(response.Body.String()))
			if err != nil {
				t.Fatal(err)
			}
			text := analyticsRecipeNodeText(doc)
			for _, want := range fixture.texts {
				if !strings.Contains(text, want) {
					t.Errorf("rendered text lost %q", want)
				}
			}
			links := inspectAnalyticsRecipeDOM(t, doc)
			if tc.linked {
				if links[fixture.journey.Current.Contract.Source.Text] != tc.href {
					t.Errorf("trusted HTTPS source link=%q, want %q", links[fixture.journey.Current.Contract.Source.Text], tc.href)
				}
			} else if _, ok := links[fixture.journey.Current.Contract.Source.Text]; ok {
				t.Errorf("unsafe source href %q reached the composed page", tc.href)
			}
			body := response.Body.String()
			if tc.long {
				if !strings.Contains(body, "Bar chart omitted: labels or values exceed") || strings.Contains(body, "data-analytics-legend-swatch") {
					t.Error("long-label lane did not remain an explicit table-only result")
				}
			} else if !strings.Contains(body, "<svg") || !strings.Contains(body, "data-analytics-legend-swatch") {
				t.Error("fitting lane lost its SVG or legend")
			}
		})
	}
}

func TestAnalyticsRecipeExportSecurityBoundary(t *testing.T) {
	fixture := newAnalyticsRecipeSecurityFixture(t, "https://example.org/report", false)
	builder := func(analyticsSelection) (analyticsJourney, error) { return fixture.journey, nil }
	get := analyticsRecipeSecurityRequest(http.MethodGet, analyticsRecipeExportPath, builder)
	if get.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", get.Code, get.Body.String())
	}
	assertAnalyticsRecipeResponseHeaders(t, get, "text/csv; charset=utf-8")
	disposition, params, err := mime.ParseMediaType(get.Header().Get("Content-Disposition"))
	if err != nil {
		t.Fatal(err)
	}
	if disposition != "attachment" || params["filename"] != "service-operations-recipe-2026-09.csv" {
		t.Fatalf("content disposition=%q %#v", disposition, params)
	}

	reader := csv.NewReader(strings.NewReader(get.Body.String()))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	context, rows := parseAnalyticsRecipeCSV(t, records)
	for key, want := range map[string]string{
		"title":       "'" + fixture.journey.Current.Contract.Title,
		"unit":        "'" + fixture.journey.Current.Contract.Unit,
		"grain":       "'" + fixture.journey.Current.Contract.Grain,
		"source":      "'" + fixture.journey.Current.Contract.Source.Text,
		"source_href": fixture.journey.Current.Contract.Source.Href,
	} {
		if context[key] != want {
			t.Errorf("context %s=%q, want %q", key, context[key], want)
		}
	}
	zero := rows["measured-zero"]
	if len(zero) != 10 {
		t.Fatalf("measured-zero row missing: %#v", zero)
	}
	if zero[1] != "'"+fixture.journey.Current.Rows[0].Label || zero[2] != "0" || zero[3] != "'"+fixture.journey.Current.Contract.Unit || zero[4] != "present" || zero[8] != "accent" || zero[9] != "'"+fixture.journey.Current.Legend[0].Label {
		t.Errorf("measured zero changed identity: %#v", zero)
	}
	present := rows["measured-present"]
	if len(present) != 10 {
		t.Fatalf("measured-present row missing: %#v", present)
	}
	if present[1] != "'"+fixture.journey.Current.Rows[1].Label || present[2] != "7" || present[3] != "'"+fixture.journey.Current.Contract.Unit || present[4] != "present" || present[8] != "info" || present[9] != "'"+fixture.journey.Current.Legend[1].Label {
		t.Errorf("present row changed identity: %#v", present)
	}
	missing := rows["missing-value"]
	if len(missing) != 10 {
		t.Fatalf("missing-value row missing: %#v", missing)
	}
	if missing[1] != "'"+fixture.journey.Current.Rows[2].Label || missing[2] != "" || missing[3] != "'"+fixture.journey.Current.Contract.Unit || missing[4] != "missing" || missing[5] != "'"+fixture.journey.Current.Rows[2].MissingReason || missing[8] != "info" || missing[9] != "'"+fixture.journey.Current.Legend[1].Label {
		t.Errorf("missing row changed identity: %#v", missing)
	}

	assertAnalyticsRecipeGETHEADParity(t, analyticsRecipeExportPath, builder)
	assertAnalyticsRecipeGETHEADParity(t, analyticsRecipePath, builder)
}

func TestAnalyticsRecipeRefusalsDoNotBuildAndFailuresDoNotLeak(t *testing.T) {
	called := 0
	never := func(analyticsSelection) (analyticsJourney, error) {
		called++
		return analyticsJourney{}, errors.New("builder must not run")
	}
	for _, tc := range []struct {
		method, target string
		status         int
	}{
		{http.MethodPost, analyticsRecipePath, http.StatusMethodNotAllowed},
		{http.MethodGet, analyticsRecipePath + "?state=imaginary", http.StatusBadRequest},
		{http.MethodGet, analyticsRecipePath + "/extra", http.StatusNotFound},
		{http.MethodGet, "/analytics/%72ecipe", http.StatusNotFound},
	} {
		response := analyticsRecipeSecurityRequest(tc.method, tc.target, never)
		if response.Code != tc.status {
			t.Errorf("%s %s status=%d, want %d", tc.method, tc.target, response.Code, tc.status)
		}
	}
	if called != 0 {
		t.Fatalf("refused requests invoked builder %d times", called)
	}

	const secret = "HOSTILE-BUILDER-SECRET"
	failing := func(analyticsSelection) (analyticsJourney, error) {
		return analyticsJourney{}, errors.New(secret)
	}
	response := analyticsRecipeSecurityRequest(http.MethodGet, analyticsRecipePath, failing)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("builder failure leaked: status=%d body=%q", response.Code, response.Body.String())
	}

	fixture := newAnalyticsRecipeSecurityFixture(t, "https://example.org/report", false)
	fixture.journey.Current.Contract.ID = "invalid id"
	fixture.journey.Current.Contract.Title = secret
	invalid := func(analyticsSelection) (analyticsJourney, error) { return fixture.journey, nil }
	response = analyticsRecipeSecurityRequest(http.MethodGet, analyticsRecipeExportPath, invalid)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), "chart_id") {
		t.Fatalf("export failure leaked data or partial CSV: status=%d body=%q", response.Code, response.Body.String())
	}
}

func analyticsRecipeSecurityRequest(method, target string, builder func(analyticsSelection) (analyticsJourney, error)) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	serveAnalyticsRecipeWithBuilder(w, httptest.NewRequest(method, target, nil), builder)
	return w
}

func assertAnalyticsRecipeResponseHeaders(t *testing.T, response *httptest.ResponseRecorder, contentType string) {
	t.Helper()
	for name, want := range map[string]string{
		"Content-Type":            contentType,
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "no-store",
		"Content-Security-Policy": analyticsRecipeCSP,
		"Content-Length":          strconv.Itoa(response.Body.Len()),
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s=%q, want %q", name, got, want)
		}
	}
}

func assertAnalyticsRecipeGETHEADParity(t *testing.T, target string, builder func(analyticsSelection) (analyticsJourney, error)) {
	t.Helper()
	get := analyticsRecipeSecurityRequest(http.MethodGet, target, builder)
	head := analyticsRecipeSecurityRequest(http.MethodHead, target, builder)
	if get.Code != http.StatusOK || head.Code != get.Code || head.Body.Len() != 0 {
		t.Fatalf("GET/HEAD status or body mismatch: get=%d head=%d head-body=%d", get.Code, head.Code, head.Body.Len())
	}
	if head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Fatalf("HEAD length=%q, want %d", head.Header().Get("Content-Length"), get.Body.Len())
	}
	for _, name := range []string{"Content-Type", "Content-Disposition", "X-Content-Type-Options", "Cache-Control", "Content-Security-Policy"} {
		if head.Header().Get(name) != get.Header().Get(name) {
			t.Errorf("GET/HEAD %s mismatch: %q / %q", name, get.Header().Get(name), head.Header().Get(name))
		}
	}
}

func parseAnalyticsRecipeCSV(t *testing.T, records [][]string) (map[string]string, map[string][]string) {
	t.Helper()
	expectedContext := map[string]bool{
		"chart_id": true, "title": true, "unit": true, "grain": true, "aggregation": true,
		"window_kind": true, "window": true, "source_kind": true, "source": true, "source_href": true,
		"source_draft": true, "evaluated_at": true, "freshness": true, "completeness": true,
		"coverage_note": true, "uncertainty": true, "decimal_places": true,
	}
	expectedHeader := []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}
	expectedRows := map[string]bool{"measured-zero": true, "measured-present": true, "missing-value": true}
	context := map[string]string{}
	rows := map[string][]string{}
	inData := false
	for _, record := range records {
		if !inData {
			if slices.Equal(record, expectedHeader) {
				if len(context) != len(expectedContext) {
					t.Fatalf("data header followed %d context records, want %d", len(context), len(expectedContext))
				}
				inData = true
				continue
			}
			if len(record) != 2 {
				t.Fatalf("invalid context record: %#v", record)
			}
			if !expectedContext[record[0]] {
				t.Fatalf("unknown context key %q", record[0])
			}
			if _, duplicate := context[record[0]]; duplicate {
				t.Fatalf("duplicate context key %q", record[0])
			}
			context[record[0]] = record[1]
			continue
		}
		if len(record) != 10 {
			t.Fatalf("invalid data record: %#v", record)
		}
		if !expectedRows[record[0]] {
			t.Fatalf("unknown data row %q", record[0])
		}
		if _, duplicate := rows[record[0]]; duplicate {
			t.Fatalf("duplicate data row %q", record[0])
		}
		rows[record[0]] = record
	}
	if !inData || len(context) != len(expectedContext) || len(rows) != len(expectedRows) {
		t.Fatalf("unexpected CSV shape: context=%d rows=%d", len(context), len(rows))
	}
	return context, rows
}

func inspectAnalyticsRecipeDOM(t *testing.T, doc *html.Node) map[string]string {
	t.Helper()
	links := map[string]string{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "iframe", "object", "embed", "img":
				t.Errorf("unexpected active/injected element %s", n.Data)
			}
			var href string
			for _, attr := range n.Attr {
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") || attr.Key == "style" {
					t.Errorf("unexpected executable/style attribute %s on %s", attr.Key, n.Data)
				}
				if attr.Key == "href" {
					href = attr.Val
					lower := strings.ToLower(strings.TrimSpace(href))
					if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
						t.Errorf("unsafe href reached page: %q", href)
					}
				}
			}
			if n.Data == "a" && href != "" {
				links[analyticsRecipeNodeText(n)] = href
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return links
}

func analyticsRecipeNodeText(n *html.Node) string {
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
	return b.String()
}
