package practicereview

import (
	"list"
	catalogkit "github.com/geoffmilleraz/agentic-kit/kits/catalog-kit:catalogkit"
)

_text: string & !=""
_digest: string & =~"^[a-f0-9]{64}$"
_key: string & =~"^[A-Za-z][A-Za-z0-9._/-]{0,127}$"
_time: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"

#Practice: {
	stable_key: _key
	version: _text
	path: _text
	title: _text
	failure_mode: _text
	benefit: _text
	remedy: _text
	sources: [..._text] & [_, ...]
	negative_case: _text
	kind: "evidence-integrity" | "bounded-work" | "effect-scope"
	definition: _text
	if kind == "bounded-work" {max_steps: int & >0 & <=1000000}
}
#Catalog: {
	spec: catalogkit.#Spec
	practices: [...#Practice] & list.MinItems(1) & list.MaxItems(32)
}
#Pin: {
	stable_key: _key
	version: _text
	definition_digest: _digest
}
#ExpectedSubject: {
	id: _key
	revision: _text
	allowed_actions: [..._text]
	criteria: [..._key] & list.MinItems(1) & list.MaxItems(32)
}
#Scope: {
	version: 1
	criteria: [...#Pin] & list.MinItems(1) & list.MaxItems(32)
	subjects: [...#ExpectedSubject] & list.MinItems(1) & list.MaxItems(32)
}
#Evidence: {
	subject_id: _key
	source: _text
	revision: _text
	text: _text
	digest: _digest
	observed_at: _time
	valid_until: _time
}
#Check: {
	stable_key: _key
	evidence: #Evidence
	work?: {steps: int & >=0}
	requested_actions?: [..._text]
}
#Subject: {id: _key, revision: _text, checks: [...#Check] & list.MaxItems(32)}
#Subjects: {version: 1, subjects: [...#Subject] & list.MinItems(1) & list.MaxItems(32)}
