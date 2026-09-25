package receipts

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func reportRequest(t *testing.T, handler http.Handler, method, target string, headers http.Header) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:8765"
	for key, values := range headers {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func newReportHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func assertReportHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	header := response.Header()
	if header.Get("Cache-Control") != "no-store" || header.Get("Referrer-Policy") != "no-referrer" || header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers changed: %v", header)
	}
	if header.Get("Content-Security-Policy") != reportCSP || header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("containment headers changed: %v", header)
	}
}

func TestHTTPAll72OverviewAndDetailSelections(t *testing.T) {
	handler := newReportHandler(t)
	count := 0
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for _, channel := range []string{"all", "app", "mobile"} {
			for _, compare := range []string{"none", "previous"} {
				for _, stale := range []bool{false, true} {
					for _, detail := range []bool{false, true} {
						selected := selection{Period: period, Channel: channel, Compare: compare, Stale: stale}
						if detail {
							selected.DetailID = map[string]string{"all": "atlas", "app": "cedar", "mobile": "beacon"}[channel]
						}
						theme := map[bool]string{false: "light", true: "dark"}[stale]
						target := reportURL("/", selected, theme)
						response := reportRequest(t, handler, http.MethodGet, target, nil)
						if response.Code != http.StatusOK {
							t.Fatalf("GET %s returned %d: %s", target, response.Code, response.Body.String())
						}
						assertReportHeaders(t, response)
						body := response.Body.String()
						for _, want := range []string{
							"<title>Receipts | app-kit</title>", `lang="en"`, `data-theme="` + theme + `"`,
							`data-app="receipts"`, `id="receipts-main"`, `id="receipts-values"`,
							`data-selection-period="` + period + `"`, `data-selection-channel="` + channel + `"`,
							`data-selection-compare="` + compare + `"`, `data-selection-stale="` + strconv.FormatBool(stale) + `"`,
						} {
							if !strings.Contains(body, want) {
								t.Fatalf("GET %s missing stable HTML contract %q", target, want)
							}
						}
						if detail && !strings.Contains(body, `data-selection-product="`+selected.DetailID+`"`) {
							t.Fatalf("GET %s lost detail identity", target)
						}
						if strings.Contains(body, "<script") || strings.Contains(body, "<style") {
							t.Fatalf("GET %s introduced inline executable/style content", target)
						}
						count++
					}
				}
			}
		}
	}
	if count != 72 {
		t.Fatalf("HTTP selection denominator=%d, want 72", count)
	}
}

