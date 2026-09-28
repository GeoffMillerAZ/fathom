package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// workChanges: twelve proven changes for one agent identity, two for
// another, across two zones.
func workChanges() []console.ProvenChange {
	h := console.History{}
	join := console.JoinResult{}
	commits := map[string]console.CommitFacts{}
	var sessions []console.SessionEffort
	add := func(sess, harness, model string, n int, zone string) {
		so := console.SessionOutcome{SessionID: sess, Harness: harness, Model: model}
		for i := 0; i < n; i++ {
			c := fmt.Sprintf("%s%02d", sess, i)
			at := fmt.Sprintf("2026-09-%02dT10:00:00Z", 1+i)
			so.Rows = append(so.Rows, console.ProvenRow{Service: "svc", Plan: "p.md", ID: c, At: at, Commit: c})
			h.Rows = append(h.Rows, console.RowHistory{Service: "svc", Plan: "p.md", ID: c, Readings: []console.RowReading{{At: "2026-08-01T00:00:00Z", State: console.HistOpen}, {At: at, State: console.HistProven, Weight: 1}}})
			commits[c] = console.CommitFacts{Commit: c, Files: []string{zone + "/x.go"}, ReworkKnown: true}
		}
		join.Outcomes = append(join.Outcomes, so)
		sessions = append(sessions, console.SessionEffort{SessionID: sess, Harness: harness, Model: model, TokensIn: 1000 * n, TokensOut: 100 * n, Turns: n})
	}
	add("sa", "claude-code", "opus", 12, "internal/pay")
	add("sb", "codex", "gpt", 2, "web")
	return console.EffortPerProvenChange(join, h, sessions, commits, console.NewZones(nil, nil))
}

// WI-11: the scorecard, interest-rate and cost-per-zone pages.
func TestConsoleWorkIntelligencePages(t *testing.T) {
	w := console.WorkIntelligence(workChanges(), nil)
	out := renderWorkIntelligence(w, console.SourceRead{Name: console.SourceVerifyLedger, Path: "/l", State: console.StatePresent}, 0, "files read")
	for _, want := range []string{`id="scorecard"`, `id="interest"`, `id="cost-per-zone"`, "data-scorecard-descriptive", "Descriptive only.",
		"claude-code", "internal/pay", "Interest rate", "CI minutes", "not measured — no CI summary file named", "Confounders:"} {
		if !strings.Contains(out, want) {
			t.Errorf("work intelligence lacks %q", want)
		}
	}
	if !strings.Contains(out, `<span class="text-ink-muted">codex</span>`) {
		t.Error("a cell below n = 10 renders greyed")
	}
	if strings.Contains(out, "data-scorecard-causal") {
		t.Error("12 and 2 units are not G2-admissible: no causal sentence")
	}
	if regexp.MustCompile(`\b(bg|text|border)-(red|green|blue|gray|slate|zinc)-\d`).MatchString(out) || strings.Contains(out, "dark:") {
		t.Error("role utilities only (cmd/fathom/CLAUDE.md)")
	}
	empty := renderWorkIntelligence(console.WorkIntelligence(nil, nil), console.SourceRead{State: console.StatePresent}, 3, "")
	if !strings.Contains(empty, "data-scorecard-empty") || !strings.Contains(empty, "3 proven rows unjoined") {
		t.Errorf("no joined change: every panel is absent with its reason")
	}

	f := newConsoleFixture(t)
	code, _, body := f.get(t, "/work")
	if code != http.StatusOK || !strings.Contains(body, `href="/work?svc=payment-service"`) {
		t.Fatalf("/work offers the repository chooser: %d", code)
	}
	code, _, body = f.get(t, "/work?svc=payment-service")
	if code != http.StatusOK {
		t.Fatalf("/work?svc= = %d: %s", code, body)
	}
	for _, id := range []string{`id="scorecard"`, `id="interest"`, `id="cost-per-zone"`, `id="friction"`, `id="hotspots"`} {
		if !strings.Contains(body, id) {
			t.Errorf("/work?svc=payment-service lacks %s", id)
		}
	}
	env := f.pageJSON(t, "/work?svc=payment-service")
	var m workPageModel
	if err := json.Unmarshal(env.Model, &m); err != nil || env.Page != "work" || m.Service != "payment-service" {
		t.Fatalf("work JSON envelope: %+v %v", env, err)
	}
}

// WI-8: the friction map on the console, remedies as proposals with no
// apply control.
func TestConsoleFrictionMap(t *testing.T) {
	v := console.FrictionWithRemedies(console.FrictionInputs{
		Rules: []console.RuleEvent{
			{Rule: "R-money", Event: "blocked", Session: "s", File: "internal/pay/a.go", At: "2026-09-02T10:00:00Z"},
			{Rule: "R-money", Event: "blocked", Session: "s", File: "internal/pay/a.go", At: "2026-09-02T10:01:00Z"},
		},
		Zones: console.NewZones(nil, nil),
	}, console.RemedyInputs{})
	out := renderFrictionMap(v)
	for _, want := range []string{"data-friction-map", "internal/pay/a.go", "data-remedies", console.PatternRepeatedBlocks, "proposed", "nothing is applied"} {
		if !strings.Contains(out, want) {
			t.Errorf("friction map lacks %q", want)
		}
	}
	if strings.Contains(out, "<button") || strings.Contains(out, "<form") {
		t.Error("remedies are proposals: no apply control (WI-X4)")
	}
	if e := renderFrictionMap(console.FrictionWithRemedies(console.FrictionInputs{}, console.RemedyInputs{})); !strings.Contains(e, "data-friction-empty") {
		t.Error("no signal renders the named empty state")
	}
}

// WI-13: the hotspots view.
func TestConsoleHotspots(t *testing.T) {
	m := console.Hotspots(console.HotspotInputs{
		Risk: certify.RiskReport{Considered: 9, Rows: []certify.RiskRow{{Symbol: "pay.Settle", File: "internal/pay/settle.go", Risk: 0.5,
			Factors: []certify.RiskFactor{{Name: "churn", Value: 0.4}, {Name: "centrality", Value: 0.7}, {Name: "uncovered_share", Value: 1}}}}},
		Spans:   map[string]console.SymbolSpan{"pay.Settle": {StartLine: 3, EndLine: 12}},
		Authors: map[string]int{"internal/pay/settle.go": 1},
	})
	out := renderHotspots(m)
	for _, want := range []string{"data-hotspots", "pay.Settle", "10", "bus-factor risk", "L1 line range", "never named"} {
		if !strings.Contains(out, want) {
			t.Errorf("hotspots lack %q", want)
		}
	}
	if e := renderHotspots(console.Hotspots(console.HotspotInputs{})); !strings.Contains(e, "data-hotspots-empty") {
		t.Error("no ranked symbol renders the named empty state")
	}

	f := newConsoleFixture(t)
	env := f.pageJSON(t, "/work?svc=payment-service")
	var wm workPageModel
	if err := json.Unmarshal(env.Model, &wm); err != nil {
		t.Fatal(err)
	}
	if wm.Hotspots == nil || len(wm.Hotspots.Rows) == 0 || wm.Hotspots.SizeProxy == "" {
		t.Fatalf("/work serves hotspots over the store's stored factors: %+v (err %q)", wm.Hotspots, wm.HotspotsErr)
	}
}
