// Package qaharness — valid test data that must unify with schema.cue.
package qaharness

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/qa-harness:qaharness"

// validItem demonstrates a well-formed evaluation item.
validItem: qaharness.#Item & {
	ID:     "item-001"
	Output: "The answer is 42."
	Metadata: {
		"model":  "gpt-4o"
		"run_id": "run-abc123"
	}
}

// validItemNoMetadata demonstrates an item with empty metadata.
validItemNoMetadata: qaharness.#Item & {
	ID:       "item-002"
	Output:   "Plain output."
	Metadata: {}
}

// validScoreRecord demonstrates a well-formed score record.
validScoreRecord: qaharness.#ScoreRecord & {
	ItemID: "item-001"
	Score:  0.85
}

// validScoreRecordPerfect demonstrates a perfect score.
validScoreRecordPerfect: qaharness.#ScoreRecord & {
	ItemID: "item-003"
	Score:  1.0
}

// validScoreRecordZero demonstrates a zero score.
validScoreRecordZero: qaharness.#ScoreRecord & {
	ItemID: "item-004"
	Score:  0.0
}

// validRubricCriterion demonstrates a well-formed rubric criterion.
validRubricCriterion: qaharness.#RubricCriterion & {
	Name:   "output-not-empty"
	Weight: 1.5
}
