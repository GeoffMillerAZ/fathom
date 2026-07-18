// testdata/valid.cue — valid archetype instances for `cue vet` smoke-testing.
//
// Each value below must satisfy the corresponding #Archetype or #NodeKind
// definition.  This file is the positive-case fixture: if `cue vet ./...`
// fails here the schema definitions are broken, not the instances.
package schemas

// A valid #Archetype representing a finding node.
_findingArchetype: #Archetype & {
	id:    "arch-finding-001"
	label: "Finding"
	kind:  "node"
}

// A valid #Archetype representing an edge relationship.
_edgeArchetype: #Archetype & {
	id:    "arch-edge-001"
	label: "Relates To"
	kind:  "edge"
}

// A valid #Archetype representing an event.
_eventArchetype: #Archetype & {
	id:    "arch-event-001"
	label: "Deployment Event"
	kind:  "event"
}

// Valid #NodeKind literals — each must be accepted by the enum.
_nodeKindService:  #NodeKind & "service"
_nodeKindFinding:  #NodeKind & "finding"
_nodeKindEvidence: #NodeKind & "evidence"
_nodeKindAgent:    #NodeKind & "agent"

// ── Invalid examples (commented out so `cue vet` stays green) ────────────────
//
// The following would fail `cue vet` if uncommented:
//
// _badArchetype: #Archetype & {
//     id:    ""           // violates string & !=""
//     label: "Bad"
//     kind:  "node"
// }
//
// _badKind: #ArchetypeKind & "unknown"  // "unknown" is not in the enum
//
// _badNodeKind: #NodeKind & "gateway"   // "gateway" is not a valid NodeKind
