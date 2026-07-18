// Package diffreporter defines the CUE-first contract for the diff-reporter
// foundation. Go types in types.go mirror these definitions; any drift is a
// bug. These constraints are evaluated with `cue vet ./...`.
package diffreporter

// #ScoreDelta holds the numeric score change for one (scenario, concern) pair.
#ScoreDelta: {
	scenario_id: string & !=""
	concern:     string & !=""

	// base_score is the baseline score; null when the pair was absent in base.
	base_score?: number & >=0 & <=1
	// cand_score is the candidate score; null when the pair is absent in cand.
	cand_score?: number & >=0 & <=1

	// delta = cand_score - base_score; positive = improvement.
	// May be absent when either score is absent.
	delta?: number & >=-1 & <=1
}

// #VerdictFlip records one scenario where the verdict changed between base and
// candidate. A verdict flip is ALWAYS a regression (canonical rule).
#VerdictFlip: {
	scenario_id: string & !=""
	concern:     string & !=""
	base_verdict: "pass" | "fail" | "absent_proven" | "abstain" | "error"
	cand_verdict: "pass" | "fail" | "absent_proven" | "abstain" | "error"
}

// #Delta is the result of Diff: a structured comparison of two eval-journal
// record sets.
#Delta: {
	// score_deltas holds per-(scenario,concern) score changes, sorted by
	// scenario_id then concern for byte-stable output.
	score_deltas: [...#ScoreDelta]

	// verdict_flips holds every scenario where the verdict changed. Sorted by
	// scenario_id then concern for byte-stable output.
	verdict_flips: [...#VerdictFlip]

	// regressions is the count of verdict flips. Every flip is a regression.
	regressions: int & >=0

	// improvements is the count of (scenario,concern) pairs where cand_score >
	// base_score by at least the caller-configured threshold.
	improvements: int & >=0
}

// #ToolCallRecord captures one tool call in a trajectory for diff purposes.
#ToolCallRecord: {
	// name is the tool name.
	name: string & !=""
	// args_hash is a byte-stable hash of the tool arguments (SHA-256 hex).
	args_hash: string & !=""
}

// #TrajDelta is the result of TrajectoryDiff.
#TrajDelta: {
	// base_calls is the ordered tool-call sequence in the base trajectory.
	base_calls: [...#ToolCallRecord]
	// cand_calls is the ordered tool-call sequence in the candidate trajectory.
	cand_calls: [...#ToolCallRecord]

	// base_stop is the final stop reason in the base trajectory.
	base_stop: string
	// cand_stop is the final stop reason in the candidate trajectory.
	cand_stop: string

	// base_turns is the turn count in the base trajectory.
	base_turns: int & >=0
	// cand_turns is the turn count in the candidate trajectory.
	cand_turns: int & >=0

	// calls_changed is true when base_calls != cand_calls.
	calls_changed: bool
	// stop_changed is true when base_stop != cand_stop.
	stop_changed: bool
	// turns_changed is true when base_turns != cand_turns.
	turns_changed: bool
}
