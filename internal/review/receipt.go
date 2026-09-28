// Package review builds fathom's change-and-compliance evidence family (U7
// the change risk receipt, U9 the migration tracker, U8 the evidence pack —
// docs/design/change-and-compliance.md, plan docs/plans/change-and-
// compliance.md, CX-) — deliberately separate from internal/analysis/review
// (the review_diff dimension pipeline, C1-1): that package composes
// impact/obligations/arch/coverage into ONE cited verdict about a diff's
// CORRECTNESS; this package composes impact, links, certificates,
// conservation, proof history, rules and zone interest into a cited verdict
// about a diff's RISK, plus the migration/trace evidence built on the same
// join (later waves).
//
// Every function here is pure, like internal/analysis/review.Run: every
// dependency (blast radius, links touched, certificates, conservation,
// claimed rows, rules, zone interest) is a caller-resolved field on Input —
// this package does no I/O, calls no clock, and never imports a forge
// client (ADR-040's authority boundary, receiptAuthority). The MCP tool
// (internal/mcp) and the CLI (cmd/fathom) resolve Input and render the
// result; they decide nothing about the band itself.
package review

import (
	"sort"
	"strconv"
)

// FactorWeight is a risk factor's contribution to the band — never a score:
// only ever compared for "strongest", never summed (ADR-051 decision 4).
type FactorWeight int

const (
	WeightNone FactorWeight = iota
	WeightMedium
	WeightHigh
)

// String renders the weight the way a RiskFactor and a receipt's band
// reason speak it.
func (w FactorWeight) String() string {
	switch w {
	case WeightHigh:
		return "high"
	case WeightMedium:
		return "medium"
	default:
		return "none"
	}
}

// Band is the receipt's own overall verdict — "low", "medium" or "high" —
// never a summed score: the strongest cited factor, always with its reason
// (design doc §2).
type Band string

const (
	BandLow    Band = "low"
	BandMedium Band = "medium"
	BandHigh   Band = "high"
)

// Fixed factor names, in the FIXED order the design doc's table lists them
// and BuildRiskReceipt always emits them (determinism contract,
// internal/CLAUDE.md: identical Input always produces an identical,
// identically-ordered Receipt).
const (
	FactorBlastRadius  = "blast_radius"
	FactorLinksTouched = "links_touched"
	FactorCertificates = "certificates_affected"
	FactorConservation = "conservation"
	FactorRowsProven   = "rows_proven"
	FactorRulesTouched = "rules_touched"
	FactorZoneInterest = "zone_interest"
	FactorUncertified  = "uncertified_contracts"
)

// LinkRef is one cross-boundary link touched by the diff — the flattened
// shape a caller joins from the (not yet built) system_impact tool's own
// output, the same "leaf package mirrors instead of importing" discipline
// internal/analysis/review's ObligationRule/CertificationRecord already
// establish (engine.go).
type LinkRef struct {
	Provider string
	Consumer string
	// ConsumerService is the consumer's own service/repository identity;
	// "" when the consumer is in the same service as Provider.
	ConsumerService string
	// Tier is core.EvidenceTier's own vocabulary at the link level: "spec"
	// or "typed" count toward the links-touched factor; anything lower
	// (e.g. "literal", "template", "heuristic") is I57 territory — listed
	// as a hint, NEVER counted (receiptLinkTiers, CX-7).
	Tier string
	// Citation is the evidence citation for this link, when known.
	Citation string
}

const (
	LinkTierSpec  = "spec"
	LinkTierTyped = "typed"
)

// CertificateRef is one certificate whose target or reach intersects the
// change — mirrors internal/analysis/review.CertificationRecord's own
// Subject/State fields (this package does not import internal/certify, the
// same reason that package does not import internal/certify either).
type CertificateRef struct {
	Subject string
	// State is the certifications layer's own state fact: live | flaky |
	// broken | unproven | unreachable. Anything other than "live" is a
	// high-weight reason (design doc §2).
	State string
}

