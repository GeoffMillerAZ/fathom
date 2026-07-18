// invalid.cue — demonstrates #Author values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
//
// The discriminated-union constraints below (Kind ↔ field coupling) are
// enforced by CUE since schema.cue uses disjunctions. Uncomment any block and
// run `cue vet .` from this directory to observe the rejection. The equivalent
// Go-level rejections are covered by Validate() tests in aiprovenance_test.go.
package aiprovenance

// _badKind: Kind must be "human" or "ai" — any other value violates the
// disjunction constraint.
// _badKind: #Author & {
// 	Kind: "robot"
// 	Human: { ID: "r2d2" }
// }

// _humanEmptyID: Human.ID must be non-empty (_nonEmpty constraint).
// _humanEmptyID: #Author & {
// 	Kind: "human"
// 	Human: { ID: "" }
// }

// _aiEmptyModelID: AI.ModelID must be non-empty (_nonEmpty constraint).
// _aiEmptyModelID: #Author & {
// 	Kind: "ai"
// 	AI: {
// 		ModelID:      ""
// 		ModelVersion: "v1"
// 		PromptHash:   "ph1"
// 		InputDigest:  "id1"
// 		At:           "2026-06-08T00:00:00Z"
// 	}
// }

// _aiEmptyPromptHash: AI.PromptHash must be non-empty (_nonEmpty constraint).
// _aiEmptyPromptHash: #Author & {
// 	Kind: "ai"
// 	AI: {
// 		ModelID:      "claude-opus-4-8"
// 		ModelVersion: "20260608"
// 		PromptHash:   ""
// 		InputDigest:  "id1"
// 		At:           "2026-06-08T00:00:00Z"
// 	}
// }

// _aiEmptyInputDigest: AI.InputDigest must be non-empty (_nonEmpty constraint).
// _aiEmptyInputDigest: #Author & {
// 	Kind: "ai"
// 	AI: {
// 		ModelID:      "claude-opus-4-8"
// 		ModelVersion: "20260608"
// 		PromptHash:   "ph1"
// 		InputDigest:  ""
// 		At:           "2026-06-08T00:00:00Z"
// 	}
// }

// _aiEmptyAt: AI.At must be non-empty (_nonEmpty constraint).
// _aiEmptyAt: #Author & {
// 	Kind: "ai"
// 	AI: {
// 		ModelID:      "claude-opus-4-8"
// 		ModelVersion: "20260608"
// 		PromptHash:   "ph1"
// 		InputDigest:  "id1"
// 		At:           ""
// 	}
// }

// ── Discriminated-union violations (new — enforced by CUE disjunction) ───────
//
// These cases were previously accepted by cue vet (the old schema had no
// disjunction enforcement). They are now rejected by the schema.

// _humanWithAI: Kind="human" but AI block is present — violates the union.
// _humanWithAI: #Author & {
// 	Kind: "human"
// 	Human: { ID: "alice" }
// 	AI: {
// 		ModelID:      "claude-opus-4-8"
// 		ModelVersion: "20260608"
// 		PromptHash:   "ph1"
// 		InputDigest:  "id1"
// 		At:           "2026-06-08T00:00:00Z"
// 	}
// }

// _humanMissingHuman: Kind="human" but Human block is absent — violates the union.
// _humanMissingHuman: #Author & {
// 	Kind: "human"
// }

// _aiWithHuman: Kind="ai" but Human block is present — violates the union.
// _aiWithHuman: #Author & {
// 	Kind: "ai"
// 	Human: { ID: "alice" }
// 	AI: {
// 		ModelID:      "claude-opus-4-8"
// 		ModelVersion: "20260608"
// 		PromptHash:   "ph1"
// 		InputDigest:  "id1"
// 		At:           "2026-06-08T00:00:00Z"
// 	}
// }

// _aiMissingAI: Kind="ai" but AI block is absent — violates the union.
// _aiMissingAI: #Author & {
// 	Kind: "ai"
// }
