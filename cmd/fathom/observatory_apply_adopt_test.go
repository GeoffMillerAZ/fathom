package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func TestObservatoryApplyReportsAdoption(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeSettings(t, root, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"fathom stop-hook","timeout":10}]}]}}`))
	target := mutationTarget{Service: "demo", Root: root}
	pv, _, err := previewFor(target)
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	handler := requireLocalMutation(stubRegistry(registeredService{Service: target.Service, Root: root}), wiringApplyHandler("://unavailable", clock))
	form := url.Values{"service": {target.Service}, "plan_digest": {pv.PlanDigest}}
	req := httptest.NewRequest(http.MethodPost, "/observatory/wiring/apply", strings.NewReader(form.Encode()))
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply status=%d: %s", rr.Code, rr.Body.String())
	}
	want := "adopted 1 fathom keys into the ledger"
	if strings.Count(rr.Body.String(), want) != 1 {
		t.Errorf("apply response lacks adoption report: %s", rr.Body.String())
	}
	var out wiringApplyResult
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if html := renderWiringApplied(out); strings.Count(html, want) != 1 {
		t.Errorf("apply page lacks adoption report: %s", html)
	}
}

func TestObservatoryApplyCompletesEmptyHookMigration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	target := mutationTarget{Service: "demo", Root: t.TempDir()}
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	handler := requireLocalMutation(stubRegistry(registeredService{Service: target.Service, Root: target.Root}), wiringApplyHandler("://unavailable", clock))
	apply := func() wiringApplyResult {
		t.Helper()
		pv, _, err := previewFor(target)
		if err != nil {
			t.Fatal(err)
		}
		form := url.Values{"service": {target.Service}, "plan_digest": {pv.PlanDigest}}
		req := httptest.NewRequest(http.MethodPost, "/observatory/wiring/apply", strings.NewReader(form.Encode()))
		req.Host = "localhost"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("apply status=%d: %s", rr.Code, rr.Body.String())
		}
		var out wiringApplyResult
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	apply()
	writeHookLedgerVersion(t, target.Root, 1)
	out := apply()
	ledger, err := wiring.LoadLedger(target.Root)
	if err != nil || ledger.NeedsHookAdoption() {
		t.Errorf("zero-adoption migration not saved: version=%d, err=%v", ledger.Version, err)
	}
	if out.Result.Receipt == nil || len(out.Result.Receipt.Files) != 1 || out.Result.Receipt.Files[0].Path != ".fathom/owned.json" {
		t.Errorf("migration must receipt only the ledger: %+v", out.Result.Receipt)
	}
	if again := apply(); again.Result.Receipt != nil {
		t.Errorf("migration repeated: %+v", again.Result.Receipt)
	}
}