func TestHTTPExportCarriesLiteralEvidenceAndSelection(t *testing.T) {
	handler := newReportHandler(t)
	selected := selection{Period: "2024-09", Channel: "app", Compare: "previous", Stale: true, DetailID: "cedar"}
	target := reportURL("/export.csv", selected, "dark")
	response := reportRequest(t, handler, http.MethodGet, target, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", target, response.Code, response.Body.String())
	}
	assertReportHeaders(t, response)
	if response.Header().Get("Content-Type") != "text/csv; charset=utf-8" || response.Header().Get("Content-Disposition") != `attachment; filename="receipts-2024-09-app-previous-stale-cedar.csv"` {
		t.Fatalf("CSV response metadata changed: %v", response.Header())
	}
	records, err := csv.NewReader(bytes.NewReader(response.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 7 || strings.Join(records[0][:8], ",") != "series,measure,selection_period,selection_compare,selection_channel,selection_stale,selection_product,product_channel" {
		t.Fatalf("CSV shape changed: rows=%d header=%v", len(records), records[0])
	}
	columns := map[string]int{}
	for index, name := range records[0] {
		columns[name] = index
	}
	want := map[string]string{
		"series": "current", "measure": "receipts", "selection_period": "2024-09", "selection_compare": "previous",
		"selection_channel": "app", "selection_stale": "true", "selection_product": "cedar", "product_channel": "app",
		"id": "cedar", "label": "Cedar", "value": "100", "unit": "USD", "state": "present",
	}
	for key, value := range want {
		if records[1][columns[key]] != value {
			t.Fatalf("literal current Cedar %s=%q, want %q", key, records[1][columns[key]], value)
		}
	}
	if got := records[4][columns["value"]]; got != "200" {
		t.Fatalf("literal previous Cedar receipts=%q, want 200", got)
	}
}

func TestHTTPHeadMatchesGETWithoutBody(t *testing.T) {
	handler := newReportHandler(t)
	for _, target := range []string{
		"/?period=2024-11&channel=all&compare=none&stale=false&theme=dark",
		"/product/atlas?period=2024-09&channel=all&compare=previous&stale=false&theme=light",
		"/export.csv?period=2024-10&channel=all&compare=none&stale=true&theme=light",
		"/assets/app.css",
	} {
		get := reportRequest(t, handler, http.MethodGet, target, nil)
		head := reportRequest(t, handler, http.MethodHead, target, nil)
		if get.Code != http.StatusOK || head.Code != get.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
			t.Fatalf("HEAD %s did not match GET metadata: get=%d/%d head=%d/%d len=%q", target, get.Code, get.Body.Len(), head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
		}
		for _, key := range []string{"Content-Type", "Content-Disposition", "Content-Security-Policy", "Cache-Control"} {
			if head.Header().Get(key) != get.Header().Get(key) {
				t.Fatalf("HEAD %s changed %s", target, key)
			}
		}
	}
}

func TestHTTPRefusesMethodsHostsOriginsAndCrossSiteRequests(t *testing.T) {
	handler := newReportHandler(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		response := reportRequest(t, handler, method, "/", nil)
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s status=%d Allow=%q", method, response.Code, response.Header().Get("Allow"))
		}
	}
	for _, host := range []string{"", "localhost:8765", "example.com", "127.0.0.1.example:8765", "127.0.0.1@evil.example", "127.0.0.1:http", "127.0.0.1:0", "[::1%25lo0]:8765", "[2001:db8::1]:8765"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Host = host
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("host %q returned %d", host, w.Code)
		}
		assertReportHeaders(t, w)
	}
	for _, headers := range []http.Header{
		{"Origin": {"https://example.com"}}, {"Origin": {"null"}}, {"Origin": {"http://127.0.0.1:9999"}},
		{"Origin": {"http://127.0.0.1:8765", "http://127.0.0.1:8765"}}, {"Sec-Fetch-Site": {"cross-site"}},
		{"Sec-Fetch-Site": {"same-site"}}, {"Sec-Fetch-Site": {"none", "same-origin"}},
	} {
		response := reportRequest(t, handler, http.MethodGet, "/", headers)
		if response.Code != http.StatusForbidden {
			t.Fatalf("headers %v returned %d", headers, response.Code)
		}
	}
	for _, headers := range []http.Header{nil, {"Origin": {""}}, {"Origin": {"http://127.0.0.1:8765"}}, {"Sec-Fetch-Site": {"same-origin"}}, {"Sec-Fetch-Site": {"none"}}} {
		if response := reportRequest(t, handler, http.MethodGet, "/", headers); response.Code != http.StatusOK {
			t.Fatalf("same-origin headers %v returned %d", headers, response.Code)
		}
	}
}

func TestHTTPClosedRoutesAndQueries(t *testing.T) {
	handler := newReportHandler(t)
	bad := []struct {
		target string
		status int
	}{
		{"/missing", 404}, {"//", 404}, {"/product", 404}, {"/product/", 404}, {"/product/unknown", 404}, {"/product/%61tlas", 404}, {"/product/atlas/", 404},
		{"/?unknown=x", 400}, {"/?theme=", 400}, {"/?theme=light&theme=dark", 400}, {"/?th%65me=light&theme=dark", 400}, {"/?stale=1", 400},
		{"/?period=2024-08", 400}, {"/?channel=web", 400}, {"/?compare=yes", 400}, {"/?product=atlas", 400}, {"/?theme=light;stale=true", 400},
		{"/product/atlas?channel=mobile", 400}, {"/export.csv?channel=mobile&product=atlas", 400}, {"/assets/app.css?theme=dark", 400},
	}
	for _, item := range bad {
		response := reportRequest(t, handler, http.MethodGet, item.target, nil)
		if response.Code != item.status {
			t.Fatalf("GET %s returned %d, want %d: %s", item.target, response.Code, item.status, response.Body.String())
		}
		assertReportHeaders(t, response)
		if strings.Contains(response.Body.String(), item.target) || !strings.Contains(response.Body.String(), `href="/"`) {
			t.Fatalf("GET %s reflected input or lost fixed recovery: %s", item.target, response.Body.String())
		}
	}
	oversized := "/?theme=" + strings.Repeat("x", maxReportRawQuery)
	if response := reportRequest(t, handler, http.MethodGet, oversized, nil); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized query returned %d", response.Code)
	}
	malformed := httptest.NewRequest(http.MethodGet, "/", nil)
	malformed.Host = "127.0.0.1:8765"
	malformed.URL.RawQuery = "bad=%zz"
	malformedResponse := httptest.NewRecorder()
	handler.ServeHTTP(malformedResponse, malformed)
	if malformedResponse.Code != http.StatusBadRequest || strings.Contains(malformedResponse.Body.String(), "%zz") {
		t.Fatalf("malformed encoding was accepted or reflected: %d %q", malformedResponse.Code, malformedResponse.Body.String())
	}
	for _, target := range []string{
		"/", "/?period=2024-10", "/?period=2024-11&channel=app&compare=none", "/?stale=true",
		"/export.csv?period=2024-10&channel=all&compare=previous&stale=false&theme=dark",
	} {
		if response := reportRequest(t, handler, http.MethodGet, target, nil); response.Code != http.StatusOK {
			t.Fatalf("accepted default/zero/partial/stale GET %s returned %d", target, response.Code)
		}
	}
}

