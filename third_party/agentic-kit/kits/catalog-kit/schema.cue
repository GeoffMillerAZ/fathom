package catalogkit

import (
	"list"
	"github.com/geoffmilleraz/agentic-kit/foundations/knowledge/catalog"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

_text: string & !=""
_slug: string & =~"^[a-z0-9]+(-[a-z0-9]+)*$"
_digest: string & =~"^[a-f0-9]{64}$"
_path: string & =~"^[a-z0-9]+(-[a-z0-9]+)*(/[a-z0-9]+(-[a-z0-9]+)*)*$"
#Field: "title" | "synopsis" | "order" | "hidden" | "facets"

// #Spec is authored intent. Graph and cross-layer identity checks run in Go.
#Spec: {
	version: 1
	layers: [...#Layer] & list.MinItems(1)
}
#Layer: {
	name: _slug
	requires?: [..._slug]
	pin?: _digest
	nodes?: [...catalog.#Node]
	overrides?: [...#Override]
}
#Override: {
	id: _path
	expected_digest: _digest
	reason: _text
	set: #Patch
}
#Patch: {
	title?: string
	synopsis?: string
	order?: int
	hidden?: bool
	facets?: [string]: [...string]
}
#Grant: {
	layer: _slug
	id: _path
	fields: [...#Field] & list.MinItems(1)
}
#Limits: {
	max_layers?: int & >=0
	max_nodes?: int & >=0
	max_depth?: int & >=0
	max_bytes?: int & >=0
}
// #Policy must be supplied by the receiver, independently of imported layers.
#Policy: {
	grants?: [...#Grant]
	limits?: #Limits
}

// #Profile selects roots and their transitive dependencies without granting overrides.
#Profile: {
	name: _slug
	roots: [..._slug] & list.MinItems(1)
}

// #Binding reuses the one decision port. Its full question shape is deliberate:
// provider-specific spellings belong only to the existing provider adapters.
#Binding: {
	node_id: _path
	node_digest: _digest
	definition_digest: _digest
	version: _text
	question: decision.#Question
	required_evidence: [..._text] & list.MinItems(1)
}
#Evidence: {
	id: _text
	source: _text
	revision: _text
	text: _text
	digest: _digest
}
#DecisionIdentity: {
	provider: _text
	requested_model: _text
	resolved_model: _text
	adapter_version: _text
	policy_version: _text
}

// #Difference is the deterministic projection of two verified bundles.
#Difference: {
	before: _digest
	after: _digest
	added: [..._path]
	removed: [..._path]
	changed: [..._path]
	moved: [...#Move]
}
#Move: {
	stable_key: _text
	before_id: _path
	after_id: _path
}
