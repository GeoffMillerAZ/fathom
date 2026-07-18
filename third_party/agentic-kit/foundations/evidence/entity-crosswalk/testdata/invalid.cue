// invalid.cue — demonstrates #MappingEntry values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the
// valid.cue sibling is the one vetted in CI). It is kept here as a
// documentation artifact so reviewers understand what the schema rejects.
//
// Per kit convention each violation is shown as commented-out CUE so this
// file parses cleanly without triggering errors itself. See e.g.
// foundations/provenance/ai-provenance/testdata/invalid.cue for the pattern.
package entitycrosswalk

// _emptyCanonicalID: empty CanonicalID violates the _nonEmpty constraint.
// _emptyCanonicalID: #MappingEntry & {
// 	CanonicalID: ""
// 	AliasID:     "repo-payments"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 	}
// 	RecordedAt: "2026-01-01T00:00:00Z"
// }

// _emptyAliasID: empty AliasID violates the _nonEmpty constraint.
// _emptyAliasID: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     ""
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 	}
// 	RecordedAt: "2026-01-01T00:00:00Z"
// }

// _selfAlias: CanonicalID == AliasID violates the !=CanonicalID constraint.
// _selfAlias: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     "service-payments"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 	}
// 	RecordedAt: "2026-01-01T00:00:00Z"
// }

// _nonRFC3339RecordedAt: "banana" is not a valid RFC3339 timestamp.
// This was the original skeptic finding — _nonEmpty accepted arbitrary strings.
// _nonRFC3339RecordedAt: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     "repo-payments"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 	}
// 	RecordedAt: "banana"
// }

// _emptyRecordedAt: empty RecordedAt violates _rfc3339 (which subsumes _nonEmpty).
// _emptyRecordedAt: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     "repo-payments"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 	}
// 	RecordedAt: ""
// }

// _oldAuthorShape: the former local #Author (Kind + Label) is no longer valid.
// aiprovenance.#Author is a discriminated union requiring Kind + Human{ID} or
// Kind + AI{ModelID, ...}. A flat {Kind, Label} object does not satisfy it.
// _oldAuthorShape: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     "repo-payments"
// 	Author: {
// 		Kind:  "human"
// 		Label: "analyst-1"
// 	}
// 	RecordedAt: "2026-01-01T00:00:00Z"
// }

// _aiAuthorMissingModelID: an AI author without ModelID violates
// aiprovenance.#AIAuthor's _nonEmpty constraint on ModelID.
// _aiAuthorMissingModelID: #MappingEntry & {
// 	CanonicalID: "service-auth"
// 	AliasID:     "workload-auth"
// 	Author: {
// 		Kind: "ai"
// 		AI: {
// 			ModelID:      ""
// 			ModelVersion: "20260608"
// 			PromptHash:   "sha256-promptABC"
// 			InputDigest:  "sha256-inputXYZ"
// 			At:           "2026-01-02T12:34:56Z"
// 		}
// 	}
// 	RecordedAt: "2026-01-02T12:34:56Z"
// }

// _aiAuthorMissingInputDigest: an AI author without InputDigest violates
// aiprovenance.#AIAuthor's _nonEmpty constraint on InputDigest.
// _aiAuthorMissingInputDigest: #MappingEntry & {
// 	CanonicalID: "service-auth"
// 	AliasID:     "workload-auth"
// 	Author: {
// 		Kind: "ai"
// 		AI: {
// 			ModelID:      "claude-sonnet-4-6"
// 			ModelVersion: "20260608"
// 			PromptHash:   "sha256-promptABC"
// 			InputDigest:  ""
// 			At:           "2026-01-02T12:34:56Z"
// 		}
// 	}
// 	RecordedAt: "2026-01-02T12:34:56Z"
// }

// _humanAuthorWithAIBlock: Kind "human" but AI block present — rejected by the
// discriminated union in aiprovenance.#Author.
// _humanAuthorWithAIBlock: #MappingEntry & {
// 	CanonicalID: "service-payments"
// 	AliasID:     "repo-payments"
// 	Author: {
// 		Kind: "human"
// 		Human: {ID: "analyst-1"}
// 		AI: {
// 			ModelID:      "claude-sonnet-4-6"
// 			ModelVersion: "20260608"
// 			PromptHash:   "sha256-promptABC"
// 			InputDigest:  "sha256-inputXYZ"
// 			At:           "2026-01-02T12:34:56Z"
// 		}
// 	}
// 	RecordedAt: "2026-01-01T00:00:00Z"
// }
