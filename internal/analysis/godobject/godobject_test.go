package godobject_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/godobject"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// buildNodes creates a type node + n method nodes in the same package.
func buildNodes(pkg, typeName string, methodCount int, sig string) []core.SymbolNode {
	nodes := []core.SymbolNode{
		{
			Service:   "svc",
			Package:   pkg,
			Name:      typeName,
			Kind:      core.KindGoType,
			Signature: sig,
		},
	}
	for i := 0; i < methodCount; i++ {
		nodes = append(nodes, core.SymbolNode{
			Service: "svc",
			Package: pkg,
			Name:    typeName + ".Method" + string(rune('A'+i)),
			Kind:    core.KindGoMethod,
		})
	}
	return nodes
}

func TestAnalyze_GodObjectByMethods(t *testing.T) {
	nodes := buildNodes("mypkg", "BigService", 12, "struct{}")
	cfg := godobject.Config{MethodThreshold: 10, FieldThreshold: 10}
	results := godobject.Analyze(nodes, cfg)
	if len(results) != 1 {
		t.Fatalf("want 1 god object, got %d", len(results))
	}
	got := results[0]
	if got.Type != "mypkg.BigService" {
		t.Errorf("wrong type: %q", got.Type)
	}
	if got.Methods != 12 {
		t.Errorf("want 12 methods, got %d", got.Methods)
	}
}

func TestAnalyze_GodObjectByFields(t *testing.T) {
	sig := "struct{\nA int\nB int\nC int\nD int\nE int\nF int\nG int\nH int\nI int\nJ int\nK int\n}"
	nodes := buildNodes("pkg", "BigStruct", 2, sig)
	cfg := godobject.Config{MethodThreshold: 10, FieldThreshold: 10}
	results := godobject.Analyze(nodes, cfg)
	if len(results) != 1 {
		t.Fatalf("want 1 god object (by fields), got %d", len(results))
	}
	if results[0].Fields < 10 {
		t.Errorf("want >=10 fields, got %d", results[0].Fields)
	}
}

func TestAnalyze_BelowThreshold(t *testing.T) {
	nodes := buildNodes("pkg", "Small", 3, "struct{A int\nB int}")
	cfg := godobject.Config{MethodThreshold: 10, FieldThreshold: 10}
	results := godobject.Analyze(nodes, cfg)
	if len(results) != 0 {
		t.Fatalf("want no god objects, got %d", len(results))
	}
}

func TestAnalyze_DefaultThresholds(t *testing.T) {
	// Zero config => defaults of 10
	nodes := buildNodes("pkg", "Big", 11, "struct{}")
	results := godobject.Analyze(nodes, godobject.Config{})
	if len(results) != 1 {
		t.Fatalf("want 1 result with default thresholds, got %d", len(results))
	}
}

func TestAnalyze_SortedDescending(t *testing.T) {
	a := buildNodes("pkg", "A", 15, "struct{}")
	b := buildNodes("pkg", "B", 12, "struct{}")
	nodes := append(a, b...)
	cfg := godobject.Config{MethodThreshold: 10}
	results := godobject.Analyze(nodes, cfg)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}
	if results[0].Methods < results[1].Methods {
		t.Errorf("expected descending order: got %d then %d", results[0].Methods, results[1].Methods)
	}
}

func TestAnalyze_EmptyNodes(t *testing.T) {
	results := godobject.Analyze(nil, godobject.Config{})
	if results != nil && len(results) != 0 {
		t.Fatal("expected empty results for nil input")
	}
}

func TestAnalyze_Determinism(t *testing.T) {
	nodes := append(buildNodes("pkg", "X", 11, "struct{}"), buildNodes("pkg", "Y", 13, "struct{}")...)
	cfg := godobject.Config{MethodThreshold: 10}
	var prev []godobject.GodObject
	for i := 0; i < 5; i++ {
		r := godobject.Analyze(nodes, cfg)
		if prev != nil {
			for j := range r {
				if r[j].Type != prev[j].Type || r[j].Methods != prev[j].Methods {
					t.Fatalf("non-deterministic output at iteration %d", i)
				}
			}
		}
		prev = r
	}
}

// A React class component (LB-F377) is still a class: its methods count.
func TestAnalyze_ReactClassComponent(t *testing.T) {
	nodes := []core.SymbolNode{{Service: "svc", Package: "ui", Name: "Dashboard", Kind: core.KindJSClassComponent}}
	for i := 0; i < 11; i++ {
		nodes = append(nodes, core.SymbolNode{Service: "svc", Package: "ui", Name: "Dashboard.handle" + string(rune('A'+i)), Kind: core.KindJSMethod})
	}
	results := godobject.Analyze(nodes, godobject.Config{MethodThreshold: 10, FieldThreshold: 10})
	if len(results) != 1 || results[0].Methods != 11 {
		t.Fatalf("want the class component with 11 methods, got %+v", results)
	}
}
