// guardverdictssarif.go — SR-47 (docs/plans/surface-reckoning.md, design
// doc §5 row SR-47, F-29): EvaluateLayerRule's own []Violation — a guard
// verdict, a served rule checked live against the index (layerrule_eval.go)
// — renders as SARIF 2.1.0 through the shared emitter
// (internal/verify.RenderSARIF, PC-21's own emitter, the one GL-20's
// obligations/review-findings SARIF already reuses) instead of a second
// one, so a code-scanning upload or an editor problem pane shows a guard
// verdict the same way it shows a gate refusal or a review finding.
package guardspec

import "github.com/geoffmilleraz/fathom/internal/verify"

// guardVerdictsSARIF renders violations as one SARIF result each: ruleId
// is the violation's own RuleID, level "error" for SeverityError and
// "warning" for SeverityWarn (LoadLayerRules' own fixed Severity
// vocabulary), message the violation's own cited Message. A layer-rule
// verdict has no single line the way an obligations finding does, so the
// location is the offending package directory (Violation.From) — the same
// "diff-level, not per-line" placement internal/analysis/review's
// changeRiskReceiptSARIF (SR-49) already uses for its own line-less
// findings.
func guardVerdictsSARIF(violations []Violation) []verify.SARIFResult {
	out := make([]verify.SARIFResult, 0, len(violations))
	for _, v := range violations {
		out = append(out, verify.SARIFResult{
			RuleID:  v.RuleID,
			Level:   guardSeverityToSARIF(v.Severity),
			Message: v.Message,
			File:    v.From,
		})
	}
	return out
}

// GuardVerdictsSARIF is guardVerdictsSARIF's exported entrypoint for
// callers outside this package (`fathom rules`, a CI SARIF upload).
func GuardVerdictsSARIF(violations []Violation) []verify.SARIFResult {
	return guardVerdictsSARIF(violations)
}

func guardSeverityToSARIF(s Severity) string {
	if s == SeverityWarn {
		return "warning"
	}
	return "error"
}
