package console

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
)

// scoreChanges builds n proven changes for one agent identity.
func scoreChanges(harness, model, effort string, n int, tokens float64, usdEach float64, retriesEvery int, zone string) []ProvenChange {
	var out []ProvenChange
	for i := 0; i < n; i++ {
		c := ProvenChange{Commit: fmt.Sprintf("%s-%d", model, i), At: fmt.Sprintf("2026-09-%02dT10:00:00Z", 1+i%28), Rows: []string{fmt.Sprintf("svc|p.md|%s-%d", model, i)},
			Harness: harness, Model: model, Effort: effort, Agent: agentIdentityKey(harness, model, effort), Known: true, Tokens: tokens, Weight: 1,
			USD: Measured(usdEach, "t"), Rework: Measured(2, "t"), Zones: []ZoneShare{{Zone: zone, Share: 1}}}
		if retriesEvery > 0 && i%retriesEvery == 0 {
			c.Retries = 1
		}
		out = append(out, c)
	}
	return out
}

// WI-9 (§3 U5): per (model, harness, effort) — proven rows, honesty, cost
// per proven row, rework, n; cells below n = 10 greyed; confounders shown.
func TestModelScorecardDescriptive(t *testing.T) {
	changes := append(scoreChanges("claude-code", "opus", "high", 12, 30000, 3, 4, "internal/pay"),
		scoreChanges("codex", "gpt", "medium", 4, 10000, 0.5, 0, "web/src")...)
	m := ModelScorecard(changes)
	if len(m.Cells) != 2 || m.Mode != ScorecardDescriptive {
		t.Fatalf("two cells, descriptive by default: %+v", m)
	}
	opus := m.Cells[0]
	if opus.Agent != "claude-code|opus|high" || opus.N != 12 || opus.ProvenRows != 12 || opus.Greyed {
		t.Fatalf("opus cell: %+v", opus)
	}
	if opus.Honesty.V != 0.2 || opus.USDPerRow.V != 3 || opus.TokensPerRow.V != 30000 || opus.Rework.V != 2 {
		t.Fatalf("opus figures: honesty %+v usd %+v tokens %+v rework %+v", opus.Honesty, opus.USDPerRow, opus.TokensPerRow, opus.Rework)
	}
	if gpt := m.Cells[1]; !gpt.Greyed || gpt.N != 4 {
		t.Fatalf("a cell below n = 10 is greyed: %+v", gpt)
	}
	joined := strings.Join(m.Confounders, " ")
	if !strings.Contains(joined, "zone mix") {
		t.Fatalf("confounders that differ between cells are named: %v", m.Confounders)
	}
	if len(opus.ZoneMix) != 1 || opus.ZoneMix[0].Zone != "internal/pay" || opus.From == "" || opus.To == "" {
		t.Fatalf("each cell shows its zone mix and period: %+v", opus)
	}
}

// WI-10 (§3 U5, law 12): a causal sentence only when G2's admissibility
// test passes for the compared cells; otherwise the page says descriptive.
func TestScorecardCausalOnlyWhenAdmissible(t *testing.T) {
	small := scorecardCausalGate(ModelScorecard(append(scoreChanges("claude-code", "opus", "high", 25, 30000, 3, 0, "a"),
		scoreChanges("codex", "gpt", "medium", 12, 10000, 0.5, 0, "a")...)))
	if small.Mode != ScorecardDescriptive || small.Causal != "" || !strings.Contains(small.Withheld, "G2") {
		t.Fatalf("12 units in one arm: descriptive, the reason named: %+v", small)
	}
	big := scorecardCausalGate(ModelScorecard(append(scoreChanges("claude-code", "opus", "high", 25, 30000, 3, 0, "a"),
		scoreChanges("codex", "gpt", "medium", 22, 10000, 0.5, 0, "a")...)))
	if big.Mode != ScorecardCausal || !strings.Contains(big.Causal, "codex|gpt|medium") || !strings.Contains(big.Causal, "cheaper per proven row") || big.Withheld != "" {
		t.Fatalf("both arms admissible: one causal sentence naming the cheaper identity: %+v", big)
	}
	one := scorecardCausalGate(ModelScorecard(scoreChanges("claude-code", "opus", "high", 30, 1, 1, 0, "a")))
	if one.Mode != ScorecardDescriptive || one.Withheld == "" {
		t.Fatalf("one identity: nothing to compare, said so: %+v", one)
	}
}

