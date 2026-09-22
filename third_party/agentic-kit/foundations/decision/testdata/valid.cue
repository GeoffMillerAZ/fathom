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
}

// A noul (yes/no probability) question.
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
}

// A Request bundling all three questions against one State.
_request: #Request & {
	State: "widget on line 3 stopped moving after the motor made a grinding noise"
	Questions: [_rootCause, _severity, _restartSafe]
}
