// Package guardsarif — SR-47 (docs/plans/surface-reckoning.md, design
// doc §5 row SR-47, F-29): EvaluateLayerRule's own []Violation — a guard
// verdict, a served rule checked live against the index (guardspec's
// layerrule_eval.go)
// — renders as SARIF 2.1.0 through the shared emitter
// (internal/verify.RenderSARIF, PC-21's own emitter, the one GL-20's
// obligations/review-findings SARIF already reuses) instead of a second
// one, so a code-scanning upload or an editor problem pane shows a guard
// verdict the same way it shows a gate refusal or a review finding.
//
// It lives in its own package, not internal/guardspec, because guardspec
// sits in the import closure of the public pkg/layerrules facade that a
// `fathom rules gen-test` file imports from an adopting repo; importing
// internal/verify there dragged the private agentic-kit module into that
// closure and broke the adopter build (TestCmdRulesGenTest_SeededViolation).
package guardsarif

import (
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// guardVerdictsSARIF renders violations as one SARIF result each: ruleId
// is the violation's own RuleID, level "error" for SeverityError and
// "warning" for SeverityWarn (LoadLayerRules' own fixed Severity
// vocabulary), message the violation's own cited Message. A layer-rule
// verdict has no single line the way an obligations finding does, so the
// location is the offending package directory (Violation.From) — the same
// "diff-level, not per-line" placement internal/analysis/review's
// changeRiskReceiptSARIF (SR-49) already uses for its own line-less
// findings.
func guardVerdictsSARIF(violations []guardspec.Violation) []verify.SARIFResult {
	out := make([]verify.SARIFResult, 0, len(violations))
	for _, v := range violations {
		out = append(out, verify.SARIFResult{
			RuleID:  v.RuleID,
			Level:   guardSeverityToSARIF(v.Severity),
			Message: v.Message,
			File:    v.From,
			Help: "A layer rule (.fathom/guard.cue, layer_rules) the package at this location violates; " +
				"the message names the rule's sentence and the evidence.",
		})
	}
	return out
}

// GuardVerdictsSARIF is guardVerdictsSARIF's exported entrypoint for
// callers outside this package (`fathom rules`, a CI SARIF upload).
func GuardVerdictsSARIF(violations []guardspec.Violation) []verify.SARIFResult {
	return guardVerdictsSARIF(violations)
}

func guardSeverityToSARIF(s guardspec.Severity) string {
	if s == guardspec.SeverityWarn {
		return "warning"
	}
	return "error"
}
