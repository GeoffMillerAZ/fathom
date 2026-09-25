package gallery

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/internal/analyticscsv"
)

const analyticsMaxQueryBytes = 16 << 10

// registerAnalytics adds a read-only synthetic journey. ServeMux retains its
// normal canonical redirects; encoded or malformed detail paths are not aliases.
func registerAnalytics(mux *http.ServeMux) {
	mux.HandleFunc(analyticsSignedPath, serveAnalyticsSigned)
	mux.HandleFunc(analyticsSignedExportPath, serveAnalyticsSigned)
	mux.HandleFunc("/analytics/recipe", serveAnalyticsRecipe)
	mux.HandleFunc("/analytics/recipe/export.csv", serveAnalyticsRecipe)
	mux.HandleFunc(analyticsRecipeTableExportPath, serveAnalyticsRecipe)
	mux.HandleFunc("/analytics", serveAnalytics)
	mux.HandleFunc("/analytics/", serveAnalytics)
}

// analyticsURL carries only this journey's four validated selection fields and
// the Gallery's theme. It never propagates arbitrary incoming query parameters.
func analyticsURL(path string, selection analyticsSelection, theme string) string {
	q := selection.Values()
	if theme = sanitizeTheme(theme); theme != "" {
		q.Set("theme", theme)
	}
	return path + "?" + q.Encode()
}

func analyticsRoutePath(u *url.URL) bool {
	if u.Opaque != "" || u.EscapedPath() != u.Path {
		return false
	}
	if u.Path == "/analytics" || u.Path == "/analytics/export.csv" {
		return true
	}
	id, ok := strings.CutPrefix(u.Path, "/analytics/")
	id = strings.TrimSuffix(id, "/export.csv")
	if !ok || len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func analyticsRequestSelection(r *http.Request) (analyticsSelection, string, error) {
	if len(r.URL.RawQuery) > analyticsMaxQueryBytes {
		return analyticsSelection{}, "", fmt.Errorf("analytics query exceeds 16 KiB")
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return analyticsSelection{}, "", fmt.Errorf("analytics query has invalid encoding")
	}
	for key, values := range q {
		if !utf8.ValidString(key) || strings.ContainsRune(key, 0) {
			return analyticsSelection{}, "", fmt.Errorf("analytics query has invalid text")
		}
		for _, value := range values {
			if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
				return analyticsSelection{}, "", fmt.Errorf("analytics query has invalid text")
			}
		}
	}
	theme := ""
	if values, ok := q["theme"]; ok {
		if len(values) != 1 || values[0] != "" && sanitizeTheme(values[0]) != values[0] {
			return analyticsSelection{}, "", fmt.Errorf("analytics theme must be one empty, light or dark value")
		}
		theme = values[0]
	}
	s, err := parseAnalyticsSelection(q)
	return s, theme, err
}

