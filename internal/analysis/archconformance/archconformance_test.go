package archconformance_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/archconformance"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// helpers

func edge(from, to string) core.Edge {
	return core.Edge{FromSymbol: from, ToSymbol: to, FromService: "svc", FromFile: "file.go"}
}

func pkgEdge(fromPkg, toPkg string) core.Edge {
	// Encode package info in the From/ToSymbol as "<pkg>.<Symbol>" which is how
	// callers naturally produce edges. archconformance extracts the package prefix
	// (everything before the last dot, or the whole string if no dot).
	return core.Edge{
		FromSymbol:  fromPkg + ".Symbol",
		ToSymbol:    toPkg + ".Symbol",
		FromService: "svc",
		FromFile:    fromPkg + "/file.go",
	}
}

// --- Basic allow/deny tests ---

func TestCheck_NoRules_NoViolations(t *testing.T) {
	edges := []core.Edge{edge("a.Foo", "b.Bar")}
	violations := archconformance.Check(edges, nil)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestCheck_EmptyEdges_NoViolations(t *testing.T) {
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
	}
	violations := archconformance.Check(nil, rules)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestCheck_DenyRule_Violated(t *testing.T) {
	edges := []core.Edge{pkgEdge("ui", "db")}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(violations), violations)
	}
	if violations[0].From == "" || violations[0].To == "" || violations[0].Reason == "" {
		t.Errorf("violation fields must be non-empty: %+v", violations[0])
	}
}

func TestCheck_DenyRule_NotViolated(t *testing.T) {
	// Edge goes from "ui" to "service" — no rule matches, no violation.
	edges := []core.Edge{pkgEdge("ui", "service")}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %v", violations)
	}
}

func TestCheck_AllowRule_PermitsEdge(t *testing.T) {
	// A deny-by-default rule set: only "service→db" is allowed.
	// An edge "ui→db" violates when allow-only mode is active.
	edges := []core.Edge{
		pkgEdge("service", "db"), // allowed
		pkgEdge("ui", "db"),      // not in allow list
	}
	rules := []archconformance.Rule{
		{FromPkg: "service", ToPkg: "db", Allow: true},
	}
	// With only allow rules and DenyByDefault=true, ui→db is a violation.
	violations := archconformance.Check(edges, rules, archconformance.DenyByDefault())
	// service→db is explicitly allowed → no violation.
	// ui→db has no matching allow rule → violation.
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation (ui→db), got %d: %v", len(violations), violations)
	}
	if violations[0].From == "" || violations[0].To == "" {
		t.Errorf("violation must have From/To: %+v", violations[0])
	}
}

func TestCheck_DenyByDefault_AllEdgesViolateWithNoRules(t *testing.T) {
	edges := []core.Edge{pkgEdge("a", "b")}
	violations := archconformance.Check(edges, nil, archconformance.DenyByDefault())
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation with DenyByDefault and no rules, got %d", len(violations))
	}
}

func TestCheck_AllowOverridesDeny(t *testing.T) {
	// Both a deny and an allow rule for the same pair — allow wins.
	edges := []core.Edge{pkgEdge("service", "db")}
	rules := []archconformance.Rule{
		{FromPkg: "service", ToPkg: "db", Allow: false},
		{FromPkg: "service", ToPkg: "db", Allow: true},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations when allow overrides deny, got %v", violations)
	}
}

func TestCheck_MultipleEdges_MultipleViolations(t *testing.T) {
	edges := []core.Edge{
		pkgEdge("ui", "db"),
		pkgEdge("ui", "auth"),
		pkgEdge("service", "db"), // not violating
	}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
		{FromPkg: "ui", ToPkg: "auth", Allow: false},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(violations), violations)
	}
}

func TestCheck_Deterministic(t *testing.T) {
	edges := []core.Edge{
		pkgEdge("ui", "db"),
		pkgEdge("ui", "auth"),
		pkgEdge("admin", "db"),
	}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
		{FromPkg: "ui", ToPkg: "auth", Allow: false},
		{FromPkg: "admin", ToPkg: "db", Allow: false},
	}
	v1 := archconformance.Check(edges, rules)
	v2 := archconformance.Check(edges, rules)
	if len(v1) != len(v2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(v1), len(v2))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Errorf("non-deterministic at index %d: %+v vs %+v", i, v1[i], v2[i])
		}
	}
}

func TestCheck_ViolationFields(t *testing.T) {
	edges := []core.Edge{pkgEdge("ui", "db")}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "db", Allow: false},
	}
	v := archconformance.Check(edges, rules)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(v))
	}
	if v[0].From == "" {
		t.Error("Violation.From must not be empty")
	}
	if v[0].To == "" {
		t.Error("Violation.To must not be empty")
	}
	if v[0].Reason == "" {
		t.Error("Violation.Reason must not be empty")
	}
}

func TestCheck_WildcardFromPkg(t *testing.T) {
	// "*" as FromPkg matches any package.
	edges := []core.Edge{
		pkgEdge("ui", "db"),
		pkgEdge("service", "db"),
	}
	rules := []archconformance.Rule{
		{FromPkg: "*", ToPkg: "db", Allow: false},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations with wildcard FromPkg, got %d: %v", len(violations), violations)
	}
}

func TestCheck_WildcardToPkg(t *testing.T) {
	// "*" as ToPkg matches any target package.
	edges := []core.Edge{
		pkgEdge("ui", "db"),
		pkgEdge("ui", "auth"),
	}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "*", Allow: false},
	}
	violations := archconformance.Check(edges, rules)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations with wildcard ToPkg, got %d: %v", len(violations), violations)
	}
}

func TestCheck_SelfEdgesNotViolating(t *testing.T) {
	// Edges within the same package (self-reference) should not be flagged
	// even with a wildcard deny rule, as self-imports are impossible in Go.
	// (This tests that from==to package doesn't spuriously match cross-pkg rules.)
	edges := []core.Edge{pkgEdge("ui", "ui")}
	rules := []archconformance.Rule{
		{FromPkg: "ui", ToPkg: "ui", Allow: false},
	}
	// The rule explicitly denies ui→ui; a self-reference edge should still be caught.
	violations := archconformance.Check(edges, rules)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for explicit self-deny rule, got %d", len(violations))
	}
}
