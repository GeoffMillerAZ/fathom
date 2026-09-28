package archconformance_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestRuleCountsAttributesCheck (CX-20): per-rule counts agree with Check —
// an allowed edge counts nowhere, a denied edge counts under its rule, a
// clean deny rule reads zero (checked, none), and deny-by-default
// violations get their own row.
func TestRuleCountsAttributesCheck(t *testing.T) {
	edges := []core.Edge{pkgEdge("a", "b"), pkgEdge("a", "b"), pkgEdge("a", "c"), pkgEdge("d", "e"), pkgEdge("x", "b")}
	rules := []archconformance.Rule{
		{FromPkg: "a", ToPkg: "b"},
		{FromPkg: "*", ToPkg: "b"},
		{FromPkg: "x", ToPkg: "b", Allow: true},
		{FromPkg: "q", ToPkg: "r"},
		{FromPkg: "a", ToPkg: "b"}, // duplicate: one row
	}
	got := archconformance.RuleCounts(edges, rules)
	want := []archconformance.RuleCount{{Rule: "* -> b", Count: 2}, {Rule: "a -> b", Count: 2}, {Rule: "q -> r", Count: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RuleCounts = %+v, want %+v", got, want)
	}
	if n := len(archconformance.Check(edges, rules)); n != 2 {
		t.Fatalf("Check found %d violations, want 2", n)
	}
	dbd := archconformance.RuleCounts(edges, rules, archconformance.DenyByDefault())
	if dbd[0] != (archconformance.RuleCount{Rule: archconformance.DenyByDefaultRule, Count: 2}) {
		t.Fatalf("deny-by-default row = %+v, want 2 (a->c, d->e)", dbd[0])
	}
	if total := len(archconformance.Check(edges, rules, archconformance.DenyByDefault())); total != 4 {
		t.Fatalf("Check deny-by-default = %d, want 4", total)
	}
}
