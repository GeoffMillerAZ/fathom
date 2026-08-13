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
	for _, want := range []string{"alpha", "123", "fathom ingest --service alpha", "open observations", "cursor-help underline decoration-dotted", "agent calls = queries an AI agent chose to make"} {
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
	if !strings.Contains(rec.Body.String(), "index-only = a remote corpus") {
		t.Error("fleet expander must carry the grade legend, including the index-only line")
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

// gradedFleetModel builds a fleet with one row per grade case the
// 2026-08-08 readable pass distinguishes: A (no remedy needed), B (one
// signal missing, remedy shown), C-with-traffic (remedy shown, not
// index-only — someone queries it, they just haven't fixed the signal),
// and C-zero-everything (index-only: a remote corpus, not a failure).
func gradedFleetModel() observatoryModel {
	m := observatoryModel{
		SchemaVersion: observatorySchemaVersion,
		FathomVersion: "test",
		GeneratedAt:   "2026-08-08T12:00:00Z",
		Arm:           obsArmBanner{Detail: "no measurement arm active"},
		Narrate:       obsArmBanner{Detail: "narrate off (default)"},
	}
	zero := 0
	one := 1

	full := obsServiceRow{}
	full.Service = "fully-wired"
	full.Grade = "A"
	full.CallsTotal, full.CallsAgent = 5, 5
	full.SessionCount = &one
	m.Services = append(m.Services, full)

	partial := obsServiceRow{}
	partial.Service = "one-gap"
	partial.Grade = "B"
	partial.GradeRaise = "cd /tmp/one-gap && fathom ingest --service one-gap"
	partial.CallsTotal, partial.CallsAgent = 3, 2
	partial.SessionCount = &one
	m.Services = append(m.Services, partial)

	queriedButUnwired := obsServiceRow{}
	queriedButUnwired.Service = "queried-c"
	queriedButUnwired.Grade = "C"
	queriedButUnwired.GradeRaise = "fathom fleet reconcile --apply (or: cd /tmp/queried-c && fathom init)"
	queriedButUnwired.CallsTotal, queriedButUnwired.CallsAgent = 9, 4
	queriedButUnwired.SessionCount = &zero
	m.Services = append(m.Services, queriedButUnwired)

	remote := obsServiceRow{}
	remote.Service = "remote-corpus"
	remote.Grade = "C"
	remote.GradeRaise = "fathom fleet reconcile --apply (or: cd /tmp/remote-corpus && fathom init)"
	remote.CallsTotal, remote.CallsAgent = 12, 0
	remote.SessionCount = &zero
	m.Services = append(m.Services, remote)

	m.Misses = []obsMissRow{
		{Tool: "find_symbol", Calls: 1234, Hits: 1200, Misses: 34},
	}
	return m
}

// TestObservatoryFleet_GradeRemedyAndIndexOnly pins the 2026-08-08 pass:
// every non-A card renders its ONE remedy (wiringGrade's own contract),
// EXCEPT a C/D grade with zero ledger sessions AND zero agent calls, which
// renders "index-only" instead of an amber letter and suppresses the
// (moot) remedy — a remote corpus is not a wiring failure.
func TestObservatoryFleet_GradeRemedyAndIndexOnly(t *testing.T) {
	body := renderObsFleet(gradedFleetModel())

	// remedy() HTML-escapes its command (&& becomes &amp;&amp;), so these
	// assertions match either side of the ampersand rather than the raw
	// shell string.
	if !strings.Contains(body, "fathom ingest --service one-gap") {
		t.Error("B-grade card must render its remedy")
	}
	if !strings.Contains(body, "fathom fleet reconcile --apply") || !strings.Contains(body, "cd /tmp/queried-c") {
		t.Error("C-grade card WITH traffic must render its remedy — it is not index-only")
	}
	if !strings.Contains(body, "index-only") {
		t.Error("C-grade card with zero sessions and zero agent calls must render the index-only chip")
	}
	// The index-only service's own remedy command must not appear (it would
	// be suppressed for that specific card, even though the SAME command
	// string is legitimately present for queried-c above) — assert by
	// counting occurrences of the remote-corpus path fragment.
	if strings.Count(body, "cd /tmp/remote-corpus && fathom init") != 0 {
		t.Error("index-only card must not render a remedy line")
	}
}

// TestObservatoryFleet_BarsShowValues pins the tool-load chart carrying
// visible, comma-grouped value labels (ShowValue), not just bar widths.
func TestObservatoryFleet_BarsShowValues(t *testing.T) {
	body := renderObsFleet(gradedFleetModel())
	if !strings.Contains(body, "1,234") {
		t.Errorf("tool-load bars must render the comma-grouped value label; body missing %q", "1,234")
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
	// Rows a/b carry the in/out split (spend-shaped: in+out feeds the
	// median); row d predates the split — its inflated tokens_total must
	// count as pre-split, never as data (the 596M-median finding).
	lines := []string{
		`{"schema_version":2,"session_id":"a","arm":"on","at":"2026-08-01T10:00:00Z","tokens_total":50100,"turn_count":10,"tokens_in":60,"tokens_out":40,"tokens_cache_read":50000,"tokens_note":"","verify_catches":3}`,
		`{"schema_version":2,"session_id":"b","arm":"on","at":"2026-08-02T10:00:00Z","tokens_total":900200,"turn_count":10,"tokens_in":120,"tokens_out":80,"tokens_cache_read":900000,"tokens_note":""}`,
		`{"schema_version":2,"session_id":"c","arm":"off","arm_coverage":"full","at":"2026-08-03T10:00:00Z","tokens_total":150,"turn_count":10,"tokens_in":90,"tokens_out":60,"tokens_note":""}`,
		`{"schema_version":2,"session_id":"d","arm":"on","at":"2026-08-04T10:00:00Z","tokens_total":596931950,"tokens_note":""}`,
	}
	if err := os.WriteFile(ledger, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imp := buildImpactModel(time.Now())
	if imp.LedgerSessions != 4 {
		t.Fatalf("sessions = %d, want 4", imp.LedgerSessions)
	}
	if imp.VerifyCatchesTotal == nil || *imp.VerifyCatchesTotal != 3 {
		t.Errorf("catches = %v, want 3", imp.VerifyCatchesTotal)
	}
	if !strings.Contains(imp.VerifyCatchesNote, "1 of 4") {
		t.Errorf("catches note must carry the denominator, got %q", imp.VerifyCatchesNote)
	}
	if imp.PilotState != "collecting" {
		t.Errorf("pilot state = %q, want collecting", imp.PilotState)
	}
	// Per-turn since the 2026-08-12 unit amendment: a=(100)/10=10, b=(200)/10=20.
	if imp.TokensOnMedian == nil || *imp.TokensOnMedian != 20 {
		t.Errorf("on median = %v, want 20/turn (upper-middle of {10, 20}; the 596M pre-split row must NOT be in the sample)", imp.TokensOnMedian)
	}
	if imp.TokensOffMedian == nil || *imp.TokensOffMedian != 15 {
		t.Errorf("off median = %v, want 15/turn (row c: 150/10)", imp.TokensOffMedian)
	}
	if imp.TokensPreSplitRows == nil || *imp.TokensPreSplitRows != 1 {
		t.Errorf("pre-split rows = %v, want 1 (row d is unknown, never data)", imp.TokensPreSplitRows)
	}
	if imp.CacheReadMedianCtx == nil || *imp.CacheReadMedianCtx == 0 {
		t.Errorf("cache-read context = %v, want a positive average for scale", imp.CacheReadMedianCtx)
	}
	if imp.TokensOffSessions != 1 {
		t.Errorf("off sessions = %d, want 1", imp.TokensOffSessions)
	}
	if imp.UncleanArmRows != 0 {
		t.Errorf("unclean rows = %d, want 0 — every off row here carries full coverage", imp.UncleanArmRows)
	}

	m := observatoryModel{Impact: imp}
	body := renderObsImpact(m)
	for _, want := range []string{"law 12", "lens", "1 of 4", "state", "median real tokens", "pre-split", "cache reads"} {
		if !strings.Contains(body, want) {
			t.Errorf("impact page missing %q", want)
		}
	}

	// The handler serves the same content over HTTP (and JSON on request).
	resetObsCache()
	h := observatoryImpactPage("http://127.0.0.1:1")
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/impact", nil))
	if rec.Code != 200 {
		t.Fatalf("GET /observatory/impact = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "1 of 4") {
		t.Error("handler response must carry the denominator")
	}
	resetObsCache()
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/observatory/impact?format=json", nil))
	if !strings.Contains(rec.Body.String(), `"verify_catches_total"`) {
		t.Error("json mode must serve the impact model")
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
