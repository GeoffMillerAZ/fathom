// Package expresslane defines the CUE schema for express-lane types.
// These constraints are evaluated with `cue vet ./...`.
package expresslane

// #Lane enumerates valid routing lanes.
#Lane: "fast" | "full"

// #Reason enumerates why a Lane was chosen.
#Reason: "size_exceeded" | "spec_incomplete" | "eligible"

// #TaskSpec carries the declarative specification of a task.
#TaskSpec: {
	description:           string
	estimated_complexity:  int & >=0
	required_capabilities: [...string]
}

// #Task is the unit of work assessed by the express-lane gate.
#Task: {
	id:      string & !=""
	payload: bytes
	spec:    #TaskSpec
}

// #Decision is the result of an Assess call.
#Decision: {
	task_id: string
	lane:    #Lane
	reason:  #Reason
}

// #Config controls the thresholds used by the gate.
#Config: {
	max_payload_bytes: int & >=1
}
