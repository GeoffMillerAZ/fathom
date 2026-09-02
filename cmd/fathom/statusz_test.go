package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// seedStatuszFixtureStore writes one observation (so ListServiceIDs/
// Services sees the service) and one compile_runs row directly via the
// store — pkg/client.Client exposes no Put-observation wrapper (ingestion
// normally flows through the connector/fusion pipeline, not this path).
func seedStatuszFixtureStore(t *testing.T, path, service, layer string) {
	t.Helper()
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: service, SourceRef: "a.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Foo",
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{
		RunID: "r1", ServiceID: service, Layer: layer, Model: "haiku",
		CostUSD: 0.05, RanAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("RecordCompileRun: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBuildStatuszReport_ServicesListed(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedStatuszFixtureStore(t, storePath, "svc-a", "obligations")

	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()

	report := buildStatuszReport(context.Background(), c, time.Now().Add(-time.Minute), time.Now())
	var found bool
	for _, s := range report.Services {
		if s.Service == "svc-a" {
			found = true
			if s.LastCompilePerLayer["obligations"] == "" {
				t.Errorf("LastCompilePerLayer = %+v, want obligations entry", s.LastCompilePerLayer)
			}
		}
	}
	if !found {
		t.Fatalf("services = %+v, want svc-a listed", report.Services)
	}
	if report.UptimeSec <= 0 {
		t.Errorf("UptimeSec = %v, want > 0", report.UptimeSec)
	}
}

func TestBuildStatuszReport_NoServices_EmptyNotError(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()

	report := buildStatuszReport(context.Background(), c, time.Now(), time.Now())
	if len(report.Services) != 0 {
		t.Fatalf("Services = %+v, want empty", report.Services)
	}
}

func TestBuildStatuszReport_ChannelsOff_NotedInReport(t *testing.T) {
	t.Setenv("FATHOM_CHANNELS", "off")
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()

	report := buildStatuszReport(context.Background(), c, time.Now(), time.Now())
	if report.ChannelsNote == "" {
		t.Fatal("ChannelsNote is empty, want a note naming FATHOM_CHANNELS=off")
	}
}

// ─── HTTP wiring: /statusz on serve-http ───────────────────────────────────

func newStatuszTestHandler(t *testing.T, modulesToken string) http.Handler {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	srv := mcp.NewServer(c)
	return newHTTPHandlerWithAuth(srv, c, modulesToken, "127.0.0.1:0")
}

func TestStatusz_JSON_Loopback_NoAuthRequired(t *testing.T) {
	handler := newStatuszTestHandler(t, "")
	req := httptest.NewRequest(http.MethodGet, "/statusz?format=json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no modules token configured)", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, rec.Body.String())
	}
	if _, ok := payload["generated_at"]; !ok {
		t.Errorf("payload = %+v, want generated_at", payload)
	}
}

// TestStatusz_HostedMode_RequiresBearerToken is the load-bearing
// phase-O-critic-finding-#2-class proof: a request to /statusz with no
// token, modules token configured, gets 401 — not a 200.
func TestStatusz_HostedMode_RequiresBearerToken(t *testing.T) {
	handler := newStatuszTestHandler(t, "s3cr3t")
	req := httptest.NewRequest(http.MethodGet, "/statusz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (hosted mode, no bearer token given)", rec.Code)
	}
}

func TestStatusz_HostedMode_ValidToken_Serves(t *testing.T) {
	handler := newStatuszTestHandler(t, "s3cr3t")
	req := httptest.NewRequest(http.MethodGet, "/statusz?format=json", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (valid bearer token)", rec.Code)
	}
}

func TestStatusz_HTML_DefaultRendering(t *testing.T) {
	handler := newStatuszTestHandler(t, "")
	req := httptest.NewRequest(http.MethodGet, "/statusz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" || ct[:9] != "text/html" {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestStatusz_MethodNotAllowed(t *testing.T) {
	handler := newStatuszTestHandler(t, "")
	req := httptest.NewRequest(http.MethodPost, "/statusz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestStatuszSchemaVersionPinned pins PC-1 (vision 26 §1.1): /statusz
// carries schema_version 2 and the daemon's own build stamp, additively —
// every v1 field is still present under its v1 name. A reader that finds
// no binary_commit is talking to a v1 daemon and must say "unknown", so the
// field is always emitted (empty string, never omitted).
func TestStatuszSchemaVersionPinned(t *testing.T) {
	if statuszSchemaVersion != 2 {
		t.Fatalf("statuszSchemaVersion = %d, want 2 — bump deliberately and update this pin", statuszSchemaVersion)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()
	report := buildStatuszReport(context.Background(), c, time.Now().Add(-time.Second), time.Now())
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "generated_at", "uptime_seconds", "channels_note", "services", "binary_commit", "binary_dirty"} {
		if _, ok := m[key]; !ok {
			t.Errorf("/statusz JSON lacks %q: %s", key, raw)
		}
	}
	if got := m["schema_version"]; got != float64(2) {
		t.Errorf("schema_version = %v, want 2", got)
	}
	// The stamp is whatever this test binary carries; the contract is that
	// it comes from internal/buildinfo, not from git or the cwd (O5).
	if report.BinaryCommit != buildinfo.Read().Revision {
		t.Errorf("binary_commit = %q, want buildinfo.Read().Revision %q", report.BinaryCommit, buildinfo.Read().Revision)
	}
}

func TestStatuszBinaryLabel(t *testing.T) {
	if got := statuszBinaryLabel(StatuszBuildInfo{}); got != "unstamped (built without -buildvcs)" {
		t.Errorf("empty stamp label = %q", got)
	}
	got := statuszBinaryLabel(StatuszBuildInfo{BinaryCommit: "6c6f4377b53aad47cf6ab86224721df8592cadf0", BinaryDirty: true, BuiltAt: "2026-09-01T00:00:00Z"})
	if got != "6c6f4377b53a-dirty built 2026-09-01T00:00:00Z" {
		t.Errorf("label = %q", got)
	}
}
