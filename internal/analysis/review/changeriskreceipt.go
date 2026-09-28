// changeriskreceipt.go — SR-49 (docs/plans/surface-reckoning.md, design
// doc §5 row SR-49): review_diff (internal/mcp/review_receipt.go's
// reviewDiffReceipt) already builds the change risk receipt (CX-5, U7 —
// blast radius, certificates, conservation, rules touched, suggested
// reviewers, internal/review.RiskReceipt) over data this package's own
// Verdict resolves. changeRiskReceipt renders that SAME receipt as SARIF
// 2.1.0 and a GitHub Checks API payload, through the shared emitters
// (internal/verify.RenderSARIF / RenderCheckRun — PC-21's own emitter,
// the one GL-20's obligations/review-findings SARIF already reuses)
// instead of a second one.
//
// Deliberately a near-duplicate of cmd/fathom/reviewreceipt.go's CLI-side
// receiptSARIF/receiptCheckRunAnnotations/receiptCheckSummary: the same
// "the MCP tool and the CLI each resolve their own Input and render their
// own result" duplication reviewreceipt.go's own doc comment already
// names (buildCLIRiskReceipt is that file's own copy of
// internal/mcp/review_receipt.go's reviewDiffReceipt) — kept here, in the
// review_diff dimension package, so the MCP tool renders through the SAME
// function this row anchors rather than importing cmd/fathom.
package review

import (
	"fmt"
	"strings"

	riskreview "github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// changeRiskReceipt renders r (already built by riskreview.BuildRiskReceipt)
// as SARIF and a GitHub check-run payload for tool: one SARIF
// result / check annotation per FIRED factor (Weight != WeightNone — a
// factor that never fired is not a finding, mirroring
// cmd/fathom/reviewreceipt.go's receiptSARIF exactly), and a check-run
// summary naming the band, its cited reason and the receipt's fixed
// ADR-040 authority statement. The check-run's conclusion is always
// "neutral", never derived from the band (CX-8: fathom cites evidence and
// never scores a pass/fail verdict of its own — CI decides what a band
// does).
func changeRiskReceipt(tool string, r riskreview.RiskReceipt) (sarif []byte, checkRun []byte) {
	sarif = verify.RenderSARIF(tool, changeRiskReceiptSARIF(r))
	checkRun = verify.RenderCheckRun(tool, "neutral", changeRiskReceiptCheckSummary(r), changeRiskReceiptCheckAnnotations(r))
	return sarif, checkRun
}

// ChangeRiskReceipt is changeRiskReceipt's exported entrypoint for callers
// outside this package (internal/mcp's review_diff tool, SR-49) — the
// anchored function itself (docs/plans/surface-reckoning.md's `symbol:
// changeRiskReceipt` contract) stays unexported so its name matches the
// plan row exactly; this is a thin, zero-logic pass-through.
func ChangeRiskReceipt(tool string, r riskreview.RiskReceipt) (sarif []byte, checkRun []byte) {
	return changeRiskReceipt(tool, r)
}

// changeRiskReceiptFile is the receipt's own located-at file: a receipt is
// a diff-level summary, not a per-line citation the way
// obligationsSARIF/reviewFindingsSARIF locate an individual rule, so every
// factor's finding is placed at the receipt's first ChangedFile (mirrors
// cmd/fathom/reviewreceipt.go's receiptSARIF/receiptCheckRunAnnotations).
func changeRiskReceiptFile(r riskreview.RiskReceipt) string {
	if len(r.ChangedFiles) > 0 {
		return r.ChangedFiles[0]
	}
	return ""
}

// changeRiskReceiptSARIF renders one SARIF result per fired factor.
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
