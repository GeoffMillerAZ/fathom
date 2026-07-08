package verify_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// fakeEvidence is a deterministic in-memory Evidence for testing the predicate.
type fakeEvidence struct {
	symbols map[string][]core.SymbolNode
	tests   map[string][]string
	reach   map[string]bool // "from->to" → bool
	contras []core.Contradiction
	specs   map[string][]string
}

func (f fakeEvidence) FindSymbol(n string) []core.SymbolNode { return f.symbols[n] }
func (f fakeEvidence) TestReaches(s string) []string         { return f.tests[s] }
func (f fakeEvidence) Reachable(from, to string) bool        { return f.reach[from+"->"+to] }
func (f fakeEvidence) Contradictions() []core.Contradiction  { return f.contras }
func (f fakeEvidence) SpecImplementors(id string) []string   { return f.specs[id] }

func node(name, file string, line int, tier core.EvidenceTier) core.SymbolNode {
	return core.SymbolNode{Name: name, FilePath: file, Line: line, Tier: tier}
}

func findCrit(v verify.TaskVerdict, name string) (verify.Criterion, bool) {
	for _, c := range v.Criteria {
		if c.Name == name {
			return c, true
		}
	}
	return verify.Criterion{}, false
}

// TestVerifyPlan_Verified: symbol present at verified tier, a test reaches it, no
// contradiction → Pass / "verified", with runtime_pass explicitly Unknown.
func TestVerifyPlan_Verified(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"ValidateToken": {node("ValidateToken", "auth/token.go", 42, core.TierVerified)}},
		tests:   map[string][]string{"ValidateToken": {"svc::auth.TestValidateToken"}},
	}
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "T-1", Title: "implement token validation", Expect: verify.Expect{Symbol: "ValidateToken"}},
	}}
	rep := verify.VerifyPlan(plan, ev)
	if len(rep.Tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(rep.Tasks))
	}
	v := rep.Tasks[0]
	if v.Status != verify.Pass || v.Label != "verified" {
		t.Fatalf("got status=%s label=%s, want pass/verified", v.Status, v.Label)
	}
	if rt, ok := findCrit(v, "runtime_pass"); !ok || rt.Status != verify.Unknown {
		t.Fatalf("runtime_pass must be Unknown, got %+v", rt)
	}
	if rep.Summary != (verify.Summary{Total: 1, Pass: 1}) {
		t.Fatalf("summary = %+v", rep.Summary)
	}
}

// TestVerifyPlan_ClaimedButAbsent: the agent claims a symbol that isn't indexed.
func TestVerifyPlan_ClaimedButAbsent(t *testing.T) {
	ev := fakeEvidence{}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Ghost"}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "claimed-but-absent" {
		t.Fatalf("got %s/%s, want fail/claimed-but-absent", v.Status, v.Label)
	}
	if c, _ := findCrit(v, "tier"); c.Status != verify.NA {
		t.Fatalf("tier should be n/a when symbol absent, got %s", c.Status)
	}
}

// TestVerifyPlan_PresentButUntested: a real symbol no test statically reaches.
func TestVerifyPlan_PresentButUntested(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
		// no tests reach Foo
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Foo"}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "present-but-untested" {
		t.Fatalf("got %s/%s, want fail/present-but-untested", v.Status, v.Label)
	}
	// Skipping the test requirement flips it to verified.
	rep2 := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Foo", SkipTest: true}},
	}}, ev)
	if rep2.Tasks[0].Status != verify.Pass {
		t.Fatalf("with SkipTest, want pass, got %s", rep2.Tasks[0].Status)
	}
}

// TestVerifyPlan_LikelyUnconfirmed: a pre-scan (heuristic) symbol can't satisfy a
// verified-tier contract.
func TestVerifyPlan_LikelyUnconfirmed(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Helm": {node("Helm", "values.yaml", 3, core.TierPreScan)}},
		tests:   map[string][]string{"Helm": {"t"}},
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Helm"}},
	}}, ev)
	if v := rep.Tasks[0]; v.Status != verify.Fail || v.Label != "likely-unconfirmed" {
		t.Fatalf("got %s/%s, want fail/likely-unconfirmed", v.Status, v.Label)
	}
	// Loosening MinTier to pre-scan accepts it.
	rep2 := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Helm", MinTier: core.TierPreScan}},
	}}, ev)
	if rep2.Tasks[0].Status != verify.Pass {
		t.Fatalf("with MinTier=pre-scan want pass, got %s", rep2.Tasks[0].Status)
	}
}

