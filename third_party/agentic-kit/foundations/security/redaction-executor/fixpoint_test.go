package redactionexecutor_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	re "github.com/geoffmilleraz/agentic-kit/foundations/security/redaction-executor"
)

// ---- Regression: the fuzz counterexample fully redacts in ONE call ----------
//
// Historically a single Redact pass left the SSN below unredacted: it is
// glued to "Bearer" (no \b at 9->B) on the first sweep and only becomes
// matchable after the token rule rewrites "Bearer tok" to "[TOKEN]". Redact
// now iterates to a fixpoint internally, so one call is complete.

func TestRedact_Counterexample_SingleCallComplete(t *testing.T) {
	exec, err := re.NewExecutor(fuzzPolicy())
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	input := "alice@example.com555-867-5309 123-45-6789Bearer tok"

	got, applied := exec.Redact(input)

	if want := "[EMAIL][PHONE] [SSN][TOKEN]"; got != want {
		t.Errorf("single Redact call must fully redact:\n got=%q\nwant=%q", got, want)
	}

	// Every rule's pattern must fail to match the single-call output.
	for _, r := range fuzzPolicy().Rules {
		if loc := regexp.MustCompile(r.Pattern).FindStringIndex(got); loc != nil {
			t.Errorf("rule %s still matches single-call output at %v: %q",
				r.ID, loc, got[loc[0]:loc[1]])
		}
	}

	// Applied: one entry per fired rule, in policy order, totals across sweeps.
	want := []re.Applied{
		{RuleID: "REDACT-PII-001", Occurrences: 1},
		{RuleID: "REDACT-PII-002", Occurrences: 1},
		{RuleID: "REDACT-PII-003", Occurrences: 1}, // the formerly-leaked SSN
		{RuleID: "REDACT-TOKEN-001", Occurrences: 1},
	}
	if !reflect.DeepEqual(applied, want) {
		t.Errorf("Applied:\n got=%+v\nwant=%+v", applied, want)
	}
}

// ---- True idempotency: Redact(Redact(x)) == Redact(x) -----------------------

func TestRedact_TrueIdempotency(t *testing.T) {
	exec, err := re.NewExecutor(fuzzPolicy())
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	inputs := []string{
		"alice@example.com555-867-5309 123-45-6789Bearer tok", // the counterexample
		"Contact alice@example.com or 555-867-5309.",
		"Bearer 123-45-6789",
		"SECRET_KEY=alice@example.com",
		"nothing sensitive here",
		"",
	}
	for _, input := range inputs {
		once, _ := exec.Redact(input)
		twice, appliedTwice := exec.Redact(once)
		if twice != once {
			t.Errorf("idempotency violated for %q:\n  once=%q\n twice=%q", input, once, twice)
		}
		if len(appliedTwice) != 0 {
			t.Errorf("re-redacting converged output of %q applied %d rules: %+v",
				input, len(appliedTwice), appliedTwice)
		}
	}
}

// ---- RedactReport: pass counts and convergence -------------------------------

func TestRedactReport_PassesAndConverged(t *testing.T) {
	exec, err := re.NewExecutor(fuzzPolicy())
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		wantPasses int
	}{
		// No matches: the first sweep observes no change.
		{"no matches", "nothing sensitive here", 1},
		// One visible match: sweep 1 redacts, sweep 2 confirms stability.
		{"single email", "Contact alice@example.com.", 2},
		// The counterexample: sweep 1 unmasks the SSN, sweep 2 redacts it,
		// sweep 3 confirms stability.
		{"unmasking chain", "alice@example.com555-867-5309 123-45-6789Bearer tok", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, report := exec.RedactReport(tt.input)
			if !report.Converged {
				t.Errorf("Converged = false, want true")
			}
			if report.Passes != tt.wantPasses {
				t.Errorf("Passes = %d, want %d", report.Passes, tt.wantPasses)
			}
		})
	}
}

// Convergence is defined as TEXTUAL stability, not match-freedom: a rule whose
// replacement matches its own pattern (a rewriting rule like
// "SECRET_KEY=..." -> "SECRET_KEY=[REDACTED]") converges because the second
// sweep replaces the match with identical text.
func TestRedactReport_SelfMatchingReplacementConverges(t *testing.T) {
	exec, err := re.NewExecutor(re.RedactionPolicy{
		Name:    "rewrite-v1",
		Version: "1.0.0",
		Rules: []re.RedactionRule{
			{ID: "REWRITE-001", Pattern: `SECRET_KEY=\S+`, Replacement: "SECRET_KEY=[REDACTED]"},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	got, report := exec.RedactReport("Config: SECRET_KEY=hunter2")
	if want := "Config: SECRET_KEY=[REDACTED]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !report.Converged {
		t.Errorf("Converged = false, want true (self-matching replacement is a textual fixpoint)")
	}

	// Idempotent on the string even though the pattern still matches it.
	again, _ := exec.Redact(got)
	if again != got {
		t.Errorf("not idempotent: %q -> %q", got, again)
	}
}

// ---- Iteration cap: pathological mutually re-triggering rules ----------------
//
// Rule A's replacement contains rule B's trigger and vice versa, arranged so
// each full sweep grows the text and never stabilizes:
//
//	sweep k:  "xy"*(k-1) + "A"  --R1-->  "xy"*(k-1) + "xB"  --R2-->  "xy"*k + "A"
//
// Redact must stop at MaxPasses, return the final sweep's output, and
// RedactReport must surface Converged == false (the documented option-(a)
// behavior: no error, honest indicator on the report shape).

func TestRedactReport_IterationCap_NonConvergentPolicy(t *testing.T) {
	exec, err := re.NewExecutor(re.RedactionPolicy{
		Name:    "oscillator-v1",
		Version: "1.0.0",
		Rules: []re.RedactionRule{
			{ID: "OSC-A", Pattern: `A`, Replacement: "xB"},
			{ID: "OSC-B", Pattern: `B`, Replacement: "yA"},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	got, report := exec.RedactReport("A")

	if report.Converged {
		t.Errorf("Converged = true, want false for a non-convergent policy")
	}
	if report.Passes != re.MaxPasses {
		t.Errorf("Passes = %d, want MaxPasses (%d)", report.Passes, re.MaxPasses)
	}
	if want := strings.Repeat("xy", re.MaxPasses) + "A"; got != want {
		t.Errorf("output must be the final sweep's result:\n got=%q\nwant=%q", got, want)
	}

	// Each rule fires exactly once per sweep.
	wantApplied := []re.Applied{
		{RuleID: "OSC-A", Occurrences: re.MaxPasses},
		{RuleID: "OSC-B", Occurrences: re.MaxPasses},
	}
	if !reflect.DeepEqual(report.Applied, wantApplied) {
		t.Errorf("Applied:\n got=%+v\nwant=%+v", report.Applied, wantApplied)
	}

	// Plain Redact returns the same last-pass output and the same Applied —
	// it just drops the explicit convergence indicator. Still deterministic.
	plain, plainApplied := exec.Redact("A")
	if plain != got {
		t.Errorf("Redact/RedactReport diverge: %q vs %q", plain, got)
	}
	if !reflect.DeepEqual(plainApplied, wantApplied) {
		t.Errorf("Redact Applied:\n got=%+v\nwant=%+v", plainApplied, wantApplied)
	}
}
