// Package risk defines the CUE schema for a repository's OPTIONAL change-risk
// rule table (U7, docs/design/change-and-compliance.md §2, plan CX-3): the
// published, overridable thresholds the change risk receipt's band
// computation reads (`.fathom/risk.cue`). A repository that ships none gets
// the binary's own built-in defaults (internal/review.DefaultRiskTable) —
// the same dual-mode convention as schema/testlayer/schema.cue.
//
// The band itself is never a summed score (ADR-051 decision 4): it is
// always the single strongest cited reason. This table only tunes the
// numeric thresholds two factors compare against — every other factor's
// weight (certificates not live, an unexcused conservation loss, a
// claimed-unproven row, a rule level, an uncertified contract) is a fixed
// mapping the design doc names directly and is not configurable here.
package risk

// #RiskTable is the top-level shape of a risk rule table file. Every field
// is optional: a partially-specified file overrides only the thresholds it
// names, the loader fills the rest from the binary's defaults.
#RiskTable: {
	// blast_radius_high_callers is the transitive-caller count strictly
	// above which the blast-radius factor cites "high" (design doc §2:
	// "high when more than 50 transitive callers or any entrypoint").
	// Reaching any entrypoint is always "high" regardless of this number.
	blast_radius_high_callers?: int & >0

	// zone_interest_medium_rate is U2's zone-interest rate strictly above
	// which the zone-interest factor cites "medium" (design doc §2:
	// "medium above 2.0").
	zone_interest_medium_rate?: number & >0
}
