// CUE owns the offline lab's JSON shapes; Go mirrors the semantic constraints.
package decisionlab

import "list"

_id: =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"
// Require a non-whitespace rune, matching Go's strings.TrimSpace behavior.
_text: string & =~"[^\\s\\x{000b}\\x{0085}\\p{Z}]"

#PackOption: close({name: _id, criteria: _text})
#PackQuestion: close({
	id:           _id
	instructions: _text
	options: [...#PackOption] & list.MinItems(2) & list.MaxItems(16)
	_optionNames: [for o in options {o.name}] & list.UniqueItems()
	_hasUnknown: list.Contains(_optionNames, "unknown") & true
	favorableOptions: [...(_id & !="unknown" & or(_optionNames))] & list.MinItems(1) & list.UniqueItems()
})
#Pack: close({
	version:          "decision-lab.pack/1"
	id:               _id
	templateVersion:  _id
	confidenceRecipe: "decision.Confidence/1"
	readingRecipe:    "decision.NewReading/1"
	provider:         _id
	resolvedModel:    _id
	policyEpoch:      _id
	owner:            _id
	fallback:         _text
	baseline:         _text
	killCriterion:    _text
	questions: [...#PackQuestion] & list.MinItems(1) & list.MaxItems(32)
	_questionIDs: [for q in questions {q.id}] & list.UniqueItems()
})
#Input: close({
	version:       "decision-lab.input/1"
	caseId:        _id
	subjectId:     _id
	revision:      _id
	capturedState: _text
})
#PortRequest: close({
	State: _text
	Questions: [...close({
		ID:           _id
		Kind:         "choice"
		Instructions: _text
		Options: [...close({Name: _id, Criteria: _text})]
		Min:    number
		Max:    number
		Rubric: string
		Levels: [...string]
		TrueCriteria:  string
		FalseCriteria: string
	})]
})
#Prepared: close({
	version:         "decision-lab.prepared/1"
	caseId:          _id
	packFileDigest:  string & =~"^sha256:[0-9a-f]{64}$"
	inputFileDigest: string & =~"^sha256:[0-9a-f]{64}$"
	packDigest:      string & =~"^sha256:[0-9a-f]{64}$"
	inputDigest:     string & =~"^sha256:[0-9a-f]{64}$"
	requestKey:      string & =~"^sha256:[0-9a-f]{64}$"
	templateDigests: [_id]: string & =~"^sha256:[0-9a-f]{64}$"
	confidenceRecipe:  "decision.Confidence/1"
	noActionAuthority: true
	request:           #PortRequest
})
#PortAnswer: close({
	ID:     _id
	Kind:   "choice"
	Choice: _id
	Probabilities: [string]: number & >=0 & <=1
	Score?:     number
	Noul?:      number
	Confidence: number & >=0 & <=1
})
#PortResponse: close({
	Provider: _id
	Model:    _id
	Answers: [_id]:   #PortAnswer
	Invalid: [_id]:   string
	Declined?: [_id]: string
	Usage: close({InputTokens: int & >=0, OutputTokens: int & >=0})
	Latency: int & >=0
})
#Capture: close({
	version:      "decision-lab.capture/1"
	caseId:       _id
	packDigest:   string & =~"^sha256:[0-9a-f]{64}$"
	inputDigest:  string & =~"^sha256:[0-9a-f]{64}$"
	requestKey:   string & =~"^sha256:[0-9a-f]{64}$"
	attemptCount: int & >=0 & <=32
	answerCacheHits: [..._id] & list.UniqueItems()
	cause?:    "no-provider" | "timeout" | "not-authorized" | "budget-exhausted" | "transport-error"
	response?: #PortResponse
}) & ({
	cause:        "no-provider" | "not-authorized" | "budget-exhausted"
	attemptCount: 0
	answerCacheHits: []
	response?: _|_
} | {
	cause:        "timeout" | "transport-error"
	attemptCount: >=1
	answerCacheHits: []
	response?: _|_
} | {
	response: #PortResponse
	cause?:   _|_
})
#Captures: [...#Capture]
#ExpectedLabel: close({choice: _id, provenance: "human" | "computed" | "frontier"})
#ExpectedCase: close({input: #Input, independentUnitId: _id, labels: [_id]: #ExpectedLabel})
#Manifest: close({
	version:     "decision-lab.manifest/1"
	datasetKind: "synthetic" | "observed"
	packDigest:  string & =~"^sha256:[0-9a-f]{64}$"
	cases: [...#ExpectedCase] & list.MinItems(1) & list.MaxItems(64)
	_caseIDs: [for c in cases {c.input.caseId}] & list.UniqueItems()
	_unitIDs: [for c in cases {c.independentUnitId}] & list.UniqueItems()
})
