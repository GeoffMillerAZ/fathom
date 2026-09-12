package indexbench

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// TestReferencesOracleSubsetPartitionsQueries pins BP-30's two exclusions:
// a method (its mention in text is the bare method name) and a function
// whose bare name some other declaration shares (no honest per-query
// verdict exists) both leave the scored set, and both are RECORDED rather
// than silently dropped.
func TestReferencesOracleSubsetPartitionsQueries(t *testing.T) {
	truth := publicloc.GoTruth{
		DeclsByFile: map[string][]string{
			"a.go": {"Unique", "Close", "Recv.Method"},
			"b.go": {"Handle.Close"},
		},
		UsesByName: map[string][]string{"Unique": {"c.go"}},
	}
	sub := NewReferencesOracleSubset(truth, "")

	if len(sub.Names) != 1 || sub.Names[0] != "Unique" {
		t.Errorf("Names = %v, want [Unique] — only an unambiguously-named receiverless function is adjudicable", sub.Names)
	}
	if len(sub.AmbiguousNames) != 1 || sub.AmbiguousNames[0] != "Close" {
		t.Errorf("AmbiguousNames = %v, want [Close] — Handle.Close shares its bare name", sub.AmbiguousNames)
	}
	if len(sub.MethodNames) != 2 {
		t.Errorf("MethodNames = %v, want both methods", sub.MethodNames)
	}
}

// TestReferencesOracleSubsetAdjudicableFiles: only Go files the type checker
// actually walked can be ruled on. A doc hit and an unwalked Go file are
// outside its competence, not wrong answers.
func TestReferencesOracleSubsetAdjudicableFiles(t *testing.T) {
	truth := publicloc.GoTruth{
		DeclsByFile: map[string][]string{"a.go": {"Unique"}},
		UsesByName:  map[string][]string{"Unique": {"c.go"}},
	}
	sub := NewReferencesOracleSubset(truth, "svc/")

	for _, c := range []struct {
		path string
		want bool
	}{
		{"svc/a.go", true},  // walked: a declaration site
		{"svc/c.go", true},  // walked: a use site
		{"svc/d.go", false}, // Go, but the checker never saw it
		{"svc/README.md", false},
		{"a.go", false}, // the prefix is part of the identity
	} {
		if got := sub.Adjudicable(c.path); got != c.want {
			t.Errorf("Adjudicable(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