// WI-12 (§3 U13): hotspots over T1's stored factors plus the L1 range as
// a named size proxy and the distinct-author count as a bus-factor flag.
func TestHotspotsOverStoredFactors(t *testing.T) {
	risk := certify.RiskReport{Considered: 40, Watermark: "w1", Rows: []certify.RiskRow{
		{Symbol: "pay.Settle", File: "internal/pay/settle.go", Risk: 0.8, Factors: []certify.RiskFactor{
			{Name: "uncovered_share", Value: 1, Cite: "no profile"}, {Name: "centrality", Value: 0.9, Cite: "c"}, {Name: "churn", Value: 0.5, Cite: "ch"}}},
		{Symbol: "store.Open", File: "internal/store/s.go", Risk: 0.3, Factors: []certify.RiskFactor{
			{Name: "uncovered_share", Value: 0.2, Cite: "p"}, {Name: "centrality", Value: 0.4, Cite: "c"}, {Name: "churn", Value: 0.1, Cite: "ch"}}},
	}}
	m := Hotspots(HotspotInputs{Risk: risk,
		Spans:   map[string]SymbolSpan{"pay.Settle": {StartLine: 10, EndLine: 90}},
		Authors: map[string]int{"internal/pay/settle.go": 1, "internal/store/s.go": 4}})
	if len(m.Rows) != 2 || m.SizeProxy == "" || m.Considered != 40 {
		t.Fatalf("hotspots: %+v", m)
	}
	s := m.Rows[0]
	if s.Size.V != 81 || s.Centrality.Value != 0.9 || s.Churn.Value != 0.5 || s.Uncovered.Value != 1 || s.Uncovered.Cite != "no profile" {
		t.Fatalf("stored factors carried with their citations: %+v", s)
	}
	if s.Authors.V != 1 || !s.BusFactorRisk {
		t.Fatalf("a single-author file is a bus-factor flag: %+v", s)
	}
	o := m.Rows[1]
	if o.Size.State != StateAbsent || o.BusFactorRisk || o.Authors.V != 4 {
		t.Fatalf("no span: size absent; four authors: no flag: %+v", o)
	}
	if none := Hotspots(HotspotInputs{Risk: risk}); none.Rows[0].Authors.State != StateAbsent {
		t.Fatalf("authors not read: absent, never 0: %+v", none.Rows[0].Authors)
	}
}

// WI-14 (I58): every work-intelligence and progress view keys rows by
// agent identity only — a human name or email never appears as a row key.
func TestWorkViewsKeyByAgentIdentityOnly(t *testing.T) {
	const email = "jane.doe@example.com"
	const author = "Jane Doe <" + email + ">"
	if k := agentIdentityKey("claude-code", author, email); strings.Contains(k, "Jane") || strings.Contains(k, "@") {
		t.Fatalf("a person is never keyed: %q", k)
	}
	join, h, sessions, commits, z := workFixture()
	join.Outcomes[0].Model = author
	sessions[0].Effort = email
	changes := EffortPerProvenChange(join, h, sessions, commits, z)
	views := []any{changes, InterestRate(changes), ModelScorecard(changes), costPerZone(changes, nil), firstAttemptSplit(changes)}

	hh := History{Readings: []PlanReading{{At: "2026-09-02T10:00:00Z", Root: "/r", Service: "svc", Plan: "p.md", Claimed: 2, Verified: 1}}}
	honest := ClaimHonesty(hh, []AgentSession{{ID: "s", Harness: email, Model: author, Repo: "/r", Start: "2026-09-02T09:00:00Z", End: "2026-09-02T11:00:00Z"}}, HistoryFilter{})
	if len(honest.Rows) != 1 {
		t.Fatalf("honesty row: %+v", honest)
	}
	views = append(views, honest)
	for i, v := range views {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if s := string(b); strings.Contains(s, "Jane") || strings.Contains(s, "@") {
			t.Fatalf("view %d keys or shows a person: %s", i, s)
		}
	}
}

// WI-15 (§4): the nearest-neighbour index holds at most the last 5,000
// proven rows' feature vectors, and is built through a memo group.
func TestRowFeatureIndexBounded(t *testing.T) {
	var changes []ProvenChange
	for i := 0; i < MaxFeatureRows+250; i++ {
		changes = append(changes, ProvenChange{Commit: fmt.Sprintf("c%05d", i), At: fmt.Sprintf("2026-09-01T%02d:%02d:%02dZ", i/3600%24, i/60%60, i%60),
			Known: true, Tokens: float64(1000 + i), Turns: float64(i % 50), Weight: 1, Zones: []ZoneShare{{Zone: "a", Share: 1}}})
	}
	changes = append(changes, ProvenChange{Commit: "unmeasured", At: "2026-09-02T00:00:00Z"})
	ix := rowFeatureIndex(changes)
	if ix.Len() != MaxFeatureRows || ix.Dropped != 250 || ix.Unmeasured != 1 {
		t.Fatalf("bounded to the newest %d measured rows: len %d dropped %d unmeasured %d", MaxFeatureRows, ix.Len(), ix.Dropped, ix.Unmeasured)
	}
	probe := changes[MaxFeatureRows+100]
	nn := ix.Nearest(FeaturesOf(probe), 5)
	if len(nn) != 5 || nn[0].Commit != probe.Commit || nn[0].Distance != 0 {
		t.Fatalf("the nearest neighbour of an indexed row is itself: %+v", nn)
	}
	for i := 1; i < len(nn); i++ {
		if nn[i].Distance < nn[i-1].Distance {
			t.Fatal("neighbours sort by distance")
		}
	}
	if old := ix.Nearest(FeaturesOf(changes[0]), 1); old[0].Commit == "c00000" {
		t.Fatal("the oldest rows were dropped from the index")
	}
	built := 0
	for i := 0; i < 2; i++ {
		got, err := RowFeatureIndexFor(t.Context(), "w1", func() []ProvenChange { built++; return changes[:10] })
		if err != nil || got.Len() != 10 {
			t.Fatalf("memoized build: %v %d", err, got.Len())
		}
	}
	if built != 1 {
		t.Fatalf("the index is built once per watermark, then served from its memo group: built %d", built)
	}
}
