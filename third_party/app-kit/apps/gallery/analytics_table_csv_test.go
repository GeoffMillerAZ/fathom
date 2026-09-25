package gallery

import (
	"bytes"
	"encoding/csv"
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
)

func TestAnalyticsTableCSVSelectionAndDownload(t *testing.T) {
	handler := New()
	for _, period := range []string{"2026-08", "2026-09"} {
		for _, group := range []string{"all", "api", "worker"} {
			for _, state := range []string{"complete", "missing", "partial", "stale"} {
				t.Run(period+"/"+group+"/"+state, func(t *testing.T) {
					query := "?period=" + period + "&category=" + group + "&state=" + state + "&compare=previous&theme=dark"
					path := analyticsRecipeTableExportPath + query
					get := httptest.NewRecorder()
					handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
					assertAnalyticsRecipeResponseHeaders(t, get, "text/csv; charset=utf-8")
					if get.Header().Get("Content-Disposition") != `attachment; filename="service-operations-table-`+period+`.csv"` {
						t.Fatal(get.Header())
					}
					records, err := csv.NewReader(bytes.NewReader(get.Body.Bytes())).ReadAll()
					if err != nil || len(records) < 2 || len(records[0]) != 27 {
						t.Fatalf("rectangular export: %v %v", err, records)
					}
					for _, row := range records[1:] {
						if !strings.Contains(row[8], "comparison=none") {
							t.Fatalf("noncanonical comparison context: %q", row[8])
						}
						if state == "missing" && (row[19] != "" || row[21] != "missing" || row[22] == "") {
							t.Fatalf("missing is not zero: %v", row)
						}
						if group == "worker" && state != "missing" && row[19] != "0" {
							t.Fatalf("measured zero lost: %v", row)
						}
					}
					selection, _ := parseAnalyticsSelection(httptest.NewRequest("GET", path, nil).URL.Query())
					selection.Compare = analyticsCompareNone
					journey, err := buildAnalyticsJourney(selection)
					if err != nil {
						t.Fatal(err)
					}
					var want bytes.Buffer
					if err := analytics.WriteTableCSV(&want, analyticsRecipeDataset(journey, "dark"), journey.Now); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(want.Bytes(), get.Body.Bytes()) {
						t.Fatal("route differs from public writer")
					}
					head := httptest.NewRecorder()
					handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
					if head.Code != get.Code || head.Body.Len() != 0 {
						t.Fatal("HEAD body/status mismatch")
					}
					for key, values := range get.Header() {
						if strings.Join(head.Header().Values(key), ",") != strings.Join(values, ",") {
							t.Fatalf("HEAD header %s differs", key)
						}
					}
					page := analyticsRecipeRequest("GET", analyticsRecipePath+query)
					link := html.EscapeString(analyticsURL(analyticsRecipeTableExportPath, selection, "dark"))
					if !strings.Contains(page.Body.String(), `href="`+link+`" aria-describedby="analytics-table-export-help"`) || !strings.Contains(page.Body.String(), "Export renderer CSV") || !strings.Contains(page.Body.String(), "Export spreadsheet CSV") {
						t.Fatal("missing canonical, explained export choices")
					}
				})
			}
		}
	}
}

func TestAnalyticsTableCSVRefusesBeforeBuilding(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"POST", analyticsRecipeTableExportPath, 405},
		{"GET", analyticsRecipeTableExportPath + "?period=bad", 400},
		{"GET", analyticsRecipeTableExportPath + "?state=complete&state=missing", 400},
		{"GET", analyticsRecipeTableExportPath + "?theme=bad", 400},
		{"GET", analyticsRecipeTableExportPath + "?period=%zz", 400},
		{"GET", "/analytics/recipe/%74able.csv", 404},
	} {
		w := httptest.NewRecorder()
		serveAnalyticsRecipeWithBuilder(w, httptest.NewRequest(tc.method, tc.path, nil), func(analyticsSelection) (analyticsJourney, error) {
			t.Fatal("invalid request reached builder")
			return analyticsJourney{}, nil
		})
		if w.Code != tc.status || w.Header().Get("Content-Disposition") != "" || strings.Contains(w.Body.String(), "context_chart_id,") {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	for _, broken := range []bool{false, true} {
		w := httptest.NewRecorder()
		serveAnalyticsRecipeWithBuilder(w, httptest.NewRequest("GET", analyticsRecipeTableExportPath, nil), func(s analyticsSelection) (analyticsJourney, error) {
			if !broken {
				return analyticsJourney{}, errors.New("PRIVATE builder detail")
			}
			j, err := buildAnalyticsJourney(s)
			j.Current.Contract.Unit = ""
			return j, err
		})
		if w.Code != 500 || w.Header().Get("Content-Disposition") != "" || strings.Contains(w.Body.String(), "context_chart_id,") || strings.Contains(w.Body.String(), "PRIVATE") {
			t.Fatalf("partial or private output leaked: %d %s", w.Code, w.Body.String())
		}
	}
}
