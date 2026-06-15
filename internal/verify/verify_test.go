package verify_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/verify"
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
