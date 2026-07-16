package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// pkgNode builds a SymbolNode with service and package set, so Key() yields a
// full canonical key ("svc::pkg.Name").
func pkgNode(pkg, name, file string, line int, tier core.EvidenceTier) core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: pkg, Name: name, FilePath: file, Line: line, Tier: tier}
}

// ambiguousRunEvidence models a bare name ("Run") that genuinely lives in two
// packages. Reachability facts are keyed ONLY by canonical key — a bare "Run"
// query resolves to nothing, exactly like an ambiguous name in the real call
// graph. The `package:` filter is the only sanctioned way to reach the keyed
// facts.
func ambiguousRunEvidence() fakeEvidence {
	return fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Run": {
			pkgNode("ladder", "Run", "foundations/ladder/types.go", 300, core.TierVerified),
			pkgNode("harness", "Run", "foundations/harness/harness.go", 42, core.TierVerified),
		}},
		tests: map[string][]string{"svc::ladder.Run": {"svc::ladder.TestRun"}},
		reach: map[string]bool{"Router->svc::ladder.Run": true},
	}
}

// TestVerifyPlan_PackageRight: with `package:` naming the directory the symbol
// lives under, the filtered node's canonical key is used for test_reaches and
// edge_wired — the ambiguous bare name verifies.
func TestVerifyPlan_PackageRight(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{
			Symbol: "Run", Package: "foundations/ladder", ReachableFrom: "Router",
		}},
	}}, ambiguousRunEvidence())
	v := rep.Tasks[0]
	if v.Status != verify.Pass || v.Label != "verified" {
		t.Fatalf("got %s/%s, want pass/verified; criteria=%+v", v.Status, v.Label, v.Criteria)
	}
	if c, ok := findCrit(v, "symbol_present"); !ok || !strings.Contains(c.Detail, "foundations/ladder/") {
		t.Fatalf("symbol_present should cite the filtered node, got %+v", c)
	}
	if c, _ := findCrit(v, "test_reaches"); c.Status != verify.Pass {
		t.Fatalf("test_reaches should pass via the canonical key, got %+v", c)
	}
	if c, _ := findCrit(v, "edge_wired"); c.Status != verify.Pass {
		t.Fatalf("edge_wired should pass via the canonical key, got %+v", c)
	}
}

// TestVerifyPlan_PackageWrong: a `package:` filter that empties the match set
// is a failed claim — claimed-but-absent, tier n/a.
func TestVerifyPlan_PackageWrong(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Run", Package: "foundations/nowhere"}},
	}}, ambiguousRunEvidence())
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "claimed-but-absent" {
		t.Fatalf("got %s/%s, want fail/claimed-but-absent", v.Status, v.Label)
	}
	if c, _ := findCrit(v, "symbol_present"); !strings.Contains(c.Detail, "foundations/nowhere") {
		t.Fatalf("symbol_present detail should name the missing directory, got %+v", c)
	}
	if c, _ := findCrit(v, "tier"); c.Status != verify.NA {
		t.Fatalf("tier should be n/a when the filter empties the set, got %s", c.Status)
	}
}

// TestVerifyPlan_PackageStillAmbiguous: a `package:` directory that contains
// TWO distinct symbols with the claimed name does not pick a winner — the
// query falls back to the (ambiguous) bare name and resolves to nothing.
// Never first-match-wins.
func TestVerifyPlan_PackageStillAmbiguous(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Run", Package: "foundations"}},
	}}, ambiguousRunEvidence())
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "present-but-untested" {
		t.Fatalf("got %s/%s, want fail/present-but-untested (ambiguity stays honest)", v.Status, v.Label)
	}
}

// TestVerifyPlan_AmbiguousBareWithoutPackage: with no `package:` annotation,
// behavior is unchanged — the bare name is queried as-is and an ambiguous
// name still resolves to nothing.
func TestVerifyPlan_AmbiguousBareWithoutPackage(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Run"}},
	}}, ambiguousRunEvidence())
	v := rep.Tasks[0]
	if c, _ := findCrit(v, "symbol_present"); c.Status != verify.Pass {
		t.Fatalf("symbol_present should still pass on a bare ambiguous name, got %+v", c)
	}
	if v.Status != verify.Fail || v.Label != "present-but-untested" {
		t.Fatalf("got %s/%s, want fail/present-but-untested (unchanged)", v.Status, v.Label)
	}
}

// TestParseMarkdownPlan_PackageKey: the `package:` annotation key maps into
// Expect.Package instead of being silently dropped.
func TestParseMarkdownPlan_PackageKey(t *testing.T) {
	md := []byte("- [x] ladder loop {symbol: Run, package: foundations/evidence/evidence-ladder}\n" +
		"- [x] no package {symbol: Foo}\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	if len(plan.Tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(plan.Tasks))
	}
	if got := plan.Tasks[0].Expect.Package; got != "foundations/evidence/evidence-ladder" {
		t.Fatalf("Expect.Package = %q, want the annotated directory", got)
	}
	if plan.Tasks[0].Expect.Symbol != "Run" {
		t.Fatalf("Expect.Symbol = %q", plan.Tasks[0].Expect.Symbol)
	}
	if got := plan.Tasks[1].Expect.Package; got != "" {
		t.Fatalf("Expect.Package should default to empty, got %q", got)
	}
}
