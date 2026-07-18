// Package qaharness defines the CUE schema for the qa-harness foundation.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package qaharness

// #Item is the unit of evaluation passed to a Judge.
#Item: {
	// ID is a non-empty stable identifier.
	ID: string & !=""

	// Output is the text produced by the model under evaluation.
	Output: string

	// Metadata carries arbitrary key/value context.
	Metadata: {[string]: string}
}

// #ScoreRecord pairs an Item ID with its evaluated score.
#ScoreRecord: {
	// ItemID is the ID of the evaluated Item.
	ItemID: string & !=""

	// Score is the evaluated score in [0, 1].
	Score: number & >=0 & <=1
}

// #RubricCriterion is a single weighted evaluation dimension.
#RubricCriterion: {
	// Name is a non-empty human-readable label.
	Name: string & !=""

	// Weight is the relative importance; must be strictly positive.
	Weight: number & >0
}
