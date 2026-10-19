package main

// doctor_decide_sitekill_test.go — KS-12 and KS-10 at the CLI
// (docs/plans/decision-port.md section J): doctor's decision_provider row
// names a site decisions.jsonl's misses turn off, and leaves the row
// unchanged without one; `fathom config` names the layer whose ask cap is
// in effect. The provider is an httptest fake; no key, no real host.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

func TestDoctorDecisionProviderNamesASiteKilledByMisses(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv(decide.KillEnv, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
	}))
	defer srv.Close()
	t.Setenv("FATHOM_TEST_KS12_DOCTOR_KEY", "good-key")
	dc := config.Decide{Provider: "typesafe", Endpoint: srv.URL, Model: "jev-latest", APIKeyEnv: "FATHOM_TEST_KS12_DOCTOR_KEY"}

	rows := func(site string, hits, misses int) string {
		var b strings.Builder
		for i := range hits {
			fmt.Fprintf(&b, `{"site":%q,"provider":"typesafe","model":"jev-latest","question_id":"q%d"}`+"\n", site, i)
		}
		for range misses {
			fmt.Fprintf(&b, `{"site":%q,"provider":"typesafe","error":"rate limited","questions":1}`+"\n", site)
		}
		return b.String()
	}
	verdict := func(t *testing.T, dc config.Decide, log string) doctorCheck {
		t.Helper()
		dir := t.TempDir()
		callLog := filepath.Join(dir, "calls.jsonl")
		if log != "" {
			if err := os.WriteFile(decide.DefaultRecorderPath(callLog), []byte(log), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return decisionProviderVerdict(config.Loaded{Config: config.Config{Decide: dc}}, nil, callLog)
	}

	base := verdict(t, dc, "")
	if base.Status != doctorPass {
		t.Fatalf("Status = %v, want pass: %+v", base.Status, base)
	}
	onLog := verdict(t, dc, rows(decide.SiteRoute, 90, 10))
	if want := verdict(t, dc, rows(decide.SiteRoute, 100, 0)); onLog.Reason != want.Reason || strings.Contains(onLog.Reason, "killed") {
		t.Errorf("a log with no killed site changed the row:\n got %q\nwant %q", onLog.Reason, want.Reason)
	}

	got := verdict(t, dc, rows(decide.SiteRoute, 89, 11)+rows(decide.SiteAssess, 0, 100))
	if got.Status != doctorPass {
		t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
	}
	for _, want := range []string{"killed by misses: site assess missed 100 of its last 100 asks", "site route missed 11 of its last 100 asks", "never reopens by itself"} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("Reason = %q, want it to hold %q", got.Reason, want)
		}
	}

	denied := dc
	denied.DenySites = []string{decide.SiteRoute}
	if got := verdict(t, denied, rows(decide.SiteRoute, 89, 11)); strings.Contains(got.Reason, "killed by misses") {
		t.Errorf("a denied site was named killed (deny_sites comes first): %q", got.Reason)
	}

	none := verdict(t, config.Decide{Provider: "none"}, rows(decide.SiteRoute, 0, 100))
	if none.Reason != "off ([decide] provider = none)" {
		t.Errorf("provider none's row = %q, want DP-35's bytes", none.Reason)
	}
}

func TestCmdConfig_NamesTheAskCapsLayer(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeUserConfig(t, "[decide]\nmax_asks_per_site_per_session = 50\n")
	writeProjectConfig(t, projectDir, "[decide]\nmax_asks_per_site_per_session = 20\n")
	var stdout, stderr bytes.Buffer
	if code := cmdConfig(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if want := "max_asks_per_site_per_session: 20  [project"; !strings.Contains(stdout.String(), want) {
		t.Errorf("config text lacks %q:\n%s", want, stdout.String())
	}
}
