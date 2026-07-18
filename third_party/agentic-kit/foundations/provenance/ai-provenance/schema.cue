// Package aiprovenance defines the CUE-first contract for AI and human
// authorship of evidence, decisions, and prose nodes. Go types in types.go
// mirror these definitions; any drift is a bug.
package aiprovenance

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #HumanAuthor carries the identity of a human producer.
#HumanAuthor: {
	// ID is a non-empty identifier for the human author (e.g. an email or
	// user UUID).
	ID: _nonEmpty
}

// #AIAuthor carries the full reproduction record of an AI inference.
#AIAuthor: {
	// ModelID is the canonical identifier of the model
	// (e.g. "claude-opus-4-8").
	ModelID: _nonEmpty

	// ModelVersion is the specific version string of the model.
	ModelVersion: _nonEmpty

	// PromptHash is the hash of the fully-rendered prompt sent to the model.
	PromptHash: _nonEmpty

	// PromptTemplateVersion is the semver-style version of the prompt
	// template used to render the prompt. Optional.
	PromptTemplateVersion: string

	// ParamsHash is the hash of the sampling parameters (temperature, top_p,
	// etc.) used for this inference. Optional.
	ParamsHash: string

	// InputDigest is the hash of the evidence or content that was fed as
	// input to the model.
	InputDigest: _nonEmpty

	// At is the RFC 3339 timestamp of the inference.
	At: _nonEmpty
}

// #Author records who or what produced an evidence span, decision, or prose
// node. The discriminated union enforces that exactly one of Human/AI is
// present and matches Kind — a value with Kind: "human" and an AI block, or
// Kind: "ai" with no AI block, is rejected by cue vet.
//
// Discriminated-union structure:
//   - Kind: "human" → Human must be present; AI must NOT be present.
//   - Kind: "ai"    → AI must be present; Human must NOT be present.
#Author: ({
	// Human branch — Kind must be "human", Human sub-struct is required,
	// AI sub-struct is forbidden.
	Kind:   "human"
	Human:  #HumanAuthor
	AI?:    _|_
}) | ({
	// AI branch — Kind must be "ai", AI sub-struct is required,
	// Human sub-struct is forbidden.
	Kind:   "ai"
	AI:     #AIAuthor
	Human?: _|_
})
