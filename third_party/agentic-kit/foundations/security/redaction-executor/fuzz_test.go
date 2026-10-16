package redactionexecutor_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	re "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/redaction-executor"
)

// fuzzPolicy is a realistic five-rule PII/secret policy used by FuzzRedact.
// It deliberately KEEPS \b-anchored patterns (phone, SSN) and bracket-token
// replacements, because that combination is what exposed the historical
// single-pass leak documented on FuzzRedact below.
func fuzzPolicy() re.RedactionPolicy {
	return re.RedactionPolicy{
		Name:    "fuzz-pii-v1",
		Version: "1.0.0",
		Rules: []re.RedactionRule{
			emailRule(), // REDACT-PII-001 -> [EMAIL]
			phoneRule(), // REDACT-PII-002 -> [PHONE]
			ssnRule(),   // REDACT-PII-003 -> [SSN]
			{
				ID:          "REDACT-TOKEN-001",
				Pattern:     `Bearer\s+[A-Za-z0-9._\-]+`,
				Replacement: "[TOKEN]",
			},
			{
				ID:          "REDACT-SECRET-001",
				Pattern:     `SECRET_KEY=\S+`,
				Replacement: "[SECRET]",
			},
		},
	}
}

// FuzzRedact exercises Executor.Redact against arbitrary, hostile, huge, and
// invalid-UTF-8 input under a fixed realistic policy (fuzzPolicy).
//
// SCOPE: robustness and the package's documented invariants. ReDoS is out of
// scope by construction — Go's regexp is RE2-based and linear-time, so no
// input can cause catastrophic backtracking.
//
// HISTORY (single-pass leak, now fixed): Redact used to make exactly one
// sweep over the rules, and the fuzzer proved that a single sweep can leave
// PII unredacted — one rule's replacement creates word boundaries that
// unmask a later match. Counterexample, retained as a seed below and as a
// regression test (TestRedact_Counterexample_SingleCallComplete):
//
//	input:   "alice@example.com555-867-5309 123-45-6789Bearer tok"
//	sweep 1: "[EMAIL][PHONE] 123-45-6789[TOKEN]"   <-- SSN survived
//	sweep 2: "[EMAIL][PHONE] [SSN][TOKEN]"
//
// The SSN rule (\b...\b) failed to fire on sweep 1 because "123-45-6789" is
// glued to "Bearer" (no word boundary at 9->B); the Token rule's replacement
// introduced the "]" boundary that unmasked it. The library now owns
// convergence: Redact iterates sweeps to a textual fixpoint internally
// (capped at MaxPasses), so a SINGLE call is complete and idempotent. This
// fuzz target asserts that directly.
//
// Invariants asserted for every input:
//   - Redact never panics (the fuzz harness converts panics into failures).
//   - Deterministic: redacting the same text twice yields identical output
//     and identical Applied records.
//   - Convergence: fuzzPolicy's replacements are inert bracket tokens that
//     can never feed any rule's pattern, so every input must converge well
//     within MaxPasses. A Converged=false here is a real finding (an
//     unmask chain deeper than the cap).
//   - Single-call completeness: no rule pattern matches the output of ONE
//     Redact call — nothing the policy should redact survives.
//   - Idempotency: Redact(Redact(x)) == Redact(x), and the second call
//     applies zero rules.
//   - Applied records reference policy rules, in policy order, at most once
//     each, with Occurrences >= 1 (totals across internal sweeps).
//   - Valid-UTF-8 input produces valid-UTF-8 output.
func FuzzRedact(f *testing.F) {
	policy := fuzzPolicy()
	exec, err := re.NewExecutor(policy)
	if err != nil {
		f.Fatalf("NewExecutor: %v", err)
	}

	// Independent compilation of the same patterns for apply-then-scan.
	compiled := make([]*regexp.Regexp, len(policy.Rules))
	ruleIndex := make(map[string]int, len(policy.Rules))
	for i, r := range policy.Rules {
		compiled[i] = regexp.MustCompile(r.Pattern)
		ruleIndex[r.ID] = i
	}

	// One trigger per rule.
	f.Add("Contact alice@example.com for details.")
	f.Add("Call 555-867-5309 after hours.")
	f.Add("SSN 123-45-6789 on file.")
	f.Add("Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig")
	f.Add("Config: SECRET_KEY=hunter2 deployed")

	// Overlapping / interacting matches:
	//   - phone-shaped digits swallowed by the earlier email rule,
	//   - adjacent emails sharing an "@" run,
	//   - an email inside a SECRET_KEY value,
	//   - an SSN directly after "Bearer ",
	//   - digit runs near phone/SSN boundaries,
	//   - the historical single-pass-leak counterexample documented above.
	f.Add("555.867.5309@example.com")
	f.Add("a@b.cc@example.com and x@y.zz@example.com")
	f.Add("SECRET_KEY=alice@example.com")
	f.Add("Bearer 123-45-6789")
	f.Add("123-45-67891 and 1123-45-6789 and 55518675309")
	f.Add("alice@example.com555-867-5309 123-45-6789Bearer tok")

	// Huge input (~1 MiB) dense with matches.
	f.Add(strings.Repeat("email alice@example.com phone 555-867-5309 ssn 123-45-6789 ", 1<<14))

	// Invalid UTF-8 around and inside triggers, quote soup, empty.
	f.Add("\xff\xfealice@example.com\x80 555-867-5309\xc3\x28")
	f.Add("SECRET_KEY=\xf0\x28\x8c\x28abc Bearer \xffzzz")
	f.Add(`"'"''"{["('\")]}'"` + "```" + `{{"''[("\\"'`)
	f.Add("")

	f.Fuzz(func(t *testing.T, text string) {
		once, report := exec.RedactReport(text)
		applied := report.Applied

		// Determinism.
		again, appliedAgain := exec.Redact(text)
		if once != again {
			t.Errorf("Redact not deterministic:\n first=%q\nsecond=%q", once, again)
		}
		if !reflect.DeepEqual(applied, appliedAgain) {
			t.Errorf("Applied not deterministic:\n first=%+v\nsecond=%+v", applied, appliedAgain)
		}

		// Convergence: this policy's replacements are inert, so the cap must
		// never be hit. Non-convergence means an unmask chain deeper than
		// MaxPasses — a reportable finding.
		if !report.Converged {
			t.Errorf("Redact did not converge within %d passes\n  input=%q\n  output=%q",
				re.MaxPasses, text, once)
		}

		// Single-call completeness: nothing the policy targets survives ONE
		// Redact call.
		for i, cre := range compiled {
			if loc := cre.FindStringIndex(once); loc != nil {
				t.Errorf("rule %s still matches single-call output at %v: matched %q\n  input=%q\n  output=%q",
					policy.Rules[i].ID, loc, once[loc[0]:loc[1]], text, once)
			}
		}

		// Idempotency: re-redacting the output is a no-op and applies nothing.
		stable, appliedStable := exec.Redact(once)
		if stable != once {
			t.Errorf("not idempotent:\n  once=%q\n  twice=%q", once, stable)
		}
		if len(appliedStable) != 0 {
			t.Errorf("second Redact still applied %d rules: %+v (input=%q)",
				len(appliedStable), appliedStable, text)
		}

		// Applied records: known rules, policy order, no dups, >=1.
		prev := -1
		for i, a := range applied {
			idx, ok := ruleIndex[a.RuleID]
			if !ok {
				t.Errorf("applied[%d] references unknown rule ID %q", i, a.RuleID)
				continue
			}
			if idx <= prev {
				t.Errorf("applied[%d] (rule %s) out of policy order or duplicated", i, a.RuleID)
			}
			prev = idx
			if a.Occurrences < 1 {
				t.Errorf("applied[%d] (rule %s) has Occurrences=%d, want >= 1", i, a.RuleID, a.Occurrences)
			}
		}

		// UTF-8 preservation.
		if utf8.ValidString(text) && !utf8.ValidString(once) {
			t.Errorf("valid UTF-8 input produced invalid UTF-8 output: %q", once)
		}
	})
}
