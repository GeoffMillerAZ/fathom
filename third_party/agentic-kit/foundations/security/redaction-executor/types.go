// Package redactionexecutor executes a redaction policy over text.
// Rules and replacements are data — they are supplied by the caller
// (from CUE, configuration, or programmatically). The package itself
// contains no org-specific literals.
//
// Redact sweeps the rule list over the text repeatedly until the output
// stabilizes (a textual fixpoint). Iteration is required for completeness:
// a single sweep can leave matches unredacted, because one rule's
// replacement may create word boundaries that expose a match an earlier
// rule could not see (e.g. an SSN glued to a Bearer token only becomes
// \b-matchable after the token is rewritten). Sweeps are capped at
// MaxPasses to guard against pathological policies whose replacements
// re-trigger one another indefinitely; RedactReport surfaces that case
// via Report.Converged.
//
// Key invariants:
//   - Fixpoint idempotency: for any policy that converges within MaxPasses
//     sweeps (every realistic policy), a single Redact call reaches a
//     stable result — Redact(Redact(x)) == Redact(x).
//   - Deterministic: rules are applied in policy order within each sweep;
//     identical input and policy always yield identical output.
//   - Applied carries provenance for every rule that fired, with occurrence
//     counts totalled across all sweeps.
//   - Pathological non-convergent policies: Redact returns the final
//     sweep's output; RedactReport reports Converged == false.
//
// Concurrency: an Executor is immutable after NewExecutor and safe for
// concurrent use by multiple goroutines.
package redactionexecutor

// RedactionRule is a single rule in a redaction policy.
// Pattern must be a valid Go regexp. Replacement is the literal string
// substituted for every match; an empty replacement removes matched text.
type RedactionRule struct {
	// ID is a unique, stable identifier for the rule (e.g. "REDACT-PII-001").
	ID string

	// Pattern is a Go regexp matched against the input text.
	Pattern string

	// Replacement is the string substituted for every match.
	Replacement string
}

// RedactionPolicy is a named, versioned collection of RedactionRules.
type RedactionPolicy struct {
	// Name identifies the policy (e.g. "pii-v1").
	Name string

	// Version is a semver-style string (e.g. "1.0.0").
	Version string

	// Rules is the ordered list of redaction rules. Rules are applied in
	// the order they appear; earlier rules see the original text, later
	// rules see text already modified by earlier rules.
	Rules []RedactionRule
}

// Applied records that a single redaction rule fired and what it replaced.
type Applied struct {
	// RuleID is the ID of the rule that fired.
	RuleID string

	// Occurrences is the number of non-overlapping matches that were
	// replaced, totalled across every sweep of a single Redact call.
	Occurrences int
}

// Report describes a full Redact run. It is returned by RedactReport;
// Redact itself returns only the Applied slice for source compatibility.
type Report struct {
	// Applied lists, in policy order, every rule that fired at least once
	// during the run, with occurrence counts totalled across all sweeps.
	Applied []Applied

	// Passes is the number of full rule sweeps executed (always >= 1).
	// When Converged is true, the final counted sweep is the one that
	// observed no change.
	Passes int

	// Converged reports whether the output reached a textual fixpoint
	// within MaxPasses sweeps. It is false only for pathological policies
	// whose replacements re-trigger rules indefinitely; the accompanying
	// text is then the final sweep's output and may still contain matches.
	Converged bool
}
