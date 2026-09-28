package verify

// proofcertified_test.go — SI-14 (docs/design/spec-interop.md §4, ADR-049):
// the plan-level `Proof: certified` header line, like `Baseline:`. Once
// set, every `symbol:` row in the plan must also carry `certified_by:` —
// a row without it is refused `certificate-absent`, through the existing
// EvaluateCertifiedBy path, with no skip_test escape (the header is the
// strongest gate: it cannot be waived).

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// proofTestEvidence is a minimal Evidence, local to this file, that also
// implements CertificationEvidence when byTest is set — for exercising
// the certified_by path end to end through VerifyPlan without depending
// on the external verify_test package's fixtures.
type proofTestEvidence struct {
	symbols map[string][]core.SymbolNode
	tests   map[string][]string
	byTest  map[string][]Certification
}

func (f proofTestEvidence) FindSymbol(n string) []core.SymbolNode { return f.symbols[n] }
func (f proofTestEvidence) TestReaches(s string) []string         { return f.tests[s] }
func (f proofTestEvidence) Reachable(_, _ string) bool            { return false }
func (f proofTestEvidence) Contradictions() []core.Contradiction  { return nil }
func (f proofTestEvidence) SpecImplementors(_ string) []string    { return nil }

func (f proofTestEvidence) CertificationsByTest(tests []string) map[string][]Certification {
	out := map[string][]Certification{}
	for _, t := range tests {
		if c, ok := f.byTest[t]; ok {
			out[t] = c
		}
	}
	return out
}

func proofTestNode(name, file string, line int, tier core.EvidenceTier) core.SymbolNode {
	return core.SymbolNode{Name: name, FilePath: file, Line: line, Tier: tier}
}

