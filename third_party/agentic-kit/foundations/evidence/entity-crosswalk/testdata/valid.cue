// valid.cue — concrete values that satisfy the #MappingEntry constraint
// defined in schema.cue. Running `cue vet ./...` from the package directory
// validates these against the schema.
package entitycrosswalk

// A mapping recorded by a human analyst.
_humanMapping: #MappingEntry & {
	CanonicalID: "service-payments"
	AliasID:     "repo-payments"
	Author: {
		Kind: "human"
		Human: {
			ID: "analyst-1"
		}
	}
	RecordedAt: "2026-01-01T00:00:00Z"
}

// A mapping recorded by an AI model (full reproduction key required).
_aiMapping: #MappingEntry & {
	CanonicalID: "service-auth"
	AliasID:     "workload-auth"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:      "claude-sonnet-4-6"
			ModelVersion: "20260608"
			PromptHash:   "sha256-promptABC123"
			InputDigest:  "sha256-inputXYZ789"
			At:           "2026-01-02T12:34:56Z"
		}
	}
	RecordedAt: "2026-01-02T12:34:56Z"
}

// A human mapping with fractional-second timestamp.
_humanMappingFractional: #MappingEntry & {
	CanonicalID: "service-billing"
	AliasID:     "repo-billing"
	Author: {
		Kind: "human"
		Human: {
			ID: "legal-analyst-senior-2026"
		}
	}
	RecordedAt: "2026-03-15T09:30:00.500Z"
}

// An AI mapping with optional fields set.
_aiMappingFull: #MappingEntry & {
	CanonicalID: "entity-payments"
	AliasID:     "workload-payments"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "claude-opus-4-5"
			ModelVersion:          "20260601"
			PromptHash:            "sha256-prompt999"
			PromptTemplateVersion: "v1.2.0"
			ParamsHash:            "sha256-params001"
			InputDigest:           "sha256-inputABC"
			At:                    "2026-01-03T08:00:00Z"
		}
	}
	RecordedAt: "2026-01-03T08:00:01Z"
}