// TestVerifyPlan_Contradicted: conflicting evidence on the symbol fails the task.
func TestVerifyPlan_Contradicted(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Conf": {node("Conf", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Conf": {"t"}},
		contras: []core.Contradiction{{
			Symbol: "Conf",
			A:      core.Observation{SourceRef: "a.go"},
			B:      core.Observation{SourceRef: "docs/x.md"},
		}},
	}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Conf"}},
	}}, ev)
	if v := rep.Tasks[0]; v.Status != verify.Fail || v.Label != "contradicted" {
		t.Fatalf("got %s/%s, want fail/contradicted", v.Status, v.Label)
	}
}

// TestVerifyPlan_EdgeWiring: ReachableFrom must reach the symbol.
func TestVerifyPlan_EdgeWiring(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Handler": {node("Handler", "h.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Handler": {"t"}},
		reach:   map[string]bool{"Router->Handler": true},
	}
	ok := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Handler", ReachableFrom: "Router"}},
	}}, ev)
	if ok.Tasks[0].Status != verify.Pass {
		t.Fatalf("wired edge should pass, got %s", ok.Tasks[0].Status)
	}
	bad := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Handler", ReachableFrom: "Nope"}},
	}}, ev)
	if v := bad.Tasks[0]; v.Status != verify.Fail || v.Label != "edge-not-wired" {
		t.Fatalf("got %s/%s, want fail/edge-not-wired", v.Status, v.Label)
	}
}

// TestVerifyPlan_SpecCoverage: declared spec IDs must have implementing symbols.
func TestVerifyPlan_SpecCoverage(t *testing.T) {
	ev := fakeEvidence{specs: map[string][]string{"[US-1]": {"Foo"}}}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", SpecIDs: []string{"[US-1]", "[US-2]"}},
	}}, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "spec-uncovered" {
		t.Fatalf("got %s/%s, want fail/spec-uncovered", v.Status, v.Label)
	}
	if c, _ := findCrit(v, "spec_covered"); c.Detail == "" {
		t.Fatal("spec_covered detail should list uncovered specs")
	}
}

// TestVerifyPlan_NoContract: a task with no checkable contract is Unknown, never
// silently "verified".
func TestVerifyPlan_NoContract(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{{ID: "T-1", Title: "vibes"}}}, fakeEvidence{})
	if v := rep.Tasks[0]; v.Status != verify.Unknown || v.Label != "no-contract" {
		t.Fatalf("got %s/%s, want unknown/no-contract", v.Status, v.Label)
	}
}

// TestVerifyPlan_ClaimedSkippedPropagation: Claimed/Skipped on Task flow through
// to TaskVerdict.
func TestVerifyPlan_ClaimedSkippedPropagation(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Foo": {"TestFoo"}},
	}
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Title: "done", Claimed: true, Expect: verify.Expect{Symbol: "Foo"}},
		{ID: "T-2", Title: "open", Claimed: false, Expect: verify.Expect{Symbol: "Foo"}},
		{ID: "T-3", Title: "skipped", Skipped: true, Expect: verify.Expect{Symbol: "Foo"}},
	}}
	rep := verify.VerifyPlan(plan, ev)
	byID := map[string]verify.TaskVerdict{}
	for _, tv := range rep.Tasks {
		byID[tv.TaskID] = tv
	}
	if !byID["T-1"].Claimed {
		t.Error("T-1 Claimed should be true")
	}
	if byID["T-2"].Claimed {
		t.Error("T-2 Claimed should be false")
	}
	if !byID["T-3"].Skipped {
		t.Error("T-3 Skipped should be true")
	}
}

// TestGate_Pass: all claimed tasks pass → gate passes.
func TestGate_Pass(t *testing.T) {
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{TaskID: "md-001", Title: "do it", Status: verify.Pass, Claimed: true},
		{TaskID: "md-002", Title: "open", Status: verify.Fail, Claimed: false},
	}}
	gr := verify.Gate(rep, false)
	if !gr.Pass {
		t.Fatalf("gate should pass; failures=%v", gr.Failures)
	}
	if len(gr.Failures) != 0 {
		t.Fatalf("want no failures, got %v", gr.Failures)
	}
}

