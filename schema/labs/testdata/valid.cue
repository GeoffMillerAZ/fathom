package labs

// Section 4.3's first seed card, on a public fixture.
lab: #Lab & {
	id: "lab-find-symbol"
	features: ["F-01"]
	surface: ["mcp:find_symbol"]
	firing: {
		should:     "the next lookup is the definition of an identifier defined in the repository"
		should_not: "a string literal, config value or log line"
		grader:     "tool_used"
	}
	helped: {
		metric:                     "calls and tokens to the gold definition"
		arm:                        "plugin-minus-find-symbol"
		direction:                  "decrease"
		minimum_effect_of_interest: 1
	}
	quality: [{instrument: "indexbench L0", reading: "definition recall"}]
	fixtures: ["internal/connector/testdata/fauxtown"]
	harness: ["replay", "fork"]
	kill: [{criterion: "firing improves but helped is harmed", reading: "K4 paired helped reading"}]
	knobs: [{name: "description", kind: "text", location: "internal/mcp"}]
	owners: []
}

scenario: #Case & {
	id:                "find-definition"
	version:           1
	provenance:        "synthetic-adversarial"
	decisive_eligible: false
	input: {query: "where is Handler defined?"}
	expected: {tool: "find_symbol"}
	rubric: {criteria: [{name: "tool", weight: 1, predicate: {kind: "contains", arg: "find_symbol"}}]}
	lab:         "lab-find-symbol"
	should_fire: true
	split:       "train"
	regression:  true
	observation: "O1"
	theme:       "T01"
	derivation:  "Rebuilt on the public fixture."
}

result: #Result & {
	run_id:  "find-symbol-v1"
	task_id: "find-definition"
	arm:     "replay"
	trials:  3
	evaluation: {criterion_id: "firing", verdict: "inconclusive", reason: "counts only"}
	surface_unused: false
	lab:            "lab-find-symbol"
	feature:        "F-01"
	stages: {
		eligible: {value: 1, interval: [0.4385, 1], n: 3}
		fired: {value: 1, interval: [0.4385, 1], n: 3}
		delivered: "not measured: transcript unavailable"
		acted:     "not measured: no acted-on instrument"
		helped:    "not measured: replay has no arm"
	}
	quality: [{instrument: "indexbench L0", reading: "definition recall"}]
	pass_k: 1
	pins: {plugin_digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", stanza_sha: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", description_sha: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	prereg_sha:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	spend_usd:   0
	spend_label: "labs-replay"
	attributes: {"gen_ai.tool.name": "find_symbol"}
}
