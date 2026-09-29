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
