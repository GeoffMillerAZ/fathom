// Package expresslane — valid test data that must unify with schema.cue.
package expresslane

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/express-lane:expresslane"

// validFastDecision — a task that passed all gates and was routed to the fast lane.
validFastDecision: expresslane.#Decision & {
	task_id: "task-001"
	lane:    "fast"
	reason:  "eligible"
}

// validFullDecision_SizeExceeded — routed to full pipeline due to payload size.
validFullDecision_SizeExceeded: expresslane.#Decision & {
	task_id: "task-002"
	lane:    "full"
	reason:  "size_exceeded"
}

// validFullDecision_SpecIncomplete — routed to full pipeline due to incomplete spec.
validFullDecision_SpecIncomplete: expresslane.#Decision & {
	task_id: "task-003"
	lane:    "full"
	reason:  "spec_incomplete"
}

// validConfig — a config with a custom payload threshold.
validConfig: expresslane.#Config & {
	max_payload_bytes: 32768
}

// validTaskSpec — a complete spec.
validTaskSpec: expresslane.#TaskSpec & {
	description:           "Summarise the Q4 revenue report"
	estimated_complexity:  3
	required_capabilities: ["summarisation"]
}

// validLanes — all lane values accepted.
_l1: expresslane.#Lane & "fast"
_l2: expresslane.#Lane & "full"

// validReasons — all reason values accepted.
_r1: expresslane.#Reason & "size_exceeded"
_r2: expresslane.#Reason & "spec_incomplete"
_r3: expresslane.#Reason & "eligible"
