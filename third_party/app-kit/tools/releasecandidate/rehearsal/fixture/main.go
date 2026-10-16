// Package main is both the embedded standalone consumer and a root-tested fixture.
package main

// This fixture-only poison must never enter CSS: bg-fuchsia-950.

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

func main() {
	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	measured, zero := 12.34, 0.0
	dataset := analytics.Dataset{
		Contract: analytics.Contract{
			ID: "candidate-bars", Title: "Candidate <readiness>", Unit: "items", Grain: "one synthetic category",
			Aggregation: "app-selected fixed observation", Decimals: 1,
			Window:       analytics.Window{Kind: analytics.SnapshotWindow, Start: now.Add(-time.Hour), TimeZone: "UTC"},
			Source:       vizframe.Derived("Frozen synthetic fixture; no real consumer data."),
			Freshness:    analytics.Freshness{UnknownReason: "synthetic source has no observation clock"},
			Completeness: analytics.Partial, CoverageNote: "one deliberately missing row", Uncertainty: "not quantified for synthetic evidence",
		},
		Rows: []analytics.Row{
			{ID: "measured", Label: `Measured <script>alert("x")</script>`, Value: &measured},
			{ID: "zero", Label: "Measured zero", Value: &zero},
			{ID: "unknown", Label: "Unknown value", MissingReason: "not observed in synthetic fixture"},
		},
		Legend: []analytics.LegendItem{{Role: "accent", Label: "Scheduled <work>"}},
	}
	bars, err := analytics.RenderBars(dataset, viz.BarsOptions{W: 1024, LabelW: 512, Max: 20}, now)
	must(err)
	fallback, err := analytics.RenderBars(dataset, viz.BarsOptions{}, now)
	must(err)
	assertFallback(fallback)
	var csvBody bytes.Buffer
	must(analytics.WriteCSV(&csvBody, dataset, now))
	nav := dashboard.RenderNav([]dashboard.NavGroup{{Heading: "Candidate <nav>", Items: []dashboard.NavItem{
		{Href: "/overview", Label: "Overview", Match: dashboard.MatchSubtree},
		{Href: `javascript:alert("x")`, Label: "Unsafe <item>"},
	}}}, "/overview/detail")
	shell := dashboard.RenderShell(dashboard.Shell{
		Title: "Candidate <shell>", Nav: template.HTML(nav), Content: template.HTML(bars),
		SkipLink: true, ContentID: "candidate-content", AriaLabel: "Candidate navigation",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(shell))
	})
	mux.HandleFunc("GET /export.csv", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		_, _ = w.Write(csvBody.Bytes())
	})

	htmlResponse := request(mux, "/dashboard", "text/html; charset=utf-8")
	csvResponse := request(mux, "/export.csv", "text/csv; charset=utf-8")
	assertHTML(string(htmlResponse))
	assertCSV(csvResponse)
	result := map[string]string{
		"html_sha256": fmt.Sprintf("%x", sha256.Sum256(htmlResponse)),
		"csv_sha256":  fmt.Sprintf("%x", sha256.Sum256(csvResponse)),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	must(enc.Encode(result))
}

func request(handler http.Handler, path, contentType string) []byte {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != contentType {
		panic(fmt.Sprintf("unexpected response for %s: status=%d content-type=%q", path, recorder.Code, recorder.Header().Get("Content-Type")))
	}
	return recorder.Body.Bytes()
}

func assertHTML(body string) {
	for _, want := range []string{
		`href="#candidate-content"`, `id="candidate-content"`, `aria-current="page"`,
		`aria-disabled="true"`, "Candidate &lt;shell&gt;", "Candidate &lt;readiness&gt;",
		`Measured &lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`, "<svg", "12.3 items", ">12.3<", ">0<",
		">Missing<", "Source freshness unknown: synthetic source has no observation clock",
		">Meaning<", "Scheduled &lt;work&gt;",
	} {
		if !strings.Contains(body, want) {
			panic("HTML missing expected evidence: " + want)
		}
	}
	for _, forbidden := range []string{"<script>alert", `href="javascript:`, "Candidate <shell>"} {
		if strings.Contains(body, forbidden) {
			panic("HTML contains unsafe/unescaped evidence: " + forbidden)
		}
	}
}

func assertFallback(body string) {
	for _, want := range []string{
		"Bar chart omitted: labels or values exceed this layout's readable space.",
		">12.3<", ">Missing<",
		">Meaning<", "Scheduled &lt;work&gt;",
	} {
		if !strings.Contains(body, want) {
			panic("table-only fallback missing expected evidence: " + want)
		}
	}
	if strings.Contains(body, "<svg") {
		panic("table-only fallback unexpectedly contains an SVG")
	}
}

func assertCSV(body []byte) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	must(err)
	header := -1
	wantHeader := []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}
	for i, row := range records {
		if slices.Equal(row, wantHeader) {
			header = i
			break
		}
	}
	if header < 0 || len(records) != header+4 {
		panic("CSV data section missing or wrong size")
	}
	wants := [][]string{
		{"measured", `Measured <script>alert("x")</script>`, "12.3", "items", "present", "", "", "", "accent", "Scheduled <work>"},
		{"zero", "Measured zero", "0", "items", "present", "", "", "", "accent", "Scheduled <work>"},
		{"unknown", "Unknown value", "", "items", "missing", "not observed in synthetic fixture", "", "", "accent", "Scheduled <work>"},
	}
	for i, want := range wants {
		if !slices.Equal(records[header+1+i], want) {
			panic(fmt.Sprintf("CSV disagreement at row %d: got %q want %q", i, records[header+1+i], want))
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
