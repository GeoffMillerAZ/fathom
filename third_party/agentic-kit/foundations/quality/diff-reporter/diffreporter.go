package diffreporter

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	evaljournal "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-journal"
)

// evalKey uniquely identifies one (scenario, concern) pair within a record set.
// We use run_id as the concern label, matching the spec: callers use run_id as
// a concern/aspect label within a run.
type evalKey struct {
	scenarioID string
	concern    string // = run_id
}

// buildIndex builds an index from (scenarioID, concern) → last Record in recs.
// When multiple records share the same key, the last one wins (per Diff docs).
func buildIndex(recs []evaljournal.Record) map[evalKey]evaljournal.Record {
	idx := make(map[evalKey]evaljournal.Record, len(recs))
	for _, r := range recs {
		k := evalKey{scenarioID: r.ScenarioID, concern: r.RunID}
		idx[k] = r // last wins
	}
	return idx
}

// Diff compares base and cand eval-journal record sets and returns a Delta
// describing per-scenario/per-concern score changes and verdict flips.
//
// Records are keyed by (ScenarioID, RunID) — callers should use RunID as a
// concern/aspect label. When multiple records share the same key, the last one
// in each slice wins.
//
// Verdict flip semantics: any change in Verdict between base and cand is a
// regression. Score-drop significance is controlled by opts.ScoreDropThreshold
// (see DiffOptions doc). Output is sorted by (ScenarioID, Concern) for
// byte-stable, golden-diffable marshalling.
func Diff(base, cand []evaljournal.Record, opts DiffOptions) Delta {
	baseIdx := buildIndex(base)
	candIdx := buildIndex(cand)

	// Union of all keys.
	keys := make(map[evalKey]struct{})
	for k := range baseIdx {
		keys[k] = struct{}{}
	}
	for k := range candIdx {
		keys[k] = struct{}{}
	}

	// Sort keys for deterministic output.
	sortedKeys := make([]evalKey, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if sortedKeys[i].scenarioID != sortedKeys[j].scenarioID {
			return sortedKeys[i].scenarioID < sortedKeys[j].scenarioID
		}
		return sortedKeys[i].concern < sortedKeys[j].concern
	})

	var scoreDeltas []ScoreDelta
	var verdictFlips []VerdictFlip
	regressions := 0
	improvements := 0

	for _, k := range sortedKeys {
		baseRec, inBase := baseIdx[k]
		candRec, inCand := candIdx[k]

		sd := ScoreDelta{
			ScenarioID: k.scenarioID,
			Concern:    k.concern,
		}

		if inBase && baseRec.Score != nil {
			v := *baseRec.Score
			sd.BaseScore = &v
		}
		if inCand && candRec.Score != nil {
			v := *candRec.Score
			sd.CandScore = &v
		}
		if sd.BaseScore != nil && sd.CandScore != nil {
			d := *sd.CandScore - *sd.BaseScore
			sd.Delta = &d
			if opts.ScoreDropThreshold > 0 {
				if d <= -opts.ScoreDropThreshold {
					regressions++
				} else if d >= opts.ScoreDropThreshold {
					improvements++
				}
			} else if d > 0 {
				improvements++
			}
		}
		scoreDeltas = append(scoreDeltas, sd)

		// Verdict flips.
		if inBase && inCand && baseRec.Verdict != candRec.Verdict {
			verdictFlips = append(verdictFlips, VerdictFlip{
				ScenarioID:  k.scenarioID,
				Concern:     k.concern,
				BaseVerdict: string(baseRec.Verdict),
				CandVerdict: string(candRec.Verdict),
			})
			regressions++ // every flip is a regression
		}
	}

	// Ensure non-nil slices for stable JSON output.
	if scoreDeltas == nil {
		scoreDeltas = []ScoreDelta{}
	}
	if verdictFlips == nil {
		verdictFlips = []VerdictFlip{}
	}

	return Delta{
		ScoreDeltas:  scoreDeltas,
		VerdictFlips: verdictFlips,
		Regressions:  regressions,
		Improvements: improvements,
	}
}

// TrajectoryDiff compares two ordered message sequences and returns a TrajDelta
// describing differences in tool-call sequences, stop causes, and turn counts.
//
// Tool calls are extracted in message order; for each BlockToolUse block the
// Name and a SHA-256 hex hash of the canonical Input JSON are recorded. The
// final StopReason is derived from the last StreamChunk-style terminal message
// (last assistant message's content is not a stop reason — we use Response
// metadata when callers embed it, but here we infer it from the last message's
// content type). Because llmclient.Message does not carry StopReason directly,
// TrajectoryDiff reports the stop cause as StopEndTurn when no tool_use block
// appears in the last assistant turn, or StopToolUse when one does — mirroring
// MockClient's normalisation. Callers with richer information may post-process
// TrajDelta.BaseStop / CandStop.
//
// Output is byte-stable given the same input messages.
func TrajectoryDiff(base, cand []llmclient.Message) TrajDelta {
	baseCalls, baseStop := extractTrajectory(base)
	candCalls, candStop := extractTrajectory(cand)

	callsChanged := !callSeqEqual(baseCalls, candCalls)
	stopChanged := baseStop != candStop
	turnsChanged := len(base) != len(cand)

	return TrajDelta{
		BaseCalls:    baseCalls,
		CandCalls:    candCalls,
		BaseStop:     baseStop,
		CandStop:     candStop,
		BaseTurns:    len(base),
		CandTurns:    len(cand),
		CallsChanged: callsChanged,
		StopChanged:  stopChanged,
		TurnsChanged: turnsChanged,
	}
}

// extractTrajectory scans messages for tool-call blocks and infers the stop
// reason from the last assistant message.
func extractTrajectory(msgs []llmclient.Message) (calls []ToolCallRecord, stop string) {
	calls = []ToolCallRecord{}
	stop = ""

	var lastAssistantHasToolUse bool
	for _, m := range msgs {
		hasToolUse := false
		for _, b := range m.Content {
			if b.Type == llmclient.BlockToolUse {
				hasToolUse = true
				calls = append(calls, ToolCallRecord{
					Name:     b.Name,
					ArgsHash: hashInput(b.Input),
				})
			}
		}
		if m.Role == llmclient.RoleAssistant {
			lastAssistantHasToolUse = hasToolUse
		}
	}

	if len(msgs) > 0 {
		if lastAssistantHasToolUse {
			stop = string(llmclient.StopToolUse)
		} else {
			stop = string(llmclient.StopEndTurn)
		}
	}
	return calls, stop
}

// hashInput returns the SHA-256 hex of the canonical compact JSON of input.
// Nil / empty input hashes as the empty string for stability.
func hashInput(input json.RawMessage) string {
	if len(input) == 0 {
		return fmt.Sprintf("%x", sha256.Sum256([]byte{}))
	}
	// Compact to remove any whitespace differences.
	var buf []byte
	buf, err := json.Marshal(json.RawMessage(input))
	if err != nil {
		// Fall back to raw bytes on marshal error.
		sum := sha256.Sum256(input)
		return fmt.Sprintf("%x", sum)
	}
	sum := sha256.Sum256(buf)
	return fmt.Sprintf("%x", sum)
}

// callSeqEqual returns true when a and b have the same length and each element
// matches (Name and ArgsHash).
func callSeqEqual(a, b []ToolCallRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].ArgsHash != b[i].ArgsHash {
			return false
		}
	}
	return true
}
