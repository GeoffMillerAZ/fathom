package governedprosekit

config: #SourcedConfig & {
	policy_version: "policy/v1"
	prompt_version: "prompt/v1"
	model:          "fixture-generator"
	limits: {max_facts: 256, max_units: 256, max_claims: 512, max_bytes: 1048576, max_tokens: 8192}
}
draft: #ProseDraft & {
	claims: [{id: "c1", assertion: "12", fact_ids: ["f1"], contradictions: []}]
	units: [{id: "u1", text: "12", claims: [{claim_id: "c1", start: 0, end: 2}]}]
}
derived: #Derivation & {version: "arithmetic/v1", operation: "difference", inputs: ["current", "prior"]}
