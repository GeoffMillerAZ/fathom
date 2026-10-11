// changeriskreceipt.go — SR-49 (docs/plans/surface-reckoning.md, design
// doc §5 row SR-49): review_diff (internal/mcp/review_receipt.go's
// reviewDiffReceipt) already builds the change risk receipt (CX-5, U7 —
// blast radius, certificates, conservation, rules touched, suggested
// reviewers, internal/review.RiskReceipt) over data this package's own
// Verdict resolves. changeRiskReceipt renders that SAME receipt as SARIF
// 2.1.0 and a GitHub Checks API payload, through the shared emitters
// (internal/verify.RenderSARIFRun / RenderCheckRun) instead of a second one.
//
// EVX-3 (docs/plans/portable-receipts.md): this is the ONE risk converter.
// The CLI's `fathom review --receipt` (cmd/fathom/reviewreceipt.go) and the
// MCP review_diff tool both render through ChangeRiskReceipt, so the CLI's
// former copy (receiptSARIF and its check-run mirrors) is gone, and every
// SARIF run carries its provenance (receipt.AnchorRun).
package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// changeRiskReceipt renders r (already built by riskreview.BuildRiskReceipt)
// as SARIF and a GitHub check-run payload for tool: one SARIF
// result / check annotation per FIRED factor (Weight != WeightNone — a
// factor that never fired is not a finding), plus one SARIF note per
// sub-typed link hint (I57), and a check-run summary naming the band, its
// cited reason and the receipt's fixed ADR-040 authority statement. The
// SARIF run carries run's provenance (receipt.AnchorRun, EVX-3). The
// check-run's conclusion is always "neutral", never derived from the band
// (CX-8: fathom cites evidence and never scores a pass/fail verdict of its
// own — CI decides what a band does).
func changeRiskReceipt(ctx context.Context, tool string, run receipt.Run, r riskreview.RiskReceipt) (sarif []byte, checkRun []byte) {
	prov, results := receipt.AnchorRun(ctx, run, changeRiskReceiptSARIF(r))
	sarif = verify.RenderSARIFRun(tool, prov, results)
	checkRun = verify.RenderCheckRun(tool, "neutral", changeRiskReceiptCheckSummary(r), changeRiskReceiptCheckAnnotations(r))
	return sarif, checkRun
}

// ChangeRiskReceipt is changeRiskReceipt's exported entrypoint for callers
// outside this package (internal/mcp's review_diff tool, SR-49; the CLI's
// `fathom review --receipt`, EVX-3) — the anchored function itself
// (docs/plans/surface-reckoning.md's `symbol: changeRiskReceipt` contract)
// stays unexported so its name matches the plan row exactly; this is a
// thin, zero-logic pass-through.
func ChangeRiskReceipt(ctx context.Context, tool string, run receipt.Run, r riskreview.RiskReceipt) (sarif []byte, checkRun []byte) {
	return changeRiskReceipt(ctx, tool, run, r)
}

// changeRiskReceiptFile is the receipt's own located-at file: a receipt is
// a diff-level summary, not a per-line citation the way obligationsSARIF
// locates an individual rule, so every factor's finding is placed at the
// receipt's first ChangedFile.
func changeRiskReceiptFile(r riskreview.RiskReceipt) string {
	if len(r.ChangedFiles) > 0 {
		return r.ChangedFiles[0]
	}
	return ""
}

// ruleLinkBelowTypedTier is the rule a sub-typed link hint renders under.
const ruleLinkBelowTypedTier = "link_below_typed_tier"

// changeRiskReceiptSARIF renders one SARIF result per fired factor (ruleId
// the factor's fixed Name, level from its Weight: high is error, medium
// warning, message its cited Reason and Evidence), then one note per link
// hint: a cross-boundary link below the typed tier is listed with its tier
// named and never counted, so it never reads level error (I57).
// RiskFactor.Evidence is a flattened []string (CX-1's contract), so every
// result is located at the receipt's first changed file.
func changeRiskReceiptSARIF(r riskreview.RiskReceipt) []verify.SARIFResult {
	file := changeRiskReceiptFile(r)
	var out []verify.SARIFResult
	for _, f := range r.Factors {
		if f.Weight == riskreview.WeightNone {
			continue
		}
		out = append(out, verify.SARIFResult{
			RuleID:  f.Name,
			Level:   changeRiskWeightToSARIF(f.Weight),
			Message: changeRiskFactorMessage(f),
			File:    file,
			Help: "A change risk factor that fired for this diff (the change risk receipt, CX-5): fathom cites the evidence, " +
				"and CI decides what the band does (CX-8).",
		})
	}
	for _, l := range r.LinkHints {
		consumer := l.Consumer
		if l.ConsumerService != "" {
			consumer += " (" + l.ConsumerService + ")"
		}
		msg := fmt.Sprintf("%s -> %s: a %s-tier link, listed and not counted (I57)", l.Provider, consumer, l.Tier)
		if l.Citation != "" {
			msg += " [" + l.Citation + "]"
		}
		out = append(out, verify.SARIFResult{
			RuleID:  ruleLinkBelowTypedTier,
			Level:   "note",
			Message: msg,
			File:    file,
			Help: "A cross-boundary link below the typed tier (literal, template) that the diff touches. It is listed with " +
				"its tier named and never moves the band or gates (I57).",
		})
	}
	return out
}

// changeRiskReceiptCheckAnnotations mirrors changeRiskReceiptSARIF's own
// per-fired-factor walk, rendered as Checks API annotations instead —
// AnnotationLevel communicates the factor's OWN weight (an informational
// marker on that one line), never the check-run's overall conclusion
// (always "neutral", CX-8).
func changeRiskReceiptCheckAnnotations(r riskreview.RiskReceipt) []verify.CheckRunAnnotation {
	file := changeRiskReceiptFile(r)
	var out []verify.CheckRunAnnotation
	for _, f := range r.Factors {
		if f.Weight == riskreview.WeightNone {
			continue
		}
		level := "warning"
		if f.Weight == riskreview.WeightHigh {
			level = "failure"
		}
		out = append(out, verify.CheckRunAnnotation{
			Path: file, StartLine: 1, EndLine: 1,
			AnnotationLevel: level,
			Title:           f.Name,
			Message:         changeRiskFactorMessage(f),
		})
	}
	return out
}

// changeRiskReceiptCheckSummary is the check-run's output.summary: the
// band, its cited reason, and the fixed authority statement (CX-8) so a
// reader of the raw check-run JSON sees the boundary even without reading
// this row's own documentation.
func changeRiskReceiptCheckSummary(r riskreview.RiskReceipt) string {
	return fmt.Sprintf("band: %s (%s). %s", r.Band, r.BandReason, r.Authority)
}

func changeRiskFactorMessage(f riskreview.RiskFactor) string {
	if len(f.Evidence) == 0 {
		return f.Reason
	}
	return fmt.Sprintf("%s (%s)", f.Reason, strings.Join(f.Evidence, "; "))
}

func changeRiskWeightToSARIF(w riskreview.FactorWeight) string {
	if w == riskreview.WeightHigh {
		return "error"
	}
	return "warning"
}
