// valid.cue — positive fixtures for the checkcatalog schema; `cue vet ./testdata/`
// from the package directory checks them against ../schema.cue.
package checkcatalog

// fatalDeterministic: the strongest severity, with the means the schema
// requires for it.
fatalDeterministic: #Check & {
	id:       "INT-01"
	name:     "artifact size"
	severity: "fatal"
	means:    "deterministic"
	rejects:  "An artifact larger than the reader's parser will accept."
	source:   "the reader's published limit (verified 2026-09-02)"
}

// majorModel: a blocking severity may rest on a model — that is the
// consumer's call, and the catalog makes it visible.
majorModel: #Check & {
	id:       "HON-09"
	name:     "unsupported claim"
	severity: "major"
	means:    "model"
	rejects:  "A claim the source material does not support."
	source:   "judgement"
}

// improvementNoSource: source is optional in the shape; a consumer that
// wants it mandatory asserts so in its own test.
improvementNoSource: #Check & {
	id:       "STY-01"
	name:     "bullet shape"
	severity: "improvement"
	means:    "deterministic"
	rejects:  "Bullets that are stubs or runners; reports the length distribution."
}

// catalog: a whole declared list.
catalog: #Catalog & [fatalDeterministic, majorModel, improvementNoSource]
