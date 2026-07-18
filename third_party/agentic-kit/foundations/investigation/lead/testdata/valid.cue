// Package lead — valid test data that must unify with schema.cue.
// Run `cue vet ./...` from the repo root to validate these against the schema.
package lead

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/lead"

// validLead_Open demonstrates a well-formed lead in the open state.
validLead_Open: lead.#Lead & {
	ID:             "lead-001"
	Prompt:         "Did the defendant access the server on 2026-01-15?"
	HypothesisID:   "hyp-42"
	TriggerSpanIDs: ["span-a", "span-b"]
	Assignee:       "alice"
	Status:         "open"
	Author: {
		Kind: "human"
		Human: {ID: "investigator-1"}
	}
	CreatedAt:     "2026-06-08T12:00:00Z"
	UpdatedAt:     "2026-06-08T12:00:00Z"
	ResultSpanIDs: []
}

// validLead_Pursuing demonstrates a lead actively being investigated.
validLead_Pursuing: lead.#Lead & {
	ID:             "lead-002"
	Prompt:         "Identify all communications between Party A and Party B in Q1."
	TriggerSpanIDs: []
	Status:         "pursuing"
	Author: {
		Kind: "human"
		Human: {ID: "analyst-7"}
	}
	CreatedAt:     "2026-06-08T09:00:00Z"
	UpdatedAt:     "2026-06-08T10:30:00Z"
	ResultSpanIDs: []
}

// validLead_Closed demonstrates a resolved lead with an outcome.
validLead_Closed: lead.#Lead & {
	ID:             "lead-003"
	Prompt:         "Was the witness present at the scene?"
	TriggerSpanIDs: ["span-c"]
	Status:         "closed"
	Author: {
		Kind: "human"
		Human: {ID: "investigator-1"}
	}
	CreatedAt:     "2026-06-07T08:00:00Z"
	UpdatedAt:     "2026-06-08T15:00:00Z"
	Outcome:       "Confirmed present via CCTV span-d"
	ResultSpanIDs: ["span-d"]
}

// validLead_Abandoned demonstrates a lead set aside without resolution.
validLead_Abandoned: lead.#Lead & {
	ID:             "lead-004"
	Prompt:         "Review financial records from 2020."
	TriggerSpanIDs: []
	Status:         "abandoned"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "claude-sonnet-4-6"
			ModelVersion:          "20260601"
			PromptHash:            "sha256-abc123"
			PromptTemplateVersion: ""
			ParamsHash:            ""
			InputDigest:           "sha256-input001"
			At:                    "2026-06-08T11:00:00Z"
		}
	}
	CreatedAt:     "2026-06-06T10:00:00Z"
	UpdatedAt:     "2026-06-08T14:00:00Z"
	Outcome:       "Out of scope for current investigation"
	ResultSpanIDs: []
}

// validEntry demonstrates a well-formed #LeadEntry snapshot.
validEntry: lead.#LeadEntry & {
	Lead: {
		ID:             "lead-005"
		Prompt:         "Verify timestamp consistency in the access logs."
		TriggerSpanIDs: ["span-x"]
		Status:         "pursuing"
		Author: {
			Kind:  "human"
			Human: {ID: "analyst-3"}
		}
		CreatedAt:     "2026-06-08T08:00:00Z"
		UpdatedAt:     "2026-06-08T09:15:00Z"
		ResultSpanIDs: []
	}
	TransitionAuthor: {
		Kind:  "human"
		Human: {ID: "analyst-3"}
	}
	RecordedAt: "2026-06-08T09:15:00Z"
}

// Verify all #LeadStatus enum values are accepted.
_s1: lead.#LeadStatus & "open"
_s2: lead.#LeadStatus & "pursuing"
_s3: lead.#LeadStatus & "closed"
_s4: lead.#LeadStatus & "abandoned"
