package risk

// A well-formed risk table: both thresholds overridden.
withEverything: #RiskTable & {
	blast_radius_high_callers: 75
	zone_interest_medium_rate: 1.5
}

// A single-field override — the loader fills the other threshold from the
// binary's default.
partial: #RiskTable & {
	zone_interest_medium_rate: 3.0
}

// The legitimate "no overrides" empty form — a repository that ships a
// risk.cue purely to document intent to override later.
empty: #RiskTable & {}
