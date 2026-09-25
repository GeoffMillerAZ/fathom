package main

// console_fixture_test.go — the console's fixture fleet: three fauxtown
// services ingested once per test binary into a store under the hermetic
// HOME (TestMain), served by the real serve-http handler behind an
// httptest server, with a fixed clock and fixture call log, session ledger,
// verify ledger and page-view log. Every console test reads this, so a page
// is always exercised through the same router, registry and projections
// production serves.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// consoleFixtureServices are the fauxtown services the fixture fleet holds.
var consoleFixtureServices = []string{"checkout-service", "ledger-service", "payment-service"}

var consoleFixtureStore struct {
	once sync.Once
	path string
	err  string
}

func fauxtownPath(service string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("..", "..", "internal", "connector", "testdata", "fauxtown", service)
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "internal", "connector", "testdata", "fauxtown", service)
}

// consoleFixtureStorePath ingests the fixture fleet once per test binary.
func consoleFixtureStorePath(t *testing.T) string {
	t.Helper()
	consoleFixtureStore.once.Do(func() {
		dir := filepath.Join(os.Getenv("HOME"), "console-fixture")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			consoleFixtureStore.err = err.Error()
			return
		}
		path := filepath.Join(dir, "fleet.sqlite")
		for _, svc := range consoleFixtureServices {
			var out bytes.Buffer
			if code := run([]string{"ingest", "--service", svc, "--path", fauxtownPath(svc), "--store", path, "--mode", "faux"}, &out, &out); code != 0 {
				consoleFixtureStore.err = "ingest " + svc + ": " + out.String()
				return
			}
		}
		consoleFixtureStore.path = path
	})
	if consoleFixtureStore.err != "" {
		t.Fatal(consoleFixtureStore.err)
	}
	return consoleFixtureStore.path
}

// consoleFixture is one running console over the fixture fleet.
type consoleFixture struct {
	ts        *httptest.Server
	deps      *consoleDeps
	client    *client.Client
	dir       string
	callLog   string
	ledger    string
	viewsLog  string
	now       time.Time
	callRows  int
	anonCalls int
}

// fixture call log: five rows, two naming no service (the rows the survey
// found the fleet tile silently dropping).
const consoleFixtureCallLog = `{"ts":"2026-09-24T10:00:00Z","tool":"find_symbol","service":"payment-service","surface":"agent","latency_ms":3}
{"ts":"2026-09-24T10:01:00Z","tool":"obligations","service":"payment-service","surface":"","latency_ms":5}
{"ts":"2026-09-24T11:00:00Z","tool":"references","service":"checkout-service","surface":"agent","latency_ms":4}
{"ts":"2026-09-25T09:00:00Z","tool":"list_services","service":"","surface":"","latency_ms":1}
{"ts":"2026-09-25T09:30:00Z","tool":"verify_plan","service":"","surface":"","latency_ms":9}
`

// fixture session ledger: three rows, two distinct sessions (the Stop hook
// appends a row per turn; the metric counts sessions, never rows).
const consoleFixtureLedger = `{"schema_version":1,"session_id":"s-1","arm":"on","repo":"payment-service","at":"2026-09-24T10:00:00Z","turn_count":3}
{"schema_version":1,"session_id":"s-1","arm":"on","repo":"payment-service","at":"2026-09-24T10:05:00Z","turn_count":7}
{"schema_version":1,"session_id":"s-2","arm":"on","repo":"checkout-service","at":"2026-09-25T09:00:00Z","turn_count":4}
`

func newConsoleFixture(t *testing.T) *consoleFixture {
	t.Helper()
	dir := hermeticObsEnv(t)
	f := &consoleFixture{dir: dir, now: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), callRows: 5, anonCalls: 2}
	f.callLog = filepath.Join(dir, "mcp-calls.jsonl")
	f.ledger = os.Getenv("FATHOM_SESSION_LEDGER")
	f.viewsLog = filepath.Join(dir, "console-views.jsonl")
	mustWrite(t, f.callLog, consoleFixtureCallLog)
	mustWrite(t, f.ledger, consoleFixtureLedger)
	t.Setenv("FATHOM_CALL_LOG", f.callLog)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(dir, "verify-ledger.jsonl"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(dir, "hooks.jsonl"))
	t.Setenv("FATHOM_CONSOLE_VIEWS", f.viewsLog)

	c, err := client.NewConstantClient(client.ModeLocal, consoleFixtureStorePath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	f.client = c
	var h http.Handler
	f.ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(f.ts.Close)
	f.deps = newConsoleDeps(c, f.ts.URL, f.now)
	f.deps.now = func() time.Time { return f.now }
	// serve-http's own server shape: the fleet backend answers list_services.
	srv := mcp.NewServerWithFleetAndLogger(c, &fleetAdapter{c: c, log: testDiscardLogger()}, nil)
	h = newHTTPHandlerDeps(srv, c, strings.TrimPrefix(f.ts.URL, "http://"), nil, f.deps)
	t.Setenv("FATHOM_HTTP", f.ts.URL)

	prev := fleetReportNow
	fleetReportNow = func() time.Time { return f.now }
	t.Cleanup(func() { fleetReportNow = prev })
	resetObsCache()
	t.Cleanup(resetObsCache)
	return f
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// get fetches path from the fixture without following redirects.
func (f *consoleFixture) get(t *testing.T, path string, header ...string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	hc := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(raw)
}

// pageJSON fetches a page's console envelope.
func (f *consoleFixture) pageJSON(t *testing.T, path string) consolePageJSONWire {
	t.Helper()
	code, _, body := f.get(t, path, "Accept", "application/json")
	if code != http.StatusOK {
		t.Fatalf("GET %s (json) = %d: %s", path, code, body)
	}
	var env consolePageJSONWire
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("GET %s: not the console envelope: %v\n%s", path, err, body)
	}
	return env
}

// consolePageJSONWire decodes the envelope with raw sections and metrics.
type consolePageJSONWire struct {
	Schema   string                     `json:"schema"`
	Page     string                     `json:"page"`
	Route    string                     `json:"route"`
	Title    string                     `json:"title"`
	Group    string                     `json:"group"`
	Question string                     `json:"question"`
	Verbs    []string                   `json:"verbs"`
	Metrics  map[string]json.RawMessage `json:"metrics"`
	Sections map[string]json.RawMessage `json:"sections"`
	Model    json.RawMessage            `json:"model"`
}

// samplePath is a concrete URL for a registered page (parameters filled
// from the fixture fleet).
func samplePath(p *consolePage) string {
	switch p.Route {
	case "/{$}":
		return "/"
	case "/projects/{svc}":
		return "/projects/payment-service"
	case "/sessions/{id}":
		return "/sessions/s-1"
	case "/search":
		return "/search?q=charge"
	}
	return p.Route
}
