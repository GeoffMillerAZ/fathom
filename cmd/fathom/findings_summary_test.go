package main

// findings_summary_test.go — FL-21 (docs/vision/30-findings-loop.md §3.4):
// GET /findings/summary and the session brief's "open findings: N" line.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestBuildFindingsSummary_CountsServedFindingsAndOldest(t *testing.T) {
	older := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "c1", ServiceID: "svc", Layer: "findings", Kind: "finding", Status: core.ClaimFresh, CompiledAt: newer, Rule: "r1", Subject: "s1"},
		{ClaimID: "c2", ServiceID: "svc", Layer: "findings", Kind: "finding", Status: core.ClaimStale, CompiledAt: older, Rule: "r2", Subject: "s2"},
		// Superseded: never Served() — must not count, must not move oldest.
		{ClaimID: "c3", ServiceID: "svc", Layer: "findings", Kind: "finding", Status: core.ClaimSuperseded, CompiledAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Rule: "r3", Subject: "s3"},
	}
	path := seedFindingsStore(t, nil, claims, nil)
	c, closeFn, err := openFindingsClient(path)
	if err != nil {
		t.Fatalf("openFindingsClient: %v", err)
	}
	defer closeFn()

	out := buildFindingsSummary(context.Background(), c, "svc")
	if out.Count != 2 {
		t.Errorf("Count = %d, want 2 (superseded excluded)", out.Count)
	}
	if out.Oldest != "2026-09-05" {
		t.Errorf("Oldest = %q, want 2026-09-05", out.Oldest)
	}
}

func TestBuildFindingsSummary_ZeroFindings(t *testing.T) {
	path := seedFindingsStore(t, nil, nil, nil)
	c, closeFn, err := openFindingsClient(path)
	if err != nil {
		t.Fatalf("openFindingsClient: %v", err)
	}
	defer closeFn()
	out := buildFindingsSummary(context.Background(), c, "svc")
	if out.Count != 0 || out.Oldest != "" {
		t.Errorf("out = %+v, want zero value", out)
	}
}

func TestFindingsSummaryHandler_ServesJSON(t *testing.T) {
	path := seedFindingsStore(t, nil, []core.SemanticClaim{
		{ClaimID: "c1", ServiceID: "svc", Layer: "findings", Kind: "finding", Status: core.ClaimFresh, CompiledAt: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), Rule: "r1", Subject: "s1"},
	}, nil)
	c, closeFn, err := openFindingsClient(path)
	if err != nil {
		t.Fatalf("openFindingsClient: %v", err)
	}
	defer closeFn()

	srv := httptest.NewServer(findingsSummaryHandler(c))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/findings/summary?service=svc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out findingsSummary
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 || out.Oldest != "2026-09-05" {
		t.Errorf("out = %+v, want Count=1 Oldest=2026-09-05", out)
	}

	// A missing service param is a client error, not a fabricated summary.
	resp2, err := http.Get(srv.URL + "/findings/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a missing service", resp2.StatusCode)
	}
}

func TestFindingsSummaryLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(findingsSummary{Service: "svc", Count: 3, Oldest: "2026-09-02"})
	}))
	defer srv.Close()
	line := findingsSummaryLine(&http.Client{}, srv.URL, "svc")
	if !strings.Contains(line, "open findings: 3") || !strings.Contains(line, "2026-09-02") {
		t.Errorf("line = %q, want it to name the count and oldest date", line)
	}

	zero := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(findingsSummary{Service: "svc", Count: 0})
	}))
	defer zero.Close()
	if got := findingsSummaryLine(&http.Client{}, zero.URL, "svc"); got != "" {
		t.Errorf("zero count must render no line, got %q", got)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	if got := findingsSummaryLine(&http.Client{}, broken.URL, "svc"); got != "" {
		t.Errorf("a failure must omit the line silently, got %q", got)
	}
}
