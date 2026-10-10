package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// fixedClock returns a now func() time.Time that advances by step on every
// call, starting at t0 — deterministic latency measurement (repo
// convention: no bare time.Now() in logic under test).
func fixedClock(t0 time.Time, step time.Duration) func() time.Time {
	cur := t0
	first := true
	return func() time.Time {
		if first {
			first = false
			return cur
		}
		cur = cur.Add(step)
		return cur
	}
}

func TestCmdSmoke_RequiresProfileOrAll(t *testing.T) {
	withIsolatedConfigHome(t)
	var stdout, stderr bytes.Buffer
	code := cmdSmoke(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected nonzero exit when neither --profile nor --all is given")
	}
	if !strings.Contains(stderr.String(), "--profile") || !strings.Contains(stderr.String(), "--all") {
		t.Errorf("stderr = %s, want it to mention both flags", stderr.String())
	}
}

func TestSmokeProfiles_ClaudeCLI_ReportsUsageAndCost(t *testing.T) {
	loaded := config.Loaded{Config: config.Config{Profiles: map[string]config.Profile{
		"work": {Provider: config.ProviderClaudeCLI, Bin: newFakeClaudeCLI(t)},
	}}}
	clock := fixedClock(time.Unix(1000, 0), 50*time.Millisecond)

	rows := smokeProfiles(loaded, []string{"work"}, clock)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if !r.OK {
		t.Fatalf("row not OK: %+v", r)
	}
	if r.TokensIn != 1 || r.TokensOut != 1 {
		t.Errorf("tokens = in=%d out=%d, want 1/1 (fakeClaudeCLIScript's canned usage)", r.TokensIn, r.TokensOut)
	}
	if r.LatencyMS != 50 {
		t.Errorf("LatencyMS = %d, want 50 (fixed clock step)", r.LatencyMS)
	}
	wantCost := float64(1)/1000*smokeInRatePerKTok + float64(1)/1000*smokeOutRatePerKTok
	if r.CostUSD != wantCost {
		t.Errorf("CostUSD = %v, want %v", r.CostUSD, wantCost)
	}
	if r.Detail != "pong" {
		t.Errorf("Detail = %q, want the completion text", r.Detail)
	}
}

func TestSmokeProfiles_ConstructionFailure_NotOK(t *testing.T) {
	loaded := config.Loaded{Config: config.Config{Profiles: map[string]config.Profile{
		"broken": {Provider: config.ProviderAzureOpenAI}, // missing endpoint/deployment
	}}}
	rows := smokeProfiles(loaded, []string{"broken"}, time.Now)
	if len(rows) != 1 || rows[0].OK {
		t.Fatalf("rows = %+v, want one failing row", rows)
	}
	if rows[0].Detail == "" {
		t.Error("Detail empty, want the construction error")
	}
}

func TestCmdSmoke_All_JSON(t *testing.T) {
	withIsolatedConfigHome(t)
	claudeBin := newFakeClaudeCLI(t)
	// A profile's bin is user-only (SEC-2): the user config names it.
	writeUserConfig(t, `
[profiles.work]
provider = "claude-cli"
bin = "`+escapeTOML(claudeBin)+`"
`)
	var stdout, stderr bytes.Buffer
	code := cmdSmoke([]string{"--all", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(rows) != 1 || rows[0]["profile"] != "work" || rows[0]["ok"] != true {
		t.Errorf("rows = %v", rows)
	}
}

func TestCmdSmoke_SingleProfileFlag_UnknownProfile(t *testing.T) {
	withIsolatedConfigHome(t)
	var stdout, stderr bytes.Buffer
	code := cmdSmoke([]string{"--profile", "nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected nonzero exit for an unknown profile")
	}
}
