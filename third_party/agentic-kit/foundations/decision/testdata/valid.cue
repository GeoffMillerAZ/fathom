// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package directory validates
// these against the schema.
package decision

// A choice question over three named options.
_rootCause: #Question & {
	ID:           "root-cause"
	Kind:         "choice"
	Instructions: "what caused the stoppage?"
	Options: [
		{Name: "mechanical", Criteria: "gears, belts, bearings"},
		{Name: "electrical", Criteria: ""},
		{Name: "operator-error", Criteria: ""},
	]
	Min:           0
	Max:           0
	Rubric:        ""
	TrueCriteria:  ""
	FalseCriteria: ""
	Levels: []
}

// A score question on a 0..10 range.
_severity: #Question & {
	ID:           "severity"
	Kind:         "score"
	Instructions: "how severe is this on a 0-10 scale?"
	Options: []
	Min:           0
	Max:           10
	Rubric:        "0 = cosmetic, 10 = line-down"
	TrueCriteria:  ""
	FalseCriteria: ""
	Levels: []
}

// A score question using Levels instead of a bare Min/Max range.
_severityLevels: #Question & {
	ID:           "severity-levels"
	Kind:         "score"
	Instructions: "how severe is this?"
	Options: []
	Min:           0
	Max:           0
	Rubric:        ""
	TrueCriteria:  ""
	FalseCriteria: ""
	Levels: ["cosmetic", "minor", "moderate", "major", "line-down"]
}

// A predicate (yes/no probability) question.
_lineStopped: #Question & {
	ID:           "line-stopped"
	Kind:         "predicate"
	Instructions: "has the line stopped?"
	Options: []
	Min:           0
	Max:           0
	Rubric:        ""
	TrueCriteria:  "the belt is not moving"
	FalseCriteria: ""
	Levels: []
}

// A predicate question written with the decode alias "noul".
_restartSafe: #Question & {
	ID:           "restart-safe"
	Kind:         "noul"
	Instructions: "is it safe to restart the line without inspection?"
	Options: []
	Min:            0
	Max:            0
	Rubric:         ""
	TrueCriteria:   "no visible damage and grinding has stopped"
	FalseCriteria:  "any visible damage or persistent noise"
	Levels: []
}

// A Request bundling every question above against one State.
_request: #Request & {
	State: "widget on line 3 stopped moving after the motor made a grinding noise"
	Questions: [_rootCause, _severity, _severityLevels, _lineStopped, _restartSafe]
}

// A Request that also carries an image (Data is base64 in JSON).
requestWithImage: #Request & {
	State: "a photo of the stopped belt on line 3"
	Questions: [_lineStopped]
	Images: [{MediaType: "image/png", Data: "iVBORw0KGgo="}]
}

// Composition policies (#Policy). These are regular fields, not hidden
// ones, so `cue vet ./testdata/` also checks that every required field is
// present (a hidden field is checked for conflicts only).

// cache(cascade(cheap, fallback(a, b))): testdata/compose_policy.json, which
// TestComposeFromConfigBuildsCascadeFallbackCache composes.
composePolicy: #Policy & {
	cache: {
		name:             "judgment-cache"
		store:            "memory"
		template_version: "v1"
		observe:          "cache-log"
		inner: cascade: {
			name:           "tiers"
			escalate_below: 0.8
			observe:        "cascade-log"
			stages: [
				{provider: "cheap"},
				{fallback: {
					name:    "backup"
					observe: "fallback-log"
					deciders: [{provider: "a"}, {provider: "b"}]
				}},
			]
		}
	}
}

// The same nodes reordered: testdata/compose_policy_reordered.json.
composePolicyReordered: #Policy & {
	cache: {
		name:             "judgment-cache"
		store:            "memory"
		template_version: "v1"
		observe:          "cache-log"
		inner: cascade: {
			name:           "tiers"
			escalate_below: 0.8
			observe:        "cascade-log"
			stages: [
				{fallback: {
					name:    "backup"
					observe: "fallback-log"
					deciders: [{provider: "b"}, {provider: "a"}]
				}},
				{provider: "cheap"},
			]
		}
	}
}

// A recorder over a cost-ordered cascade that never escalates on
// confidence (escalate_below 0), and a replay of a named transcript.
recordPolicy: #Policy & {
	record: {
		name: "rec"
		inner: cascade: {
			order_by_cost:  true
			escalate_below: 0
			stages: [{provider: "cheap"}, {provider: "b"}]
		}
	}
}

replayPolicy: #Policy & {replay: {name: "offline", transcript: "run-1"}}
