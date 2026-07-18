// Package contracts defines canonical CUE-first type contracts shared across
// the agentic-kit platform. These definitions are the authoritative source of
// truth for the value spaces that the Go types in types.go mirror.
package contracts

// _nodeTypeEnum enumerates the valid substrate node kinds (unexported to
// avoid incomplete-value noise at cue vet -c time).
_nodeTypeEnum: "compute" | "storage" | "gateway" | "inference" | "orchestrator"

// _fieldKindEnum enumerates the valid kinds a FieldSpec may declare.
_fieldKindEnum: "string" | "number" | "boolean" | "list" | "map"

// #NodeTypeEnum is the exported definition form of the node type enum,
// used when consumers need to reference the disjunction by name.
#NodeTypeEnum: _nodeTypeEnum

// #FieldKindEnum is the exported definition form of the field kind enum.
#FieldKindEnum: _fieldKindEnum

// #EventShape is an elementary platform event. Every telemetry and routing
// event in the system must satisfy this definition.
// Fields marked with '!' are required — omitting them is a vet error.
#EventShape: {
	// ID is a unique identifier for the event.
	ID!: string & !=""

	// Kind classifies the event domain.
	Kind!: string & !=""

	// Source identifies the emitting component or agent.
	Source!: string & !=""

	// Timestamp is an RFC 3339 / ISO 8601 timestamp string.
	Timestamp!: string & !=""

	// Payload carries event-specific fields. Keys must be strings.
	Payload!: {[string]: _}
}

// #NodeType represents a governed substrate node in the topology graph.
#NodeType: {
	// ID uniquely identifies the node.
	ID!: string & !=""

	// Type is the node's substrate classification.
	Type!: _nodeTypeEnum

	// Labels are arbitrary key/value metadata attached to the node.
	Labels!: {[string]: string}
}

// #AnswerArchetype is a reusable answer-shape template used by the knowledge
// and investigation foundations.
#AnswerArchetype: {
	// Name is the human-readable identifier for this archetype.
	Name!: string & !=""

	// Sections is the ordered list of prose section names the answer must
	// contain. At least one section is required.
	Sections!: [string & !="", ...string & !=""]

	// RequiredEvidence indicates whether the archetype mandates that at
	// least one evidence span be cited.
	RequiredEvidence!: bool
}

// #FieldSpec describes a single field in a KnowledgeCard.
#FieldSpec: {
	// Name is the field identifier.
	Name!: string & !=""

	// Kind is the primitive or composite type of the field.
	Kind!: _fieldKindEnum

	// Required indicates whether the field must be present on every card
	// that uses this archetype.
	Required!: bool
}

// #KnowledgeCardArchetype defines the canonical template for a knowledge card.
#KnowledgeCardArchetype: {
	// Name is the human-readable identifier for this archetype.
	Name!: string & !=""

	// Fields is the list of field specifications. At least one field is
	// required.
	Fields!: [#FieldSpec, ...#FieldSpec]
}
