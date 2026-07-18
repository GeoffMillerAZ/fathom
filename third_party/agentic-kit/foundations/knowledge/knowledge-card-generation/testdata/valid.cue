// valid.cue — concrete values that satisfy the #KnowledgeCard constraint
// defined in schema.cue. Running `cue vet ./...` from the package root or
// the repo root validates these against the schema.
package knowledgecardgeneration

// A fully-populated knowledge card for a service-capability archetype.
_serviceCapabilityCard: #KnowledgeCard & {
	Title: "service-capability: Payments Service"
	Fields: {
		"description":  "Handles payment processing and refunds."
		"slo":          "99.9% uptime, p99 latency < 200ms."
		"dependencies": "auth-service, ledger-service"
	}
	SourceID: "docs/payments/README.md"
}

// A card with an empty Fields map (no extractable fields in source).
_emptyFieldsCard: #KnowledgeCard & {
	Title:    "deployment-fact: Staging Config"
	Fields:   {}
	SourceID: "config/staging.yaml"
}

// A minimal valid card with a single field.
_minimalCard: #KnowledgeCard & {
	Title: "security-control: TLS Enforcement"
	Fields: {
		"requirement": "All traffic must use TLS 1.2 or higher."
	}
	SourceID: "runbooks/security-baseline.md"
}
