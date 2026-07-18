// valid.cue — concrete values that satisfy the #Event and #Kind constraints
// defined in schema.cue.  Running `cue vet ./...` from the package directory
// or the repo root validates these against the schema.
package event

import "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"

// A well-formed tool_call event with all required fields.
_toolCall: event.#Event & {
	ID:        "llmclient/ladder/1704067200000000000"
	Kind:      "tool_call"
	Source:    "llmclient/ladder"
	Timestamp: "2024-01-01T00:00:00Z"
	Attributes: {
		tool_name: "search"
		latency_ms: "142"
	}
}

// A well-formed llm_call event with no attributes (optional field absent).
_llmCall: event.#Event & {
	ID:        "orchestration/workflow-engine/1704067260000000000"
	Kind:      "llm_call"
	Source:    "orchestration/workflow-engine"
	Timestamp: "2024-01-01T00:01:00Z"
}

// A well-formed error event.
_errorEvent: event.#Event & {
	ID:        "security/injection-detector/1704067320000000000"
	Kind:      "error"
	Source:    "security/injection-detector"
	Timestamp: "2024-01-01T00:02:00Z"
	Attributes: {
		error_code: "INJECT_001"
		severity:   "high"
	}
}

// A well-formed info event.
_infoEvent: event.#Event & {
	ID:        "investigation/dad-investigator/1704067380000000000"
	Kind:      "info"
	Source:    "investigation/dad-investigator"
	Timestamp: "2024-01-01T00:03:00Z"
	Attributes: {
		phase: "decompose"
	}
}

// A trace-stamped event: the reserved trace.* correlation keys written by
// event.Apply are ordinary entries in the open #Attributes map (dotted keys
// are quoted in CUE).
_tracedEvent: event.#Event & {
	ID:        "orchestration/workflow-engine/1704067440000000000"
	Kind:      "tool_call"
	Source:    "orchestration/workflow-engine"
	Timestamp: "2024-01-01T00:04:00Z"
	Attributes: {
		tool_name:               "search"
		"trace.run_id":          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		"trace.span_id":         "bbbbbbbbbbbbbbbb"
		"trace.parent_span_id":  "cccccccccccccccc"
	}
}

// Verify all #Kind enum values are accepted.
_k1: event.#Kind & "tool_call"
_k2: event.#Kind & "llm_call"
_k3: event.#Kind & "error"
_k4: event.#Kind & "info"
