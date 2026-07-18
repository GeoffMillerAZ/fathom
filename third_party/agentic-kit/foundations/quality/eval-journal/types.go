// Package evaljournal provides the append-only truth record of every judge
// call in an eval loop. It is a record type plus persistence — not a journal
// engine, not a scoring engine, not a correction facility.
//
// # Record shape
//
// An EvalRecord captures the full context of one judge invocation: which run,
// which scenario, which producer generated the output, which judge evaluated
// it, the verdict, and the rationale. The decisive field signals whether the
// record contributes to aggregate statistics; an empty rationale MUST pair
// with decisive=false (enforced by Validate and Append).
//
// # Persistence
//
// Append(w, rec) validates and writes one record to any io.Writer as a single
// JSONL line. Load(r) reads a JSONL stream back into a []Record slice. Both
// use foundations/persistence/jsonl-log exactly as investigation-journal does.
//
// # Corrections
//
// Corrections (marking records INVALID, annotating baselines) are NOT
// re-implemented here. When needed, apply foundations/review/correction-ledger
// over run_id as a merge-map; the ledger's INVALID annotation supersedes any
// record from this package.
//
// # Marshal stability
//
// Records are marshalled with sorted JSON keys (the default for encoding/json
// structs). The field ordering of Record is the canonical wire order and
// matches schema.cue. Do not reorder fields in the struct without updating
// golden test fixtures.
//
// # Concurrency contract
//
// Append is safe for concurrent use insofar as the underlying io.Writer is:
// jsonl-log's Writer wraps one with a mutex; bare io.Writers are the caller's
// problem. Load is stateless; call it concurrently on independent readers
// freely. The Record type itself is immutable after construction; share freely.
package evaljournal

import "time"

// Verdict enumerates the permissible judge outcomes. The set is closed; use
// the named constants and switch exhaustively where outcomes matter.
type Verdict string

const (
	// VerdictPass: the judge found the output correct.
	VerdictPass Verdict = "pass"
	// VerdictFail: the judge found the output incorrect.
	VerdictFail Verdict = "fail"
	// VerdictAbsentProven: the judge found absence proven (requires a complete
	// CoverageManifest in the supporting evidence — this package does not
	// validate that contract; the evidence-ladder foundation does).
	VerdictAbsentProven Verdict = "absent_proven"
	// VerdictAbstain: the judge declined to render a verdict.
	VerdictAbstain Verdict = "abstain"
	// VerdictError: the judge encountered an error and could not produce a verdict.
	VerdictError Verdict = "error"
)

// Producer describes the component that generated the output under evaluation.
//
// CUE mirror: #EvalRecord.producer in schema.cue.
type Producer struct {
	// Model is the model ID of the producing component. Required.
	Model string `json:"model"`

	// PromptHash, when non-empty, is a hash of the prompt template used.
	PromptHash string `json:"prompt_hash,omitempty"`

	// ParamsHash, when non-empty, is a hash of the inference parameters used.
	ParamsHash string `json:"params_hash,omitempty"`
}

// Judge describes the judge that evaluated the output.
//
// CUE mirror: #EvalRecord.judge in schema.cue.
type Judge struct {
	// Model is the model ID of the judge. Required.
	Model string `json:"model"`

	// Family is the family tag of the judge model (org-defined, opaque). Required.
	// Used for independence validation (see modelcatalog.ValidateIndependence).
	Family string `json:"family"`
}

// Trace carries optional distributed-tracing identifiers for the call that
// produced this record.
//
// CUE mirror: #EvalRecord.trace in schema.cue.
type Trace struct {
	// RunID is an optional tracing run/trace ID.
	RunID string `json:"run_id,omitempty"`

	// SpanID is an optional tracing span ID.
	SpanID string `json:"span_id,omitempty"`
}

// Record is a single, immutable eval-journal entry. Append validates and
// writes one; Load reads many back.
//
// CUE mirror: #EvalRecord in schema.cue.
type Record struct {
	// RunID groups all records from one evaluation run. Required.
	RunID string `json:"run_id"`

	// ScenarioID identifies the scenario under evaluation. Required.
	ScenarioID string `json:"scenario_id"`

	// Producer describes the component that generated the output. Required.
	Producer Producer `json:"producer"`

	// Judge describes the model that evaluated the output. Required.
	Judge Judge `json:"judge"`

	// Verdict is the outcome of this judge call.
	Verdict Verdict `json:"verdict"`

	// Decisive reports whether this verdict is final and usable in aggregate
	// statistics. An empty Rationale MUST imply Decisive=false.
	Decisive bool `json:"decisive"`

	// Score is an optional numeric quality score in [0, 1].
	Score *float64 `json:"score,omitempty"`

	// Rationale is the judge's explanation. An empty Rationale requires
	// Decisive=false (enforced by Validate).
	Rationale string `json:"rationale"`

	// EvidenceRefs are identifiers pointing to evidence that informed the verdict.
	EvidenceRefs []string `json:"evidence_refs"`

	// Trace carries optional distributed-tracing IDs.
	Trace Trace `json:"trace"`

	// OccurredAt is the RFC3339 timestamp at which this record was created.
	// Injected from the caller's clock by callers that use the high-level
	// Append function; set directly when constructing Records for tests.
	OccurredAt time.Time `json:"occurred_at"`
}
