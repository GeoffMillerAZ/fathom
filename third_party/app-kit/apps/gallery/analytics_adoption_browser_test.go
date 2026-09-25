package gallery

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/apps/styleguide"
)

// TestAnalyticsAdoptionBrowserFixture is an opt-in, synthetic-only browser target.
// It exposes real recipe, CSV, quality-record, CSS and style-guide handlers, not other
// routes or any consumer data. The harness must explicitly stop it after use.
func TestAnalyticsAdoptionBrowserFixture(t *testing.T) {
	if os.Getenv("APPKIT_ANALYTICS_BROWSER") != "1" {
		t.Skip("set APPKIT_ANALYTICS_BROWSER=1 to run the isolated browser fixture")
	}

	const guideSource = "../../cue/tokens/kit.cue"
	source, err := os.ReadFile(guideSource)
	if err != nil {
		t.Fatal(err)
	}
	checks := []styleguide.ContrastCheck{
		{ID: "body-light", Foreground: "ink", Background: "surface", Scheme: "light", Minimum: 4.5},
		{ID: "body-dark", Foreground: "ink", Background: "surface", Scheme: "dark", Minimum: 4.5},
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{
		Source: source, SourceLabel: "Synthetic app-kit token source", KitRevision: "synthetic-browser-fixture",
		DarkVariant: tokens.DarkVariantAttribute,
		Fonts:       map[string]string{"system-sans": "system-ui, sans-serif"}, Checks: checks,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(nil)
	server.EnableHTTP2 = false
	authority := server.Listener.Addr().String()
	origin := "http://" + authority
	guide, err := styleguide.New(styleguide.Options{
		Engine: engine, BasePath: "/dev/theme", Origin: origin,
		Title: "Synthetic analytics theme guide", Stylesheets: []string{"/static/app.css"},
		Specimens: []styleguide.Specimen{{ID: "analytics", Name: "Synthetic partial analytics", Render: func(_ context.Context, w io.Writer) error {
			selection, _ := parseAnalyticsSelection(nil)
			selection.State = analyticsStatePartial
			journey, err := adoptionBrowserJourney(selection)
			if err != nil {
				return err
			}
			markup, err := analytics.RenderBars(journey.Current, viz.BarsOptions{}, journey.Now)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, markup)
			return err
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	gallery := New()
	stopped := make(chan struct{})
	var stopOnce sync.Once
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() || r.Host != authority || r.URL.EscapedPath() != r.URL.Path {
			http.Error(w, "fixture requires exact loopback origin", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/__fixture/stop" {
			if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("X-Fixture-Token") != "synthetic-session" {
				http.Error(w, "fixture stop requires its synthetic token", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			// Acknowledge the request before test cleanup closes client connections.
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("flush fixture stop acknowledgement: %v", err)
			}
			stopOnce.Do(func() { close(stopped) })
			return
		}
		if r.URL.Path == "/dev/theme" || strings.HasPrefix(r.URL.Path, "/dev/theme/") {
			guide.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "synthetic fixture is read-only", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case analyticsSignedPath, analyticsSignedExportPath:
			serveAnalyticsSigned(w, r)
		case analyticsRecipePath, analyticsRecipeExportPath:
			serveAnalyticsRecipeWithBuilder(w, r, adoptionBrowserJourney)
		case "/quality", "/quality/core.analytics":
			gallery.ServeHTTP(w, r)
		case "/static/app.css":
			if r.URL.RawQuery != "" {
				http.NotFound(w, r)
				return
			}
			gallery.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	server.Start()
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})
	if server.URL != origin {
		t.Fatalf("fixture origin changed: %s != %s", server.URL, origin)
	}
	fmt.Printf("APPKIT_FIXTURE_URL=%s\n", server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("analytics browser fixture timed out without an explicit stop")
	}
}

func adoptionBrowserJourney(selection analyticsSelection) (analyticsJourney, error) {
	if selection.State != analyticsStatePartial && selection.State != analyticsStateComplete {
		return analyticsJourney{}, fmt.Errorf("fixture supports only partial or complete state")
	}
	journey, err := buildAnalyticsJourney(selection)
	if err != nil {
		return analyticsJourney{}, err
	}
	zero, other, recorded := float64(0), float64(7), float64(3)
	journey.Current.Rows = []analytics.Row{
		{ID: "zero", Label: "Measured zero", Value: &zero},
		{ID: "other", Label: "Other service", Role: "accent", Value: &other},
		{ID: "recorded", Label: "Recorded service", Role: "danger", Value: &recorded},
	}
	journey.Current.Legend = []analytics.LegendItem{
		{Role: "danger", Label: "Recorded"},
		{Role: "accent", Label: "Scheduled"},
	}
	if selection.State == analyticsStatePartial {
		journey.Current.Rows[2].Value = nil
		journey.Current.Rows[2].MissingReason = "Source unavailable"
		journey.Current.Contract.Completeness = analytics.Partial
		journey.Current.Contract.CoverageNote = "Synthetic selected rows include one unavailable source; observed counts are lower bounds, not a complete-period total."
	} else {
		journey.Current.Contract.Completeness = analytics.Complete
		journey.Current.Contract.CoverageNote = ""
	}
	if err := journey.Current.Validate(journey.Now); err != nil {
		return analyticsJourney{}, err
	}
	return journey, nil
}

func TestAdoptionBrowserJourneySynthetic(t *testing.T) {
	for _, state := range []analyticsState{analyticsStatePartial, analyticsStateComplete} {
		t.Run(string(state), func(t *testing.T) {
			selection, _ := parseAnalyticsSelection(nil)
			selection.State = state
			journey, err := adoptionBrowserJourney(selection)
			if err != nil {
				t.Fatal(err)
			}
			if len(journey.Current.Rows) != 3 || journey.Current.Rows[0].Value == nil || *journey.Current.Rows[0].Value != 0 {
				t.Fatal("fixture must retain a measured zero")
			}
			if state == analyticsStatePartial {
				if journey.Current.Rows[2].Value != nil || journey.Current.Rows[2].MissingReason != "Source unavailable" || journey.Current.Contract.Completeness != analytics.Partial {
					t.Fatal("partial fixture must retain a missing row and partial provenance")
				}
			} else if journey.Current.Rows[2].Value == nil || journey.Current.Contract.Completeness != analytics.Complete {
				t.Fatal("complete fixture must have only measured rows")
			}
		})
	}
}
