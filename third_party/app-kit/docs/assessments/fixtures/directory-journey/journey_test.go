package directoryjourney

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func journeyRenderer(_ context.Context, v View) ([]byte, error) {
	var b strings.Builder
	b.WriteString("<section data-rendered=\"directory\">")
	for i, row := range v.Rows {
		b.WriteString("<article data-id=\"")
		b.WriteString(row.ID)
		b.WriteString("\"><a class=\"journey-control\" href=\"")
		b.WriteString(v.DetailURLs[i])
		b.WriteString("\">")
		b.WriteString(v.EditLabel)
		b.WriteString("</a><span>")
		b.WriteString(row.Name)
		b.WriteString("</span></article>")
	}
	b.WriteString("</section>")
	return []byte(b.String()), nil
}

func journeyHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := NewHandler(journeyRenderer, []byte(".source{color:var(--color-gray-900)}"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func journeyRequest(h http.Handler, method, target string, headers http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:43129"
	r.RemoteAddr = "127.0.0.1:49152"
	for key, values := range headers {
		r.Header[key] = append([]string(nil), values...)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRowsAndRevisionAreFrozen(t *testing.T) {
	got := Rows()
	if len(got) != 12 || got[0] != (Row{ID: "p01", Name: "Ada Vale", Title: "Software Engineer", Email: "p01@example.invalid", Role: "Member"}) || got[11] != (Row{ID: "p12", Name: "=1+1", Title: "Security Analyst", Email: "p12@example.invalid", Role: "Member"}) {
		t.Fatalf("frozen rows = %#v", got)
	}
	if got[1].Role != "Owner" || got[2].Role != "Viewer" || len(DatasetRevision()) != 64 || strings.Contains(DatasetRevision(), ":") || DatasetRevision() != frozenDatasetRevision() {
		t.Fatalf("roles/revision = %#v got=%q want=%q", got[:3], DatasetRevision(), frozenDatasetRevision())
	}
	got[0].Name = "mutated"
	if Rows()[0].Name != "Ada Vale" {
		t.Fatal("Rows leaked its backing dataset")
	}
}

func TestDirectoryListDetailAndCSVSelection(t *testing.T) {
	h := journeyHandler(t)
	list := journeyRequest(h, http.MethodGet, "/directory?q=engineer&page=2&theme=dark", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", list.Code, list.Body.String())
	}
	body := list.Body.String()
	for _, want := range []string{"data-id=\"p10\"", "q=engineer", "theme=dark", "state=complete", "Download all 4 matching observed records (CSV)", `id="directory-query"`, `id="directory-status" data-page-count="2" data-matching-count="4" data-coverage="complete"`, `id="directory-previous"`, `id="directory-export"`, `name="dataset-revision" content="` + DatasetRevision() + `"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("list missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "data-id=\"p01\"") || strings.Contains(body, "data-id=\"p04\"") {
		t.Fatalf("page 2 selection leaked other rows: %s", body)
	}
	css := journeyRequest(h, http.MethodGet, "/assets/journey.css", nil)
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), "min-height: 44px") || !strings.Contains(css.Body.String(), ".journey-control:focus-visible") || !strings.Contains(css.Body.String(), "var(--color-gray-900)") || !strings.Contains(css.Body.String(), "var(--color-white)") {
		t.Fatalf("journey CSS = %d: %s", css.Code, css.Body.String())
	}
	detail := journeyRequest(h, http.MethodGet, "/directory/p10?q=engineer&page=2&theme=dark", nil)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `data-record-id="p10"`) || !strings.Contains(detail.Body.String(), `data-field="Name">Jules Mora`) || !strings.Contains(detail.Body.String(), `id="directory-back"`) || !strings.Contains(detail.Body.String(), `id="directory-status" data-page-count="2" data-matching-count="4" data-coverage="complete"`) || !strings.Contains(detail.Body.String(), "← Back to results") || !strings.Contains(detail.Body.String(), "page=2") {
		t.Fatalf("detail = %d: %s", detail.Code, detail.Body.String())
	}
	if got := journeyRequest(h, http.MethodGet, "/directory/p02?q=engineer&page=2&theme=dark", nil); got.Code != http.StatusNotFound {
		t.Fatalf("excluded detail = %d", got.Code)
	}
	csv := journeyRequest(h, http.MethodGet, "/directory.csv?q=engineer&page=2&theme=dark", nil)
	if csv.Code != http.StatusOK || csv.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("csv = %d %q", csv.Code, csv.Header().Get("Content-Type"))
	}
	for _, id := range []string{"p01", "p04", "p07", "p10"} {
		if !strings.Contains(csv.Body.String(), ","+id+",") {
			t.Fatalf("CSV missed %s: %q", id, csv.Body.String())
		}
	}
	if strings.Contains(csv.Body.String(), ",p02,") || strings.Count(csv.Body.String(), "\n") != 5 {
		t.Fatalf("CSV scope/order = %q", csv.Body.String())
	}
}

func TestDirectoryPartialUnavailableAndMalformedSelection(t *testing.T) {
	h := journeyHandler(t)
	partial := journeyRequest(h, http.MethodGet, "/directory?state=partial", nil)
	if partial.Code != http.StatusOK || !strings.Contains(partial.Body.String(), "coverage is unknown") || strings.Contains(partial.Body.String(), "p07") {
		t.Fatalf("partial truth = %d: %s", partial.Code, partial.Body.String())
	}
	partialCSV := journeyRequest(h, http.MethodGet, "/directory.csv?state=partial", nil)
	if partialCSV.Code != http.StatusOK || strings.Count(partialCSV.Body.String(), "\n") != 7 || !strings.Contains(partialCSV.Body.String(), ",partial,") {
		t.Fatalf("partial CSV = %d: %q", partialCSV.Code, partialCSV.Body.String())
	}
	for _, target := range []string{"/directory?state=unavailable", "/directory.csv?state=unavailable"} {
		got := journeyRequest(h, http.MethodGet, target, nil)
		if got.Code != http.StatusServiceUnavailable || strings.Contains(got.Body.String(), "<table") || strings.Contains(got.Body.String(), "matching") || strings.Contains(got.Body.String(), "directory-status") || strings.Contains(got.Body.String(), "directory-export") || !strings.Contains(got.Body.String(), "journey-control") || !strings.Contains(got.Body.String(), `data-coverage="unavailable"`) {
			t.Fatalf("unavailable %s = %d: %q", target, got.Code, got.Body.String())
		}
	}
	for _, target := range []string{
		"/directory?page=0", "/directory?page=two", "/directory?page=3&q=engineer", "/directory?state=bad",
		"/directory.csv?page=999", "/directory.csv?state=partial&page=3",
		"/directory?theme=sepia", "/directory?q=a&q=b", "/directory?unknown=x", "/directory?q=%00", "/directory?q=%0Aengineer", "/directory?q=engineer%0A", "/directory?%zz",
	} {
		if got := journeyRequest(h, http.MethodGet, target, nil); got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "Reset directory") || strings.Contains(got.Body.String(), "directory-status") || strings.Contains(got.Body.String(), "directory-export") {
			t.Fatalf("malformed %s = %d", target, got.Code)
		}
	}
	if got := journeyRequest(h, http.MethodGet, "/directory/missing?q=engineer", nil); got.Code != http.StatusNotFound || !strings.Contains(got.Body.String(), "Back to results") || strings.Contains(got.Body.String(), "directory-status") {
		t.Fatalf("unknown syntactically valid detail = %d", got.Code)
	}
}

func TestDirectoryCSVQuotesAndFormulaEscapes(t *testing.T) {
	h := journeyHandler(t)
	got := journeyRequest(h, http.MethodGet, "/directory.csv", nil)
	if got.Code != http.StatusOK {
		t.Fatal(got.Code)
	}
	body := got.Body.String()
	if !strings.Contains(body, `"Kai ""Quoted"", Vale"`) || !strings.Contains(body, ",'=1+1,") {
		t.Fatalf("CSV quoting/formula safety = %q", body)
	}
	if strings.Contains(journeyRequest(h, http.MethodGet, "/directory", nil).Body.String(), "'=1+1") {
		t.Fatal("spreadsheet encoding changed displayed source text")
	}
	filtered := journeyRequest(h, http.MethodGet, "/directory.csv?q=%3D1", nil)
	if filtered.Code != http.StatusOK {
		t.Fatal(filtered.Code)
	}
	parsed, err := csv.NewReader(strings.NewReader(filtered.Body.String())).ReadAll()
	if err != nil || len(parsed) != 2 || parsed[1][2] != "'=1" || parsed[1][3] != "p12" {
		t.Fatalf("formula query CSV = %#v, %v", parsed, err)
	}
	view := journeyRequest(h, http.MethodGet, "/directory?q=%3D1", nil).Body.String()
	if !strings.Contains(view, `value="=1"`) || strings.Contains(view, `value="'=1"`) {
		t.Fatalf("display query changed by CSV safety encoding: %s", view)
	}
}

func TestDirectoryGuardsAssetsHeadAndRenderFailure(t *testing.T) {
	h := journeyHandler(t)
	for _, tc := range []struct {
		target string
		head   http.Header
	}{
		{"/directory", http.Header{"Origin": {"http://127.0.0.1:43130"}}},
		{"/directory", http.Header{"Sec-Fetch-Site": {"cross-site"}}},
		{"/directory", http.Header{"Sec-Fetch-Site": {"same-site"}}},
	} {
		if got := journeyRequest(h, http.MethodGet, tc.target, tc.head); got.Code != http.StatusForbidden {
			t.Fatalf("guard %v = %d", tc.head, got.Code)
		}
	}
	badHost := httptest.NewRequest(http.MethodGet, "/directory", nil)
	badHost.Host, badHost.RemoteAddr = "example.invalid", "127.0.0.1:40000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, badHost)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bad host = %d", w.Code)
	}
	for _, target := range []string{"/assets/style.css", "/assets/journey.css"} {
		got := journeyRequest(h, http.MethodHead, target, nil)
		if got.Code != http.StatusOK || got.Body.Len() != 0 || got.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("HEAD %s = %d body=%d headers=%v", target, got.Code, got.Body.Len(), got.Header())
		}
	}
	if got := journeyRequest(h, http.MethodPost, "/directory", nil); got.Code != http.StatusMethodNotAllowed || got.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d Allow=%q", got.Code, got.Header().Get("Allow"))
	}
	if got := journeyRequest(h, http.MethodGet, "/assets/%73tyle.css", nil); got.Code != http.StatusNotFound {
		t.Fatalf("encoded asset = %d", got.Code)
	}
	if _, err := NewHandler(nil, []byte("x")); err == nil {
		t.Fatal("accepted nil renderer")
	}
	if _, err := NewHandler(journeyRenderer, nil); err == nil {
		t.Fatal("accepted empty CSS")
	}
	failing, err := NewHandler(func(context.Context, View) ([]byte, error) {
		return []byte("<partial>secret</partial>"), errors.New("render failed")
	}, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	failed := journeyRequest(failing, http.MethodGet, "/directory", nil)
	if failed.Code != http.StatusInternalServerError || strings.Contains(failed.Body.String(), "secret") || strings.Contains(failed.Body.String(), "render failed") {
		t.Fatalf("render failure leaked output = %d: %q", failed.Code, failed.Body.String())
	}
}

func TestDirectoryNoInputMutation(t *testing.T) {
	mutating, err := NewHandler(func(_ context.Context, v View) ([]byte, error) {
		v.Rows[0].Name, v.DetailURLs[0] = "mutated", "/mutated"
		return []byte("<section></section>"), nil
	}, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if got := journeyRequest(mutating, http.MethodGet, "/directory", nil); got.Code != http.StatusOK {
		t.Fatal(got.Code)
	}
	if Rows()[0].Name != "Ada Vale" {
		t.Fatal("renderer mutated frozen dataset")
	}
}

func TestDirectoryWrapperHasExplicitThemeAndInputBoundary(t *testing.T) {
	h := journeyHandler(t)
	css := journeyRequest(h, http.MethodGet, "/assets/journey.css", nil).Body.String()
	for _, want := range []string{
		`:root.dark { color: var(--color-white); background: var(--color-gray-900); }`,
		`body { min-height: 100vh; }`,
		`border: 1px solid currentColor`,
		`input.journey-control { display: inline-block; flex: 1 1 16rem; min-width: 0;`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("missing wrapper affordance %q", want)
		}
	}
	page := journeyRequest(h, http.MethodGet, "/directory", nil).Body.String()
	start, end := strings.Index(page, "<form"), strings.Index(page, "</form>")
	if start < 0 || end < start || strings.Contains(page[start:end], "Reset directory") {
		t.Fatal("filters must not duplicate the generated directory reset action")
	}
}

func TestDirectoryRendererGetsScopedView(t *testing.T) {
	var got View
	h, err := NewHandler(func(_ context.Context, v View) ([]byte, error) {
		got = cloneView(v)
		return []byte("<section></section>"), nil
	}, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if response := journeyRequest(h, http.MethodGet, "/directory?q=engineer&page=1&theme=dark", nil); response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	if got.Heading != "Synthetic directory" || got.AddLabel != "Reset directory" || got.EditLabel != "View" || len(got.Rows) != 3 || got.Rows[0].ID != "p01" || got.Rows[2].ID != "p07" || len(got.DetailURLs) != 3 || !strings.Contains(got.DetailURLs[0], "/directory/p01?") || !strings.Contains(got.DetailURLs[0], "q=engineer") || strings.Contains(got.AddURL, "q=engineer") {
		t.Fatalf("renderer view = %#v", got)
	}
}
