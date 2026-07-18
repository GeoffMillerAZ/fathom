// valid.cue — concrete values that satisfy the #Entry constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package investigationjournal

// A well-formed entry at the hypothesis-formation step (no Author — system step).
_entry1: #Entry & {
	Step: "hypothesis-formed"
	Note: "Formed initial hypothesis about ownership gap in payments service."
	At:   "2024-01-15T09:00:00Z"
}

// An entry recording evidence collection, with evidence span references.
_entry2: #Entry & {
	Step: "evidence-collected"
	Note: "Pulled 12 evidence spans from source registry covering code and docs."
	At:   "2024-01-15T09:05:00Z"
	SpanRefs: ["span-001", "span-002"]
}

// An entry recording convergence reached.
_entry3: #Entry & {
	Step: "convergence-reached"
	Note: "Oracle returned converged verdict after 3 rounds of hypothesis ranking."
	At:   "2024-01-15T09:30:00Z"
}

// A fully-provenanced entry with a human Author.
_entry4: #Entry & {
	Step: "evidence-reviewed"
	Note: "Human analyst reviewed 5 spans and confirmed ownership attribution."
	At:   "2024-01-15T10:00:00Z"
	Author: {
		Kind: "human"
		Human: {
			ID: "alice@example.com"
		}
	}
}

// A fully-provenanced entry with an AI Author.
_entry5: #Entry & {
	Step: "hypothesis-ranked"
	Note: "AI model ranked 3 hypotheses by evidential support."
	At:   "2024-01-15T10:05:00Z"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:      "claude-sonnet-4-6"
			ModelVersion: "20260608"
			PromptHash:   "sha256-promptABC123"
			InputDigest:  "sha256-inputXYZ789"
			At:           "2024-01-15T10:04:58Z"
		}
	}
}