// TestGate_FailOnClaimed: a claimed task that fails verification blocks the gate.
func TestGate_FailOnClaimed(t *testing.T) {
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{
			TaskID: "md-001", Title: "missing ghost", Status: verify.Fail,
			Label:   "claimed-but-absent",
			Claimed: true,
			Criteria: []verify.Criterion{
				{Name: "symbol_present", Status: verify.Fail, Detail: `"Ghost" not found in the index`},
			},
		},
	}}
	gr := verify.Gate(rep, false)
	if gr.Pass {
		t.Fatal("gate should fail for a claimed failing task")
	}
	if len(gr.Failures) != 1 {
		t.Fatalf("want 1 failure, got %d: %v", len(gr.Failures), gr.Failures)
	}
	if !strings.Contains(gr.Failures[0], "md-001") {
		t.Errorf("failure should mention task ID: %q", gr.Failures[0])
	}
	if !strings.Contains(gr.Failures[0], "Ghost") {
		t.Errorf("failure should cite the criterion detail: %q", gr.Failures[0])
	}
}

// TestGate_SkippedExempt: [-] skipped tasks never gate, even if they would fail.
func TestGate_SkippedExempt(t *testing.T) {
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{
			TaskID: "md-001", Status: verify.Fail, Claimed: true, Skipped: true,
			Criteria: []verify.Criterion{
				{Name: "symbol_present", Status: verify.Fail, Detail: "absent"},
			},
		},
	}}
	gr := verify.Gate(rep, false)
	if !gr.Pass {
		t.Fatalf("skipped task should not gate; failures=%v", gr.Failures)
	}
}

// TestGate_StrictUnknown: under --strict, a claimed task with Unknown status gates.
func TestGate_StrictUnknown(t *testing.T) {
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{
			TaskID: "md-001", Title: "no contract", Status: verify.Unknown,
			Label:   "no-contract",
			Claimed: true,
			Criteria: []verify.Criterion{
				{Name: "runtime_pass", Status: verify.Unknown, Detail: "no CI evidence ingested"},
			},
		},
	}}
	// non-strict: unknown does not gate
	gr := verify.Gate(rep, false)
	if !gr.Pass {
		t.Fatalf("non-strict: unknown claimed task should not gate; failures=%v", gr.Failures)
	}
	// strict: unknown gates
	gr2 := verify.Gate(rep, true)
	if gr2.Pass {
		t.Fatal("strict: unknown claimed task should gate")
	}
	if len(gr2.Failures) == 0 {
		t.Fatal("strict: want at least one failure")
	}
	if !strings.Contains(gr2.Failures[0], "md-001") {
		t.Errorf("failure should mention task ID: %q", gr2.Failures[0])
	}
}

// TestGate_Deterministic: failure list is in report order (sorted by task ID).
func TestGate_Deterministic(t *testing.T) {
	rep := verify.Report{Tasks: []verify.TaskVerdict{
		{
			TaskID: "md-002", Title: "b", Status: verify.Fail, Claimed: true,
			Criteria: []verify.Criterion{{Name: "symbol_present", Status: verify.Fail, Detail: "B absent"}},
		},
		{
			TaskID: "md-001", Title: "a", Status: verify.Fail, Claimed: true,
			Criteria: []verify.Criterion{{Name: "symbol_present", Status: verify.Fail, Detail: "A absent"}},
		},
	}}
	gr := verify.Gate(rep, false)
	if gr.Pass {
		t.Fatal("should fail")
	}
	// VerifyPlan sorts by task ID, so the report tasks are already sorted.
	// Gate preserves that order.
	if len(gr.Failures) != 2 || !strings.Contains(gr.Failures[0], "md-002") {
		t.Errorf("gate failures not in report order: %v", gr.Failures)
	}
}

// TestVerifyPlan_Deterministic: tasks come out sorted by ID, criteria in a fixed
// order, identical across runs.
func TestVerifyPlan_Deterministic(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"A": {node("A", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"A": {"t"}},
	}
	plan := verify.Plan{Tasks: []verify.Task{
		{ID: "T-3", Expect: verify.Expect{Symbol: "A"}},
		{ID: "T-1", Expect: verify.Expect{Symbol: "A"}},
		{ID: "T-2", Expect: verify.Expect{Symbol: "A"}},
	}}
	r1 := verify.VerifyPlan(plan, ev)
	r2 := verify.VerifyPlan(plan, ev)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("VerifyPlan is not deterministic")
	}
	ids := []string{r1.Tasks[0].TaskID, r1.Tasks[1].TaskID, r1.Tasks[2].TaskID}
	if !reflect.DeepEqual(ids, []string{"T-1", "T-2", "T-3"}) {
		t.Fatalf("tasks not sorted by ID: %v", ids)
	}
}
