package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// resetObsCache forces the next cachedObservatoryModel call to rebuild.
func resetObsCache() {
	obsCache.Lock()
	obsCache.at = time.Time{}
	obsCache.Unlock()
}

// hermeticObsEnv points every user-central input at an empty temp dir so a
// developer's real ~/.fathom state never leaks into assertions.
func hermeticObsEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(dir, "arm.json"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(dir, "narrate.json"))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(dir, "ledger.jsonl"))
	resetObsCache()
	return dir
}

// TestBuildObservatoryModel_DegradedIsNamed pins OB-1: an unreachable
// registry and absent instruments degrade to NAMED notes — never zeros,
// never empty-as-healthy.
func TestBuildObservatoryModel_DegradedIsNamed(t *testing.T) {
	hermeticObsEnv(t)
	m := buildObservatoryModel("http://127.0.0.1:1", time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC))

	if m.DegradedNote == "" {
		t.Error("unreachable registry must set DegradedNote")
	}
	if len(m.Services) != 0 {
		t.Errorf("degraded model must carry no service rows, got %d", len(m.Services))
	}
	if m.Arm.Active || m.Arm.Detail == "" {
		t.Errorf("absent arm state must render inactive WITH detail, got %+v", m.Arm)
	}
	if m.Impact.VerifyCatchesTotal != nil {
		t.Error("no ledger: VerifyCatchesTotal must be nil, not zero")
	}
	if !strings.Contains(m.Impact.VerifyCatchesNote, "unknown") && !strings.Contains(m.Impact.VerifyCatchesNote, "not measured") && !strings.Contains(m.Impact.VerifyCatchesNote, "no ledger") {
		t.Errorf("catches note must name the absence, got %q", m.Impact.VerifyCatchesNote)
	}
	if m.Impact.PilotState != "idle" {
		t.Errorf("no OFF-arm rows: pilot state = %q, want idle", m.Impact.PilotState)
	}
}

// TestMetricCell_AbsenceNamed pins OB-2 (law 3).
func TestMetricCell_AbsenceNamed(t *testing.T) {
	if got := metricCell(nil, "instrument absent", ""); !strings.Contains(got, "instrument absent") || strings.Contains(got, ">0<") {
		t.Errorf("nil metric must render its absence note and never a zero: %q", got)
	}
	n := 7
	if got := metricCell(&n, "", "in 2 of 9 sessions"); !strings.Contains(got, ">7<") || !strings.Contains(got, "in 2 of 9 sessions") {
		t.Errorf("present metric must keep its denominator beside the number: %q", got)
	}
	if got := metricCell(&n, "would be wrong here", ""); strings.Contains(got, "would be wrong here") {
		t.Errorf("absence note must never render beside a present value: %q", got)
	}
	if got := metricCell(nil, "", ""); !strings.Contains(got, "not measured") {
		t.Errorf("nil metric with no note must still say not measured: %q", got)
	}
}

// syntheticModel builds a small fleet model for pure-render assertions.
func syntheticModel() observatoryModel {
	open := 2
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "test",
		GeneratedAt:   "2026-08-08T12:00:00Z",
		Arm:           obsArmBanner{Detail: "no measurement arm active"},
		Narrate:       obsArmBanner{Detail: "narrate off (default)"},
	}
	row := obsServiceRow{}
	row.Service = "alpha"
	row.Path = "/tmp/alpha"
	row.Symbols = 123
	row.Freshness = "stale: indexed abc, HEAD def"
	row.FreshnessRemedy = "cd /tmp/alpha && fathom ingest --service alpha"
	row.CallsTotal = 10
	row.CallsAgent = 7
	row.OpenObservations = &open
	m.Services = append(m.Services, row)
	return m
}

