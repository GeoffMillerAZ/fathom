package console

// explain.go — the serving explainer's projection (vision 35 §4.3, design
// §3 "Serving explainer", plan OC-32/OC-34): semantic.ExplainObligations
// (every candidate by scope tier, the ranking, the cap, the served list and
// each displaced claim's reason) plus what the obligations tool adds around
// the compiled rules before an agent reads them — the raw-fallback rows for
// an uncompiled governing guide (ADR-037) and the layer's coverage note.
// `fathom obligations --explain --json` prints exactly this; /explain
// renders it. A pure fold: the caller runs the explanation and the two
// enrichments against one store.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ServingInputs is what ServingView folds.
type ServingInputs struct {
	Explanation semantic.ObligationsExplanation
	// Raw are the raw-fallback rows the tool prepends, RawWhy the coverage
	// defect they answer; RawOK is false when there was nothing to add.
	Raw    []semantic.Obligation
	RawWhy string
	RawOK  bool
	// Coverage is the obligations layer's live coverage; CoverageFound is
	// false when no compile run was ever recorded; CoverageErr names a read
	// failure.
	Coverage      semantic.LayerCoverage
	CoverageFound bool
	CoverageErr   error
	// RerankProvider is the decision-port provider serving re-ranks within
	// tiers with ([decide] rank_within_tier), "" when that is off.
	RerankProvider string
}

// ServingExplanation is the serving explainer.
type ServingExplanation struct {
	semantic.ObligationsExplanation
	// AgentSees is the list the obligations tool injects, in order: the
	// raw-fallback rows first, then the served compiled rules.
	AgentSees   []semantic.Obligation `json:"agent_sees"`
	RawFallback []semantic.Obligation `json:"raw_fallback"`
	RawWhy      string                `json:"raw_why,omitempty"`
	// Coverage is the obligations layer's coverage of its corpus;
	// CoverageNote names the uncompiled guide governing this file, with
	// the resume command.
	Coverage     Value[semantic.LayerCoverage] `json:"coverage"`
	CoverageNote string                        `json:"coverage_note,omitempty"`
	// Rerank says whether serving re-orders rules within a tier with a
	// decision-port provider; the explanation never calls one.
	Rerank string `json:"rerank"`
}

// ServingView folds the explanation and its enrichments.
func ServingView(in ServingInputs) ServingExplanation {
	out := ServingExplanation{ObligationsExplanation: in.Explanation, RawFallback: []semantic.Obligation{}}
	if in.RawOK {
		out.RawFallback = append(out.RawFallback, in.Raw...)
		out.RawWhy = in.RawWhy
	}
	out.AgentSees = append(append([]semantic.Obligation{}, out.RawFallback...), in.Explanation.Served.Rules...)
	const covSource = "the obligations layer's compile runs against its current corpus"
	switch {
	case in.CoverageErr != nil:
		out.Coverage = Errored[semantic.LayerCoverage]("coverage unreadable: " + in.CoverageErr.Error()).WithSource(covSource)
	case !in.CoverageFound:
		out.Coverage = Absent[semantic.LayerCoverage]("no obligations compile run recorded for " + in.Explanation.Service + " — coverage is not measured").WithSource(covSource)
	default:
		cov := in.Coverage
		out.Coverage = Present(cov, covSource).WithDenominator(fmt.Sprintf("%d of %d guide sections compiled", cov.SectionsCompiled, cov.SectionsTotal))
		if guide, ok := semantic.NearestUncompiledGuide(cov, in.Explanation.File); ok {
			coverage := fmt.Sprintf("%d/%d sections compiled", cov.SectionsCompiled, cov.SectionsTotal)
			if cov.BudgetStopped {
				coverage = fmt.Sprintf("budget stopped at %d/%d sections", cov.SectionsCompiled, cov.SectionsTotal)
			}
			out.CoverageNote = fmt.Sprintf("%s governs this path and was not compiled: %s; run `fathom compile --layer obligations --service %s --continue`", guide, coverage, in.Explanation.Service)
		}
	}
	out.Rerank = "off — serving orders rules deterministically, as shown"
	if in.RerankProvider != "" {
		out.Rerank = "on — serving re-orders rules within a scope tier with the " + in.RerankProvider + " decision port; this explanation shows the deterministic order and never calls a provider"
	}
	return out
}
