package main

// decide_verbsite_test.go — O1246: the CLI verbs that ask the decision port
// (certify suggest, assess, plans context --preflight --score) ask from
// their own site, so the port's per-site cap (KS-10) and miss kill (KS-12)
// judge them. The provider is a fake transport that answers every question,
// so an ask the miss kill failed to stop would dial it; no socket, no key,
// $0.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// verbSiteMissRows is a decisions.jsonl body whose last 100 asks from site
// hold 20 misses, past KS-12's 10%.
func verbSiteMissRows(site string) string {
	var b strings.Builder
	for i := range 80 {
		fmt.Fprintf(&b, `{"schema_version":2,"site":%q,"provider":"typesafe","model":"jev-test","question_id":"q%d","kind":"predicate","noul":0.7}`+"\n", site, i)
	}
	for range 20 {
		fmt.Fprintf(&b, `{"schema_version":2,"site":%q,"provider":"typesafe","model":"","error":"Post \"https://provider.example.test/v1\": context deadline exceeded","questions":1}`+"\n", site)
	}
	return b.String()
}

// openSiteKilledPort opens a typesafe port over tr whose decisions log
// already holds site's misses; disclose collects the port's lines.
func openSiteKilledPort(t *testing.T, site string, tr http.RoundTripper, disclose io.Writer) *decide.Ports {
	t.Helper()
	t.Setenv("FATHOM_HOME", t.TempDir()) // no kill file
	t.Setenv(decide.KillEnv, "")
	t.Setenv(jgKeyVar, "test-key-jg-not-a-real-key")
	rec := decide.NewRecorder(io.Discard, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
	rec.Seed(strings.NewReader(verbSiteMissRows(site)))
	cfg := config.Decide{Provider: "typesafe", APIKeyEnv: jgKeyVar, Sites: decide.AllSites}
	ports, err := decide.Open(cfg, nil, "", rec, decide.WithHTTPClient(&http.Client{Transport: tr}), decide.WithDisclose(disclose), decide.WithUserConfig(""),
		decide.WithKeyIO(config.KeyIO{
			LookupEnv: func(name string) (string, bool) { return "test-key-jg-not-a-real-key", name == jgKeyVar },
		}))
	if err != nil {
		t.Fatalf("decide.Open: %v", err)
	}
	return ports
}

func TestCLIVerbsAskFromTheirOwnSiteSoTheMissKillRefusesThem(t *testing.T) {
	refused := func(t *testing.T, site string, tr *jgAnswering, disclose *bytes.Buffer, out string) {
		t.Helper()
		if n := tr.calls(); n != 0 {
			t.Errorf("site %s: the provider was dialed %d times past its miss kill\n%s", site, n, out)
		}
		if line := disclose.String(); !strings.Contains(line, "kill_source misses") || !strings.Contains(line, "site "+site) {
			t.Errorf("site %s: the port's line %q, want the miss kill named for site %s", site, line, site)
		}
	}

	t.Run(decide.SiteCertifySuggest, func(t *testing.T) {
		service, storePath := jgIngestCertifyFixture(t)
		tr, disclose := &jgAnswering{}, &bytes.Buffer{}
		ports := openSiteKilledPort(t, decide.SiteCertifySuggest, tr, disclose)
		stdout, stderr, _ := jgCertify(t, ports, service, storePath)
		refused(t, decide.SiteCertifySuggest, tr, disclose, stdout+stderr)
	})

	t.Run(decide.SiteAssess, func(t *testing.T) {
		f := newAssessFixture(t)
		f.enable(t)
		tr, disclose := &jgAnswering{}, &bytes.Buffer{}
		ports := openSiteKilledPort(t, decide.SiteAssess, tr, disclose)
		orig := assessOpenPorts
		assessOpenPorts = func(string, io.Writer) *decide.Ports { return ports }
		t.Cleanup(func() { assessOpenPorts = orig })
		var out, errb bytes.Buffer
		if code := cmdAssess([]string{"--session", f.sid, "--project", f.root}, &out, &errb); code == 0 {
			t.Errorf("a killed site judged a unit: exit 0\n%s", out.String())
		}
		refused(t, decide.SiteAssess, tr, disclose, out.String()+errb.String())
	})

	t.Run(decide.SitePreflight, func(t *testing.T) {
		preflightTestEnv(t)
		storePath := seedPlansFixtureStore(t)
		pf := writeTempPlan(t, "# C\n\n- [ ] A row {id: T-1, symbol: validateCharge}\n")
		tr, disclose := &jgAnswering{}, &bytes.Buffer{}
		ports := openSiteKilledPort(t, decide.SitePreflight, tr, disclose)
		orig := preflightOpenPorts
		preflightOpenPorts = func(io.Writer) *decide.Ports { return ports }
		t.Cleanup(func() { preflightOpenPorts = orig })
		var out, errb bytes.Buffer
		if code := run([]string{"plans", "context", pf, "--row", "T-1", "--service", "payment-service", "--store", storePath, "--score"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		refused(t, decide.SitePreflight, tr, disclose, out.String())
	})
}
