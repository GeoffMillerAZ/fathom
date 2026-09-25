package gallery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

const (
	analyticsScalePath       = "/__test/analytics-scale"
	analyticsScaleExportPath = "/__test/analytics-scale/export.csv"
	analyticsScaleHTMLBudget = 256 << 10
	analyticsScaleCSSBudget  = 128 << 10
)

var analyticsScaleNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func analyticsScaleDataset(long bool) analytics.Dataset {
	rows := make([]analytics.Row, analytics.MaxRows)
	for i := range rows {
		v := float64(i)
		row := analytics.Row{ID: fmt.Sprintf("service-%03d", i+1), Label: fmt.Sprintf("Service %03d", i+1), Role: "accent", Value: &v}
		if i%2 == 1 {
			row.Role = "info"
		}
		if i > 0 && i%17 == 0 {
			row.Value = nil
			row.MissingReason = "Synthetic source unavailable for this selected row."
		} else if (i > 0 && i%13 == 0) || i == analytics.MaxRows-1 {
			row.Bounds = &analytics.Interval{Low: v - .25, High: v + .25}
		}
		rows[i] = row
	}
	if long {
		rows[0].Label = strings.Repeat("Long service label ", 18)
	}
	return analytics.Dataset{
		Contract: analytics.Contract{
			ID: "maximum-service-events", Title: "Selected service events", Unit: "events", Decimals: 2,
			Grain: "selected service", Aggregation: "synthetic event count per selected service",
			Window:       analytics.Window{Kind: analytics.RangeWindow, Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), TimeZone: "UTC"},
			Source:       vizframe.Derived("Synthetic scale fixture; no operational data or claim."),
			Freshness:    analytics.Freshness{ObservedAt: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC), MaxAge: 2 * time.Hour},
			Completeness: analytics.Partial, CoverageNote: "11 unavailable of 200 selected rows.",
			Uncertainty: "Synthetic scale fixture; uncertainty is not quantified.",
		},
		Rows: rows,
		Legend: []analytics.LegendItem{
			{Role: "accent", Label: "API services"},
			{Role: "info", Label: "Worker services"},
		},
	}
}

func analyticsScaleQuery(r *http.Request) (variant, theme string, ok bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", "", false
	}
	q := r.URL.Query()
	if len(q) != 2 || len(q["variant"]) != 1 || len(q["theme"]) != 1 {
		return "", "", false
	}
	variant, theme = q.Get("variant"), q.Get("theme")
	return variant, theme, (variant == "normal" || variant == "long") && (theme == "light" || theme == "dark")
}

func renderAnalyticsScale(d analytics.Dataset, variant, theme string) ([]byte, error) {
	rendered, err := analytics.RenderBars(d, viz.BarsOptions{}, analyticsScaleNow)
	if err != nil {
		return nil, err
	}
	content, err := htmlf("analytics-scale", `<header><p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Synthetic maximum-row acceptance fixture</p><h1 class="mt-2 text-2xl font-semibold text-ink">200 selected services</h1><p class="mt-2 max-w-[72ch] text-sm text-ink-muted">No operational data or claim. This test-only page checks the public renderer and export at their declared row limit.</p><form action="`+analyticsScalePath+`" method="get" class="mt-4 flex flex-wrap items-end gap-3"><input type="hidden" name="theme" value="{{.Theme}}"><label class="text-sm font-medium">Variant <select name="variant" class="ml-2 min-h-11 rounded-md border border-line-strong bg-surface-raised px-3"><option value="normal"{{if eq .Variant "normal"}} selected{{end}}>Normal labels</option><option value="long"{{if eq .Variant "long"}} selected{{end}}>Long-label fallback</option></select></label><button class="min-h-11 rounded-md bg-accent px-4 py-2 font-medium text-accent-ink">Apply</button></form><a data-scale-export href="`+analyticsScaleExportPath+`?variant={{.Variant}}&amp;theme={{.Theme}}" class="mt-3 inline-flex min-h-11 items-center text-accent underline">Export all 200 selected rows</a></header><section data-scale-specimen data-variant="{{.Variant}}" class="mt-8 min-w-0">{{.Rendered}}</section>`, struct {
		Theme, Variant string
		Rendered       template.HTML
	}{Theme: theme, Variant: variant, Rendered: template.HTML(rendered)})
	if err != nil {
		return nil, err
	}
	links := append([]navLink(nil), nav...)
	body, err := render(pageData{
		Title: "Maximum analytics selection", Nav: links, Theme: theme, Page: "analytics", Content: content,
		ThemeLight:  analyticsScalePath + "?variant=" + variant + "&theme=light",
		ThemeDark:   analyticsScalePath + "?variant=" + variant + "&theme=dark",
		ThemeSystem: analyticsScalePath + "?variant=" + variant + "&theme=light",
	})
	if err != nil {
		return nil, err
	}
	if len(body) > analyticsScaleHTMLBudget {
		return nil, fmt.Errorf("synthetic maximum-row HTML exceeds %d bytes", analyticsScaleHTMLBudget)
	}
	return body, nil
}

