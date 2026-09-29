package review

import (
	"context"
	"errors"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

const adrFixture = "# ADRs\n\n" +
	"## ADR-3 — Layers\n\n" +
	"The `internal/connector` package never imports `internal/query`.\n" +
	"Handlers call services, never stores.\n\n" +
	"## ADR-4 — Unrelated\n\nWe chose SQLite.\n\n" +
	"## ADR-5 — Purity\n\nAnalysis must not import mcp.\n" +
	"The `internal/analysis` packages import neither `internal/mcp` nor `internal/store`.\n"

// TestProposeADRRulesCited (CX-19): each dependency statement becomes a
// proposed deny rule with its ADR, file:line and verbatim quote; prose
// without a dependency statement proposes nothing; nothing is accepted.
func TestProposeADRRulesCited(t *testing.T) {
	got := ProposeADRRules("ADR.md", []byte(adrFixture))
	want := []struct{ from, to, adr, cite string }{
		{"connector", "query", "ADR-3", "ADR.md:5"},
		{"handlers", "stores", "ADR-3", "ADR.md:6"},
		{"analysis", "mcp", "ADR-5", "ADR.md:14"},
		{"analysis", "store", "ADR-5", "ADR.md:15"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d proposals, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		p := got[i]
		if p.FromPkg != w.from || p.ToPkg != w.to || p.Decision != w.adr || p.Citation != w.cite {
			t.Errorf("proposal %d = %+v, want %+v", i, p, w)
		}
		if p.Allow || p.Status != "proposed" || p.Quote == "" || p.Source != "lexical" {
			t.Errorf("proposal %d not a cited, proposed deny rule: %+v", i, p)
		}
	}
	r := RenderRuleProposal(got[0])
	for _, s := range []string{"ADR-3", "ADR.md:5", "never imports", `from_pkg: "connector"`, "status: proposed"} {
		if !strings.Contains(r, s) {
			t.Errorf("rendered proposal lacks %q:\n%s", s, r)
		}
	}
	// Deterministic.
	again := ProposeADRRules("ADR.md", []byte(adrFixture))
	for i := range got {
		if got[i] != again[i] {
			t.Fatal("two runs differ")
		}
	}
}

// TestProposeRuleFromADROnlyWithProvider (CX-23): no provider, no
// proposal; with a provider the chosen rule is proposed, cited, never
// accepted.
func TestProposeRuleFromADROnlyWithProvider(t *testing.T) {
	sec := ADRSection{Decision: "ADR-9", File: "ADR.md", Line: 40, Prose: "Transport stays thin; it hands work to the service layer and never reaches storage.", Packages: []string{"transport", "service", "storage"}}
	ctx := context.Background()

	if p, err := proposeRuleFromADR(ctx, nil, sec); !errors.Is(err, ErrNoDecideProvider) || p != nil {
		t.Errorf("nil decider: %v, %v", p, err)
	}
	if p, err := proposeRuleFromADR(ctx, decision.None{}, sec); !errors.Is(err, ErrNoDecideProvider) || p != nil {
		t.Errorf("none decider: %v, %v", p, err)
	}

	mock := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{
		Provider: "mock",
		Answers:  map[string]decision.Answer{"adr_rule": {ID: "adr_rule", Kind: decision.KindChoice, Choice: "transport -> storage"}},
	}})
	p, err := proposeRuleFromADR(ctx, mock, sec)
	if err != nil || p == nil {
		t.Fatalf("with provider: %v, %v", p, err)
	}
	if p.FromPkg != "transport" || p.ToPkg != "storage" || p.Status != "proposed" || p.Citation != "ADR.md:40" || p.Source != "decide:mock" {
		t.Errorf("proposal = %+v", p)
	}
	calls := mock.Calls()
	if len(calls) != 1 || len(calls[0].Questions) != 1 || calls[0].Questions[0].Kind != decision.KindChoice {
		t.Fatalf("calls = %+v", calls)
	}
	if n := len(calls[0].Questions[0].Options); n != 7 { // 3*2 ordered pairs + none
		t.Errorf("options = %d, want 7", n)
	}

	// "none" chosen: no proposal.
	mock2 := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{
		Provider: "mock", Answers: map[string]decision.Answer{"adr_rule": {ID: "adr_rule", Kind: decision.KindChoice, Choice: "none"}},
	}})
	if p, err := proposeRuleFromADR(ctx, mock2, sec); err != nil || p != nil {
		t.Errorf("none chosen: %v, %v", p, err)
	}
}
