// Package schemas — canonical CUE schema root for agentic-kit.
//
// Defines shared archetype definitions that all analytical and governance
// surfaces reference.  Go types in this package must remain in sync with these
// definitions; drift is detected by `cue vet ./...`.
package schemas

// #ArchetypeKind classifies the broad category of a governed archetype.
#ArchetypeKind: "node" | "edge" | "event"

// #NodeKind classifies the role a node plays in the agentic-kit knowledge
// graph.
#NodeKind: "service" | "finding" | "evidence" | "agent"

// #Archetype is the base definition shared by every governed type in the kit.
// It carries a stable opaque identifier, a human label, and its classification.
#Archetype: {
	// id is a stable, opaque identifier for this archetype instance.
	id: string & !=""

	// label is a short human-readable name.
	label: string & !=""

	// kind classifies the archetype.
	kind: #ArchetypeKind
}