func TestParseProofHeader(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
		ok   bool
	}{
		{"Proof: certified", "certified", true},
		{"proof: certified", "certified", true},
		{"  Proof:   certified  ", "certified", true},
		{"Proof:certified", "certified", true}, // no space required, same as Baseline:
		{"Baseline: abc123", "", false},
		{"- [ ] Proof: certified {symbol: Foo}", "", false}, // a checkbox row, not the header
		{"", "", false},
	} {
		got, ok := parseProofHeader(tc.line)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseProofHeader(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

// TestParseMarkdownPlan_ProofHeader: the header is read into Plan.Proof
// exactly like Baseline:, and is absent by default.
func TestParseMarkdownPlan_ProofHeader(t *testing.T) {
	md := "# A plan\n\nProof: certified\n\n- [ ] x {symbol: Foo}\n"
	p := ParseMarkdownPlan("svc", []byte(md))
	if p.Proof != "certified" {
		t.Fatalf("Proof = %q, want %q", p.Proof, "certified")
	}
	if len(p.Tasks) != 1 {
		t.Fatalf("the header line must not become a task row: got %d tasks", len(p.Tasks))
	}

	p2 := ParseMarkdownPlan("svc", []byte("- [ ] x {symbol: Foo}\n"))
	if p2.Proof != "" {
		t.Fatalf("no header line: Proof = %q, want empty", p2.Proof)
	}
}

// TestProofCertifiedHeaderRequiresCertificates (SI-14): under Proof:
// certified, a symbol: row with no certified_by: is refused
// certificate-absent; a row that carries certified_by: still verifies
// through the ordinary EvaluateCertifiedBy path; with no header, the same
// row without certified_by: verifies through the ordinary test_reaches
// path unchanged. skip_test does not waive the header's requirement.
//
// certifies: invariant SI-14 (asserts)
func TestProofCertifiedHeaderRequiresCertificates(t *testing.T) {
	sym := func(name, file string) map[string][]core.SymbolNode {
		return map[string][]core.SymbolNode{name: {proofTestNode(name, file, 10, core.TierVerified)}}
	}
	// Every plan below is built through ParseMarkdownPlan (never a Plan{}
	// literal), so this test's own reachability to parseProofHeader is
	// real, not incidental: the header line it hands ParseMarkdownPlan is
	// read by the exact function under test.
	planFrom := func(md string) Plan { return ParseMarkdownPlan("svc", []byte(md)) }

	t.Run("certified header, no certified_by: refused certificate-absent", func(t *testing.T) {
		plan := planFrom("Proof: certified\n\n- [x] t1 {id: T-1, symbol: Foo}\n")
		ev := proofTestEvidence{symbols: sym("Foo", "pkg/foo.go"), tests: map[string][]string{"Foo": {"TestFoo"}}}
		rep := VerifyPlan(plan, ev)
		v := rep.Tasks[0]
		if v.Status != Fail || v.Label != "certificate-absent" {
			t.Fatalf("status/label = %s/%s, want fail/certificate-absent", v.Status, v.Label)
		}
	})

	t.Run("certified header, skip_test does not waive the requirement", func(t *testing.T) {
		plan := planFrom(`Proof: certified

- [x] t1 {id: T-1, symbol: Foo, skip_test: true, skip: "manual QA"}
`)
		ev := proofTestEvidence{symbols: sym("Foo", "pkg/foo.go")}
		rep := VerifyPlan(plan, ev)
		v := rep.Tasks[0]
		if v.Status != Fail || v.Label != "certificate-absent" {
			t.Fatalf("status/label = %s/%s, want fail/certificate-absent (skip_test must not escape the header)", v.Status, v.Label)
		}
	})

	t.Run("certified header, certified_by: present goes through EvaluateCertifiedBy and can pass", func(t *testing.T) {
		plan := planFrom("Proof: certified\n\n- [x] t1 {id: T-1, symbol: Foo, certified_by: TestFoo}\n")
		ev := proofTestEvidence{
			symbols: sym("Foo", "pkg/foo.go"),
			byTest: map[string][]Certification{
				"TestFoo": {{Test: "TestFoo", Strength: "asserts", State: "live", Scope: []string{"pkg/foo.go"}}},
			},
		}
		rep := VerifyPlan(plan, ev)
		v := rep.Tasks[0]
		if v.Status != Pass || v.Label != "verified" {
			t.Fatalf("status/label = %s/%s, want pass/verified", v.Status, v.Label)
		}
	})

	t.Run("certified header, certified_by: present but unproven still fails through EvaluateCertifiedBy", func(t *testing.T) {
		plan := planFrom("Proof: certified\n\n- [x] t1 {id: T-1, symbol: Foo, certified_by: TestFoo}\n")
		ev := proofTestEvidence{
			symbols: sym("Foo", "pkg/foo.go"),
			byTest: map[string][]Certification{
				"TestFoo": {{Test: "TestFoo", Strength: "asserts", State: "unproven", Scope: []string{"pkg/foo.go"}}},
			},
		}
		rep := VerifyPlan(plan, ev)
		v := rep.Tasks[0]
		if v.Status != Fail || v.Label != "certificate-unproven" {
			t.Fatalf("status/label = %s/%s, want fail/certificate-unproven", v.Status, v.Label)
		}
	})

	t.Run("no header: the same row without certified_by verifies through ordinary test_reaches, unchanged", func(t *testing.T) {
		plan := planFrom("- [x] t1 {id: T-1, symbol: Foo}\n")
		if plan.Proof != "" {
			t.Fatalf("this plan carries no header line: Proof = %q", plan.Proof)
		}
		ev := proofTestEvidence{symbols: sym("Foo", "pkg/foo.go"), tests: map[string][]string{"Foo": {"TestFoo"}}}
		rep := VerifyPlan(plan, ev)
		v := rep.Tasks[0]
		if v.Status != Pass || v.Label != "verified" {
			t.Fatalf("status/label = %s/%s, want pass/verified (no header: ordinary path)", v.Status, v.Label)
		}
	})
}
