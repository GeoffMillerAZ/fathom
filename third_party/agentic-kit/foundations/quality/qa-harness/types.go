// Package qaharness provides the QA judge interface and deterministic rubric
// scoring for agentic outputs.
//
// The central abstraction is the Judge interface: a scorer that accepts an
// Item and returns a float64 score in [0, 1]. The package ships two
// implementations:
//
//   - RubricJudge — scores against a set of weighted RubricCriteria
//     deterministically; no LLM involved.
//   - MockJudge   — returns a fixed score, for use in tests.
//
// Concurrency: values are immutable and safe for concurrent use.
package qaharness

import "context"

// Item is the unit of evaluation — a model output together with optional
// contextual metadata.
type Item struct {
	// ID is a stable identifier for this item, used in score records.
	ID string

	// Output is the text produced by the model under evaluation.
	Output string

	// Metadata carries arbitrary key/value context (e.g. prompt hash, model
	// ID, run ID). It is passed through unchanged to the score record.
	Metadata map[string]string
}

// ScoreRecord pairs an Item ID with its evaluated score.
type ScoreRecord struct {
	// ItemID is the ID of the evaluated Item.
	ItemID string

	// Score is the evaluated score in [0, 1].
	Score float64
}

// Judge is the interface through which all QA scoring is performed.
// Implementations may be deterministic (RubricJudge) or model-backed.
type Judge interface {
	// Score evaluates the Item and returns a score in [0, 1].
	// An error is returned only when evaluation cannot proceed (e.g.
	// a required external call fails); returning 0.0 with a nil error is a
	// valid "zero score" outcome.
	Score(ctx context.Context, item Item) (float64, error)
}

// RubricCriterion is a single weighted evaluation dimension used by RubricJudge.
type RubricCriterion struct {
	// Name is a human-readable label for this dimension.
	Name string

	// Weight is the relative importance of this criterion. Must be > 0.
	Weight float64

	// Check is the predicate applied to the Item. It returns true when the
	// criterion is satisfied.
	Check func(item Item) bool
}

// RubricJudge scores an Item against a set of RubricCriteria deterministically.
// The composite score is:
//
//	score = Σ(weight of passing criteria) / Σ(all weights)
//
// Zero when there are no criteria or all weights are non-positive.
type RubricJudge struct {
	// Criteria is the ordered set of evaluation dimensions.
	Criteria []RubricCriterion
}

// MockJudge always returns a fixed score. Useful in tests.
type MockJudge struct {
	// FixedScore is the value returned by every Score call.
	FixedScore float64

	// Err, when non-nil, is returned as the error from every Score call.
	Err error
}
