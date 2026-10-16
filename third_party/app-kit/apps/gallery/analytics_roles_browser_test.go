package gallery

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

const analyticsRolesLongPath = "/__test/analytics-roles/long"
const analyticsRolesLongExportPath = "/__test/analytics-roles/long.csv"

func TestAnalyticsRolesBrowser(t *testing.T) {
	parent := os.Getenv("APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT")
	if parent == "" {
		t.Skip("set APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT to an existing private output directory for the isolated Chrome proof")
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		t.Fatal("APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT must be an existing real 0700 directory")
	}
	real, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Clean(real) != filepath.Clean(parent) {
		t.Fatal("APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT must not traverse symlinks")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("reviewed Node executable is required for the opt-in browser proof")
	}
	harness, err := filepath.Abs(filepath.Join("..", "..", "docs", "assessments", "fixtures", "gallery-analytics", "roles.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(harness); err != nil {
		t.Fatalf("browser harness unavailable: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(analyticsRolesLongPath, serveAnalyticsRolesLong)
	mux.HandleFunc(analyticsRolesLongExportPath, serveAnalyticsRolesLong)
	mux.Handle("/", New())
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, harness, server.URL)
	cmd.Env = []string{
		"APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT=" + parent,
		"APPKIT_PLAYWRIGHT_ENTRY=/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs",
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + parent,
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("analytics role browser proof exceeded its 90 second bound: %s", output.String())
		}
		t.Fatalf("analytics role browser proof failed: %v\n%s", err, output.String())
	}
	if output.Len() > 16<<10 || !strings.Contains(output.String(), `"failures":0`) {
		t.Fatalf("browser proof returned an invalid or oversized summary: %s", output.String())
	}
	t.Log(strings.TrimSpace(output.String()))
}

func serveAnalyticsRolesLong(w http.ResponseWriter, r *http.Request) {
	selection := analyticsSelection{Period: analyticsPeriodSeptember, Compare: analyticsCompareNone, Category: analyticsCategoryAll, State: analyticsStateComplete}
	theme := r.URL.Query().Get("theme")
	if (theme != "" && theme != "light" && theme != "dark") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.Error(w, "invalid synthetic request", http.StatusBadRequest)
		return
	}
	journey, err := buildAnalyticsJourney(selection)
	if err != nil {
		http.Error(w, "synthetic fixture unavailable", http.StatusInternalServerError)
		return
	}
	journey.Current.Rows = append([]analytics.Row(nil), journey.Current.Rows...)
	journey.Current.Rows[0].Label = strings.Repeat("Long service label ", 18)
	if r.URL.Path == analyticsRolesLongExportPath {
		var body bytes.Buffer
		if err := analytics.WriteCSV(&body, journey.Current, journey.Now); err != nil {
			http.Error(w, "synthetic export unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="analytics-role-fallback.csv"`)
		writeAnalyticsResponse(w, r, http.StatusOK, "text/csv; charset=utf-8", body.Bytes())
		return
	}
	if r.URL.Path != analyticsRolesLongPath {
		http.NotFound(w, r)
		return
	}
	rendered, err := analytics.RenderBars(journey.Current, viz.BarsOptions{}, journey.Now)
	if err != nil {
		http.Error(w, "synthetic renderer unavailable", http.StatusInternalServerError)
		return
	}
	content, err := htmlf("analytics-role-fallback", `<header><p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Analytics role fallback · synthetic data</p><h1 class="mt-2 text-2xl font-semibold text-ink">Table-only semantic fallback</h1><a data-role-export href="`+analyticsRolesLongExportPath+`?theme={{.Theme}}" class="mt-3 inline-flex min-h-11 items-center text-accent underline">Export CSV</a></header><section class="mt-8">{{.Rendered}}</section>`, struct {
		Theme    string
		Rendered template.HTML
	}{Theme: theme, Rendered: template.HTML(rendered)})
	writeAnalyticsPage(w, r, http.StatusOK, "Analytics role fallback", content, err, selection, theme, analyticsRolesLongPath)
}
