package mcp

// review_receipt.go — CX-5 (docs/plans/change-and-compliance.md, design doc
// §2): review_diff gains an ADDITIVE `receipt` section, the change risk
// receipt (CX-1..CX-4, internal/review.BuildRiskReceipt) built over data
// review_diff already resolves for its own composed verdict — no new store
// capability, no new I/O beyond the risk table's own dual-mode load
// (internal/review.LoadRiskTable, same "missing file -> binary default"
// discipline internal/testlayer.LoadLayers already established).
//
// Every dependency this file maps is something the caller (registerReviewDiff)
// already has in hand by the time it builds the receipt:
//   - blast radius reuses the SAME call graph GL-22's layerDiffByBlastRadius
//     already walks (riskBlastRadius, via blastRadiusByDepth).
//   - links reuse the SAME cross-service hops (serviceRefHopsInto) VB-30
//     already resolves — always heuristic-tier (serviceref.Tier), so I57
//     places every one of them as a LinkHint, never a counted factor (the
//     "not yet built system_impact tool" gap review_tool.go's own doc
//     comment already names is exactly what would supply a spec/typed-tier
//     link; until it exists, this file never claims one).
//   - certificates/conservation reuse the composed review.Input's own
//     CertificationsByFile/ConservationPlanes fields.
//   - rules touched reuses the composed Verdict's own obligations/
//     arch_conformance findings, ACTIVE ones only (a refuted or superseded
//     finding is not a live risk reason — ApplyClaimStatus's own served-
//     only discipline, mirrored here rather than re-derived).
//
// Deliberately NOT wired here (left at BuildRiskReceipt's own "nothing
// here" zero value — the exact state TestRiskReceiptLowBand already proves
// is a legitimate, non-fabricated result, not an unchecked gap this file
// hides): rows proven (needs a plan-verify join review_diff does not
// perform), zone interest (U2's rate has no producer yet anywhere in this
// repo), uncertified contracts and suggested reviewers (both need joins —
// system_impact, internal/ownership — this row's own `needs: "CX-2"` does
// not authorize). Later CX rows, not this one.

import (
	"sort"
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	arreview "github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
	riskreview "github.com/geoffmilleraz/fathom/internal/review"
)

// reviewDiffReceipt assembles the change risk receipt's Input from data the
// caller already resolved (g/nodes: the same graph GL-22's layering walks;
// in/v: the composed review.Input/Verdict; hops: serviceRefHopsInto's
// result; table: the repo's own .fathom/risk.cue, or the binary default)
// and hands it to riskreview.BuildRiskReceipt — this function itself does
// no I/O (the same "pure composition over caller-resolved fields"
// discipline layerDiffByBlastRadius already keeps).
func reviewDiffReceipt(g *callgraph.Graph, nodes []core.SymbolNode, in arreview.Input, v arreview.Verdict, hops []serviceref.Hop, table riskreview.RiskTable) riskreview.RiskReceipt {
	callers, entrypoints := riskBlastRadius(g, nodes, in.ChangedFiles)

	input := riskreview.Input{
		ChangedFiles:          in.ChangedFiles,
		TransitiveCallers:     callers,
		Entrypoints:           entrypoints,
		Links:                 riskLinkHintsFromHops(hops),
		CertificatesAffected:  riskCertificatesAffected(in.CertificationsByFile, in.ChangedFiles),
		ConservationUnexcused: riskConservationUnexcused(in.ConservationPlanes),
		ConservationDetail:    riskConservationDetail(in.ConservationPlanes),
		RulesTouched:          riskRulesTouched(v.Dimensions),
	}
	return riskreview.BuildRiskReceipt(input, table)
}

// riskBlastRadius walks the SAME reverse-graph buckets blastRadiusByDepth
// already computes for `by_depth`/GL-22's layering: transitiveCallers is
// the total symbol count at every depth beyond the changed files
// themselves (depth 0), entrypoints the sorted subset of those reached
// keys that symbolnav.Entrypoints also names — design doc §2 row 1's own
// two inputs ("callers counted by depth", "reaches an entrypoint").
func riskBlastRadius(g *callgraph.Graph, nodes []core.SymbolNode, changedFiles []string) (transitiveCallers int, entrypoints []string) {
	buckets := blastRadiusByDepth(g, nodes, changedFiles)

	epSet := make(map[string]bool, len(nodes))
	for _, n := range symbolnav.Entrypoints(nodes) {
		epSet[n.Key()] = true
	}

	var eps []string
	for _, b := range buckets {
		if b.Depth == 0 {
			continue // the changed symbols themselves, not a caller
		}
		transitiveCallers += b.Count
		for _, s := range b.Symbols {
			if epSet[s.Key] {
				eps = append(eps, s.Key)
			}
		}
	}
	sort.Strings(eps)
	return transitiveCallers, eps
}

