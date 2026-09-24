package verify

// lexicalbounds_test.go — CV-29 (ADR-044 §3.1): lexicalBoundsVerdict
// evaluates a lexical row's max/min/eq bounds against its pattern's match
// count, not merely "at least one match."

import (
	"strings"
	"testing"
)

func TestLexicalOracle_MaxMinEq(t *testing.T) {
	body := []byte("x x x\nx\n") // 4 occurrences of "x"
	locs := [][]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}}

	two, three, four, five, zero := 2, 3, 4, 5, 0

	cases := []struct {
		name       string
		task       Task
		wantStatus CriterionStatus
		wantLabel  string
	}{
		{"no bounds, matches present", Task{OraclePath: "f", OraclePattern: "x"}, Pass, ""},
		{"eq exact match", Task{OraclePath: "f", OraclePattern: "x", Eq: &four}, Pass, ""},
		{"eq mismatch", Task{OraclePath: "f", OraclePattern: "x", Eq: &three}, Fail, "bounds-unmet"},
		{"min satisfied", Task{OraclePath: "f", OraclePattern: "x", Min: &two}, Pass, ""},
		{"min unmet", Task{OraclePath: "f", OraclePattern: "x", Min: &five}, Fail, "bounds-unmet"},
		{"max satisfied", Task{OraclePath: "f", OraclePattern: "x", Max: &five}, Pass, ""},
		{"max exceeded", Task{OraclePath: "f", OraclePattern: "x", Max: &two}, Fail, "bounds-unmet"},
		{"range satisfied", Task{OraclePath: "f", OraclePattern: "x", Min: &two, Max: &five}, Pass, ""},
		{"eq zero proves absence", Task{OraclePath: "f", OraclePattern: "x", Eq: &zero}, Fail, "bounds-unmet"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			crit, label := lexicalBoundsVerdict(c.task, body, locs)
			if crit.Status != c.wantStatus || label != c.wantLabel {
				t.Errorf("status/label = %s/%s, want %s/%s (detail: %s)", crit.Status, label, c.wantStatus, c.wantLabel, crit.Detail)
			}
		})
	}

	// Zero matches: the default ("at least one") behaviour is unchanged,
	// and an explicit Eq:0/Min:0 now PROVES absence — the gone: pattern
	// use case (design §3.1).
	t.Run("zero matches, no bounds, fails as before", func(t *testing.T) {
		crit, label := lexicalBoundsVerdict(Task{OraclePath: "f", OraclePattern: "x"}, body, nil)
		if crit.Status != Fail || label != "pattern-unmatched" {
			t.Errorf("status/label = %s/%s, want fail/pattern-unmatched", crit.Status, label)
		}
	})
	t.Run("zero matches, eq zero, proves absence", func(t *testing.T) {
		crit, label := lexicalBoundsVerdict(Task{OraclePath: "f", OraclePattern: "x", Eq: &zero}, body, nil)
		if crit.Status != Pass || label != "" {
			t.Errorf("status/label = %s/%s, want pass/'' (detail: %s)", crit.Status, label, crit.Detail)
		}
	})
	t.Run("zero matches, min zero, proves absence", func(t *testing.T) {
		crit, label := lexicalBoundsVerdict(Task{OraclePath: "f", OraclePattern: "x", Min: &zero}, body, nil)
		if crit.Status != Pass || label != "" {
			t.Errorf("status/label = %s/%s, want pass/'' (detail: %s)", crit.Status, label, crit.Detail)
		}
	})
}

// TestLexicalContractError_BoundsShapeRefusals pins the contract-shape
// refusals CV-29 adds: eq combined with max/min, a negative bound, an
// inverted max<min range, and a malformed (non-integer) bound annotation —
// each refused by BOTH LintPlan and verify (lexicalContractError is the
// single source both read).
func TestLexicalContractError_BoundsShapeRefusals(t *testing.T) {
	one, negOne, five, two := 1, -1, 5, 2
	cases := []struct {
		name string
		task Task
		want string
	}{
		{"eq with max", Task{Oracle: "lexical", OraclePath: "f", Eq: &one, Max: &five}, "eq cannot be combined"},
		{"eq with min", Task{Oracle: "lexical", OraclePath: "f", Eq: &one, Min: &one}, "eq cannot be combined"},
		{"negative max", Task{Oracle: "lexical", OraclePath: "f", Max: &negOne}, "must be non-negative"},
		{"negative min", Task{Oracle: "lexical", OraclePath: "f", Min: &negOne}, "must be non-negative"},
		{"negative eq", Task{Oracle: "lexical", OraclePath: "f", Eq: &negOne}, "must be non-negative"},
		{"inverted range", Task{Oracle: "lexical", OraclePath: "f", Max: &two, Min: &five}, "less than min"},
		{"malformed bound", Task{Oracle: "lexical", OraclePath: "f", BoundsInvalid: []string{"max"}}, "malformed integer bound"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := lexicalContractError(c.task)
			if got == "" {
				t.Fatalf("lexicalContractError returned no error, want one containing %q", c.want)
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("lexicalContractError = %q, want it to contain %q", got, c.want)
			}
		})
	}
}
