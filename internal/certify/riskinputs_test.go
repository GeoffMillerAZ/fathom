package certify

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// TestRiskInputsSeverityAndIncidents proves TQ-3 (docs/plans/
// test-intelligence.md; design doc §1 owner decision, §3 T1): governance
// severity per file is 1.0 under an enforced guard, 0.5 under a fresh
// obligation claim, 0.1 otherwise — a stale claim or an advisory guard
// does not raise it — and the incident count per file sums incident
// records whose body names the file and served finding claims scoped to
// it, each cited.
func TestRiskInputsSeverityAndIncidents(t *testing.T) {
	guards := []guardspec.Guard{
		{Scopes: []string{"internal/store/**"}, Enforce: true, Note: "store purity"},
		{Scopes: []string{"internal/mcp/**"}, Enforce: false, Note: "advisory only"},
	}
	claims := []core.SemanticClaim{
		{ClaimID: "c-obl", Kind: "obligation", Subject: "CLAUDE.md#determinism", Scope: []string{"internal/fusion/**"}, Status: core.ClaimFresh},
		{ClaimID: "c-stale", Kind: "obligation", Subject: "CLAUDE.md#old", Scope: []string{"internal/mcp/**"}, Status: core.ClaimStale},
		{ClaimID: "f-open", Kind: "finding", Subject: "finding:race", Scope: []string{"internal/fusion/churn.go"}, Status: core.ClaimFresh},
		{ClaimID: "f-refuted", Kind: "finding", Subject: "finding:nope", Scope: []string{"internal/fusion/churn.go"}, Status: core.ClaimRefuted},
	}
	incidents := []incidentRecord{
		{ID: "INC-3", Line: 10, Body: "What: internal/store/filechurn.go lost rows"},
		{ID: "INC-9", Line: 20, Body: "What: churn.go and internal/fusion/churn.go both"},
	}
	files := []string{"internal/store/filechurn.go", "internal/fusion/churn.go", "internal/mcp/tools.go", "README.md"}
	got := riskInputs(files, guards, claims, incidents)

	sev := map[string]float64{}
	inc := map[string]int{}
	for f, r := range got {
		sev[f], inc[f] = r.Severity, r.Incidents
		if r.SeverityCite == "" {
			t.Errorf("%s: severity has no citation", f)
		}
		if len(r.IncidentCites) != r.Incidents {
			t.Errorf("%s: %d incidents but %d citations", f, r.Incidents, len(r.IncidentCites))
		}
	}
	wantSev := map[string]float64{
		"internal/store/filechurn.go": 1.0,
		"internal/fusion/churn.go":    0.5,
		"internal/mcp/tools.go":       0.1, // advisory guard + stale claim only
		"README.md":                   0.1,
	}
	if !reflect.DeepEqual(sev, wantSev) {
		t.Errorf("severity = %v, want %v", sev, wantSev)
	}
	wantInc := map[string]int{
		"internal/store/filechurn.go": 1, // INC-3
		"internal/fusion/churn.go":    2, // INC-9 + the open finding (the refuted one never counts)
		"internal/mcp/tools.go":       0,
		"README.md":                   0,
	}
	if !reflect.DeepEqual(inc, wantInc) {
		t.Errorf("incidents = %v, want %v", inc, wantInc)
	}
	if c := got["internal/fusion/churn.go"].IncidentCites; len(c) != 2 || c[0] != "docs/intent/incidents.md#INC-9" {
		t.Errorf("churn.go incident cites = %v", c)
	}
}

// TestIncidentRecordsSplitsSections: each "### INC-N" heading's section
// runs to the next heading of the same or higher level.
func TestIncidentRecordsSplitsSections(t *testing.T) {
	body := "# Incidents\n\n## Open\n\n### INC-1 a\nWhat: a.go\n\n### INC-2 b\nWhat: b.go\n\n## Closed\nc.go\n"
	got := parseIncidentRecords([]byte(body))
	if len(got) != 2 || got[0].ID != "INC-1" || got[1].ID != "INC-2" {
		t.Fatalf("records = %+v", got)
	}
	if got[0].Line != 5 {
		t.Errorf("INC-1 line = %d, want 5", got[0].Line)
	}
	if !strings.Contains(got[0].Body, "a.go") || strings.Contains(got[0].Body, "b.go") || strings.Contains(got[1].Body, "c.go") {
		t.Errorf("sections bleed: %+v", got)
	}
}
