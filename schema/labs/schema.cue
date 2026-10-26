// Package labs validates the file-only feature-lab contracts.
package labs

#Text:  string & =~"\\S"
#LabID: string & =~"^lab-[a-z0-9]+(-[a-z0-9]+)*$"
#SHA:   string & =~"^[0-9a-f]{64}$"
#Quality: [...{instrument!: #Text, reading!: #Text}]

#Lab: {
	id!: #LabID
	features!: [...#Text]
	surface!: [string & =~"^(mcp:|hook:|skill:|agent:|stanza:|decide:)\\S.*$", ...string & =~"^(mcp:|hook:|skill:|agent:|stanza:|decide:)\\S.*$"]
	firing!: {
		should!:     #Text
		should_not!: #Text
		grader!:     "tool_used" | "tool_order" | "skill_invoked" | "hook_context"
	} | {mode!: "n/a", reason!: #Text}
	helped!: {
		metric!: #Text
		// The arm names the plugin minus this feature; removal scope is
		// checked by LoadLabs, not inferred from the arm's prose here.
		arm!:                        #Text
		direction!:                  "increase" | "decrease"
		minimum_effect_of_interest!: number & >0
	}
	quality!: #Quality
	fixtures!: [#Text, ...#Text]
	harness!: [#Harness, ...#Harness]
	kill!: [{criterion!: #Text, reading!: #Text}, ...{criterion!: #Text, reading!: #Text}]
	knobs!: [...{name!: #Text, kind!: "text" | "cap" | "threshold" | "membership" | "default", location!: #Text}]
	owners!: [...#Text]
}

// Existing benches retain their instrument names rather than a new runner.
#Harness: "replay" | "fork" | "scenario" | "hook-path" | "decide" | "indexbench" | "reviewbench" | "contextbench" | "langbench"

// production-captured cases are refused at commit by the case loader
// (the kit's scenariobank.Validate), not by this schema.
#Case: {
	#Scenario
	id:                 string & =~"^[A-Za-z0-9][A-Za-z0-9._-]*$"
	provenance!:        #Provenance
	decisive_eligible!: bool
	input!: {...}
	lab!:         #LabID
	should_fire!: bool
	split!:       "train" | "select" | "confirm"
	regression!:  bool
	observation?: string & =~"^O[0-9]+$"
	theme?:       string & =~"^T[0-9]+$"
	derivation?:  #Text
	if provenance == "synthetic-adversarial" {decisive_eligible: false}
	if provenance == "production-derived-redacted" {derivation!: #Text}
}

#Stage: {
	value!: number & >=interval[0] & <=interval[1]
	interval!: [number, number]
	interval: [<=interval[1], >=interval[0]]
	n!: int & >0
} | (string & =~"^not measured: \\S.*$")

#Result: {
	run_id!:  #Text
	task_id!: #Text
	arm!:     #Text
	trials!:  int & >=0
	// LAB-12 maps the prereg three-outcome rule when writing a row:
	// confirmed -> pass, refuted -> fail, inconclusive -> indeterminate.
	evaluation!: {criterion_id!: #Text, verdict!: "pass" | "fail" | "indeterminate", reason!: #Text}
	surface_unused?:    bool
	instrument_defect?: #Text
	lab!:               #LabID
	feature!:           #Text
	stages!: {eligible!: #Stage, fired!: #Stage, delivered!: #Stage, acted!: #Stage, helped!: #Stage}
	quality!: #Quality
	pass_k!:  number & >=0 & <=1
	// RecordArmPins owns the arm's pin keys. These lab-specific pins are
	// required beside those keys, without closing the upstream record.
	pins!: {plugin_digest!: #SHA, stanza_sha!: #SHA, description_sha!: #SHA, ...}
	prereg_sha!:  #SHA
	spend_usd!:   number & >=0
	spend_label!: #Text
	attributes!: {[string]: _}
	// Optional for firing rows; decision rows carry the kit ReadingKey.
	reading_key?: {site!: #Text, template_version!: #Text, provider!: #Text, resolved_model!: #Text}
	tpr?:          #Stage
	tnr?:          #Stage
	coverage?:     #Stage
	abstention?:   #Stage
	brier?:        number & >=0 & <=1
	auroc?:        number & >=0 & <=1
	rogan_gladen?: #Stage
}

// Mirrored field for field: the kit is a nested CUE module with no local
// dependency mapping in fathom. TestLabScenarioMirror guards this copy.
// #Provenance enumerates the permissible provenance tags for a scenario. The
// choice of provenance tag carries PII-safety implications — see Go doc for
// details.
#Provenance: "production-captured" | "production-derived-redacted" | "synthetic-adversarial"

// #PredicateKind enumerates the interpretable predicate vocabulary. Predicates
// are compiled into qa-harness RubricCriteria by RubricJudgeFrom; they are
// never Go closures in committed data.
#PredicateKind: "contains" | "regex" | "json_path_eq" | "nonempty"

// #Criterion is one weighted rubric criterion with a declarative predicate.
#Criterion: {
	// name is a non-empty human-readable identifier for this criterion.
	name: string & !=""

	// weight is the relative importance. Must be strictly positive.
	weight: number & >0

	// predicate is the interpretable machine-checkable claim.
	predicate: {
		// kind selects the check algorithm.
		kind: #PredicateKind

		// arg is the primary argument (substring, regex pattern, JSON path, or
		// empty for "nonempty").
		arg: string

		// value is the expected value for json_path_eq predicates; optional
		// for other kinds.
		value?: string
	}
}

// #Rubric is an ordered set of weighted criteria that a RubricJudge evaluates.
#Rubric: {
	// criteria is the ordered list of evaluation dimensions. May be empty (a
	// rubric with no criteria scores 0 via qa-harness semantics).
	criteria: [...#Criterion]
}

// #Scenario is one versioned evaluation scenario. Provenance gates apply:
// production-captured scenarios are rejected in committed testdata.
//
// Key invariant: decisive_eligible must be false when provenance is
// synthetic-adversarial (enforced in Go by Validate).
#Scenario: {
	// id is a non-empty stable identifier for this scenario.
	id: string & !=""

	// version is a positive integer tracking the scenario revision.
	version: int & >=1

	// provenance records the origin of the scenario data.
	provenance: #Provenance

	// decisive_eligible indicates whether records against this scenario may be
	// marked decisive. Synthetic-adversarial scenarios MUST have this false.
	decisive_eligible: bool

	// input is the scenario's input payload. Shape is caller-defined.
	input: {...}

	// expected is an optional gold-standard expected output.
	expected?: {...}

	// rubric is an optional evaluation rubric for this scenario.
	rubric?: #Rubric
}
