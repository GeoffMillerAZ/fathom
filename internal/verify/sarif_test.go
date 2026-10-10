package verify

// sarif_test.go — PC-21: refusals render as SARIF 2.1.0 with one rule per
// criterion and the plan line as the location.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderSARIF_ShapeAndDeterminism(t *testing.T) {
	results := []SARIFResult{
		{RuleID: "test_reaches", Message: "no Test* reaches it", File: "docs/plans/a.md", Line: 7},
		{RuleID: "symbol_present", Level: "error", Message: "not found", File: "docs/plans/a.md", Line: 3},
		{RuleID: "test_reaches", Level: "warning", Message: "again", File: "docs/plans/b.md"},
	}
	a := renderSARIF("fathom verify", results)
	b := renderSARIF("fathom verify", results)
	if string(a) != string(b) {
		t.Fatal("renderSARIF is not deterministic")
	}
	var doc struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Message   struct{ Text string }
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct{ URI string }
						Region           *struct{ StartLine int }
					}
				}
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(a, &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, a)
	}
	if doc.Version != "2.1.0" || !strings.Contains(doc.Schema, "sarif-2.1.0") || len(doc.Runs) != 1 {
		t.Fatalf("not a SARIF 2.1.0 single-run log: %s", a)
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Name != "fathom verify" {
		t.Errorf("driver = %q", run.Tool.Driver.Name)
	}
	if len(run.Tool.Driver.Rules) != 2 || run.Tool.Driver.Rules[0].ID != "symbol_present" || run.Tool.Driver.Rules[1].ID != "test_reaches" {
		t.Errorf("rules must be one per distinct criterion, sorted: %+v", run.Tool.Driver.Rules)
	}
	if len(run.Results) != 3 {
		t.Fatalf("results = %d", len(run.Results))
	}
	r0 := run.Results[0]
	if r0.RuleID != "test_reaches" || r0.Level != "error" || r0.Locations[0].PhysicalLocation.ArtifactLocation.URI != "docs/plans/a.md" || r0.Locations[0].PhysicalLocation.Region == nil || r0.Locations[0].PhysicalLocation.Region.StartLine != 7 {
		t.Errorf("result 0 wrong: %+v", r0)
	}
	if run.Results[2].Locations[0].PhysicalLocation.Region != nil {
		t.Error("line 0 must omit the region, never claim line 0")
	}
	if run.Results[2].Level != "warning" {
		t.Error("explicit level must be kept")
	}
	// Zero results is still one run — "ran and refused nothing".
	if !strings.Contains(string(renderSARIF("x", nil)), `"results": []`) {
		t.Error("empty results must render as an empty array inside one run")
	}
}

func TestGateSARIF_OneResultPerFailingCriterionAndStrictUnknown(t *testing.T) {
	reports := []Report{{Service: "svc", Tasks: []TaskVerdict{
		{TaskID: "A-1", Title: "absent", Status: Fail, Label: "claimed-but-absent", Claimed: true, Line: 4, Criteria: []Criterion{
			{"symbol_present", Fail, `"Foo" not found in the index`},
			{"tier", NA, "symbol absent"},
			// TC-29: an attested run that failed is informational — never a
			// SARIF refusal of its own (I8).
			{"runtime_pass", Fail, "TestFoo failed in its newest attested run"},
		}},
		{TaskID: "A-2", Title: "no contract", Status: Unknown, Label: "no-contract", Claimed: true, Line: 5, Criteria: []Criterion{{"runtime_pass", Unknown, "no certifying test"}}},
		{TaskID: "A-3", Title: "fine", Status: Pass, Label: "verified", Claimed: true, Line: 6},
		{TaskID: "A-4", Title: "skipped", Status: Fail, Skipped: true, Line: 7},
		{TaskID: "A-5", Title: "open", Status: Fail, Line: 8},
	}}}
	got := GateSARIF([]string{"docs/plans/a.md"}, reports, false)
	if len(got) != 1 || got[0].RuleID != "symbol_present" || got[0].Line != 4 || got[0].File != "docs/plans/a.md" || got[0].Level != "error" {
		t.Errorf("non-strict: want one symbol_present error at line 4, got %+v", got)
	}
	got = GateSARIF([]string{"docs/plans/a.md"}, reports, true)
	if len(got) != 2 || got[1].RuleID != "no-contract" || got[1].Level != "warning" || got[1].Line != 5 {
		t.Errorf("strict: want the no-contract warning added at line 5, got %+v", got)
	}
	// The warning names the row's own reason, never runtime_pass's.
	if len(got) == 2 && strings.Contains(got[1].Message, "certifying test") {
		t.Errorf("strict warning cites runtime_pass: %q", got[1].Message)
	}
	// Parity with Gate: every SARIF result corresponds to a gate failure.
	if g := Gate(reports[0], true); len(g.Failures) != 2 {
		t.Errorf("Gate strict failures = %d, SARIF results = %d", len(g.Failures), len(got))
	}
}
