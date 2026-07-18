// valid.cue — concrete values that satisfy the #Author constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package aiprovenance

// A well-formed human author.
_human1: #Author & {
	Kind: "human"
	Human: {
		ID: "user-001"
	}
}

// A human author identified by email.
_human2: #Author & {
	Kind: "human"
	Human: {
		ID: "alice@example.com"
	}
}

// A well-formed AI author with all optional fields populated.
_ai1: #Author & {
	Kind: "ai"
	AI: {
		ModelID:               "claude-opus-4-8"
		ModelVersion:          "20260608"
		PromptHash:            "sha256-abc123def456"
		PromptTemplateVersion: "1.2.0"
		ParamsHash:            "sha256-params789"
		InputDigest:           "sha256-input000"
		At:                    "2026-06-08T12:00:00Z"
	}
}

// An AI author with optional fields empty (allowed by schema).
_ai2: #Author & {
	Kind: "ai"
	AI: {
		ModelID:               "claude-sonnet-4-6"
		ModelVersion:          "20260601"
		PromptHash:            "sha256-promptXYZ"
		PromptTemplateVersion: ""
		ParamsHash:            ""
		InputDigest:           "sha256-inputABC"
		At:                    "2026-06-08T09:30:00Z"
	}
}
