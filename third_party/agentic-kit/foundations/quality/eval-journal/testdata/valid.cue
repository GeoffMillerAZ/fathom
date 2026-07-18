// Package evaljournal — valid test fixtures that must unify with schema.cue.
// Run `cue vet ./...` from the package root or `cue vet ./testdata/` to
// validate these against the schema.
package evaljournal

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-journal:evaljournal"

// validRecordPass is a well-formed passing record with all required fields.
validRecordPass: evaljournal.#EvalRecord & {
	run_id:      "run-abc123"
	scenario_id: "scenario-001"
	producer: {
		model:       "producer-model-a"
		prompt_hash: "sha256:aabbcc"
	}
	judge: {
		model:  "judge-model-b"
		family: "family-b"
	}
	verdict:       "pass"
	decisive:      true
	rationale:     "Output contains required keywords and follows format."
	evidence_refs: ["span://file.go#L1-L10"]
	trace: {
		run_id:  "trace-001"
		span_id: "span-abc"
	}
	occurred_at: "2026-07-08T00:00:00Z"
}

// validRecordFail is a well-formed failing record.
validRecordFail: evaljournal.#EvalRecord & {
	run_id:        "run-abc123"
	scenario_id:   "scenario-002"
	producer: {model: "producer-model-a"}
	judge: {
		model:  "judge-model-b"
		family: "family-b"
	}
	verdict:       "fail"
	decisive:      true
	score:         0.3
	rationale:     "Output missing required section."
	evidence_refs: []
	trace:         {}
	occurred_at:   "2026-07-08T01:00:00Z"
}

// validRecordAbstain is a valid abstain record with empty rationale and decisive=false.
validRecordAbstain: evaljournal.#EvalRecord & {
	run_id:      "run-abc123"
	scenario_id: "scenario-003"
	producer: {model: "producer-model-a"}
	judge: {
		model:  "judge-model-b"
		family: "family-b"
	}
	verdict:       "abstain"
	decisive:      false
	rationale:     ""
	evidence_refs: []
	trace:         {}
	occurred_at:   "2026-07-08T02:00:00Z"
}

// validRecordError is a valid error record; decisiveness is false, rationale empty.
validRecordError: evaljournal.#EvalRecord & {
	run_id:      "run-abc123"
	scenario_id: "scenario-004"
	producer: {model: "producer-model-a"}
	judge: {
		model:  "judge-model-b"
		family: "family-b"
	}
	verdict:       "error"
	decisive:      false
	rationale:     ""
	evidence_refs: []
	trace:         {}
	occurred_at:   "2026-07-08T03:00:00Z"
}

// validRecordAbsentProven exercises the absent_proven verdict.
validRecordAbsentProven: evaljournal.#EvalRecord & {
	run_id:      "run-xyz789"
	scenario_id: "scenario-005"
	producer: {
		model:       "producer-model-c"
		params_hash: "sha256:ddeeff"
	}
	judge: {
		model:  "judge-model-d"
		family: "family-d"
	}
	verdict:       "absent_proven"
	decisive:      true
	score:         1.0
	rationale:     "Exhaustive Tier-2 search over declared scope found zero occurrences."
	evidence_refs: ["manifest://coverage-abc"]
	trace: {span_id: "span-xyz"}
	occurred_at: "2026-07-08T04:00:00Z"
}
