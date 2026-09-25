package gallery

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func analyticsRecipeRequest(method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	serveAnalyticsRecipe(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestAnalyticsRecipeUsesPublicRendererAndMatchingCSV(t *testing.T) {
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker} {
			for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
				t.Run(string(period)+"/"+string(category)+"/"+string(state), func(t *testing.T) {
					selection := analyticsSelection{Period: period, Compare: analyticsCompareNone, Category: category, State: state}
					query := selection.Values().Encode() + "&theme=dark"
					page := analyticsRecipeRequest(http.MethodGet, analyticsRecipePath+"?"+query)
					if page.Code != http.StatusOK {
						t.Fatalf("page returned %d: %s", page.Code, page.Body.String())
					}
					body := page.Body.String()
					for _, want := range []string{
						"analytics.RenderBars", "analytics.WriteCSV", `name="compare" value="none"`,
						"compare=none", "Values for Failed service operations", "Synthetic Gallery operating ledger v1",
					} {
						if !strings.Contains(body, want) {
							t.Fatalf("public renderer page missing %q", want)
						}
					}
					if state == analyticsStateMissing {
						if strings.Contains(body, "<svg") || !strings.Contains(body, "No measured values in this selection") {
							t.Fatal("missing recipe plotted a value or hid its empty state")
						}
					} else if !strings.Contains(body, "<svg") {
						t.Fatal("measured recipe omitted public SVG output")
					}

					exported := analyticsRecipeRequest(http.MethodGet, analyticsRecipeExportPath+"?"+query)
					if exported.Code != http.StatusOK || exported.Header().Get("Content-Disposition") == "" {
						t.Fatalf("export returned %d without download disposition", exported.Code)
					}
					journey, err := buildAnalyticsJourney(selection)
					if err != nil {
						t.Fatal(err)
					}
					rendered, err := analytics.RenderBars(journey.Current, viz.BarsOptions{}, journey.Now)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Count(body, rendered) != 1 {
						t.Fatal("recipe did not embed exactly one complete analytics.RenderBars result")
					}
					var want bytes.Buffer
					if err := analytics.WriteCSV(&want, journey.Current, journey.Now); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(exported.Body.Bytes(), want.Bytes()) {
						t.Fatal("recipe export diverged from analytics.WriteCSV")
					}
					if page.Body.Len() > 32<<10 {
						t.Fatalf("recipe exceeded 32 KiB HTML: %d", page.Body.Len())
					}
				})
			}
		}
	}
}

func TestAnalyticsRecipeIsRegisteredAtExactMuxRoute(t *testing.T) {
	selection := analyticsSelection{Period: analyticsPeriodSeptember, Compare: analyticsCompareNone, Category: analyticsCategoryAll, State: analyticsStateComplete}
	journey, err := buildAnalyticsJourney(selection)
	if err != nil {
		t.Fatal(err)
	}
	want, err := analytics.RenderBars(journey.Current, viz.BarsOptions{}, journey.Now)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	New().ServeHTTP(w, httptest.NewRequest(http.MethodGet, analyticsRecipePath+"?"+selection.Values().Encode(), nil))
	if w.Code != http.StatusOK || strings.Count(w.Body.String(), want) != 1 {
		t.Fatalf("registered recipe route did not serve one exact public renderer result: status=%d", w.Code)
	}
}

func TestAnalyticsRecipeCanonicalizesComparisonAndPreservesTheme(t *testing.T) {
	r := analyticsRecipeRequest(http.MethodGet, analyticsRecipePath+"?period=2026-08&compare=previous&category=api&state=partial&theme=light")
	if r.Code != http.StatusOK {
		t.Fatalf("page returned %d", r.Code)
	}
	body := r.Body.String()
	if strings.Contains(body, "compare=previous") || !strings.Contains(body, "compare=none") || !strings.Contains(body, "theme=light") {
		t.Fatal("recipe links did not canonicalize comparison while preserving theme")
	}
	if !strings.Contains(body, "comparison=none") || strings.Contains(body, "comparison=previous") {
		t.Fatal("renderer dataset retained the unsupported paired comparison")
	}
}

func TestAnalyticsRecipeHEADAndFailures(t *testing.T) {
	target := analyticsRecipePath + "?period=2026-09&compare=none&category=all&state=complete"
	get := analyticsRecipeRequest(http.MethodGet, target)
	head := analyticsRecipeRequest(http.MethodHead, target)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
		t.Fatalf("HEAD disagrees with GET: code=%d body=%d length=%q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}

	for _, tc := range []struct {
		name, method, target string
		status               int
	}{
		{"method", http.MethodPost, analyticsRecipePath, http.StatusMethodNotAllowed},
		{"invalid query", http.MethodGet, analyticsRecipePath + "?state=imaginary", http.StatusBadRequest},
		{"duplicate query", http.MethodGet, analyticsRecipePath + "?period=2026-09&period=2026-08", http.StatusBadRequest},
		{"invalid path", http.MethodGet, analyticsRecipePath + "/extra", http.StatusNotFound},
		{"escaped path", http.MethodGet, "/analytics/%72ecipe", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := analyticsRecipeRequest(tc.method, tc.target)
			if r.Code != tc.status {
				t.Fatalf("got %d, want %d", r.Code, tc.status)
			}
			if tc.status == http.StatusMethodNotAllowed && r.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("method refusal omitted Allow header")
			}
		})
	}
}
