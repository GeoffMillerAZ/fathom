// Package diffreporter — valid test fixtures that must unify with schema.cue.
// Run `cue vet ./...` from the package root or `cue vet ./testdata/` to
// validate these against the schema.
package diffreporter

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/diff-reporter:diffreporter"

// validScoreDeltaImprovement is a well-formed score delta showing improvement.
validScoreDeltaImprovement: diffreporter.#ScoreDelta & {
	scenario_id: "scenario-001"
	concern:     "run-base"
	base_score:  0.5
	cand_score:  0.8
	delta:       0.3
}

// validScoreDeltaRegression is a well-formed score delta showing regression.
validScoreDeltaRegression: diffreporter.#ScoreDelta & {
	scenario_id: "scenario-002"
	concern:     "run-base"
	base_score:  0.9
	cand_score:  0.6
	delta:       -0.3
}

// validScoreDeltaNewScenario is a delta for a scenario only in cand.
validScoreDeltaNewScenario: diffreporter.#ScoreDelta & {
	scenario_id: "scenario-new"
	concern:     "run-cand"
	cand_score:  0.7
}

// validVerdictFlip is a well-formed verdict flip (always a regression).
validVerdictFlip: diffreporter.#VerdictFlip & {
	scenario_id:  "scenario-003"
	concern:      "run-concern"
	base_verdict: "pass"
	cand_verdict: "fail"
}

// validDeltaEmpty is a well-formed empty Delta.
validDeltaEmpty: diffreporter.#Delta & {
	score_deltas:  []
	verdict_flips: []
	regressions:   0
	improvements:  0
}

// validDeltaWithFlip is a Delta with one regression.
validDeltaWithFlip: diffreporter.#Delta & {
	score_deltas: [{
		scenario_id: "s1"
		concern:     "concern-a"
		base_score:  0.9
		cand_score:  0.4
		delta:       -0.5
	}]
	verdict_flips: [{
		scenario_id:  "s1"
		concern:      "concern-a"
		base_verdict: "pass"
		cand_verdict: "fail"
	}]
	regressions:  1
	improvements: 0
}

// validToolCallRecord is a well-formed tool call record.
validToolCallRecord: diffreporter.#ToolCallRecord & {
	name:      "search_files"
	args_hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}

// validTrajDeltaUnchanged is a TrajDelta with no differences.
validTrajDeltaUnchanged: diffreporter.#TrajDelta & {
	base_calls: [{name: "read_file", args_hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]
	cand_calls: [{name: "read_file", args_hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]
	base_stop:     "end_turn"
	cand_stop:     "end_turn"
	base_turns:    2
	cand_turns:    2
	calls_changed: false
	stop_changed:  false
	turns_changed: false
}
