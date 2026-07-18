// valid.cue — concrete values that satisfy the #Posture, #PostureRule,
// #Policy, and #Decision constraints defined in schema.cue. Running
// `cue vet ./...` from the package root validates these against the schema.
package prosepolicy

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/prose-policy:prosepolicy"

// All four posture values are valid.
_p1: prosepolicy.#Posture & "open"
_p2: prosepolicy.#Posture & "internal"
_p3: prosepolicy.#Posture & "restricted"
_p4: prosepolicy.#Posture & "secret"

// An open policy with no rules.
_openPolicy: prosepolicy.#Policy & {
	Posture: "open"
	Rules:   []
}

// A restricted policy with a rule that activates at the "restricted" level.
_restrictedPolicy: prosepolicy.#Policy & {
	Posture: "restricted"
	Rules: [
		{
			Threshold: "restricted"
			Terms: ["CONFIDENTIAL", "project-codename"]
		},
	]
}

// A secret policy with layered rules.
_secretPolicy: prosepolicy.#Policy & {
	Posture: "secret"
	Rules: [
		{
			Threshold: "internal"
			Terms: ["internal-only"]
		},
		{
			Threshold: "secret"
			Terms: ["EYES ONLY", "key-material"]
		},
	]
}

// A decision where the segment was allowed.
_allowedDecision: prosepolicy.#Decision & {
	Allowed:    true
	Redactions: []
}

// A decision where redaction was triggered.
_redactedDecision: prosepolicy.#Decision & {
	Allowed:    false
	Redactions: ["CONFIDENTIAL"]
}
