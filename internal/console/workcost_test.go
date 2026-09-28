package console

import (
	"math"
	"strings"
	"testing"
)

func usd(v float64) *float64 { return &v }

// workFixture: three sessions and four proven changes.
//
//	s1 (claude-code, opus, high) proved A-1 at commit c1 (touching
//	  internal/pay + internal/store) and A-2 at c2 (internal/pay) — its
//	  effort splits across the two changes.
//	s2 (codex, gpt, medium) proved A-3 at c3 (web/src) after one
//	  claimed-unproven reading.
//	s3 (claude-code, opus, high) proved A-4 at c4 (internal/pay), unpriced.
func workFixture() (JoinResult, History, []SessionEffort, map[string]CommitFacts, Zones) {
	h := History{Rows: []RowHistory{
		{Service: "svc", Plan: "p.md", ID: "A-1", Readings: []RowReading{{At: "2026-09-01T00:00:00Z", State: HistOpen}, {At: "2026-09-02T10:00:00Z", State: HistProven, Commit: "c1"}}},
		{Service: "svc", Plan: "p.md", ID: "A-2", Readings: []RowReading{{At: "2026-09-01T00:00:00Z", State: HistOpen}, {At: "2026-09-02T11:00:00Z", State: HistProven, Commit: "c2"}}},
		{Service: "svc", Plan: "p.md", ID: "A-3", Readings: []RowReading{{At: "2026-09-01T00:00:00Z", State: HistOpen}, {At: "2026-09-03T09:00:00Z", State: HistUnproven, Commit: "c3a"}, {At: "2026-09-03T12:00:00Z", State: HistProven, Commit: "c3"}}},
		{Service: "svc", Plan: "p.md", ID: "A-4", Readings: []RowReading{{At: "2026-09-01T00:00:00Z", State: HistOpen}, {At: "2026-09-04T12:00:00Z", State: HistProven, Commit: "c4"}}},
	}}
	join := JoinResult{Outcomes: []SessionOutcome{
		{SessionID: "s1", Harness: "claude-code", Model: "opus", Rows: []ProvenRow{
			{Service: "svc", Plan: "p.md", ID: "A-1", At: "2026-09-02T10:00:00Z", Commit: "c1", By: JoinedByCommit},
			{Service: "svc", Plan: "p.md", ID: "A-2", At: "2026-09-02T11:00:00Z", Commit: "c2", By: JoinedByCommit}}},
		{SessionID: "s2", Harness: "codex", Model: "gpt", Rows: []ProvenRow{
			{Service: "svc", Plan: "p.md", ID: "A-3", At: "2026-09-03T12:00:00Z", Commit: "c3", By: JoinedByWindow}}},
		{SessionID: "s3", Harness: "claude-code", Model: "opus", Rows: []ProvenRow{
			{Service: "svc", Plan: "p.md", ID: "A-4", At: "2026-09-04T12:00:00Z", Commit: "c4", By: JoinedByWindow}}},
	}}
	sessions := []SessionEffort{
		{SessionID: "s1", Harness: "claude-code", Model: "opus", Effort: "high", TokensIn: 30000, TokensOut: 10000, Turns: 40, WallSeconds: 3600, Reverts: 2, USD: usd(4)},
		{SessionID: "s2", Harness: "codex", Model: "gpt", Effort: "medium", TokensIn: 8000, TokensOut: 2000, Turns: 10, WallSeconds: 600, USD: usd(1)},
		{SessionID: "s3", Harness: "claude-code", Model: "opus", Effort: "high", TokensIn: 50000, TokensOut: 10000, Turns: 30, WallSeconds: 1800},
	}
	commits := map[string]CommitFacts{
		"c1": {Commit: "c1", Files: []string{"internal/pay/a.go", "internal/store/s.go"}, ReworkLines: 12, ReworkKnown: true},
		"c2": {Commit: "c2", Files: []string{"internal/pay/b.go"}, ReworkKnown: true},
		"c3": {Commit: "c3", Files: []string{"web/src/app.ts"}},
		"c4": {Commit: "c4", Files: []string{"internal/pay/c.go"}, ReworkLines: 3, ReworkKnown: true},
	}
	return join, h, sessions, commits, NewZones(nil, []string{"web/src"})
}