func serveAnalytics(w http.ResponseWriter, r *http.Request) {
	defaults, _ := parseAnalyticsSelection(nil)
	if !analyticsRoutePath(r.URL) {
		writeAnalyticsError(w, r, http.StatusNotFound, "This analytics page does not exist. Return to the service comparison.", defaults, "")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAnalyticsError(w, r, http.StatusMethodNotAllowed, "This synthetic journey is read-only. Open it with GET or HEAD; no data has changed.", defaults, "")
		return
	}
	selection, theme, err := analyticsRequestSelection(r)
	if err != nil {
		writeAnalyticsError(w, r, http.StatusBadRequest, "These filters could not be read. Choose one listed value per filter and light, dark or system appearance; reset the selection to try again.", defaults, theme)
		return
	}
	if selection.State == analyticsStateError {
		writeAnalyticsSourceError(w, r, selection, theme, "/analytics")
		return
	}
	j, err := buildAnalyticsJourney(selection)
	if err != nil {
		writeAnalyticsError(w, r, http.StatusInternalServerError, "The synthetic fixture could not be prepared. Return to the comparison and retry.", selection, theme)
		return
	}
	export := strings.HasSuffix(r.URL.Path, "/export.csv")
	var detail *analyticsDetail
	title := "Service operations"
	if r.URL.Path != "/analytics" && r.URL.Path != "/analytics/export.csv" {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/analytics/"), "/export.csv")
		selected, ok := j.Detail(id)
		if !ok {
			writeAnalyticsError(w, r, http.StatusNotFound, "This service is unknown or outside the selected category. Return to the comparison to choose a visible service.", selection, theme)
			return
		}
		detail = &selected
		title = selected.Label + " — service operations"
	}
	if export {
		serviceID := ""
		if detail != nil {
			serviceID = detail.ID
			j = analyticsServiceExport(j, *detail)
		}
		body, err := buildAnalyticsCSV(j, serviceID)
		if err != nil {
			writeAnalyticsError(w, r, http.StatusInternalServerError, "The export could not be prepared. Return to the comparison and retry.", selection, theme)
			return
		}
		filename := "service-operations-" + string(selection.Period)
		if serviceID != "" {
			filename += "-" + serviceID
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
		writeAnalyticsResponse(w, r, http.StatusOK, "text/csv; charset=utf-8", body)
		return
	}
	content, err := buildAnalyticsContent(j, detail, theme)
	writeAnalyticsPage(w, r, http.StatusOK, title, content, err, selection, theme, r.URL.Path)
}

func writeAnalyticsError(w http.ResponseWriter, r *http.Request, status int, message string, selection analyticsSelection, theme string) {
	content, err := buildAnalyticsError(status, message, analyticsURL("/analytics", selection, theme))
	writeAnalyticsPage(w, r, status, "Service operations — request needs attention", content, err, selection, theme, "/analytics")
}

func writeAnalyticsPage(w http.ResponseWriter, r *http.Request, status int, title string, content template.HTML, buildErr error, selection analyticsSelection, theme, currentPath string) {
	if buildErr != nil {
		writeAnalyticsResponse(w, r, http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Analytics could not be displayed. Return to /analytics and retry.\n"))
		return
	}
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == "analytics"
		if links[i].Active {
			links[i].Href = analyticsURL("/analytics", selection, theme)
		} else if theme != "" {
			links[i].Href += "?theme=" + theme
		}
	}
	body, err := render(pageData{
		Title: title, Nav: links, Theme: theme, Page: "analytics", Content: content,
		ThemeLight: analyticsURL(currentPath, selection, "light"),
		ThemeDark:  analyticsURL(currentPath, selection, "dark"), ThemeSystem: analyticsURL(currentPath, selection, ""),
	})
	if err != nil {
		writeAnalyticsResponse(w, r, http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("Analytics could not be displayed. Return to /analytics and retry.\n"))
		return
	}
	writeAnalyticsResponse(w, r, status, "text/html; charset=utf-8", body)
}

func writeAnalyticsResponse(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// These are the existing shared CSV contract's exact keys, not a parallel
// formatter. Drift must be reviewed before this Gallery-only projection resumes.
var analyticsCSVContextKeys = analyticscsv.ContextKeys()

var analyticsCSVDataKeys = analyticscsv.DataKeys()

// buildAnalyticsCSV makes one rectangular document: one row per selected service
// and series. Numeric, missing and formula-safe cells come unchanged from WriteCSV;
// each row carries its own dataset window/context, never a combined-period sum.
func buildAnalyticsCSV(j analyticsJourney, serviceID string) ([]byte, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	header := []string{"series", "selection_period", "selection_compare", "selection_category", "selection_state", "selection_service"}
	for _, key := range analyticsCSVContextKeys {
		header = append(header, "context_"+key)
	}
	header = append(header, analyticsCSVDataKeys...)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	writeDataset := func(series string, d analytics.Dataset) error {
		var shared bytes.Buffer
		if err := analytics.WriteCSV(&shared, d, j.Now); err != nil {
			return err
		}
		context, rows, err := analyticsCSVSection(shared.Bytes(), len(d.Rows))
		if err != nil {
			return err
		}
		for _, cells := range rows {
			row := []string{series, string(j.Selection.Period), string(j.Selection.Compare), string(j.Selection.Category), string(j.Selection.State), serviceID}
			row = append(row, context...)
			row = append(row, cells...)
			if err := w.Write(row); err != nil {
				return err
			}
		}
		return nil
	}
	if err := writeDataset("current", j.Current); err != nil {
		return nil, err
	}
	if j.Previous != nil {
		if err := writeDataset("previous", *j.Previous); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return out.Bytes(), w.Error()
}

// Narrow by copying the selected datasets, not by changing the shared selection
// or summing rows. The original journey remains available for detail/back links.
func analyticsServiceExport(j analyticsJourney, detail analyticsDetail) analyticsJourney {
	withDetail := func(d analytics.Dataset, row analytics.Row) analytics.Dataset {
		d.Rows = []analytics.Row{row}
		d.Contract.ID += "-" + detail.ID
		d.Contract.Title += " · " + detail.Label
		d.Contract.Source.Text += " Detail service=" + detail.ID + "."
		return d
	}
	j.Current = withDetail(j.Current, detail.Current)
	if j.Previous != nil && detail.Previous != nil {
		previous := withDetail(*j.Previous, *detail.Previous)
		j.Previous = &previous
	}
	return j
}

func analyticsCSVSection(body []byte, expectedRows int) ([]string, [][]string, error) {
	return analyticscsv.Section(body, expectedRows)
}
