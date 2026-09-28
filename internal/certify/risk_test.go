package certify

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// riskFixture seeds a store and a corpus root with every T1 factor source:
// stored centrality (TQ-2), stored churn (TQ-1), an enforced guard and a
// fresh obligation claim (TQ-3 severity), and an incident record naming a
// file (TQ-3 incidents).
func riskFixture(t *testing.T) (store.Store, string) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.SetSymbolCentrality(ctx, "svc", "wm1", []store.SymbolCentrality{
		{Symbol: "svc::p.A", File: "p/a.go", Callers: 10, Centrality: 1},
		{Symbol: "svc::p.B", File: "p/b.go", Callers: 5, Centrality: 0.5},
		{Symbol: "svc::p.C", File: "p/c.go", Callers: 0, Centrality: 0},
		{Symbol: "svc::p.TestA", File: "p/a_test.go", Callers: 0, Centrality: 0},
		{Symbol: "svc::p.Helper", File: "p/a_test.go", Callers: 3, Centrality: 0.3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileChurn(ctx, "svc", []store.FileChurn{{Path: "p/a.go", Commits180d: 4}, {Path: "p/b.go", Commits180d: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{{
		ClaimID: "c1", ServiceID: "svc", Layer: "obligations", Kind: "obligation",
		Subject: "CLAUDE.md#rule", Rule: "r", Scope: []string{"p/b.go"}, Status: core.ClaimFresh,
	}}, nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".fathom/guard.cue", `guards: [{
	scopes: ["p/a.go"]
	note: "a is guarded"
	enforce: true
	checks: ["secret_literal"]
}]
`)
	write("docs/intent/incidents.md", "# Incidents\n\n### INC-7 2026-09-01 a broke\nWhat: p/a.go dropped rows\n")
	return st, root
}

// TestRiskWeightedCoverageExplainable proves TQ-4 (docs/plans/
// test-intelligence.md; design doc §3 T1): risk is the product
// uncovered_share x centrality x (1 + churn) x severity x (1 + incidents),
// each factor normalized within the service and shown with its citation;
// the list is sorted by risk and cut at the limit; test code is never a
// risk target; and a symbol absent from a supplied coverage profile is
// treated as uncovered, said so in its citation.
func TestRiskWeightedCoverageExplainable(t *testing.T) {
	st, root := riskFixture(t)
	ctx := context.Background()
	rep, err := RiskWeightedCoverage(ctx, st, "svc", root, RiskOptions{
		Limit:    2,
		Coverage: map[string]float64{"svc::p.A": 0.25},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Watermark != "wm1" || !rep.CoverageSupplied {
		t.Errorf("watermark=%q coverageSupplied=%v", rep.Watermark, rep.CoverageSupplied)
	}
	if rep.Considered != 3 {
		t.Errorf("considered = %d, want 3 (test-file symbols excluded)", rep.Considered)
	}
	if len(rep.Rows) != 2 {
		t.Fatalf("rows = %d, want the limit 2", len(rep.Rows))
	}
	a, b := rep.Rows[0], rep.Rows[1]
	if a.Symbol != "svc::p.A" || b.Symbol != "svc::p.B" {
		t.Fatalf("order = %s, %s; want A then B", a.Symbol, b.Symbol)
	}
	// A: uncovered 0.75, centrality 1, churn 4/4, severity 1.0 (enforced
	// guard), incidents 1/1 -> 0.75 * 1 * 2 * 1 * 2 = 3.
	// B: uncovered 1 (not in the profile), centrality 0.5, churn 2/4,
	// severity 0.5 (obligation), incidents 0 -> 1 * .5 * 1.5 * .5 * 1 = .375.
	if math.Abs(a.Risk-3) > 1e-9 || math.Abs(b.Risk-0.375) > 1e-9 {
		t.Errorf("risk A=%v B=%v, want 3 and 0.375", a.Risk, b.Risk)
	}
	wantNames := []string{"uncovered_share", "centrality", "churn", "severity", "incidents"}
	for _, row := range rep.Rows {
		if len(row.Factors) != len(wantNames) {
			t.Fatalf("%s: %d factors, want %d", row.Symbol, len(row.Factors), len(wantNames))
		}
		prod := 1.0
		for i, f := range row.Factors {
			if f.Name != wantNames[i] {
				t.Errorf("%s factor %d = %s, want %s", row.Symbol, i, f.Name, wantNames[i])
			}
			if f.Cite == "" {
				t.Errorf("%s factor %s has no citation", row.Symbol, f.Name)
			}
			if f.Value < 0 || f.Value > 1 {
				t.Errorf("%s factor %s = %v, not normalized", row.Symbol, f.Name, f.Value)
			}
			if f.Name == "churn" || f.Name == "incidents" {
				prod *= 1 + f.Value
			} else {
				prod *= f.Value
			}
		}
		if math.Abs(prod-row.Risk) > 1e-9 {
			t.Errorf("%s: factors multiply to %v, risk says %v", row.Symbol, prod, row.Risk)
		}
	}
	if c := b.Factors[0].Cite; c != "not in the supplied coverage profile: treated as uncovered" {
		t.Errorf("B uncovered cite = %q", c)
	}
	if c := a.Factors[3].Cite; c != ".fathom/guard.cue: enforced guard p/a.go" {
		t.Errorf("A severity cite = %q", c)
	}
	if c := a.Factors[4].Cite; c != "docs/intent/incidents.md#INC-7" {
		t.Errorf("A incidents cite = %q", c)
	}

	// No coverage supplied: every share is 1, said so.
	rep2, err := RiskWeightedCoverage(ctx, st, "svc", root, RiskOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.CoverageSupplied || rep2.Rows[0].Factors[0].Cite != "no coverage supplied: treated as uncovered" {
		t.Errorf("no-coverage report: %+v", rep2.Rows[0].Factors[0])
	}
}
