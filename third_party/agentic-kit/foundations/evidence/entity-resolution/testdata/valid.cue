// valid.cue — concrete values that satisfy the #Candidate, #ReviewState, and
// #Match constraints defined in schema.cue. Running `cue vet ./...` from the
// package root validates these against the schema.
package entityresolution

// _humanAuthor is a reusable well-formed human author for tests.
_humanAuthor: #Author & {
	Kind: "human"
	Human: {ID: "analyst-1"}
}

// _aiAuthor is a reusable well-formed AI author for tests.
_aiAuthor: #Author & {
	Kind: "ai"
	AI: {
		ModelID:      "claude-sonnet-4-6"
		ModelVersion: "20260608"
		PromptHash:   "sha256:abcdef1234567890"
		PromptTemplateVersion: "1.0.0"
		ParamsHash:   "sha256:params0000000000"
		InputDigest:  "sha256:input00000000000"
		At:           "2026-06-08T12:00:00Z"
	}
}

// _grade1 is a well-formed documentary-tier grade.
_grade1: #Grade & {
	Tier:       "documentary"
	Confidence: 0.9
}

// _grade2 is a well-formed inferred-tier grade (AI-produced candidate).
_grade2: #Grade & {
	Tier:       "inferred"
	Confidence: 0.7
}

// A well-formed candidate with a high similarity score and human author.
_cand1: #Candidate & {
	LeftID:  "entity-alice-j"
	RightID: "entity-alice-jones"
	Score:   0.87
	Method:  "name-similarity"
	Author:  _humanAuthor
	Grade:   _grade1
}

// A candidate at the minimum score boundary with AI author.
_cand2: #Candidate & {
	LeftID:  "entity-bob"
	RightID: "entity-robert"
	Score:   0.0
	Method:  "name-similarity"
	Author:  _aiAuthor
	Grade:   _grade2
}

// A candidate at the maximum score boundary with span IDs.
_cand3: #Candidate & {
	LeftID:        "entity-jane-doe"
	RightID:       "entity-jane-doe-duplicate"
	Score:         1.0
	Method:        "name-similarity"
	Author:        _humanAuthor
	Grade:         _grade1
	SourceSpanIDs: ["span-abc123", "span-def456"]
}

// A proposed match (no reviewer yet).
_match1: #Match & {
	Candidate: _cand1
	State:     "proposed"
	CreatedAt: "2026-06-08T12:00:00Z"
}

// An accepted match with reviewer, note, and timestamps.
_match2: #Match & {
	Candidate:  _cand2
	State:      "accepted"
	CreatedAt:  "2026-06-08T12:00:00Z"
	ReviewedBy: "analyst-1"
	ReviewedAt: "2026-06-08T13:00:00Z"
	Note:       "confirmed same individual"
}

// A rejected match with reviewer, note, and timestamps.
_match3: #Match & {
	Candidate:  _cand3
	State:      "rejected"
	CreatedAt:  "2026-06-08T12:00:00Z"
	ReviewedBy: "analyst-2"
	ReviewedAt: "2026-06-08T14:00:00Z"
	Note:       "different people with same name"
}

// A proposed match with no optional fields.
_match4: #Match & {
	Candidate: _cand3
	State:     "proposed"
	CreatedAt: "2026-06-08T12:00:00Z"
}