func TestHTTPAbsoluteTargetsAndHeadErrorsDoNotLeakBodies(t *testing.T) {
	handler := newReportHandler(t)
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/", nil)
	r.Host = "127.0.0.1:8765"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("absolute-form request returned %d", w.Code)
	}
	response := reportRequest(t, handler, http.MethodHead, "/?unsafe=%3Cscript%3E", nil)
	if response.Code != http.StatusBadRequest || response.Body.Len() != 0 || response.Header().Get("Content-Length") == "0" {
		t.Fatalf("HEAD error leaked a body or lost representation length: status=%d body=%q headers=%v", response.Code, response.Body.String(), response.Header())
	}
}

func TestHTTPRenderFailureIsBufferedAndDoesNotLeakDiagnostics(t *testing.T) {
	handler := newReportHandler(t).(*reportHandler)
	failing := *handler
	failing.render = func(projection, string) (string, error) {
		return "partial unsafe output", errors.New("private renderer diagnostic")
	}
	response := reportRequest(t, &failing, http.MethodGet, "/", nil)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("renderer failure returned %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "partial unsafe output") || strings.Contains(response.Body.String(), "private renderer diagnostic") || !strings.Contains(response.Body.String(), `href="/"`) {
		t.Fatalf("renderer failure leaked output or lost recovery: %q", response.Body.String())
	}
	if response.Header().Get("Content-Length") != strconv.Itoa(response.Body.Len()) {
		t.Fatalf("renderer error was not fully buffered: %v", response.Header())
	}
}

func TestHTTPConcurrentRequestsDoNotMutateFixture(t *testing.T) {
	handler := newReportHandler(t)
	internal := handler.(*reportHandler)
	before, err := json.Marshal(internal.fixture)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errors := make(chan error, 48)
	for index := 0; index < 48; index++ {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			period := []string{"2024-09", "2024-10", "2024-11"}[index%3]
			channel := []string{"all", "app", "mobile"}[index%3]
			selected := selection{Period: period, Channel: channel, Compare: []string{"none", "previous"}[index%2], Stale: index%2 == 1}
			target := reportURL("/", selected, []string{"light", "dark"}[index%2])
			response := reportRequest(t, handler, http.MethodGet, target, nil)
			if response.Code != http.StatusOK {
				errors <- fmt.Errorf("GET %s returned %d", target, response.Code)
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	after, err := json.Marshal(internal.fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("concurrent requests mutated the embedded fixture")
	}
}

func TestReportURLUsesPathStateAndCanonicalQuery(t *testing.T) {
	selected := selection{Period: "2024-11", Channel: "app", Compare: "none", Stale: true, DetailID: "cedar"}
	if got, want := reportURL("/", selected, "dark"), "/product/cedar?period=2024-11&channel=app&compare=none&stale=true&theme=dark"; got != want {
		t.Fatalf("detail URL=%q, want %q", got, want)
	}
	if got, want := reportURL("/export.csv", selected, "dark"), "/export.csv?period=2024-11&channel=app&compare=none&stale=true&theme=dark&product=cedar"; got != want {
		t.Fatalf("export URL=%q, want %q", got, want)
	}
	selected.DetailID = ""
	if got := reportURL("/", selected, "light"); strings.Contains(got, "product") || !strings.HasPrefix(got, "/?period=") {
		t.Fatalf("overview URL carried detail state: %q", got)
	}
}
