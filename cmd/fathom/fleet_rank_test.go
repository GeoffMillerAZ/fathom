package main

// fleet_rank_test.go — RG-4 on the product path: the fleet page's order is
// the one buildObservatoryModel applies through rankFleetByUnrealized, not
// the order the test-only rankByUnrealized computes over reports.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildObservatoryModel_RanksTheFleet pins RG-4's product wiring: the
// rows buildObservatoryModel returns are in rankFleetByUnrealized's order,
// not the registry's name order. With no session ledger every row scores 0,
// so the traffic key decides: beta (two calls in the log) before alpha
// (none), where the name order would put alpha first.
func TestBuildObservatoryModel_RanksTheFleet(t *testing.T) {
	dir := hermeticObsEnv(t)
	callLog := filepath.Join(dir, "calls.jsonl")
	rows := `{"ts":"2026-08-25T10:00:00Z","tool":"find_symbol","service":"beta"}` + "\n" +
		`{"ts":"2026-08-25T10:01:00Z","tool":"search","service":"beta"}` + "\n"
	if err := os.WriteFile(callLog, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_CALL_LOG", callLog)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rpc") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[` +
			`{"service":"alpha","symbol_count":3,"languages":["go"]},` +
			`{"service":"beta","symbol_count":3,"languages":["go"]}],"count":2}}`))
	}))
	defer srv.Close()

	m := buildObservatoryModel(srv.URL, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if m.DegradedNote != "" {
		t.Fatalf("registry answered, DegradedNote = %q", m.DegradedNote)
	}
	if len(m.Services) != 2 {
		t.Fatalf("services = %d, want 2", len(m.Services))
	}
	for _, r := range m.Services {
		if s := unrealizedScore(r); s != 0 {
			t.Fatalf("no session ledger: %s must score 0, got %d", r.Service, s)
		}
	}
	got := m.Services[0].Service + " " + m.Services[1].Service
	if got != "beta alpha" || m.Services[0].CallsTotal != 2 || m.Services[1].CallsTotal != 0 {
		t.Fatalf("fleet order = %q (calls %d, %d), want \"beta alpha\" (calls 2, 0): the model must be ranked", got, m.Services[0].CallsTotal, m.Services[1].CallsTotal)
	}
}

// fleetRow is a fleet-page row with a realization ladder (nil when the
// ladder could not be built) and a call total.
func fleetRow(svc, path string, realized, available, sessions, calls int, ladder bool) obsServiceRow {
	r := obsServiceRow{}
	r.Service, r.Path, r.CallsTotal = svc, path, calls
	if ladder {
		rr := realizationReport{Service: svc, Realized: realized, Available: available, Sessions: sessions}
		r.Realization = &rr
	}
	return r
}

func fleetOrder(rows []obsServiceRow) string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Service + "@" + r.Path
	}
	return strings.Join(out, " ")
}

// TestFleetPageRanksByUnrealizedValue pins RG-4's live ordering: unrealized
// value (unmet rungs × sessions in the window) descending, then calls in
// the window descending, then the service name; stable under equal keys,
// and a row whose ladder could not be built is ranked (score 0), never
// dropped.
func TestFleetPageRanksByUnrealizedValue(t *testing.T) {
	rows := []obsServiceRow{
		fleetRow("perfect", "/r/perfect", 5, 5, 900, 99999, true), // busiest, nothing unrealized: score 0
		fleetRow("small", "/r/small", 2, 5, 10, 5, true),          // 3 × 10 = 30, calls 5: after the two 30s with 7
		fleetRow("noladder", "/r/noladder", 0, 0, 0, 500, false),  // nil Realization: score 0, calls 500
		fleetRow("busy", "/r/busy", 3, 5, 100, 1, true),           // 2 × 100 = 200
		fleetRow("tieb", "/r/tieb", 4, 5, 30, 7, true),            // 1 × 30 = 30, calls 7
		fleetRow("tiea", "/r/tiea", 4, 5, 30, 7, true),            // 1 × 30 = 30, calls 7: name breaks the tie
		fleetRow("quiet", "/r/quiet", 5, 5, 0, 0, true),           // score 0, calls 0
	}
	n := len(rows)
	rankFleetByUnrealized(rows)
	want := "busy@/r/busy tiea@/r/tiea tieb@/r/tieb small@/r/small perfect@/r/perfect noladder@/r/noladder quiet@/r/quiet"
	if got := fleetOrder(rows); got != want {
		t.Fatalf("fleet order:\n got %s\nwant %s\n(unrealized desc, then calls desc, then name)", got, want)
	}
	if len(rows) != n {
		t.Fatalf("ranking dropped rows: %d of %d", len(rows), n)
	}

	t.Run("a row with no ladder is kept and ranked on the traffic key", func(t *testing.T) {
		rows := []obsServiceRow{
			fleetRow("gap", "/r/gap", 4, 5, 1, 0, true),          // score 1
			fleetRow("unknown", "/r/unknown", 0, 0, 0, 3, false), // nil: score 0, calls 3
			fleetRow("idle", "/r/idle", 5, 5, 0, 9, true),        // score 0, calls 9
		}
		rankFleetByUnrealized(rows)
		if got, want := fleetOrder(rows), "gap@/r/gap idle@/r/idle unknown@/r/unknown"; got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
		if rows[2].Realization != nil {
			t.Fatalf("the nil-ladder row must keep its nil ladder: %+v", rows[2].Realization)
		}
	})

	t.Run("deterministic whatever the input order, stable under equal keys", func(t *testing.T) {
		// Two rows equal on every key (same name, score and calls) keep
		// their input order; the rest land the same from any permutation.
		base := []obsServiceRow{
			fleetRow("same", "/first", 4, 5, 10, 2, true),
			fleetRow("same", "/second", 4, 5, 10, 2, true),
			fleetRow("zeta", "/r/zeta", 3, 5, 10, 1, true),
			fleetRow("alpha", "/r/alpha", 5, 5, 10, 1, true),
		}
		want := "zeta@/r/zeta same@/first same@/second alpha@/r/alpha"
		perms := [][]int{{0, 1, 2, 3}, {3, 2, 0, 1}, {2, 0, 3, 1}, {0, 3, 1, 2}}
		for _, p := range perms {
			rows := make([]obsServiceRow, 0, len(p))
			for _, i := range p {
				rows = append(rows, base[i])
			}
			rankFleetByUnrealized(rows)
			if got := fleetOrder(rows); got != want {
				t.Fatalf("permutation %v: got %s, want %s", p, got, want)
			}
		}
		// Equal keys in the other input order keep that order: the sort is
		// stable, not name-then-path.
		rows := []obsServiceRow{base[1], base[0]}
		rankFleetByUnrealized(rows)
		if got, want := fleetOrder(rows), "same@/second same@/first"; got != want {
			t.Fatalf("equal keys must keep their input order: got %s, want %s", got, want)
		}
	})
}
