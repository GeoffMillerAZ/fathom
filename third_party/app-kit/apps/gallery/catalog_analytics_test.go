package gallery

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// Follow the advertised link, not a separately hardcoded specimen request. A
// successful HTTP response alone could still demonstrate a different renderer.
func TestCatalogAnalyticsPreviewUsesAdvertisedRendererAndCSV(t *testing.T) {
	h := NewWithOptions(Options{})
	selection := analyticsSelection{analyticsPeriodSeptember, analyticsCompareNone, analyticsCategoryAll, analyticsStateComplete}
	j, err := buildAnalyticsJourney(selection)
	if err != nil {
		t.Fatal(err)
	}
	want, err := analytics.RenderBars(j.Current, viz.BarsOptions{}, j.Now)
	if err != nil {
		t.Fatal(err)
	}
	var csv bytes.Buffer
	if err := analytics.WriteCSV(&csv, j.Current, j.Now); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"", "light", "dark"} {
		t.Run("theme="+theme, func(t *testing.T) {
			query := url.Values{"q": {"analytics"}, "profile": {"go-web"}}
			if theme != "" {
				query.Set("theme", theme)
			}
			detail := catalogHTML(t, h, "/catalog/analytics.comparison?"+query.Encode())
			link := catalogTextElement(detail, "a", "Open live specimen →")
			target := catalogAttribute(link, "href")
			if target != previewURL("/analytics/recipe", theme) {
				t.Fatalf("advertised specimen = %q; must open the exact public renderer with only theme context", target)
			}
			response, body := get(t, h, target)
			if response.StatusCode != http.StatusOK || strings.Count(body, want) != 1 {
				t.Fatal("catalog link must render exactly one complete public analytics.RenderBars result")
			}
			page := catalogHTML(t, h, target)
			export := catalogTextElement(page, "a", "Export renderer CSV")
			exportURL := catalogAttribute(export, "href")
			if exportURL != analyticsURL("/analytics/recipe/export.csv", selection, theme) {
				t.Fatalf("renderer export lost the selected dataset/theme: %q", exportURL)
			}
			exported, body := get(t, h, exportURL)
			if exported.StatusCode != http.StatusOK || !strings.HasPrefix(exported.Header.Get("Content-Type"), "text/csv") || exported.Header.Get("Content-Disposition") == "" || body != csv.String() {
				t.Fatal("advertised renderer download must be the direct analytics.WriteCSV output")
			}
			journey := catalogTextElement(page, "a", "Back to the comparison journey")
			if catalogAttribute(journey, "href") != analyticsURL("/analytics", selection, theme) {
				t.Fatal("exact renderer specimen must retain the separate comparison journey")
			}
		})
	}
}
