package main

// obligationssarif.go — GL-20 (docs/plans/governance-learning.md, design
// doc §8 "SARIF"): obligations and review findings render as SARIF 2.1.0
// through the PC-21 emitter (internal/verify.RenderSARIF), each with its
// own rule vocabulary, exactly like intent audit (intentAuditSARIF) and
// config audit (configAuditSARIF) already do — one more caller of the
// same emitter, never a second one.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// obligationsSARIF renders a served obligations list as SARIF results: rule
// id = the obligation's own Tier ("compiled", "raw", …) so a code-scanning
// pane groups by how confidently the rule was found, message = the rule
// text (plus Why, when the layer attached one — the raw-fallback coverage
// note, rawfallback.go), location parsed from Src ("path#Lstart-Lend",
// core.CiteRef's own shape). A row whose Src does not parse (defensive;
// every current producer sets a well-formed one) is located at line 0
// against its own Src as the file, never dropped — the same "never silent"
// discipline intentAuditSARIF and configAuditSARIF already follow.
func obligationsSARIF(obligations []semantic.Obligation) []verify.SARIFResult {
	out := make([]verify.SARIFResult, 0, len(obligations))
	for _, o := range obligations {
		file, line, end, ok := core.ParseCiteRef(o.Src)
		if !ok {
			file = o.Src
		}
		msg := o.Rule
		if o.Why != "" {
			msg = fmt.Sprintf("%s (%s)", o.Rule, o.Why)
		}
		out = append(out, verify.SARIFResult{
			RuleID: o.Tier, Level: "warning",
			Message: msg, File: file, Line: line, EndLine: end,
			Help: fmt.Sprintf("A rule that governs this path, served by fathom obligations at the %s tier: "+
				"the message is the rule, the location its source.", o.Tier),
		})
	}
	return out
}

// reviewFindingsSARIF renders review.Finding rows as SARIF results: rule id
// = the finding's own Kind ("arch_violation", "spec_uncovered", an
// obligations layer's served Kind, …), level from Severity ("error" →
// "error", "warn" → "warning", "info"/anything else → "note" — SARIF's own
// three-level vocabulary), location straight from File/Line (Finding
// already carries a resolved repo-relative location, C1-1's "one cited
// verdict structure").
func reviewFindingsSARIF(findings []review.Finding) []verify.SARIFResult {
	out := make([]verify.SARIFResult, 0, len(findings))
	for _, f := range findings {
		out = append(out, verify.SARIFResult{
			RuleID: f.Kind, Level: reviewSeverityToSARIF(f.Severity),
			Message: f.Message, File: f.File, Line: f.Line,
		})
	}
	return out
}

// reviewSeverityToSARIF maps review.Finding's own three-value Severity
// vocabulary ("info" | "warn" | "error") onto SARIF's own three result
// levels ("note" | "warning" | "error") — a straight rename, since both are
// closed three-level scales; an unrecognized value (defensive; every
// producer in internal/analysis/review sets one of the three) reads as
// "note", the least alarming level, never "error".
func reviewSeverityToSARIF(sev string) string {
	switch sev {
	case "error":
		return "error"
	case "warn":
		return "warning"
	default:
		return "note"
	}
}