func serveAnalyticsScale(w http.ResponseWriter, r *http.Request) {
	variant, theme, ok := analyticsScaleQuery(r)
	if !ok || (r.URL.Path != analyticsScalePath && r.URL.Path != analyticsScaleExportPath) {
		http.Error(w, "invalid synthetic scale request", http.StatusBadRequest)
		return
	}
	d := analyticsScaleDataset(variant == "long")
	if r.URL.Path == analyticsScaleExportPath {
		var body bytes.Buffer
		if err := analytics.WriteCSV(&body, d, analyticsScaleNow); err != nil {
			http.Error(w, "synthetic export unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="analytics-scale.csv"`)
		writeAnalyticsResponse(w, r, http.StatusOK, "text/csv; charset=utf-8", body.Bytes())
		return
	}
	body, err := renderAnalyticsScale(d, variant, theme)
	if err != nil {
		writeAnalyticsResponse(w, r, http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Synthetic scale fixture could not be rendered.\n"))
		return
	}
	writeAnalyticsResponse(w, r, http.StatusOK, "text/html; charset=utf-8", body)
}

func TestAnalyticsScaleRejectsInvalidDatasetAndRequests(t *testing.T) {
	d := analyticsScaleDataset(false)
	v := float64(analytics.MaxRows)
	d.Rows = append(d.Rows, analytics.Row{ID: "service-201", Label: "Service 201", Role: "accent", Value: &v})
	if body, err := renderAnalyticsScale(d, "normal", "light"); err == nil || len(body) != 0 {
		t.Fatalf("private renderer accepted 201 rows: bytes=%d err=%v", len(body), err)
	}
	h := http.HandlerFunc(serveAnalyticsScale)
	for _, target := range []string{
		analyticsScalePath,
		analyticsScalePath + "?variant=normal",
		analyticsScalePath + "?variant=normal&theme=light&extra=1",
		analyticsScalePath + "?variant=normal&variant=long&theme=light",
		analyticsScalePath + "?variant=other&theme=light",
		analyticsScaleExportPath + "?variant=normal&theme=system",
	} {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", target, w.Code)
		}
	}
}

type analyticsScaleManifest struct {
	Kind             string            `json:"kind"`
	Verification     string            `json:"verification"`
	ExecutablePath   string            `json:"executable_path"`
	ExecutableSHA256 string            `json:"executable_sha256"`
	Inputs           map[string]string `json:"inputs"`
	Responses        map[string]string `json:"responses"`
	Expected         struct {
		Cases        int     `json:"cases"`
		Rows         int     `json:"rows"`
		Plotted      int     `json:"plotted"`
		Missing      int     `json:"missing"`
		Bounds       int     `json:"bounds"`
		HTMLMaxBytes int     `json:"html_max_bytes"`
		CSSMaxBytes  int     `json:"css_max_bytes"`
		DOMMaxNodes  int     `json:"dom_max_nodes"`
		DCLP95MaxMS  float64 `json:"dcl_p95_max_ms"`
	} `json:"expected"`
}

func scaleDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func scaleRead(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, fmt.Errorf("bounded regular input required: %s", path)
	}
	body, err := os.ReadFile(path)
	if err != nil || int64(len(body)) != info.Size() {
		return nil, fmt.Errorf("read stable input %s: %w", path, err)
	}
	return body, nil
}

func scaleWrite(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(body)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func scaleExecutableDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, 256<<20))
	if err != nil {
		return "", err
	}
	info, err := f.Stat()
	if err != nil || n != info.Size() {
		return "", fmt.Errorf("test executable exceeds bound or changed")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestAnalyticsScaleBrowser(t *testing.T) {
	parent := os.Getenv("APPKIT_ANALYTICS_SCALE_OUTPUT_PARENT")
	if parent == "" {
		t.Skip("set APPKIT_ANALYTICS_SCALE_OUTPUT_PARENT to an existing private output directory for the isolated Chrome proof")
	}
	info, err := os.Lstat(parent)
	if !filepath.IsAbs(parent) || err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		t.Fatal("APPKIT_ANALYTICS_SCALE_OUTPUT_PARENT must be an existing real absolute 0700 directory")
	}
	real, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Clean(real) != filepath.Clean(parent) {
		t.Fatal("APPKIT_ANALYTICS_SCALE_OUTPUT_PARENT must not traverse symlinks")
	}
	playwright := os.Getenv("APPKIT_PLAYWRIGHT_ENTRY")
	if !filepath.IsAbs(playwright) {
		t.Fatal("APPKIT_PLAYWRIGHT_ENTRY must be an explicit absolute path")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("reviewed Node executable is required")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{
		"renderer":     filepath.Join(repo, "appkit", "ui", "analytics", "render.go"),
		"model_test":   filepath.Join(repo, "appkit", "ui", "analytics", "max_rows_test.go"),
		"fixture_test": filepath.Join(repo, "apps", "gallery", "analytics_scale_browser_test.go"),
		"harness":      filepath.Join(repo, "docs", "assessments", "fixtures", "gallery-analytics", "scale.mjs"),
		"css":          filepath.Join(repo, "apps", "gallery", "ui", "app.css"),
		"playwright":   playwright,
	}
	inputBytes := map[string][]byte{}
	inputHashes := map[string]string{}
	for name, path := range paths {
		body, err := scaleRead(path, 2<<20)
		if err != nil {
			t.Fatal(err)
		}
		if name == "css" && len(body) > analyticsScaleCSSBudget {
			t.Fatalf("Gallery CSS exceeds %d bytes", analyticsScaleCSSBudget)
		}
		inputBytes[name], inputHashes[name] = body, scaleDigest(body)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	executableSHA, err := scaleExecutableDigest(executable)
	if err != nil {
		t.Fatal(err)
	}

	manifest := analyticsScaleManifest{Kind: "private-analytics-scale-browser-input", Verification: "not-verified", ExecutablePath: executable, ExecutableSHA256: executableSHA, Inputs: inputHashes, Responses: map[string]string{}}
	manifest.Expected.Cases, manifest.Expected.Rows, manifest.Expected.Plotted, manifest.Expected.Missing, manifest.Expected.Bounds = 4, 200, 189, 11, 16
	manifest.Expected.HTMLMaxBytes, manifest.Expected.CSSMaxBytes, manifest.Expected.DOMMaxNodes, manifest.Expected.DCLP95MaxMS = analyticsScaleHTMLBudget, analyticsScaleCSSBudget, 3500, 500
	for _, theme := range []string{"light", "dark"} {
		for _, variant := range []string{"normal", "long"} {
			body, err := renderAnalyticsScale(analyticsScaleDataset(variant == "long"), variant, theme)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Responses[theme+"-"+variant+"-html"] = scaleDigest(body)
		}
	}
	for _, variant := range []string{"normal", "long"} {
		var body bytes.Buffer
		if err := analytics.WriteCSV(&body, analyticsScaleDataset(variant == "long"), analyticsScaleNow); err != nil {
			t.Fatal(err)
		}
		manifest.Responses[variant+"-csv"] = scaleDigest(body.Bytes())
	}
	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestBody = append(manifestBody, '\n')
	manifestSHA := scaleDigest(manifestBody)
	runOut, err := os.MkdirTemp(parent, "analytics-scale-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(runOut, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"manifest.json":        manifestBody,
		"fixture.go.txt":       inputBytes["fixture_test"],
		"max_rows_test.go.txt": inputBytes["model_test"],
		"renderer.go.txt":      inputBytes["renderer"],
		"css.txt":              inputBytes["css"],
		"harness.mjs":          inputBytes["harness"],
	} {
		if err := scaleWrite(filepath.Join(runOut, name), body); err != nil {
			t.Fatal(err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc(analyticsScalePath, serveAnalyticsScale)
	mux.HandleFunc(analyticsScaleExportPath, serveAnalyticsScale)
	mux.Handle("/", New())
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, filepath.Join(runOut, "harness.mjs"), server.URL, runOut, manifestSHA)
	cmd.Env = []string{
		"APPKIT_PLAYWRIGHT_ENTRY=" + playwright,
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + parent,
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("analytics scale browser proof exceeded its three-minute bound; output=%s path=%s", output.String(), runOut)
		}
		t.Fatalf("analytics scale browser proof failed: %v; output=%s path=%s", err, output.String(), runOut)
	}
	if output.Len() > 16<<10 {
		t.Fatalf("browser summary exceeded bound; path=%s", runOut)
	}
	if _, err := os.Stat(filepath.Join(runOut, "failure.json")); !os.IsNotExist(err) {
		t.Fatalf("browser retained a failure receipt; path=%s", runOut)
	}
	reportBody, err := scaleRead(filepath.Join(runOut, "report.json"), 2<<20)
	if err != nil {
		t.Fatalf("persisted report required: %v; path=%s", err, runOut)
	}
	var report struct {
		Schema         int    `json:"schema"`
		ManifestSHA256 string `json:"manifest_sha256"`
		Cases          int    `json:"cases"`
		Failures       int    `json:"failures"`
		Errors         []any  `json:"errors"`
		Checks         []struct {
			Name string `json:"name"`
			Pass bool   `json:"pass"`
		} `json:"checks"`
		Lanes     []any `json:"lanes"`
		Artifacts []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"artifacts"`
		Cleanup struct {
			Graceful          bool `json:"graceful"`
			OwnedParentAbsent bool `json:"owned_parent_absent"`
		} `json:"cleanup"`
	}
	if err := json.Unmarshal(reportBody, &report); err != nil || report.Schema != 1 || report.ManifestSHA256 != manifestSHA || report.Cases != 4 || report.Failures != 0 || len(report.Errors) != 0 || len(report.Checks) == 0 || len(report.Lanes) != 4 || !report.Cleanup.Graceful || !report.Cleanup.OwnedParentAbsent {
		t.Fatalf("invalid persisted browser report: %+v err=%v path=%s", report, err, runOut)
	}
	for _, check := range report.Checks {
		if check.Name == "" || !check.Pass {
			t.Fatalf("browser report retained a failed or unnamed check: %+v", check)
		}
	}
	wantArtifacts := map[string]bool{"owned-browser.json": true}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"light", "dark"} {
			lane := fmt.Sprintf("%d-%s", width, theme)
			for _, suffix := range []string{"normal-chart-first.png", "normal-first-label.png", "normal-first-meaning.png", "normal-last-meaning.png", "long-first-label.png", "long-first-meaning.png", "long-last-meaning.png", "normal.csv", "long.csv"} {
				wantArtifacts[lane+"-"+suffix] = true
			}
		}
	}
	seenArtifacts := map[string]bool{}
	for _, artifact := range report.Artifacts {
		if !filepath.IsLocal(artifact.Path) || filepath.Base(artifact.Path) != artifact.Path {
			t.Fatalf("unsafe browser artifact path: %q", artifact.Path)
		}
		if seenArtifacts[artifact.Path] || !wantArtifacts[artifact.Path] {
			t.Fatalf("duplicate or unexpected browser artifact: %q", artifact.Path)
		}
		seenArtifacts[artifact.Path] = true
		body, err := scaleRead(filepath.Join(runOut, artifact.Path), 8<<20)
		if err != nil || scaleDigest(body) != artifact.SHA256 {
			t.Fatalf("browser artifact drift: %q err=%v", artifact.Path, err)
		}
	}
	if !reflect.DeepEqual(seenArtifacts, wantArtifacts) {
		t.Fatalf("browser artifact inventory differs: got=%v want=%v", seenArtifacts, wantArtifacts)
	}
	for name, path := range paths {
		body, err := scaleRead(path, 2<<20)
		if err != nil || scaleDigest(body) != inputHashes[name] {
			t.Fatalf("input changed during browser proof: %s", name)
		}
	}
	if after, err := scaleExecutableDigest(executable); err != nil || after != executableSHA {
		t.Fatal("executing test binary changed during browser proof")
	}
	t.Logf("analytics scale browser report=%s sha256=%s", filepath.Join(runOut, "report.json"), scaleDigest(reportBody))
}

func TestAnalyticsScaleFixtureMatchesModelContract(t *testing.T) {
	d := analyticsScaleDataset(false)
	copyBefore := analyticsScaleDataset(false)
	var out bytes.Buffer
	if err := analytics.WriteCSV(&out, d, analyticsScaleNow); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(bytes.NewReader(out.Bytes()))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil || !reflect.DeepEqual(d, copyBefore) {
		t.Fatalf("fixture export failed or mutated input: %v", err)
	}
	rows := 0
	for _, record := range records {
		if len(record) == 10 && record[0] != "id" {
			rows++
		}
	}
	if rows != analytics.MaxRows {
		t.Fatalf("fixture exported %d rows", rows)
	}
}