// riskLinkHintsFromHops maps VB-30's cross-service hops into LinkRefs —
// always Tier serviceref.Tier ("heuristic": that package's own doc comment
// says every hop it produces is, never typed), so receiptLinkTiers (CX-7)
// always places every one of these as a LinkHint, never a counted factor
// input. Provider/Consumer fall back to the route template / FromService
// when the handler/enclosing-symbol name is unknown (the same
// best-effort-naming discipline diffLayerFile's own Consumers field
// already keeps).
func riskLinkHintsFromHops(hops []serviceref.Hop) []riskreview.LinkRef {
	if len(hops) == 0 {
		return nil
	}
	out := make([]riskreview.LinkRef, 0, len(hops))
	for _, h := range hops {
		provider := h.ToHandler
		if provider == "" {
			provider = h.Method + " " + h.Template
		}
		consumer := h.FromSymbol
		if consumer == "" {
			consumer = h.FromService
		}
		cite := h.ToFile
		if h.ToLine > 0 {
			cite = h.ToFile + "#L" + strconv.Itoa(h.ToLine)
		}
		out = append(out, riskreview.LinkRef{
			Provider:        provider,
			Consumer:        consumer,
			ConsumerService: h.FromService,
			Tier:            serviceref.Tier,
			Citation:        cite,
		})
	}
	return out
}

// riskCertificatesAffected flattens the composed Input's own
// CertificationsByFile (already scoped to the changed files by
// buildReviewInput) into one deduped-by-Subject CertificateRef list —
// certificatesFactor sorts its own input, so no ordering is imposed here.
func riskCertificatesAffected(byFile map[string][]arreview.CertificationRecord, changedFiles []string) []riskreview.CertificateRef {
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

// riskConservationUnexcused reports whether any of the composed Input's
// own ConservationPlanes (ADR-044's planes, already run by the caller when
// --since/since resolved) verdicted "losses" — internal/conserve.Verdict's
// own three-value vocabulary (ConservationPlaneSummary's own doc comment).
// false — "no unexcused loss" — whenever conservation was never run at
// all (ConservationSupported false, so ConservationPlanes is empty): the
// SAME zero-value state CX-1's own TestRiskReceiptLowBand already treats
// as a legitimate "nothing to report", not a fabricated clean bill.
func riskConservationUnexcused(planes []arreview.ConservationPlaneSummary) bool {
	for _, p := range planes {
		if p.Verdict == "losses" {
			return true
		}
	}
	return false
}

// riskConservationDetail names the first "losses"-verdicted plane's own
// evidence — the first finding's Detail when one exists, else a bare
// "<plane>: conservation loss" fallback so the factor's Evidence is never
// silently empty when Unexcused is true.
func riskConservationDetail(planes []arreview.ConservationPlaneSummary) string {
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

// riskRulesTouched maps the composed Verdict's own obligations/
// arch_conformance findings (design doc §2 row 6's exact two sources: a
// governing rule, or an architecture rule) into RuleRefs — ACTIVE findings
// only (a refuted or superseded finding already lost its "governs this
// diff" standing; ApplyClaimStatus's own served-only discipline is
// mirrored here, not re-derived). Severity "info" (a fresh, unremarkable
// obligation) governs but is not itself a risk reason (design doc §2 row
// 6: "medium on warn, high on error") — such a finding is simply not
// emitted as a RuleRef, exactly like rulesTouchedFactor already treats an
// absent rule.
func riskRulesTouched(dims []arreview.Dimension) []riskreview.RuleRef {
	var out []riskreview.RuleRef
	for _, d := range dims {
		if d.Name != arreview.DimObligations && d.Name != arreview.DimArch {
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

// riskReceiptPayload renders a RiskReceipt as the snake_case JSON shape
// review_diff's payload already speaks everywhere else — internal/review
// stays a leaf package with no JSON tags of its own (RuleRef/LinkRef's own
// doc comments: "this package does no I/O"), so the caller renders, the
// same discipline conservationPlaneSummaries/withServiceRefHops already
// keep for their own domains.
func riskReceiptPayload(r riskreview.RiskReceipt) map[string]any {
	factors := make([]map[string]any, 0, len(r.Factors))
	for _, f := range r.Factors {
		factors = append(factors, map[string]any{
			"name":     f.Name,
			"weight":   f.Weight.String(),
			"reason":   f.Reason,
			"evidence": f.Evidence,
		})
	}
	hints := make([]*reviewLinkLocation, 0, len(r.LinkHints))
	for _, l := range r.LinkHints {
		hints = append(hints, linkRefPayload(l))
	}
	reviewers := make([]map[string]any, 0, len(r.SuggestedReviewers))
	for _, o := range r.SuggestedReviewers {
		reviewers = append(reviewers, map[string]any{"owner": o.Owner, "files": o.Files, "source": o.Source})
	}
	return map[string]any{
		"changed_files":       r.ChangedFiles,
		"band":                string(r.Band),
		"band_reason":         r.BandReason,
		"factors":             factors,
		"link_hints":          hints,
		"suggested_reviewers": reviewers,
		"authority":           r.Authority,
	}
}

// reviewLinkLocation is an index-sourced provider location in a risk hint.
type reviewLinkLocation struct {
	CitationStatus  *core.CitationStatus `json:"citation_status,omitempty"`
	Citation        string               `json:"citation,omitempty"`
	Consumer        string               `json:"consumer"`
	ConsumerService string               `json:"consumer_service,omitempty"`
	Provider        string               `json:"provider"`
	Tier            string               `json:"tier"`
}

func linkRefPayload(l riskreview.LinkRef) *reviewLinkLocation {
	return &reviewLinkLocation{Citation: l.Citation, Consumer: l.Consumer, ConsumerService: l.ConsumerService, Provider: l.Provider, Tier: l.Tier}
}
