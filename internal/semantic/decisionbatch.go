package semantic

// decisionbatch.go — DP-22 (O109, docs/plans/decision-port.md): how many
// decision.Question a single Decide call may carry, per Decider, and the
// data-only failure record both chunked call sites (PrefilterSections,
// rerankWithinTiers) use to report a hard Decide error to the caller —
// internal/semantic stays free of the decide.Recorder type itself (as
// today; see decisionrank.go's own doc comment on the same discipline), so
// pkg/client turns each DecisionError into one decide.Recorder.RecordError
// row.
//
// O109's live finding: the obligations re-rank sent ONE batched noul
// request of ~160 candidate claims against the kit's "llm" adapter
// (OpenRouter, openai/gpt-6-luna); the reply truncated before it parsed as
// JSON, the whole batch's Decide call errored, and nothing ranked — a real,
// billed round trip spent for nothing. The compile pre-filter's own
// prefilterChunkSize (20) already chunks the SAME adapter without incident,
// so the fix is to give the re-rank the same discipline, through one shared
// rule both sites read.
import (
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// maxQuestionser is the optional interface a decision.Decider MAY implement
// to name its own per-call batch ceiling directly: MaxQuestions() returns
// the largest number of decision.Question one Decide call should carry at
// once, 0 meaning unbounded (never chunk). The kit does not define this
// interface as of this writing — no shipped Decider implements it —
// decisionBatchSize is written against the interface, not any concrete kit
// type, so the day a real adapter grows a genuine MaxQuestions() method
// (the motivating case: a future TypeSafe AI System One adapter, which
// evaluates a whole batch in parallel natively and bills the provider's
// state once per REQUEST rather than per question, so it must NEVER be
// chunked), decisionBatchSize picks it up automatically with NO CHANGE to
// this file.
type maxQuestionser interface {
	MaxQuestions() int
}

// decisionBatchSize returns how many decision.Question one Decide call
// against d should carry — the chunk size both PrefilterSections (the
// compile pre-filter) and rerankWithinTiers (the obligations re-rank) split
// their batches at, so the two sites share ONE rule rather than drifting
// independently.
//
// Resolution order:
//
//  1. d implements maxQuestionser: its own MaxQuestions() decides, 0
//     meaning unbounded (no chunking) — see the type's own doc comment.
//  2. d.Name() == "llm": prefilterChunkSize (20) — the kit's LLM adapter
//     renders a whole batch into one prompt and one completion; O109
//     measured a ~160-question batch fail to come back as valid JSON at
//     all, and the compile pre-filter already chunks this same adapter at
//     20 without incident.
//  3. Otherwise (an unrecognized Decider — including decision.None, and
//     any test double that does not opt in via case 1): 0, unbounded.
//     Never chunk a Decider this helper knows nothing about; a caller with
//     a small matched/section set pays no extra round trips either way.
func decisionBatchSize(d decision.Decider) int {
	if mq, ok := d.(maxQuestionser); ok {
		return mq.MaxQuestions()
	}
	if d.Name() == "llm" {
		return prefilterChunkSize
	}
	return 0
}

// DecisionError (O109, DP-22) is data describing one decision-port Decide
// call that hard-failed: no answer, no decision.Record, just an error —
// data only, so internal/semantic stays free of the decide.Recorder type
// (as today). The caller (pkg/client, which owns the Recorder) turns each
// of these into one decide.Recorder.RecordError row, so a failed decision
// leaves a trace in decisions.jsonl instead of pure silence.
type DecisionError struct {
	// Site identifies the call site ("compile_prefilter", "obligations") —
	// the same string Response.Records' own site argument uses.
	Site string
	// Provider is the failing Decider's own Name().
	Provider string
	// Questions is how many decision.Question this one failed call
	// carried (a chunk's own size, not the whole batch's).
	Questions int
	// Latency is how long the call took before it failed.
	Latency time.Duration
	// Err is the underlying error, verbatim — never a secret (the kit's
	// own adapters already keep API keys and raw provider bodies out of
	// their returned errors).
	Err error
}
