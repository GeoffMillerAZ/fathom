package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// statuszServer is a canned daemon: list_services for the verify probe,
// verify_plan returning one claimed-and-passing row, and /statusz with the
// given build stamp (schema 2) or a v1 body when commit == "".
func statuszServer(t *testing.T, commit string, dirty bool, indexed string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/statusz") {
			w.Header().Set("Content-Type", "application/json")
			if commit == "" {
				_, _ = w.Write([]byte(`{"generated_at":"2026-09-01T00:00:00Z","uptime_seconds":1,"channels_note":"","services":[{"service":"svc","indexed_commit":"` + indexed + `"}]}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"schema_version": statuszSchemaVersion,
				"binary_commit":  commit,
				"binary_dirty":   dirty,
				"built_at":       "2026-09-01T00:00:00Z",
				"services":       []map[string]any{{"service": "svc", "indexed_commit": indexed}},
			})
			return
		}
		body, _ := io_ReadAll(r)
		if strings.Contains(body, `"list_services"`) {
			writeRPCText(w, `{"services":[{"service":"svc","symbol_count":42}]}`)
			return
		}
		inner := map[string]any{
			"service": "svc",
			"summary": map[string]any{"total": 1, "pass": 1, "fail": 0, "unknown": 0},
			"tasks": []any{map[string]any{
				"id": "md-001", "title": "t", "status": "pass", "label": "verified", "claimed": true,
				"criteria": []any{map[string]any{"name": "symbol_present", "status": "pass", "detail": "ok"}},
			}},
		}
		raw, _ := json.Marshal(inner)
		writeRPCText(w, string(raw))
	}))
}

func io_ReadAll(r *http.Request) (string, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r.Body)
	return b.String(), err
}

// TestVerifyLedgerProvenance_HTTP pins PC-2: the ledger row written by an
// HTTP-mode verify carries the daemon's stamp and the service watermark.
func TestVerifyLedgerProvenance_HTTP(t *testing.T) {
	ts := statuszServer(t, "6c6f4377b53aad47cf6ab86224721df8592cadf0", false, "3f0c3b1deadbeef")
	defer ts.Close()
	ledger := filepath.Join(t.TempDir(), "v.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", ledger)

	plan := writeTempPlan(t, "- [x] claimed work {symbol: Foo}\n")
	var out, errBuf bytes.Buffer
	if code := run([]string{"verify", "--service", "svc", "--url", ts.URL, "--plan", plan}, &out, &errBuf); code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, errBuf.String())
	}
	rows := readVerifyLedger()
	if len(rows) != 1 {
		t.Fatalf("ledger rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.SchemaVersion != verifyLedgerSchemaVersion {
		t.Errorf("schema_version = %d, want %d", r.SchemaVersion, verifyLedgerSchemaVersion)
	}
	if r.DaemonCommit != "6c6f4377b53aad47cf6ab86224721df8592cadf0" {
		t.Errorf("daemon_commit = %q", r.DaemonCommit)
	}
	if r.StoreCommit != "3f0c3b1deadbeef" {
		t.Errorf("store_commit = %q", r.StoreCommit)
	}
	if r.BinaryCommit != buildinfo.Version() {
		t.Errorf("binary_commit = %q, want this binary's %q", r.BinaryCommit, buildinfo.Version())
	}
	// The daemon's commit is not this test binary's: exactly one
	// provenance line, on stderr, naming both stamps (PC-3).
	lines := 0
	for _, l := range strings.Split(errBuf.String(), "\n") {
		if strings.HasPrefix(l, "provenance:") {
			lines++
			if !strings.Contains(l, "6c6f4377b53a") {
				t.Errorf("provenance line lacks the daemon stamp: %q", l)
			}
		}
	}
	if lines != 1 {
		t.Errorf("provenance lines = %d, want exactly 1; stderr: %s", lines, errBuf.String())
	}
}

// TestVerifyLedgerProvenance_Store pins the store-mode half: store_commit
// is the service_meta watermark, daemon_commit stays empty, no notice.
func TestVerifyLedgerProvenance_Store(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	st, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: "/x", IndexedCommit: "abc123def456"}); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	p := verifyLedgerProvenance(nil, "", storePath, "svc")
	if p.StoreCommit != "abc123def456" {
		t.Errorf("StoreCommit = %q", p.StoreCommit)
	}
	if p.DaemonCommit != "" || p.DaemonKnown {
		t.Errorf("store mode must not claim a daemon: %+v", p)
	}
	if n := provenanceNotice(p); n != "" {
		t.Errorf("store mode notice = %q, want none", n)
	}
}

// TestProvenanceNotice pins PC-3's three states: same binary → silent;
// different → one line naming both; unknown (v1 daemon) → one line saying
// unknown. Never a comparison to HEAD: the function takes no repo.
func TestProvenanceNotice(t *testing.T) {
	same := verifyProvenance{DaemonKnown: true, DaemonCommit: "6c6f4377b53aad47cf6ab86224721df8592cadf0", BinaryCommit: "6c6f4377b53a-dirty"}
	if n := provenanceNotice(same); n != "" {
		t.Errorf("same revision (dirty CLI) must be silent, got %q", n)
	}
	diff := verifyProvenance{DaemonKnown: true, DaemonCommit: "6c6f4377b53aad47cf6ab86224721df8592cadf0", BinaryCommit: "3f0c3b1deadb", StoreCommit: "abcdef"}
	n := provenanceNotice(diff)
	if !strings.Contains(n, "6c6f4377b53a") || !strings.Contains(n, "3f0c3b1deadb") || !strings.Contains(n, "abcdef") {
		t.Errorf("notice must name daemon, binary and watermark: %q", n)
	}
	if strings.Count(n, "\n") != 0 {
		t.Errorf("notice must be one line: %q", n)
	}
	unknown := verifyProvenance{Note: "daemon build stamp unknown: its /statusz is schema v0 without binary_commit", BinaryCommit: "3f0c3b1"}
	if n := provenanceNotice(unknown); !strings.Contains(n, "unknown") {
		t.Errorf("unknown daemon must say so: %q", n)
	}
}

// TestFetchStatusz_V1DaemonIsUnknownNeverFresh: a daemon whose /statusz
// predates schema 2 yields DaemonKnown=false, and the ledger row's
// daemon_commit stays empty rather than inventing a match.
func TestFetchStatusz_V1DaemonIsUnknownNeverFresh(t *testing.T) {
	ts := statuszServer(t, "", false, "w1")
	defer ts.Close()
	p := verifyLedgerProvenance(&http.Client{}, ts.URL, "", "svc")
	if p.DaemonKnown || p.DaemonCommit != "" {
		t.Errorf("v1 daemon must be unknown: %+v", p)
	}
	if p.StoreCommit != "w1" {
		t.Errorf("v1 daemon still reports the watermark: %+v", p)
	}
	if !strings.Contains(p.Note, "schema v0") {
		t.Errorf("note must name the schema: %q", p.Note)
	}
	_ = os.Stdout
}
