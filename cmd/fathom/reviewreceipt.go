package main

// reviewreceipt.go — CX-6 (docs/plans/change-and-compliance.md, design doc
// §2): `fathom review --receipt --format sarif|check` renders the change
// risk receipt (CX-1..CX-5) through the shared emitters (verify.
// RenderSARIFRun, GL-21 verify.RenderCheckRun) instead of the full review,
// by way of internal/analysis/review's one risk converter (EVX-3).
//
// buildCLIRiskReceipt is the CLI's OWN copy of internal/mcp/review_receipt.
// go's reviewDiffReceipt — the same "the MCP tool and the CLI each resolve
// their own Input and render their own result" duplication
// applyConservationDimension/conservationPlaneSummaries already keep
// between internal/mcp/review_tool.go and cmd/fathom/review.go/
// reviewconservation.go, since the two run over different backends
// (QueryInterface vs *client.Client/store.Store) and — here — different
// blast-radius inputs (the CLI has no cross-service serviceref join: `fathom
// review` is store-direct against ONE service, so Links/LinkHints are
// always empty here, an honest "no fleet join available", never a
// fabricated one).
//
// Not yet wired (BuildRiskReceipt's own honest zero value, the same state
// TestRiskReceiptLowBand already proves is legitimate): rows proven, zone
// interest, uncertified contracts, suggested reviewers — review_receipt.go's
// own doc comment names exactly why, and it is unchanged here.

import (
	"context"
	"io"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
)

// buildCLIRiskReceipt assembles the change risk receipt's Input from the
// CLI's own already-resolved review.Input/Verdict (in/v — no I/O of its
// own, mirroring internal/mcp's reviewDiffReceipt exactly) and hands it to
// riskreview.BuildRiskReceipt.
func buildCLIRiskReceipt(in review.Input, v review.Verdict, table riskreview.RiskTable) riskreview.RiskReceipt {
	callers, entrypoints := cliRiskBlastRadius(in.Nodes, in.ChangedFiles, in.TypedEdges)

	input := riskreview.Input{
		ChangedFiles:          in.ChangedFiles,
		TransitiveCallers:     callers,
		Entrypoints:           entrypoints,
		CertificatesAffected:  cliRiskCertificatesAffected(in.CertificationsByFile, in.ChangedFiles),
		ConservationUnexcused: cliRiskConservationUnexcused(in.ConservationPlanes),
		ConservationDetail:    cliRiskConservationDetail(in.ConservationPlanes),
		RulesTouched:          cliRiskRulesTouched(v.Dimensions),
	}
	return riskreview.BuildRiskReceipt(input, table)
}

// cliRiskBlastRadius reuses impact.BlastRadiusWithTypedEdges (the SAME
// walk DiffImpact/review's own impact dimension already runs) rather than
// building a callgraph.Graph itself: Result.Symbols is the seed (every
// symbol defined in a changed file) plus every transitive caller, always
// sorted (impact package's own contract) — transitiveCallers is that set
// minus the seed, entrypoints the sorted subset symbolnav.Entrypoints also
// names (design doc §2 row 1's two inputs, the same split riskBlastRadius
// makes in internal/mcp/review_receipt.go).
func cliRiskBlastRadius(nodes []core.SymbolNode, changedFiles []string, typed []core.Edge) (transitiveCallers int, entrypoints []string) {
	res := impact.BlastRadiusWithTypedEdges(nodes, changedFiles, typed)

	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}
	seed := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if changed[n.FilePath] {
			seed[n.Key()] = true
		}
	}
	epSet := make(map[string]bool, len(nodes))
	for _, n := range symbolnav.Entrypoints(nodes) {
		epSet[n.Key()] = true
	}

	var eps []string
	for _, k := range res.Symbols {
		if seed[k] {
			continue // the changed symbols themselves, not a caller
		}
		transitiveCallers++
		if epSet[k] {
			eps = append(eps, k)
		}
	}
	return transitiveCallers, eps
}

// cliRiskCertificatesAffected mirrors internal/mcp/review_receipt.go's
// riskCertificatesAffected exactly, over the CLI's own identically-typed
// Input.CertificationsByFile.
func cliRiskCertificatesAffected(byFile map[string][]review.CertificationRecord, changedFiles []string) []riskreview.CertificateRef {
	seen := map[string]bool{}
	var out []riskreview.CertificateRef
	for _, f := range changedFiles {
		for _, r := range byFile[f] {
			if seen[r.Subject] {
				continue
			}
			seen[r.Subject] = true
			out = append(out, riskreview.CertificateRef{Subject: r.Subject, State: r.State})
		}
	}
	return out
}

// cliRiskConservationUnexcused mirrors internal/mcp/review_receipt.go's
// riskConservationUnexcused exactly.
func cliRiskConservationUnexcused(planes []review.ConservationPlaneSummary) bool {
	for _, p := range planes {
		if p.Verdict == "losses" {
			return true
		}
	}
	return false
}

// cliRiskConservationDetail mirrors internal/mcp/review_receipt.go's
// riskConservationDetail exactly.
func cliRiskConservationDetail(planes []review.ConservationPlaneSummary) string {
	for _, p := range planes {
		if p.Verdict != "losses" {
			continue
		}
		for _, f := range p.Findings {
			if f.Detail != "" {
				return p.Plane + ": " + f.Detail
			}
		}
		return p.Plane + ": conservation loss"
	}
	return ""
}

// cliRiskRulesTouched mirrors internal/mcp/review_receipt.go's
// riskRulesTouched exactly, over the CLI's own identically-typed
// Verdict.Dimensions (both surfaces run the SAME internal/analysis/review.
// Run and the SAME review.BuildPersist → ApplyClaimStatus label pass before
// this point).
func cliRiskRulesTouched(dims []review.Dimension) []riskreview.RuleRef {
	var out []riskreview.RuleRef
	for _, d := range dims {
		if d.Name != review.DimObligations && d.Name != review.DimArch {
			continue
		}
		for _, f := range d.Findings {
			if f.Status == string(core.ClaimRefuted) || f.Status == string(core.ClaimSuperseded) {
				continue
			}
			var level string
			switch f.Severity {
			case "error":
				level = riskreview.RuleLevelError
			case "warn":
				level = riskreview.RuleLevelWarn
			default:
				continue
			}
			out = append(out, riskreview.RuleRef{File: f.File, Rule: f.Message, Level: level, Citation: f.Citation})
		}
	}
	return out
}

// cmdReviewReceipt renders r as format ("sarif" or "check" — runReview's
// own flag validation already refused any other value before this is
// reached) through internal/analysis/review's ChangeRiskReceipt: the ONE
// risk converter review_diff renders through too (EVX-3), whose SARIF run
// carries run's provenance. The check-run's conclusion is always
// "neutral", never derived from the band: CX-8/ADR-040's authority
// boundary means fathom cites evidence and never scores a pass/fail
// verdict of its own — CI decides what a band does.
func cmdReviewReceipt(stdout io.Writer, format string, run receipt.Run, r riskreview.RiskReceipt) int {
	sarif, check := review.ChangeRiskReceipt(context.Background(), "fathom review --receipt", run, r)
	switch format {
	case "sarif":
		_, _ = stdout.Write(sarif)
	case "check":
		_, _ = stdout.Write(check)
	}
	return 0
}
