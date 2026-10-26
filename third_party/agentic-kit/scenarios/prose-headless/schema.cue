package proseheadless

import (
	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit:governedprosekit"
)

// Wire shapes of the headless composition. Document, session, profile and plan
// shapes belong to governed-prose-kit; HITL and Assignment Kit own theirs. Law
// (authority, staleness, closure, truth, delivery) is enforced by Go at entry points.
_nonempty: string & !=""
_digest:   string & =~"^[0-9a-f]{64}$"
_time:     string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T"

#Question: {
	id:              _nonempty
	kind:            "conflict" | "ambiguity" | "missing"
	subject:         _nonempty
	subject_version: _nonempty
	document_digest: _digest
	evidence: [_nonempty, ..._nonempty]
	prompt:   _nonempty
	required: bool
}
#Answer: {
	question_id:  _nonempty
	outcome:      "resolve" | "correct" | "annotate" | "defer" | "dispute" | "unknown"
	text:         string
	chosen:       string
	new_value:    string
	seen_version: string
}
#Record: {question: #Question, answer: #Answer, actor: _nonempty, at: _time}
#CriterionObservation: {
	goal_id:        _nonempty
	subject_digest: _digest
	policy_epoch:   _nonempty
	outcome:        "pass" | "fail" | "unknown" | "not-applicable"
	method:         "deterministic" | "heuristic" | "empirical" | "human"
	reason:         _nonempty
	artifact_digests: [_digest, ..._digest]
	observed_at: _time
	valid_until: _time
}
#Destination: {kind: "local" | "remote", account: _nonempty}
#Policy: {
	epoch:  _nonempty
	tenant: _nonempty
	purposes: [_nonempty, ..._nonempty]
	destinations: {[_nonempty]: #Destination}
	approver_role: _nonempty
	shared_credentials: [..._nonempty] | null
}
#PublishOp: {
	document_digest: _digest
	markdown_digest: _digest
	session_digest:  _digest
	profile_digest:  _digest
	evidence_digest: _digest
	policy_epoch:    _nonempty
	tenant:          _nonempty
	purpose:         _nonempty
	destination:     _nonempty
	account:         _nonempty
	not_after:       _time
}
#VariantSpec: {id: _nonempty, audience: string, voice: string, technical_level: string}
#VariantMetrics: {usage_state: "unqualified", model_calls: int & >=0, input_tokens: int & >=0, output_tokens: int & >=0, leaf_total: int & >=0, bytes_total: int & >=0}
#VariantResult: {
	id:          _nonempty
	status:      "ready" | "failed" | "held-snapshot-changed" | "skipped"
	err:         string
	version:     prose.#DocumentVersion | null
	metrics:     #VariantMetrics
	group_ready: bool
}
#BatchResult: {
	snapshot: string
	mode:     "independent" | "all-required"
	variants: [...#VariantResult]
	group_status: "ready" | "partial" | "failed" | "held"
	reasons: [...string]
}

#QueueReport: {
	open: [...#Question] | null
	overflow: [...#Question] | null
	held: [...#Record] | null
	closed: [...#Record] | null
	dropped_advisory: int & >=0
	stopped:          bool
}
#ProjectionDelta: {numerator: int & >=0, denominator: int & >=0, added: [...string] | null, rebound: [...string] | null, review_only: [...string] | null, removed: [...string] | null}
#ApplyReport: {corrected: [...string] | null, annotated: [...string] | null, unapplied: [...string] | null}
