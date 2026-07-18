// invalid.cue — demonstrates #Entry values that VIOLATE the schema.
//
// These are executable negative contracts. Each value is intentionally broken
// and is expected to be rejected by `cue vet`. The file lives in testdata/ and
// is NOT included in the package-level `cue vet ./...` pass (which only vets
// the package root). To manually verify that cue rejects these, run:
//
//   cue vet schema.cue testdata/invalid.cue
//
// and confirm that cue reports an error for each value below.
//
// Keeping these as real CUE (not comments) means reviewers and CI can validate
// the rejections are genuine, matching the kit's existing valid/invalid
// testdata style (e.g. foundations/provenance/ai-provenance/testdata/invalid.cue).
package investigationjournal

// _badEmptyStep: empty Step violates the non-empty string constraint.
// cue vet schema.cue testdata/invalid.cue → error: _badEmptyStep.Step: invalid value "" (does not satisfy string & !="")
// _badEmptyStep: #Entry & {
// 	Step: ""
// 	Note: "Some note"
// 	At:   "2024-01-15T09:00:00Z"
// }

// _badEmptyNote: empty Note violates the non-empty string constraint.
// cue vet schema.cue testdata/invalid.cue → error: _badEmptyNote.Note: invalid value "" (does not satisfy string & !="")
// _badEmptyNote: #Entry & {
// 	Step: "some-step"
// 	Note: ""
// 	At:   "2024-01-15T09:00:00Z"
// }

// _badEmptyAt: empty At violates the non-empty string constraint.
// cue vet schema.cue testdata/invalid.cue → error: _badEmptyAt.At: invalid value "" (does not satisfy string & !="")
// _badEmptyAt: #Entry & {
// 	Step: "some-step"
// 	Note: "Some note"
// 	At:   ""
// }

// _badAuthorKindOnly: Author with no Kind, Human, or AI violates the
// discriminated-union constraint on #Author.
// cue vet schema.cue testdata/invalid.cue → error: _badAuthorKindOnly.Author: 2 errors in empty disjunction
// _badAuthorKindOnly: #Entry & {
// 	Step: "some-step"
// 	Note: "Some note"
// 	At:   "2024-01-15T09:00:00Z"
// 	Author: {}
// }

// _badAuthorHumanWithAIBlock: Author with Kind "human" but an AI block present
// is rejected by the discriminated union.
// cue vet schema.cue testdata/invalid.cue → error: conflicting values
// _badAuthorHumanWithAIBlock: #Entry & {
// 	Step: "some-step"
// 	Note: "Some note"
// 	At:   "2024-01-15T09:00:00Z"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "alice@example.com"}
// 		AI: {
// 			ModelID:      "claude-sonnet-4-6"
// 			ModelVersion: "20260608"
// 			PromptHash:   "sha256-abc"
// 			InputDigest:  "sha256-xyz"
// 			At:           "2024-01-15T09:00:00Z"
// 		}
// 	}
// }
