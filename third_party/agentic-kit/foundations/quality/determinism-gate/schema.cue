// Package determinismgate defines the CUE-first contract for the
// determinism-gate foundation. Go types in types.go mirror these definitions;
// any drift is a bug. These constraints are evaluated with `cue vet ./...`.
package determinismgate

// #Stability reports the measured stability verdict from a Probe run.
#Stability: "stable" | "unstable"

// #ProbeResult is the output of Probe — the measured variance across K judge
// runs against the same scenario.
#ProbeResult: {
	// scenario_id identifies the scenario that was probed.
	scenario_id: string & !=""

	// k is the number of runs performed.
	k: int & >=1

	// agreement_rate is the fraction of run pairs that agreed on the verdict
	// (pairwise agreement over all K*(K-1)/2 pairs), in [0,1].
	agreement_rate: number & >=0 & <=1

	// verdict_flip_count is the number of run pairs where the verdict differed.
	verdict_flip_count: int & >=0

	// per_field_entropy is a map from field name to measured Shannon entropy
	// (in bits) across the K runs. Higher = more variable.
	per_field_entropy: {[string]: number & >=0}

	// stability is the overall verdict: "stable" when agreement_rate >= the
	// caller-configured stability_band, "unstable" otherwise.
	stability: #Stability

	// stability_band is the agreement_rate threshold used for this probe,
	// in [0, 1]. Callers consume the measured band, never a hard-coded constant.
	stability_band: number & >=0 & <=1
}