// RowVerdict is one plan row the change claims, and fathom verify's own
// verdict on it — mirrors the same "claimed but unproven" shape
// docs/CLAUDE.md's gate speaks about.
type RowVerdict struct {
	ID string
	// Verdict is "proven" or "unproven". Only claimed (checked) rows
	// belong in Input.ClaimedRows — an unclaimed row is not this diff's
	// business.
	Verdict string
}

const (
	RowProven   = "proven"
	RowUnproven = "unproven"
)

// RuleRef is one governing rule scoped to a changed file — mirrors
// internal/analysis/review.ObligationRule's own File/Rule fields.
type RuleRef struct {
	File string
	Rule string
	// Level is "warn" or "error" (medium/high respectively, design §2).
	Level string
	// Citation is the rule's own evidence citation, when known.
	Citation string
}

const (
	RuleLevelWarn  = "warn"
	RuleLevelError = "error"
)

// Input bundles every already-resolved dependency BuildRiskReceipt needs.
// Every I/O (blast_radius, system_impact, certificate reach, `fathom
// conserve`, plan row verdicts, governing rules, U2's zone-interest rate)
// happens in the caller — this package does none of it (repo convention,
// internal/analysis/review.Input's own doc comment).
type Input struct {
	ChangedFiles []string

	// blast radius (design §2 row 1): TransitiveCallers is the caller
	// count beyond the changed-file seed set (impact.Result, minus the
	// seed); Entrypoints is the sorted set of entrypoint symbol keys the
	// blast radius reaches, empty when none.
	TransitiveCallers int
	Entrypoints       []string

	// links touched (design §2 row 2, I57). Links is every cross-boundary
	// link the diff touches, at every tier; receiptLinkTiers (CX-7) splits
	// it into counted (spec/typed) and hint (everything lower) sets.
	Links []LinkRef

	// certificates affected (design §2 row 3).
	CertificatesAffected []CertificateRef

	// conservation (design §2 row 4): ADR-044's planes, already run by the
	// caller (`fathom conserve`) — Unexcused is true on any unexcused
	// loss, Detail carries the caller's own evidence string.
	ConservationUnexcused bool
	ConservationDetail    string

	// rows proven (design §2 row 5).
	ClaimedRows []RowVerdict

	// rules touched (design §2 row 6).
	RulesTouched []RuleRef

	// zone interest (design §2 row 7): U2's own rate for the changed
	// zones, already computed by the caller.
	ZoneInterestRate float64

	// uncertified contracts (design §2 row 8, T5): the subset of Links at
	// spec/typed tier the caller already found has no contract
	// certificate. Caller-joined for the same reason CertificatesAffected
	// is caller-joined — this package does not know how a "contract
	// certificate" is looked up.
	UncertifiedContracts []LinkRef

	// suggested reviewers (design §2, CX-4): CODEOWNERS matches and
	// frequent-committer matches for the changed and impacted files,
	// already resolved by the caller (internal/ownership) — this package
	// only merges and orders them (suggestReviewers), never looks either
	// up itself.
	CodeownersMatches        []OwnerFiles
	FrequentCommitterMatches []OwnerFiles
}

// RiskFactor is one cited reason within a RiskReceipt (design §2: "every
// reason cited"). Evidence is always sorted (determinism contract).
type RiskFactor struct {
	Name     string
	Weight   FactorWeight
	Reason   string
	Evidence []string
}

// RiskReceipt is the change risk receipt: every factor cited, plus the
// overall Band and the single strongest reason it rests on. LinkHints
// carries the sub-typed-tier links I57 requires be listed but never moved
// the band (CX-7) — separate from Factors so a renderer can show them
// clearly as "not counted".
type RiskReceipt struct {
	ChangedFiles []string
	Factors      []RiskFactor
	LinkHints    []LinkRef
	Band         Band
	// BandReason is the strongest factor's own Reason — the band is never
	// a summed score, always the strongest cited reason (ADR-051 decision
	// 4, riskBand).
	BandReason string
	// SuggestedReviewers is CODEOWNERS ownership plus frequent committers
	// to the impacted providers, alphabetically ordered — never ranked
	// (I58, CX-4).
	SuggestedReviewers []OwnerFiles
	// Authority is the fixed ADR-040 boundary statement every receipt
	// carries (receiptAuthority, CX-8): fathom never posts, approves or
	// blocks on its own authority.
	Authority string
}

