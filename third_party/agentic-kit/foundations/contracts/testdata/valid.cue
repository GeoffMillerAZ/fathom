// valid.cue — a complete, constraint-satisfying example for every type
// defined in schema.cue. This file must unify with the schema on every
// `cue vet schema.cue testdata/valid.cue` run.
package contracts

import "github.com/geoffmilleraz/agentic-kit/foundations/contracts"

// validEvent satisfies #EventShape.
validEvent: contracts.#EventShape & {
	ID:        "evt-001"
	Kind:      "tool.call"
	Source:    "agent/recon-01"
	Timestamp: "2024-01-15T10:30:00Z"
	Payload: {
		tool:    "web_search"
		latency: "142ms"
	}
}

// validNode satisfies #NodeType.
validNode: contracts.#NodeType & {
	ID:   "node-compute-01"
	Type: "compute"
	Labels: {
		region: "us-east-1"
		tier:   "worker"
	}
}

// validAnswer satisfies #AnswerArchetype.
validAnswer: contracts.#AnswerArchetype & {
	Name:             "investigation-summary"
	Sections:         ["executive-summary", "findings", "recommendations"]
	RequiredEvidence: true
}

// validCard satisfies #KnowledgeCardArchetype.
validCard: contracts.#KnowledgeCardArchetype & {
	Name: "vulnerability-record"
	Fields: [
		{Name: "cve-id", Kind: "string", Required: true},
		{Name: "severity", Kind: "number", Required: true},
		{Name: "mitigated", Kind: "boolean", Required: false},
	]
}