// TestObservatoryFleetPage pins OB-4: tiles render with their remedies, and
// the handler serves both HTML and JSON from the same model.
func TestObservatoryFleetPage(t *testing.T) {
	hermeticObsEnv(t)

	body := renderObsFleet(syntheticModel())
	for _, want := range []string{"alpha", "123", "fathom ingest --service alpha", "open observations"} {
		if !strings.Contains(body, want) {
			t.Errorf("fleet page missing %q", want)
		}
	}

	h := observatoryHandler("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /observatory = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "fathom observatory") {
		t.Error("page shell missing")
	}

	resetObsCache()
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory?format=json", nil))
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("json content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"schema_version"`) {
		t.Error("json body missing schema_version")
	}
}

// TestObservatoryServicePage pins OB-5.
func TestObservatoryServicePage(t *testing.T) {
	hermeticObsEnv(t)

	body, ok := renderObsService(syntheticModel(), "alpha")
	if !ok {
		t.Fatal("alpha must resolve")
	}
	for _, want := range []string{"agent-surface share", "7/10", "fathom ingest --service alpha", "<svg"} {
		if !strings.Contains(body, want) {
			t.Errorf("service page missing %q", want)
		}
	}
	if _, ok := renderObsService(syntheticModel(), "nope"); ok {
		t.Error("unknown id must not resolve")
	}

	h := observatoryServicePage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/service?id=ghost", nil))
	if rec.Code != 404 {
		t.Errorf("unknown service = %d, want 404", rec.Code)
	}
}

// TestObservatoryImpactPage pins OB-6: denominators on every count, the
// pilot as a state machine, the lens banner on stories.
func TestObservatoryImpactPage(t *testing.T) {
	dir := hermeticObsEnv(t)
	ledger := filepath.Join(dir, "ledger.jsonl")
	lines := []string{
		`{"schema_version":2,"session_id":"a","arm":"on","at":"2026-08-01T10:00:00Z","tokens_total":100,"tokens_note":"","verify_catches":3}`,
		`{"schema_version":2,"session_id":"b","arm":"on","at":"2026-08-02T10:00:00Z","tokens_total":200,"tokens_note":""}`,
		`{"schema_version":2,"session_id":"c","arm":"off","at":"2026-08-03T10:00:00Z","tokens_total":150,"tokens_note":""}`,
	}
	if err := os.WriteFile(ledger, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Now())
	if imp.LedgerSessions != 3 {
		t.Fatalf("sessions = %d, want 3", imp.LedgerSessions)
	}
	if imp.VerifyCatchesTotal == nil || *imp.VerifyCatchesTotal != 3 {
		t.Errorf("catches = %v, want 3", imp.VerifyCatchesTotal)
	}
	if !strings.Contains(imp.VerifyCatchesNote, "1 of 3") {
		t.Errorf("catches note must carry the denominator, got %q", imp.VerifyCatchesNote)
	}
	if imp.PilotState != "collecting" {
		t.Errorf("pilot state = %q, want collecting", imp.PilotState)
	}
	if imp.TokensOnMedian == nil || *imp.TokensOnMedian != 200 {
		t.Errorf("on median = %v, want 200", imp.TokensOnMedian)
	}
	if imp.TokensOffSessions != 1 {
		t.Errorf("off sessions = %d, want 1", imp.TokensOffSessions)
	}

	m := observatoryModel{Impact: imp}
	body := renderObsImpact(m)
	for _, want := range []string{"law 12", "lens", "1 of 3", "state"} {
		if !strings.Contains(body, want) {
			t.Errorf("impact page missing %q", want)
		}
	}
}

// TestLandingPage pins the front door: GET / links every surface; unknown
// paths still 404 (the "/" mux pattern must not swallow them).
func TestLandingPage(t *testing.T) {
	hermeticObsEnv(t)
	h := landingHandler("http://127.0.0.1:1")

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"/observatory", "/observatory/impact", "/statusz", "/metrics", "/healthz", "/rpc/agent"} {
		if !strings.Contains(body, want) {
			t.Errorf("landing page missing %q", want)
		}
	}

	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/no-such-page", nil))
	if rec.Code != 404 {
		t.Errorf("GET /no-such-page = %d, want 404", rec.Code)
	}
}