// BuildRiskReceipt assembles the change risk receipt's eight factors, in
// the design doc's own fixed order, then bands them via riskBand. Every
// factor is always present (even at WeightNone) so a renderer can show the
// full checklist, not just the reasons that fired.
func BuildRiskReceipt(in Input, table RiskTable) RiskReceipt {
	counted, hints := receiptLinkTiers(in.Links)

	factors := []RiskFactor{
		blastRadiusFactor(in, table),
		linksTouchedFactor(counted),
		certificatesFactor(in.CertificatesAffected),
		conservationFactor(in),
		rowsProvenFactor(in.ClaimedRows),
		rulesTouchedFactor(in.RulesTouched),
		zoneInterestFactor(in.ZoneInterestRate, table),
		uncertifiedContractsFactor(in.UncertifiedContracts),
	}

	band, reason := riskBand(factors, table)

	changed := append([]string(nil), in.ChangedFiles...)
	sort.Strings(changed)

	return RiskReceipt{
		ChangedFiles:       changed,
		Factors:            factors,
		LinkHints:          hints,
		Band:               band,
		BandReason:         reason,
		SuggestedReviewers: suggestReviewers(in.CodeownersMatches, in.FrequentCommitterMatches),
		Authority:          receiptAuthority(),
	}
}

func blastRadiusFactor(in Input, table RiskTable) RiskFactor {
	ep := append([]string(nil), in.Entrypoints...)
	sort.Strings(ep)

	threshold := table.blastRadiusHighCallers()
	switch {
	case len(ep) > 0:
		return RiskFactor{
			Name:     FactorBlastRadius,
			Weight:   WeightHigh,
			Reason:   "blast radius reaches an entrypoint",
			Evidence: ep,
		}
	case in.TransitiveCallers > threshold:
		return RiskFactor{
			Name:   FactorBlastRadius,
			Weight: WeightHigh,
			Reason: "blast radius exceeds the configured caller threshold",
			Evidence: []string{
				strconv.Itoa(in.TransitiveCallers) + " transitive callers (threshold " + strconv.Itoa(threshold) + ")",
			},
		}
	default:
		return RiskFactor{
			Name:   FactorBlastRadius,
			Weight: WeightNone,
			Reason: "blast radius within the configured caller threshold",
			Evidence: []string{
				strconv.Itoa(in.TransitiveCallers) + " transitive callers (threshold " + strconv.Itoa(threshold) + ")",
			},
		}
	}
}

func linksTouchedFactor(counted []LinkRef) RiskFactor {
	var cross []LinkRef
	for _, l := range counted {
		if l.ConsumerService != "" {
			cross = append(cross, l)
		}
	}
	sortLinks(cross)
	sortLinks(counted)

	if len(cross) > 0 {
		return RiskFactor{
			Name:     FactorLinksTouched,
			Weight:   WeightHigh,
			Reason:   "a touched provider has a consumer in another service",
			Evidence: linkEvidence(cross),
		}
	}
	return RiskFactor{
		Name:     FactorLinksTouched,
		Weight:   WeightNone,
		Reason:   "no touched provider has a cross-service consumer",
		Evidence: linkEvidence(counted),
	}
}

func certificatesFactor(certs []CertificateRef) RiskFactor {
	sorted := append([]CertificateRef(nil), certs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Subject < sorted[j].Subject })

	var notLive []string
	var all []string
	for _, c := range sorted {
		all = append(all, c.Subject+" ("+c.State+")")
		if c.State != "live" {
			notLive = append(notLive, c.Subject+" ("+c.State+")")
		}
	}
	if len(notLive) > 0 {
		return RiskFactor{
			Name:     FactorCertificates,
			Weight:   WeightHigh,
			Reason:   "a touched certificate is not live",
			Evidence: notLive,
		}
	}
	return RiskFactor{
		Name:     FactorCertificates,
		Weight:   WeightNone,
		Reason:   "every touched certificate is live",
		Evidence: all,
	}
}

