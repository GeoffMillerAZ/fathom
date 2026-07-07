package naming_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/naming"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func node(name string, kind core.SourceKind) core.SymbolNode {
	return core.SymbolNode{Name: name, Kind: kind}
}

func TestCheck_ExportedUnderscore(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Get_User", core.KindGoFunc),
	}
	issues := naming.Check(nodes)
	if !hasRule(issues, "exported-no-underscore") {
		t.Fatalf("expected exported-no-underscore, got %v", issues)
	}
}

func TestCheck_UnexportedUnderscore(t *testing.T) {
	nodes := []core.SymbolNode{
		node("get_user", core.KindGoFunc),
	}
	issues := naming.Check(nodes)
	if !hasRule(issues, "unexported-no-underscore") {
		t.Fatalf("expected unexported-no-underscore, got %v", issues)
	}
}

func TestCheck_AcronymCaps(t *testing.T) {
	nodes := []core.SymbolNode{
		node("GetId", core.KindGoFunc),
	}
	issues := naming.Check(nodes)
	if !hasRule(issues, "acronym-caps") {
		t.Fatalf("expected acronym-caps for GetId, got %v", issues)
	}
}

func TestCheck_InterfaceErSuffix(t *testing.T) {
	nodes := []core.SymbolNode{
		node("UserManager", core.KindGoInterface),
	}
	issues := naming.Check(nodes)
	if !hasRule(issues, "interface-er-suffix") {
		t.Fatalf("expected interface-er-suffix, got %v", issues)
	}
}

func TestCheck_InterfaceErSuffix_OK(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Storer", core.KindGoInterface),
	}
	issues := naming.Check(nodes)
	for _, iss := range issues {
		if iss.Rule == "interface-er-suffix" {
			t.Fatalf("unexpected interface-er-suffix for %q", iss.Symbol)
		}
	}
}

func TestCheck_Clean(t *testing.T) {
	nodes := []core.SymbolNode{
		node("GetUser", core.KindGoFunc),
		node("userID", core.KindGoVar),
		node("Storer", core.KindGoInterface),
	}
	issues := naming.Check(nodes)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestCheck_Blank(t *testing.T) {
	nodes := []core.SymbolNode{
		node("_", core.KindGoVar),
	}
	// Blank identifier should not trigger any rule.
	issues := naming.Check(nodes)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for blank identifier, got %v", issues)
	}
}

func TestCheck_Deterministic(t *testing.T) {
	nodes := []core.SymbolNode{
		node("Get_Id", core.KindGoFunc),
		node("set_value", core.KindGoFunc),
		node("Manager", core.KindGoInterface),
	}
	a := naming.Check(nodes)
	b := naming.Check(nodes)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at [%d]: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func hasRule(issues []naming.Issue, rule string) bool {
	for _, iss := range issues {
		if iss.Rule == rule {
			return true
		}
	}
	return false
}
