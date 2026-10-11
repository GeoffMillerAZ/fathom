package main

// obligationssarif.go — GL-20 (docs/plans/governance-learning.md, design
// doc §8 "SARIF"): obligations render as SARIF 2.1.0 through the shared
// emitter (internal/verify.RenderSARIFRun, EVX-3) with their own rule
// vocabulary, exactly like intent audit (intentAuditSARIF) and config audit
// (configAuditSARIF) already do — one more caller of the same emitter,
// never a second one. Review findings render through the change risk
// receipt's one converter (internal/analysis/review's
// changeRiskReceiptSARIF, `fathom review --receipt`); the CLI-only
// reviewFindingsSARIF that no product path called is gone (EVX-4).

import (
	"fmt"

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
