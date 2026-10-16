// Package diffreporter provides one primitive with two roles:
//
//  1. Diff(base, cand []evaljournal.Record) Delta — compares two eval-journal
//     record sets and returns per-scenario/per-concern score deltas, verdict
//     flips, improvement and regression counts.
//
//  2. TrajectoryDiff(base, cand []llmclient.Message) TrajDelta — compares two
//     ordered tool-call sequences (name + args-hash), stop causes, and turn
//     counts.
//
// # Diff semantics
//
// Records are keyed by (ScenarioID, Concern). "Concern" is derived from the
// run_id field, which callers should use as a concern/aspect label within a
// run. When multiple records share the same (ScenarioID, run_id) pair, Diff
// uses the LAST record in each set — callers should emit one decisive record
// per (scenario, concern) pair per run.
//
// A verdict flip is ALWAYS a regression. The score-drop significance threshold
// is caller config (DiffOptions.ScoreDropThreshold with explicit units in
// [0,1]). Until conviction-metrics statistics are landed, the threshold is a
// simple numeric delta.
//
// Delta output is sorted by (ScenarioID, Concern) for byte-stable, diffable
// marshal.
//
// # TrajectoryDiff semantics
//
// Messages are scanned for BlockToolUse blocks. For each tool call, the name
// and a SHA-256 hex hash of the canonical JSON-marshalled Input are recorded.
// The final StopReason from the last terminal message is extracted for the
// stop-cause comparison. Turn count = number of Messages.
//
// TrajDelta output is byte-stable: tool calls are in message order; the args
// hash is SHA-256 of the canonical (sorted-key) JSON.
//
// # Concurrency contract
//
// Diff and TrajectoryDiff are pure functions; call concurrently freely.
// Delta and TrajDelta are immutable value types; share freely.
package diffreporter

import llmclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"

// ScoreDelta holds the numeric score change for one (scenario, concern) pair.
//
// CUE mirror: #ScoreDelta in schema.cue.
type ScoreDelta struct {
	// ScenarioID identifies the scenario.
	ScenarioID string `json:"scenario_id"`

	// Concern identifies the eval concern/aspect (from run_id).
	Concern string `json:"concern"`

	// BaseScore is nil when the pair was absent in the base set.
	BaseScore *float64 `json:"base_score,omitempty"`

	// CandScore is nil when the pair is absent in the candidate set.
	CandScore *float64 `json:"cand_score,omitempty"`

	// Delta = CandScore - BaseScore. Nil when either score is nil.
	Delta *float64 `json:"delta,omitempty"`
}

// VerdictFlip records one scenario where the verdict changed between base and
// candidate. A verdict flip is always a regression.
//
// CUE mirror: #VerdictFlip in schema.cue.
type VerdictFlip struct {
	// ScenarioID identifies the scenario.
	ScenarioID string `json:"scenario_id"`

	// Concern identifies the eval concern/aspect (from run_id).
	Concern string `json:"concern"`

	// BaseVerdict is the verdict in the base record set.
	BaseVerdict string `json:"base_verdict"`

	// CandVerdict is the verdict in the candidate record set.
	CandVerdict string `json:"cand_verdict"`
}

// DiffOptions configures Diff behaviour. The zero value is valid.
type DiffOptions struct {
	// ScoreDropThreshold, in [0,1], is the minimum score decrease to count as
	// a regression by score. Zero means any drop is reported in ScoreDeltas but
	// is NOT counted in Regressions (only verdict flips count there). Set to a
	// positive value to also flag score drops as regressions.
	//
	// Unit: fraction of score (e.g. 0.05 = 5 percentage-points).
	//
	// Note: verdict flips are ALWAYS regressions regardless of this threshold.
	ScoreDropThreshold float64
}

// Delta is the output of Diff. It is a pure value type; marshal it with
// encoding/json for byte-stable, golden-diffable output.
//
// CUE mirror: #Delta in schema.cue.
type Delta struct {
	// ScoreDeltas holds per-(scenario,concern) score changes, sorted by
	// (ScenarioID, Concern) for byte-stable output.
	ScoreDeltas []ScoreDelta `json:"score_deltas"`

	// VerdictFlips holds every (scenario,concern) pair where the verdict
	// changed. Sorted by (ScenarioID, Concern) for byte-stable output.
	VerdictFlips []VerdictFlip `json:"verdict_flips"`

	// Regressions is the count of verdict flips (always regressions) plus any
	// score drops >= DiffOptions.ScoreDropThreshold (when that threshold > 0).
	Regressions int `json:"regressions"`

	// Improvements is the count of (scenario,concern) pairs where the candidate
	// score exceeds the base score by at least ScoreDropThreshold (when > 0),
	// or any positive delta when the threshold is zero.
	Improvements int `json:"improvements"`
}

// ToolCallRecord captures one tool call in a trajectory for diff purposes.
//
// CUE mirror: #ToolCallRecord in schema.cue.
type ToolCallRecord struct {
	// Name is the tool name.
	Name string `json:"name"`

	// ArgsHash is a SHA-256 hex hash of the canonical JSON-marshalled tool input.
	// Byte-stable: keys are sorted by encoding/json's struct-field ordering, and
	// json.RawMessage inputs are re-marshalled via json.Compact.
	ArgsHash string `json:"args_hash"`
}

// TrajDelta is the output of TrajectoryDiff.
//
// CUE mirror: #TrajDelta in schema.cue.
type TrajDelta struct {
	// BaseCalls is the ordered tool-call sequence from the base trajectory.
	BaseCalls []ToolCallRecord `json:"base_calls"`

	// CandCalls is the ordered tool-call sequence from the candidate trajectory.
	CandCalls []ToolCallRecord `json:"cand_calls"`

	// BaseStop is the final stop reason from the base trajectory ("" if none).
	BaseStop string `json:"base_stop"`

	// CandStop is the final stop reason from the candidate trajectory.
	CandStop string `json:"cand_stop"`

	// BaseTurns is the turn count in the base trajectory.
	BaseTurns int `json:"base_turns"`

	// CandTurns is the turn count in the candidate trajectory.
	CandTurns int `json:"cand_turns"`

	// CallsChanged is true when BaseCalls != CandCalls (by value).
	CallsChanged bool `json:"calls_changed"`

	// StopChanged is true when BaseStop != CandStop.
	StopChanged bool `json:"stop_changed"`

	// TurnsChanged is true when BaseTurns != CandTurns.
	TurnsChanged bool `json:"turns_changed"`
}

// Ensure the llmclient import is used (Message is referenced in TrajectoryDiff
// in the impl file).
var _ = llmclient.RoleUser
