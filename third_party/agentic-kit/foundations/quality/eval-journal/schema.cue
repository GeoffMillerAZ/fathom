// Package evaljournal defines the CUE-first contract for the eval-journal
// foundation. Go types in types.go mirror these definitions; any drift is a
// bug. These constraints are evaluated with `cue vet ./...`.
package evaljournal

// #Verdict enumerates the permissible judge outcomes.
#Verdict: "pass" | "fail" | "absent_proven" | "abstain" | "error"

// #EvalRecord is the append-only truth of every judge call. Fields mirror
// types.go exactly; any structural drift is a bug.
//
// Key invariant (mirrored in Go): an empty Rationale MUST pair with
// decisive=false. A decisive=true record with an empty rationale is invalid
// because no human or automated consumer can act on an unexplained verdict.
#EvalRecord: {
	// run_id groups all records produced by one evaluation run.
	run_id: string & !=""

	// scenario_id identifies the scenario under evaluation.
	scenario_id: string & !=""

	// producer describes the model and optional hash-based identifiers of the
	// component that generated the output being evaluated.
	producer: {
		model:        string & !=""
		prompt_hash?: string
		params_hash?: string
	}

	// judge describes the model and family used to produce the verdict.
	judge: {
		model:  string & !=""
		family: string & !=""
	}

	// verdict is the outcome of this judge call.
	verdict: #Verdict

	// decisive indicates whether this verdict is considered final and usable in
	// aggregate statistics. An empty rationale MUST imply decisive=false.
	decisive: bool

	// score is an optional numeric score in [0, 1].
	score?: number & >=0 & <=1

	// rationale is the judge's explanation. An empty rationale implies
	// decisive=false (enforced in Go).
	rationale: string

	// evidence_refs are identifiers pointing to evidence that informed the verdict.
	evidence_refs: [...string]

	// trace carries optional distributed-tracing identifiers.
	trace: {
		run_id?:  string
		span_id?: string
	}

	// occurred_at is the RFC3339 timestamp at which this record was created,
	// injected from the caller's clock.
	occurred_at: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"
}
