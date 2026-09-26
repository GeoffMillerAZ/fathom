package console

import (
	"strings"
	"testing"
)

// TestCertMatrix_TargetsByState pins OC-24: every target the
// certifications layer considers is counted under its best certificate's
// state (live, flaky, broken, unproven, unreachable — or none), per kind;
// the gap list carries each gap's remedy and source; a service that
// declares nothing is named with its skip, never a zero; a service whose
// report failed makes the totals an error, never a smaller count (I38).
func TestCertMatrix_TargetsByState(t *testing.T) {
	proof := CertServiceFacts{
		Service: "proof", Declared: 4, Live: 1, Gaps: 3, JoinedCases: 2, Cases: 2,
		Targets: []CertTargetFact{
			{Kind: "invariant", ID: "I1", Certificates: []CertFact{{Test: "TestA", State: "unproven"}, {Test: "TestB", State: "live"}}},
			{Kind: "invariant", ID: "I2", Gap: true, Remedy: "add a certifies marker", SourceFile: "docs/intent/invariants.md", SourceLine: 6},
			{Kind: "invariant", ID: "I3", Gap: true, Remedy: "fix the flake", Certificates: []CertFact{{Test: "TestC", State: "flaky"}}},
			{Kind: "incident", ID: "INC-1", Gap: true, Remedy: "add an incident certificate"},
		},
	}
	quiet := CertServiceFacts{Service: "quiet", Skip: "no manifests found — nothing declared"}
	v := CertMatrix([]CertServiceFacts{proof, quiet}, "", nil)
	if strings.Join(v.States, ",") != "live,flaky,broken,unproven,unreachable,none" {
		t.Fatalf("states = %v", v.States)
	}
	if len(v.Rows) != 2 || v.Rows[0].Kind != "incident" || v.Rows[1].Kind != "invariant" {
		t.Fatalf("rows = %+v", v.Rows)
	}
	inv := v.Rows[1]
	// I1's best certificate is live; I3's flaky; I2 has none.
	want := []int{1, 1, 0, 0, 0, 1}
	for i, c := range inv.Cells {
		if c.Count != want[i] {
			t.Errorf("invariant × %s = %d, want %d", v.States[i], c.Count, want[i])
		}
	}
	if inv.Total != 3 || v.Rows[0].Cells[5].Count != 1 {
		t.Errorf("totals = %+v / %+v", inv, v.Rows[0])
	}
	if len(v.Gaps) != 3 || v.Gaps[0].ID != "INC-1" || v.Gaps[1].Source != "docs/intent/invariants.md:6" || v.Gaps[2].State != "flaky" {
		t.Errorf("gaps = %+v", v.Gaps)
	}
	if v.Declared.V != 4 || v.Live.V != 1 || v.GapCount.V != 3 || v.GapCount.State != StatePresent {
		t.Errorf("totals = %+v %+v %+v", v.Declared, v.Live, v.GapCount)
	}
	rows := map[string]CertServiceRow{}
	for _, r := range v.Services {
		rows[r.Service] = r
	}
	if rows["quiet"].Declared.State != StateAbsent || !strings.Contains(rows["quiet"].Declared.Reason, "nothing declared") {
		t.Errorf("a service that declares nothing is absent with its skip, never zero: %+v", rows["quiet"].Declared)
	}
	if rows["proof"].JoinRate.V != 100 || rows["quiet"].JoinRate.State != StateAbsent {
		t.Errorf("join rates = %+v / %+v", rows["proof"].JoinRate, rows["quiet"].JoinRate)
	}
	if rows["proof"].Href != "/certifications?svc=proof" {
		t.Errorf("service drill-down = %q", rows["proof"].Href)
	}
	if BestCertState(CertTargetFact{}) != "none" || BestCertState(CertTargetFact{Certificates: []CertFact{{State: "broken"}, {State: "unreachable"}}}) != "broken" {
		t.Error("best state order is live > flaky > broken > unproven > unreachable > none")
	}
	// One failed report: the totals are an error naming it.
	v = CertMatrix([]CertServiceFacts{proof, {Service: "broken", Err: "store locked"}}, "", nil)
	if v.GapCount.State != StateError || !strings.Contains(v.GapCount.Reason, "broken") {
		t.Errorf("a failed report must make the totals an error: %+v", v.GapCount)
	}
	// Nothing declared anywhere: absent.
	if v := CertMatrix([]CertServiceFacts{quiet}, "quiet", nil); v.GapCount.State != StateAbsent || len(v.Rows) != 0 {
		t.Errorf("nothing declared = %+v", v)
	}
}
