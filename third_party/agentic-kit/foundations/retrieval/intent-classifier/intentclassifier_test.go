package intentclassifier_test

import (
	"testing"

	intentclassifier "github.com/geoffmilleraz/agentic-kit/foundations/retrieval/intent-classifier"
)

var testRules = []intentclassifier.Rule{
	{Intent: "search", Keywords: []string{"find", "search", "look up"}},
	{Intent: "summarize", Keywords: []string{"summarize", "summary", "tldr"}},
	{Intent: "explain", Keywords: []string{"explain", "what is", "how does"}},
	{Intent: "compare", Keywords: []string{"compare", "versus", "vs"}},
}

func TestClassifyFirstMatch(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(testRules)
	got := c.Classify("please find the document")
	if got != "search" {
		t.Errorf("expected 'search', got %q", got)
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(testRules)
	cases := []struct {
		query string
		want  intentclassifier.Intent
	}{
		{"SEARCH for invoices", "search"},
		{"Give me a SUMMARY", "summarize"},
		{"TLDR this doc", "summarize"},
		{"What Is the capital of France", "explain"},
		{"Compare option A versus B", "compare"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			got := c.Classify(tc.query)
			if got != tc.want {
				t.Errorf("Classify(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestClassifyUnknownWhenNoMatch(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(testRules)
	got := c.Classify("hello there")
	if got != intentclassifier.Unknown {
		t.Errorf("expected Unknown, got %q", got)
	}
}

func TestClassifyEmptyQueryIsUnknown(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(testRules)
	got := c.Classify("")
	if got != intentclassifier.Unknown {
		t.Errorf("expected Unknown for empty query, got %q", got)
	}
}

func TestClassifyFirstRuleWins(t *testing.T) {
	t.Parallel()

	// "search" keyword appears first in rules order; "compare" comes later.
	// A query containing both keywords should return "search".
	rules := []intentclassifier.Rule{
		{Intent: "search", Keywords: []string{"find"}},
		{Intent: "compare", Keywords: []string{"find"}}, // also matches "find"
	}
	c := intentclassifier.NewClassifier(rules)
	got := c.Classify("please find and compare")
	if got != "search" {
		t.Errorf("expected first-match 'search', got %q", got)
	}
}

func TestClassifyEmptyKeywordsRuleSkipped(t *testing.T) {
	t.Parallel()

	rules := []intentclassifier.Rule{
		{Intent: "empty", Keywords: []string{}},
		{Intent: "search", Keywords: []string{"find"}},
	}
	c := intentclassifier.NewClassifier(rules)
	got := c.Classify("find something")
	if got != "search" {
		t.Errorf("expected 'search', got %q", got)
	}
}

func TestClassifyIsDeterministic(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(testRules)
	query := "search for a summary of versus comparison"
	first := c.Classify(query)
	for i := 0; i < 100; i++ {
		got := c.Classify(query)
		if got != first {
			t.Fatalf("non-deterministic: run %d returned %q, first was %q", i, got, first)
		}
	}
}

func TestClassifyNoRulesAlwaysUnknown(t *testing.T) {
	t.Parallel()

	c := intentclassifier.NewClassifier(nil)
	got := c.Classify("anything at all")
	if got != intentclassifier.Unknown {
		t.Errorf("expected Unknown with empty rule set, got %q", got)
	}
}
