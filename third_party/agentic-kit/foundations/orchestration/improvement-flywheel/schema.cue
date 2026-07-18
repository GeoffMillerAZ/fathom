// Package improvementflywheel defines the CUE schema for flywheel types.
// These constraints are evaluated with `cue vet ./...`.
package improvementflywheel

// #PatternType enumerates the kinds of repeated signal.
#PatternType: "false_positive" | "missed_claim" | "repeated_correction" | "operator_friction"

// #ChangeTarget enumerates the platform artifact types a proposal may target.
#ChangeTarget: "catalog_entry" | "prose_policy" | "prompt_section" | "cue_constraint" | "foundation_behavior"

// #Signal is a single workflow-outcome event that feeds the flywheel.
#Signal: {
	type:        #PatternType
	key:         string & !=""
	workflow_id: string
	detail:      string
}

// #PatternObservation groups repeated signals into a named observation.
#PatternObservation: {
	type:         #PatternType
	key:          string & !=""
	frequency:    int & >=1
	workflow_ids: [...string]
	examples:     [...string]
}

// #Proposal is a structured improvement suggestion derived from observations.
#Proposal: {
	id:                       string & !=""
	observation_key:          string & !=""
	pattern_type:             #PatternType
	change_target:            #ChangeTarget
	summary:                  string & !=""
	frequency:                int & >=1
	supporting_workflow_ids:  [...string]
}
