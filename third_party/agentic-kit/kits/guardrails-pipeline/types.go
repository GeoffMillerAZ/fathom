// Package guardrailspipeline provides a declarative gate-chain that composes
// existing security and quality foundations into ordered pre-LLM and post-LLM
// decision pipelines.
//
// # Pre-LLM chain
//
// Stages (default order): wrap → inject-detect → classify → redact.
// Each stage may be skipped by omitting the corresponding dep (nil dep →
// StageResult{Allowed:true, Action:"skipped"}).
//
// # Post-LLM chain
//
// Stages (default order): integrity → honesty → dlp → disclaimer.
// Same nil-dep skip semantics apply.
//
// # Decision shape
//
// Both chains return a Decision. Allowed is false if any stage denied or if
// any stage returned an error (fail-closed). Transformed carries the
// cumulative text after all transformation stages (wrap, redact, disclaimer).
//
// # Concurrency
//
// PreLLMChain and PostLLMChain are safe for concurrent use: they hold no
// mutable state after construction, and all deps are themselves documented as
// safe for concurrent use.
package guardrailspipeline

import (
	"context"

	dlpevaluator "github.com/geoffmilleraz/agentic-kit/foundations/quality/dlp-evaluator"
	honestygate "github.com/geoffmilleraz/agentic-kit/foundations/quality/honesty-gate"
	injectiondetector "github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"
	prellmdlpgate "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate"
	sensitivityclassifier "github.com/geoffmilleraz/agentic-kit/foundations/security/sensitivity-classifier"
	untrustedcontentwrapper "github.com/geoffmilleraz/agentic-kit/foundations/security/untrusted-content-wrapper"
)

// Decision is the outcome of a chain Run call.
type Decision struct {
	// Allowed is true only when every stage allowed the text.
	Allowed bool `json:"allowed"`

	// Stages lists each stage's individual result, in execution order.
	Stages []StageResult `json:"stages"`

	// Transformed is the text after all transformation stages have been
	// applied (wrap, redact, disclaimer). When Allowed is false due to a
	// blocking stage, Transformed reflects transformations applied before
	// the blocking stage.
	Transformed string `json:"transformed"`

	// Err carries the first error encountered during chain execution (if any).
	// A non-nil Err always implies Allowed == false (fail-closed). Err is
	// deliberately excluded from JSON to avoid leaking internal details.
	Err error `json:"-"`
}

// StageResult is the outcome of one pipeline stage.
type StageResult struct {
	// Stage is the name of the stage (e.g. "wrap", "inject-detect").
	Stage string `json:"stage"`

	// Allowed is true when the stage permitted the text to continue.
	Allowed bool `json:"allowed"`

	// Action summarises what the stage did: "allowed", "denied", "skipped",
	// "transformed", or "error".
	Action string `json:"action"`

	// Findings is a list of human-readable strings describing what the stage
	// found (rule IDs, excerpts, violation details, etc.). Empty when nothing
	// was found or the stage was skipped.
	Findings []string `json:"findings"`

	// Severity is the highest severity/risk string encountered by this stage,
	// or "" if not applicable. The exact values depend on the stage
	// (e.g. "high", "critical", "confidential").
	Severity string `json:"severity"`
}

// EventSink receives audit events after each chain Run. Implement this
// interface to integrate with a telemetry or audit-log pipeline.
type EventSink interface {
	// Emit is called once per Run with the chain name ("pre-llm" or
	// "post-llm") and the full Decision. Emit must not block Run; if
	// asynchronous, the caller is responsible for synchronisation.
	Emit(ctx context.Context, chain string, d Decision)
}

// ClaimProvider derives a honestygate.Claim from model output text. Implement
// this interface to connect the honesty stage to a claim-extraction strategy
// appropriate for the application. If nil, the honesty stage is skipped.
type ClaimProvider interface {
	ClaimFor(text string) honestygate.Claim
}

// PreLLMConfig controls which stages run in the pre-LLM chain and their
// thresholds. The zero value is valid and uses all defaults.
type PreLLMConfig struct {
	// Stages is the ordered list of stage names to run. Valid values:
	// "wrap", "inject-detect", "classify", "redact". An empty slice runs
	// all four in the default order.
	Stages []string

	// InjectDetectThreshold is the minimum Severity string that causes the
	// inject-detect stage to deny. Default is "high".
	InjectDetectThreshold string

	// ClassifyThreshold is the minimum SensitivityLevel name that causes
	// the classify stage to deny. Default is "confidential".
	ClassifyThreshold string
}

// PostLLMConfig controls which stages run in the post-LLM chain and their
// thresholds. The zero value is valid and uses all defaults.
type PostLLMConfig struct {
	// Stages is the ordered list of stage names to run. Valid values:
	// "integrity", "honesty", "dlp", "disclaimer". An empty slice runs all
	// four in the default order.
	Stages []string

	// DLPThreshold is the minimum normalised score [0,1] that causes the
	// dlp stage to deny. Default is 0.67.
	DLPThreshold float64

	// Disclaimer is the text appended to Transformed by the disclaimer stage.
	// If empty, the disclaimer stage is a no-op pass-through.
	Disclaimer string
}

// PreLLMDeps holds the foundation instances used by the pre-LLM chain.
// Any nil dep causes its stage to be skipped.
type PreLLMDeps struct {
	Wrapper    *untrustedcontentwrapper.Wrapper
	Detector   *injectiondetector.Detector
	Classifier *sensitivityclassifier.Classifier
	Gate       *prellmdlpgate.Gate
	Sink       EventSink
}

// PostLLMDeps holds the foundation instances used by the post-LLM chain.
// Any nil dep causes its stage to be skipped (except IntegrityKnownRefs
// which defaults to an empty map).
type PostLLMDeps struct {
	// IntegrityKnownRefs maps citation keys to resolved values, passed to
	// integrityevaluator.Evaluate. A nil map is treated as empty (no known
	// refs, so all cited refs are fabricated).
	IntegrityKnownRefs map[string]string

	HonestyGate   *honestygate.Gate
	ClaimProvider ClaimProvider
	DLPEvaluator  *dlpevaluator.Evaluator
	Sink          EventSink
}