// WI-4: effort per proven change — tokens, turns, wall time, rework in 7
// days, reverted edits, claimed-unproven retries; a session's effort
// splits evenly across the changes it proved.
func TestEffortPerProvenChange(t *testing.T) {
	join, h, sessions, commits, z := workFixture()
	got := EffortPerProvenChange(join, h, sessions, commits, z)
	if len(got) != 4 {
		t.Fatalf("four proven changes, got %d: %+v", len(got), got)
	}
	c1 := got[0]
	if c1.Commit != "c1" || c1.Tokens != 20000 || c1.Turns != 20 || c1.WallSeconds != 1800 || c1.Reverts != 1 {
		t.Fatalf("s1's effort splits across its two changes: %+v", c1)
	}
	if c1.USD.V != 2 || !c1.USD.Known() {
		t.Fatalf("priced cost splits too: %+v", c1.USD)
	}
	if c1.Rework.V != 12 || !c1.Rework.Known() || len(c1.Zones) != 2 {
		t.Fatalf("rework and zones from the commit: %+v", c1)
	}
	c3 := got[2]
	if c3.Retries != 1 || c3.FirstAttempt() || c3.Rework.Known() || c3.Rework.Reason == "" {
		t.Fatalf("A-3 was claimed-unproven once before proof; its rework is unmeasured, named: %+v", c3)
	}
	if c4 := got[3]; c4.USD.Known() || c4.USD.State != StateAbsent {
		t.Fatalf("an unpriced model's cost is absent, never 0: %+v", c4.USD)
	}
	if got[0].Harness != "claude-code" || got[0].Effort != "high" || got[0].Agent != "claude-code|opus|high" {
		t.Fatalf("agent identity carried: %+v", got[0])
	}
	// A proven change with no session effort record is kept, unmeasured.
	partial := EffortPerProvenChange(join, h, sessions[:1], commits, z)
	if len(partial) != 4 || partial[2].Known {
		t.Fatalf("a change whose session has no effort row is kept and marked unmeasured: %+v", partial)
	}
}

// WI-2: U1 per zone, and CI minutes from a CI summary file bound to a
// commit.
func TestCostPerZoneAndCIMinutes(t *testing.T) {
	join, h, sessions, commits, z := workFixture()
	changes := EffortPerProvenChange(join, h, sessions, commits, z)

	junit := []byte(`<?xml version="1.0"?><testsuites><testsuite name="a" time="90"></testsuite><testsuite name="b" time="30.5"></testsuite></testsuites>`)
	ci, err := ParseCISummary("junit.xml", junit, "c1")
	if err != nil || len(ci) != 1 || ci[0].Commit != "c1" || math.Abs(ci[0].Minutes-120.5/60) > 1e-9 {
		t.Fatalf("JUnit time totals bound to the named commit: %+v %v", ci, err)
	}
	runs := []byte(`{"workflow_runs":[{"head_sha":"c4","run_started_at":"2026-09-04T12:00:00Z","updated_at":"2026-09-04T12:06:00Z"}]}`)
	wr, err := ParseCISummary("runs.json", runs, "")
	if err != nil || len(wr) != 1 || wr[0].Commit != "c4" || wr[0].Minutes != 6 {
		t.Fatalf("workflow-run export: %+v %v", wr, err)
	}
	if _, err := ParseCISummary("junit.xml", junit, ""); err == nil {
		t.Fatal("a JUnit summary with no commit is refused: CI minutes are bound to a commit")
	}

	zc := costPerZone(changes, append(ci, wr...))
	byZone := map[string]ZoneCost{}
	for _, c := range zc {
		byZone[c.Zone] = c
	}
	pay := byZone["internal/pay"]
	// c1 half (10000), c2 whole (20000), c4 whole (60000).
	if pay.N != 3 || pay.Tokens != 90000 || math.Abs(pay.Changes-2.5) > 1e-9 {
		t.Fatalf("internal/pay: %+v", pay)
	}
	if pay.USD.State != StatePartial || pay.USD.V != 3 {
		t.Fatalf("c4 is unpriced, so the zone's cost is a named lower bound: %+v", pay.USD)
	}
	if pay.CIMinutes.State != StatePartial || math.Abs(pay.CIMinutes.V-(120.5/60/2+6)) > 0.01 {
		t.Fatalf("CI minutes split like the change: %+v", pay.CIMinutes)
	}
	if web := byZone["web/src"]; web.CIMinutes.State != StateAbsent {
		t.Fatalf("no CI run for c3: absent, never 0: %+v", web.CIMinutes)
	}
	none := costPerZone(changes, nil)
	if none[0].CIMinutes.State != StateAbsent || !strings.Contains(none[0].CIMinutes.Reason, "CI summary") {
		t.Fatalf("no CI summary named: absent with its reason: %+v", none[0].CIMinutes)
	}
	if zc[0].Zone > zc[len(zc)-1].Zone {
		t.Fatal("zones sort by name")
	}
}