func conservationFactor(in Input) RiskFactor {
	if in.ConservationUnexcused {
		ev := []string{}
		if in.ConservationDetail != "" {
			ev = append(ev, in.ConservationDetail)
		}
		return RiskFactor{
			Name:     FactorConservation,
			Weight:   WeightHigh,
			Reason:   "an unexcused conservation loss",
			Evidence: ev,
		}
	}
	return RiskFactor{
		Name:   FactorConservation,
		Weight: WeightNone,
		Reason: "no unexcused conservation loss",
	}
}

func rowsProvenFactor(rows []RowVerdict) RiskFactor {
	sorted := append([]RowVerdict(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var unproven []string
	for _, r := range sorted {
		if r.Verdict == RowUnproven {
			unproven = append(unproven, r.ID)
		}
	}
	if len(unproven) > 0 {
		return RiskFactor{
			Name:     FactorRowsProven,
			Weight:   WeightHigh,
			Reason:   "a claimed row is unproven",
			Evidence: unproven,
		}
	}
	return RiskFactor{
		Name:   FactorRowsProven,
		Weight: WeightNone,
		Reason: "every claimed row is proven",
	}
}

func rulesTouchedFactor(rules []RuleRef) RiskFactor {
	sorted := append([]RuleRef(nil), rules...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return sorted[i].Rule < sorted[j].Rule
	})

	var errored, warned []string
	for _, r := range sorted {
		ev := r.File + ": " + r.Rule
		switch r.Level {
		case RuleLevelError:
			errored = append(errored, ev)
		case RuleLevelWarn:
			warned = append(warned, ev)
		}
	}
	switch {
	case len(errored) > 0:
		return RiskFactor{
			Name:     FactorRulesTouched,
			Weight:   WeightHigh,
			Reason:   "an error-level rule governs a changed file",
			Evidence: errored,
		}
	case len(warned) > 0:
		return RiskFactor{
			Name:     FactorRulesTouched,
			Weight:   WeightMedium,
			Reason:   "a warn-level rule governs a changed file",
			Evidence: warned,
		}
	default:
		return RiskFactor{
			Name:   FactorRulesTouched,
			Weight: WeightNone,
			Reason: "no governing rule at warn or error touches a changed file",
		}
	}
}

func zoneInterestFactor(rate float64, table RiskTable) RiskFactor {
	threshold := table.zoneInterestMediumRate()
	if rate > threshold {
		return RiskFactor{
			Name:   FactorZoneInterest,
			Weight: WeightMedium,
			Reason: "the changed zones' interest rate exceeds the configured threshold",
			Evidence: []string{
				formatRate(rate) + " (threshold " + formatRate(threshold) + ")",
			},
		}
	}
	return RiskFactor{
		Name:   FactorZoneInterest,
		Weight: WeightNone,
		Reason: "the changed zones' interest rate is within the configured threshold",
		Evidence: []string{
			formatRate(rate) + " (threshold " + formatRate(threshold) + ")",
		},
	}
}

func formatRate(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func uncertifiedContractsFactor(links []LinkRef) RiskFactor {
	sorted := append([]LinkRef(nil), links...)
	sortLinks(sorted)
	if len(sorted) > 0 {
		return RiskFactor{
			Name:     FactorUncertified,
			Weight:   WeightMedium,
			Reason:   "a spec/typed-tier link has no contract certificate",
			Evidence: linkEvidence(sorted),
		}
	}
	return RiskFactor{
		Name:   FactorUncertified,
		Weight: WeightNone,
		Reason: "every spec/typed-tier link has a contract certificate",
	}
}

func sortLinks(links []LinkRef) {
	sort.Slice(links, func(i, j int) bool {
		if links[i].Provider != links[j].Provider {
			return links[i].Provider < links[j].Provider
		}
		return links[i].Consumer < links[j].Consumer
	})
}

func linkEvidence(links []LinkRef) []string {
	if len(links) == 0 {
		return nil
	}
	out := make([]string, 0, len(links))
	for _, l := range links {
		s := l.Provider + " -> " + l.Consumer
		if l.ConsumerService != "" {
			s += " (" + l.ConsumerService + ")"
		}
		out = append(out, s)
	}
	return out
}
