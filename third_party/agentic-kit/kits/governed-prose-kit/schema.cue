package governedprosekit

import (
	es "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span:evidencespan"
	ap "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance:aiprovenance"
)

_nonempty: string & !=""
_digest:   string & =~"^[0-9a-f]{64}$"
_time:     string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T"

#SourceRef: {id: _nonempty, version: _nonempty, digest: _digest}
#SourceSnapshot: {
	ref:          #SourceRef
	content:      string
	media_type:   string & =~"^text/"
	acquired_by:  _nonempty
	acquired_at:  _time
	access:       "public" | "restricted"
	retain_until: _time
	revoked:      bool
}
#Derivation: {
	version:   "arithmetic/v1"
	operation: "sum" | "difference" | "ratio"
	inputs: [_nonempty, _nonempty, ..._nonempty]
}

// Exact byte boundaries, derivation ordering/arithmetic, access, source versions,
// provenance, graph closure and review results are checked by Go at the entry point.
// #SpanJSON mirrors the canonical Go Span's JSON projection, without changing
// that foundation's authoring schema: Go emits inactive Author pointers as null
// and the exact zero Locator for derived facts. Scalar constraints stay canonical.
#SpanJSON: {
	ID:         es.#Span.ID
	EntityID:   es.#Span.EntityID
	Field:      es.#Span.Field
	Value:      es.#Span.Value
	Confidence: es.#Span.Confidence
	SourceID:   string
	RecordID:   string
	RecordHash: string
	OccurredAt: (es.#Span & {OccurredAt: _}).OccurredAt
	ObservedAt: (es.#Span & {ObservedAt: _}).ObservedAt
	Locator: es.#Locator | {Kind: "", Value: "", Start: 0, End: 0}
	Author: {Kind: "human", Human: ap.#HumanAuthor, AI: null} |
		{Kind: "ai", Human: null, AI: ap.#AIAuthor}
	Grade: (es.#Span & {Grade: _}).Grade
}
#Fact: {
	span: #SpanJSON
	source: {id: string, version: string, digest: string}
	unit:        string
	historical:  bool
	derivation?: #Derivation
}
#ProseClaim: {
	id:        _nonempty
	assertion: _nonempty
	fact_ids: [_nonempty, ..._nonempty]
	contradictions: [..._nonempty] | null
}
#ClaimRange: {claim_id: _nonempty, start: int & >=0, end: int & >start}
#ProseUnit: {
	id:   _nonempty
	text: _nonempty
	claims: [...#ClaimRange] | null
	non_factual_reason?: string
}
#ProseDraft: {claims: [...#ProseClaim] | null, units: [#ProseUnit, ...#ProseUnit]}
#SupportDecision: {
	status:   "supported" | "unsupported" | "inconclusive"
	reviewer: _nonempty
	method:   _nonempty
	reason:   _nonempty
}
#SourcedRequest: {principal: _nonempty, topic: _nonempty, facts: [#Fact, ...#Fact]}
#ProseLimits: {
	max_facts:  int & >0 & <=4096
	max_units:  int & >0 & <=4096
	max_claims: int & >0 & <=8192
	max_bytes:  int & >0 & <=16777216
	max_tokens: int & >0 & <=131072
}
#SourcedConfig: {policy_version: _nonempty, prompt_version: _nonempty, model: _nonempty, limits: #ProseLimits, allow_restricted_generation?: bool}
#ProseReceipt: {
	version:         "sourced-prose/v1"
	artifact_digest: _digest
	evidence_digest: _digest
	prompt_digest:   _digest
	config_digest:   _digest
	model:           _nonempty
	generation:      "model" | "supplied"
	checked_at:      _time
	integrity:       "pass" | "invalidated"
	support: {[string]: #SupportDecision} | null
}
#SourcedArtifact: {
	request: #SourcedRequest
	draft:   #ProseDraft
	receipt: #ProseReceipt
	invalid_facts: [..._nonempty] | null
	invalid_units: [..._nonempty] | null
	approval_ref?: _nonempty
}
#SourceMapEntry: {
	unit_id: _nonempty
	start:   int & >=0
	end:     int & >start
	claims: [...#ProseClaim] | null
	facts: [...#Fact] | null
	ranges: [...#ClaimRange] | null
}
#ProseExport: {
	markdown:        _nonempty
	markdown_digest: _digest
	sidecar_digest:  _digest
	artifact_digest: _digest
	evidence_digest: _digest
	audience:        "public" | "restricted"
	source_map: [...#SourceMapEntry] | null
	redacted_units: int & >=0
}

// Session-governed documents (additive; SourcedPipeline contracts above are unchanged).
// Digest recomputation, tree/edge/closure laws, goal binding and review are enforced
// by Go at the entry points; this mirror pins shape and enums for the wire. Caller
// inputs (#NodeSpec lists, expected_edges) may be null; Go normalizes them to arrays.
_kind: "document" | "section" | "list" | "table" | "heading" | "paragraph" | "sentence" | "summary" | "list-item" | "caption"
#ProfileOrigin: {
	kind:               "supplied" | "catalog"
	catalog_key?:       _nonempty
	definition_digest?: _digest
	layer_evidence: [..._nonempty]
}
#Goal: {
	id:         _nonempty
	class:      "required" | "advisory"
	statement:  _nonempty
	scope:      _nonempty
	method:     _nonempty
	acceptance: _nonempty
	origin:     "profile" | "session" | "task"
}
#SectionSpec: {id: _nonempty, title: _nonempty, required: bool, fact_families: [...string], guidance: string}
#PresentationDefaults: {audience: _nonempty, voice: _nonempty, technical_level: _nonempty}
#DocumentTypeProfile: {
	key:      _nonempty
	version:  _nonempty
	origin:   #ProfileOrigin
	freeform: bool
	sections: [#SectionSpec, ...#SectionSpec]
	goals: [#Goal, ...#Goal]
	defaults:              #PresentationDefaults
	permit_extra_sections: bool
	digest:                _digest
	digest_version:        "session-prose/v1"
}
#ProfileRef: {key: _nonempty, version: _nonempty, digest: _digest}
#DocumentSession: {
	id:           _nonempty
	revision:     int & >=1
	parent:       string
	purpose:      _nonempty
	profile:      #ProfileRef
	presentation: #PresentationDefaults
	defaults_from: {[string]: _nonempty}
	goals: [#Goal, ...#Goal]
	expected_goal_ids: [..._nonempty]
	expected_section_ids: [..._nonempty]
	constraints: [...string]
	intent: string
	digest: _digest
}
#NodeSpec: {
	id:          _nonempty
	kind:        _kind
	parent_id:   string
	order:       int
	instruction: string
	fact_ids: [...string] | null
	whole_input: bool
	context_node_ids: [...string] | null
	summarizes: [...string] | null
	goal_ids: [...string] | null
	section_id: string
}
#Edge: {from: _nonempty, to: _nonempty, kind: "summary" | "context"}
#DocumentSpec: {id: _nonempty, nodes: [#NodeSpec, ...#NodeSpec], expected_edges: [...#Edge] | null}
#DocumentInput: {principal: _nonempty, facts: [#Fact, ...#Fact]}
#DocumentTarget: {session: #DocumentSession, profile: #DocumentTypeProfile, spec: #DocumentSpec, input: #DocumentInput}
#ManifestFact: {id: _nonempty, digest: _digest}
#ReadManifest: {
	role:    "generation" | "review"
	node_id: _nonempty
	facts: [...#ManifestFact]
	whole_input: bool
	context: [...{id: _nonempty, content_digest: string}]
	goals_digest:       string
	session_digest:     string
	section_digest:     string
	profile_digest:     string
	config_digest:      string
	template_digest:    string
	instruction_digest: string
	text_digest:        string
	digest:             _digest
}
#GenerationRecord: {usage_state: "unqualified", attempt: int & >=1, model: _nonempty, stop_reason: string, input_tokens: int & >=0, output_tokens: int & >=0, prompt_digest: _digest}
#DocNode: {
	id:        _nonempty
	kind:      _kind
	parent_id: string
	order:     int
	text:      string
	claims: [...#ProseClaim]
	ranges: [...#ClaimRange]
	non_factual_reason: string
	support: {[string]: #SupportDecision}
	gen_manifest:    #ReadManifest | null
	review_manifest: #ReadManifest | null
	generation:      #GenerationRecord | null
	reused:          bool
	content_digest:  _digest
	version:         _digest
}
#GoalBinding: {goal_id: _nonempty, class: "required" | "advisory", nodes: [...string], assessed: bool}
#DocumentReceipt: {
	version:         "session-prose/v1"
	document_digest: _digest
	session_digest:  _digest
	profile_digest:  _digest
	config_digest:   _digest
	evidence_digest: _digest
	checked_at:      _time
	integrity:       "pass"
}
#DocumentVersion: {
	number: int & >=1
	parent: string
	target: #DocumentTarget
	nodes: [#DocNode, ...#DocNode]
	edges: [...#Edge]
	goals: [...#GoalBinding]
	receipt: #DocumentReceipt
}
#NodeImpact: {node_id: _nonempty, reasons: [..._nonempty]}
#ImpactPlan: {
	regenerate: [...#NodeImpact]
	rerender: [...#NodeImpact]
	rebind: [...#NodeImpact]
	review_again: [...#NodeImpact]
	reuse: [..._nonempty]
	hold: [...#NodeImpact]
	removed: [..._nonempty]
}
#DocumentDelta: {
	words_changed: [...string], provenance_changed: [...string], added: [...string], removed: [...string]
	regenerated_leaves: int & >=0, leaf_total: int & >=0, model_calls: int & >=0, bytes_changed: int & >=0, bytes_total: int & >=0
}
#RenderedDocument: {
	markdown:        string
	markdown_digest: _digest
	document_digest: _digest
	span_map: [...{node_id: _nonempty, start: int & >=0, end: int & >=0}]
}

// Private model response for session documents. Go derives IDs, byte offsets,
// status and review; a label is valid only as a heading's single segment (Go checks).
#DocumentSegment: {
	kind: "assertion" | "label"
	text: string & !=""
	fact_ids: [...string]
	contradictions: [...string]
	reason: string
}
#DocumentResponse: {segments: [#DocumentSegment, ...#DocumentSegment]}