// WI-3: U1 split between first-attempt proofs and proofs after
// claimed-unproven retries.
func TestFirstAttemptSplit(t *testing.T) {
	join, h, sessions, commits, z := workFixture()
	s := firstAttemptSplit(EffortPerProvenChange(join, h, sessions, commits, z))
	if s.Fleet.First.N != 3 || s.Fleet.Retried.N != 1 || s.Fleet.Retried.Tokens != 10000 {
		t.Fatalf("fleet split: %+v", s.Fleet)
	}
	var web AttemptSplit
	for _, zs := range s.Zones {
		if zs.Zone == "web/src" {
			web = zs
		}
	}
	if web.Retried.N != 1 || web.First.N != 0 || web.First.MedianTokens.State != StateAbsent {
		t.Fatalf("web/src was only proven after a retry: %+v", web)
	}
}

// WI-5: U2 — the zone's median effort per proven change over the fleet
// median, with n, the IQR and the model mix.
func TestInterestRateByZone(t *testing.T) {
	join, h, sessions, commits, z := workFixture()
	m := InterestRate(EffortPerProvenChange(join, h, sessions, commits, z))
	if m.Measure == "" || !m.FleetMedian.Known() {
		t.Fatalf("the measure and the fleet median are named: %+v", m)
	}
	// Attributed tokens: pay {10000, 20000, 60000}, store {10000}, web {10000}.
	// Fleet median over the five attributions is 10000.
	if m.FleetMedian.V != 10000 {
		t.Fatalf("fleet median = %v", m.FleetMedian.V)
	}
	var pay ZoneInterest
	for _, zi := range m.Zones {
		if zi.Zone == "internal/pay" {
			pay = zi
		}
	}
	if pay.N != 3 || pay.Rate.V != 2 || pay.Median != 20000 || pay.Q1 != 15000 || pay.Q3 != 40000 {
		t.Fatalf("internal/pay interest: %+v", pay)
	}
	if len(pay.ModelMix) != 1 || pay.ModelMix[0].Agent != "claude-code|opus|high" || pay.ModelMix[0].N != 3 {
		t.Fatalf("model mix names the agent identities behind the zone: %+v", pay.ModelMix)
	}
	if m.Zones[0].Rate.V < m.Zones[len(m.Zones)-1].Rate.V {
		t.Fatal("zones sort by rate, highest first")
	}
	if e := InterestRate(nil); e.FleetMedian.State != StateAbsent || len(e.Zones) != 0 {
		t.Fatalf("no proven change: absent, never a rate of 0: %+v", e)
	}
}
