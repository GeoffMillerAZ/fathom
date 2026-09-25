package main

// reviewconservation.go — DimConservation's cmd/fathom wiring (ADR-044
// §1.6/§2, wave 2): when `fathom review --since <ref>` resolves a base
// (buildReviewInputFromClient already sets review.Input.Base, CV-13),
// this builds a real internal/conserve.Input via conserve.NewInput and
// runs it through conserve.Run(ctx, in, conserve.Planes()) — every
// registered plane, the exact composition cmd/fathom/conserve.go's own
// cmdConserve already uses — so `fathom review --since`'s conservation
// dimension reports the SAME planes `fathom conserve` would, not a
// hand-rolled subset. Before this file, DimConservation always read
// checked:false ("conservation input could not be built") on every
// review run, whatever --since named: review.Input.ConservationSupported/
// ConservationPlanes were never set by any caller.
import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// applyConservationDimension fills in.ConservationSupported/
// ConservationPlanes when in.Base is set (a no-op otherwise —
// DimConservation's own "needs a base ref" gate already covers that
// case). conserve.NewInput does its own git resolution independently of
// the caller's already-resolved in.Base/in.ChangedFiles (in.Base is
// itself a valid, already-resolved ref to hand it — resolving an
// already-resolved sha is a no-op `git rev-parse`), exactly like
// cmd/fathom/conserve.go's cmdConserve builds its own Input rather than
// reusing review.Input's fields. Fails open: an error building the
// conserve.Input (an unreadable corpus, since Base is by construction
// already resolvable) leaves ConservationSupported false rather than
// failing the whole review run — the same discipline
// applyIntegrityDimension already follows for the sibling "integrity"
// plane.
func applyConservationDimension(ctx context.Context, in *review.Input, st store.Store, corpusRoot, service string) {
	if in == nil || in.Base == "" || corpusRoot == "" {
		return
	}
	cin, err := conserve.NewInput(ctx, corpusRoot, in.Base, st, service, discoverPlanPaths(corpusRoot))
	if err != nil {
		return
	}
	// CV-65/design §3.4: the conservation dimension carries the SAME
	// reading set `fathom conserve` would report for this range, execution
	// evidence included — never silently stuck on static reach just
	// because this caller, unlike cmdConserve, never loaded a coverage
	// profile of its own before this fix.
	conserve.LoadConfiguredCoverage(&cin, corpusRoot, "")
	in.ConservationSupported = true
	rep := conserve.Run(ctx, cin, conserve.Planes())
	in.ConservationPlanes = conservationPlaneSummaries(rep.Planes)
	in.ConservationResidue = conservationResidueSummary(conserve.Residue(ctx, cin, conserve.DefaultResidueMax))
}

// conservationPlaneSummaries flattens internal/conserve.PlaneReport into
// review.ConservationPlaneSummary 1:1 — review.Input's own doc comment on
// ConservationPlanes names this exact conversion (the review package
// never imports internal/conserve, so both callers, this file and
// internal/mcp/review_tool.go, carry their own copy of it — the same
// intentional duplication CertificationRecord's two builders already
// have).
func conservationPlaneSummaries(planes []conserve.PlaneReport) []review.ConservationPlaneSummary {
	out := make([]review.ConservationPlaneSummary, 0, len(planes))
	for _, p := range planes {
		findings := make([]review.ConservationFinding, 0, len(p.Findings))
		for _, f := range p.Findings {
			findings = append(findings, review.ConservationFinding{
				Plane: f.Plane, Kind: f.Kind, Key: f.Key, Detail: f.Detail,
				Base: f.Base, Head: f.Head, Declared: f.Declared, Remedy: f.Remedy,
			})
		}
		out = append(out, review.ConservationPlaneSummary{
			Plane: p.Plane, Verdict: p.Verdict, Missing: p.Missing, Findings: findings,
		})
	}
	return out
}

// conservationResidueSummary flattens internal/conserve.ResidueReport into
// review.ConservationResidueSummary 1:1 (CV-65, design §3.4) —
// conservationPlaneSummaries' own precedent just above: the review package
// never imports internal/conserve, so both callers (this file and
// internal/mcp/review_tool.go) carry their own copy of this conversion.
func conservationResidueSummary(rep conserve.ResidueReport) review.ConservationResidueSummary {
	items := make([]review.ConservationResidueItem, 0, len(rep.Items))
	for _, it := range rep.Items {
		items = append(items, review.ConservationResidueItem{
			Symbol: it.Symbol, File: it.File, Line: it.Line, Change: it.Change,
			Class: it.Class, Tests: it.Tests, Callers: it.Callers,
			Evidence: it.Evidence, UnexecutedLines: it.UnexecutedLines,
		})
	}
	return review.ConservationResidueSummary{
		Changed: rep.Changed, Attested: rep.Attested, Unattested: rep.Unattested, Unreached: rep.Unreached,
		Partial: rep.Partial, Unexecuted: rep.Unexecuted, UnanalyzedFunctions: rep.UnanalyzedFunctions,
		Items: items, Truncated: rep.Truncated, Unanalyzed: rep.Unanalyzed, Missing: rep.Missing,
		CoverageUnusable: rep.CoverageUnusable,
		CoverageUnmapped: rep.CoverageUnmapped, CoverageUnmappedSample: rep.CoverageUnmappedSample,
	}
}
