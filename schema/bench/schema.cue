// Package bench validates the file-only benchmark inputs from
// docs/design/bench-prep.md sections 5 and 10. It checks declared criteria;
// vetting receipts, power, sign-off and the no-read policy need runtime proof.
package bench

import (
	"list"
	"struct"
	"time"
)

#Text:   string & =~"\\S"
#SHA256: string & =~"^[0-9a-f]{64}$"
#Date:   string & time.Format("2006-01-02")

// #Subject preserves corpus.d's repo-entry names and adds bench metadata.
// subpath remains optional, including the empty string for the repo root.
// C1's licence allowlist is checked by subject vetting at the pinned commit;
// dataset licences are recorded separately and must not inherit that list.
#Subject: {
	name!:    #Text
	url!:     #Text
	ref!:     #Text
	sha!:     string & =~"^[0-9a-f]{40}$"
	licence!: #Text
	subpath?: string
	idioms?: [...string]
	sparse?: [...string]
	notes?: string
	only?: [...string]
	bench!: {
		toolchain!: {
			// Map keys are tool names, not a fixed list of languages.
			versions!: struct.MinFields(1) & {
				[string]: string & =~"^v?[0-9]+\\.[0-9]+\\.[0-9]+((a|b|rc)[0-9]+|-[0-9A-Za-z.-]+)?(\\+[0-9A-Za-z.-]+)?$"
			}
			lockfile_sha256!: #SHA256
		}
		// Ordered shell commands; setup may be empty for an already warm tree.
		setup!: [...#Text]
		test!: [#Text, ...#Text]
		suite_seconds!: number & >0
		flake_runs!:    int & >=3
		suite_tests?:   int & >0
		// Leaf tests only, counted from a machine-readable suite run (D28).
		excluded?: [...{test!: #Text, reason!: #Text, log!: #Text}]
		if excluded != _|_ {
			if len(excluded) > 0 {
				suite_tests!:    int & >0
				_excluded_limit: true & (len(excluded) <= suite_tests*2/100)
			}
		}
		l0_reading!: #Text
		// The controller assigns held-out at vetting (BX-44), after Q2.
		split?: "dev" | "held-out"
		task_sources!: [#TaskSource, ...#TaskSource]
		contamination!: {
			// Age in days at vetting; the band records popularity separately.
			age!: number & >=0
			// GitHub stars at vetting: low < 1,000; medium 1,000–10,000;
			// high > 10,000 (D23). The observed count belongs in the reading.
			popularity_band!: "low" | "medium" | "high"
			// Committed fathom files that already read this subject (D25).
			prior_reads?: [...#Text]
			// Q2's pinned model sets this cutoff; omit it until then.
			fresh_after?: #Date
		}
		if contamination.prior_reads != _|_ {
			if len(contamination.prior_reads) > 0 {
				split: "dev"
			}
		}
	}
}

#TaskSource: {
	dataset!:  #Text
	revision!: #Text
	licence!:  #Text
}

// Directions describe the primary metric, so a cost/error reduction can be
// predicted without treating every hypothesis as an increase. A predicted
// null must declare its positive, symmetric TOST margin in that metric's units.
#Prediction: {
	stratum!:            #Text
	arm!:                #Text
	direction!:          "increase" | "decrease" | "null"
	equivalence_margin?: number & >0
	if direction == "null" {
		equivalence_margin!: number & >0
	}
}

#Hypothesis: {
	id!:        #Text
	statement!: #Text
	predictions!: [#Prediction, ...#Prediction]
	primary_metric!:             #Text
	minimum_effect_of_interest!: number & >0
	n!:                          int & >0
	test!:                       #Text
}

// #Prereg is the machine-evaluable half of a signed preregistration. The
// primary metric, MEI, fixed n and statistical test are per hypothesis;
// Holm's family names their ids. No statistical result is authored here.
#Prereg: {
	let Strata = strata
	let Arms = arms
	let Baseline = baseline_arm
	let PrimaryIDs = [for h in hypotheses {h.id}]
	id!: #Text
	strata!: [#Text, ...#Text]
	arms!: [#Text, ...#Text]
	baseline_arm!: #Text & or(Arms)
	hypotheses!: [#Hypothesis, ...#Hypothesis] & [...{
		predictions: [...{
			stratum: or(Strata)
			arm:     or(Arms)
		}]
		// Every declared stratum needs a prediction in each hypothesis.
		for s in Strata {
			predictions: list.MatchN(>=1, {stratum: s, ...})
		}

		// Baseline-relative predictions cover every comparison arm.
		for a in Arms if a != Baseline {
			predictions: list.MatchN(>=1, {arm: a, ...})
		}
	}]
	alpha!: 0.05
	multiplicity_correction!: {
		method!: "holm"
		family!: [#Text & or(PrimaryIDs), ...#Text & or(PrimaryIDs)] & list.UniqueItems
		for id in PrimaryIDs {
			family: list.MatchN(1, id)
		}
	}
	// On benefit-oriented, Holm-adjusted intervals. Refutation wins the
	// overlap: an interval entirely within (0, MEI) is refuted, not confirmed.
	three_outcome_rule!: {
		confirmed!:    "lower_bound_above_zero"
		refuted!:      "upper_bound_below_mei"
		inconclusive!: "otherwise"
		tie!:          "refuted"
		// Predicted nulls use TOST at alpha 0.05, not the win rule.
		predicted_null!: {
			equivalent!:   "90_percent_interval_inside_symmetric_margin"
			harm!:         "95_percent_upper_bound_below_zero_counts_as_loss"
			inconclusive!: "otherwise"
			above_delta!:  "unexpected_effect_not_win"
		}
	}
	turn_cap!:     int & >0
	cost_cap_usd!: number & >=0
	seed!:         int
	// Both are required even when empty: absence is not an empty log.
	exclusions!: [...#Text]
	deviation_log!: [...{
		date!:        #Date
		description!: #Text
		reason!:      #Text
	}]
}
