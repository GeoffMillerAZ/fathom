// Package schemas is the canonical CUE schema root for agentic-kit.
//
// It provides:
//   - A Registry of known CUE schema package paths so that tooling can
//     enumerate and validate all governed schema surfaces.
//   - Go type mirrors for the shared CUE archetypes defined in schema.cue.
//
// The authoritative definitions live in schema.cue; Go types here must remain
// in sync with that file.  Drift is detected by `cue vet ./...`.
package schemas

// ArchetypeKind identifies the broad category of a governed archetype.
// It mirrors the #ArchetypeKind enum in schema.cue.
type ArchetypeKind string

const (
	// ArchetypeKindNode is a structural node in an analytical graph.
	ArchetypeKindNode ArchetypeKind = "node"

	// ArchetypeKindEdge is a directed relationship between two nodes.
	ArchetypeKindEdge ArchetypeKind = "edge"

	// ArchetypeKindEvent is a point-in-time occurrence.
	ArchetypeKindEvent ArchetypeKind = "event"
)

// NodeKind classifies the role a node plays within the agentic-kit knowledge
// graph.  It mirrors the #NodeKind enum in schema.cue.
type NodeKind string

const (
	// NodeKindService represents a deployable software service.
	NodeKindService NodeKind = "service"

	// NodeKindFinding represents an analytical finding (a surfaced issue or
	// observation from an investigation workflow).
	NodeKindFinding NodeKind = "finding"

	// NodeKindEvidence represents a piece of evidence that supports or refutes
	// a finding.
	NodeKindEvidence NodeKind = "evidence"

	// NodeKindAgent represents an agentic workflow participant.
	NodeKindAgent NodeKind = "agent"
)

// Archetype is the Go mirror of the #Archetype CUE definition.  Every governed
// type in the kit is an Archetype — it carries a stable ID, a human label, and
// its classification.
type Archetype struct {
	// ID is the stable, opaque identifier for this archetype instance.
	ID string `json:"id"`

	// Label is a short human-readable name.
	Label string `json:"label"`

	// Kind classifies the archetype.
	Kind ArchetypeKind `json:"kind"`
}
