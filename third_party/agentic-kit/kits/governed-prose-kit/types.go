// Package governedprosekit wires together the prose governance foundations
// into a single generation→review→regeneration pipeline.
//
// The pipeline composes:
//   - foundations/security/pre-llm-dlp-gate  (redact input before LLM)
//   - foundations/prose/prose-policy         (DLP posture evaluation)
//   - foundations/llmclient                  (LLM interface, injected)
//   - foundations/prose/prose-as-code        (structured prose segments)
//   - foundations/prose/traceable-prose      (provenance + regeneration marking)
//   - foundations/prose/evidence-citation    (deterministic citation strings)
//   - foundations/review/concern-flags       (raise policy violations)
//   - foundations/review/correction-ledger   (record and replay corrections)
//   - foundations/quality/eval-gate          (publishability gate)
//
// Concurrency: Pipeline is NOT safe for concurrent use; it holds mutable
// state (correction ledger, regeneration log) and callers must not share a
// Pipeline across goroutines without external synchronisation.
package governedprosekit

import (
	traceableprose "github.com/geoffmilleraz/agentic-kit/foundations/prose/traceable-prose"
	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
	concernflags "github.com/geoffmilleraz/agentic-kit/foundations/review/concern-flags"
)

// Request is the input to Pipeline.Generate.
type Request struct {
	// Topic is the subject matter for the prose to be generated.
	// Must be non-empty.
	Topic string

	// Audience names the target audience tier (e.g. "executive", "technical-internal").
	// Informational only; surfaced in generated nodes and citations.
	Audience string

	// EvidenceRefs are the identifiers of the evidence spans that support the
	// generation. Passed through to the generated TraceableNodes and used by
	// evidence-citation to produce deterministic citation strings.
	EvidenceRefs []string

	// SystemPrompt is an optional system prompt injected into the LLM Request.
	SystemPrompt string
}

// Result is the output of Pipeline.Generate.
type Result struct {
	// Nodes holds the governed, traceable prose nodes produced by the pipeline.
	// Each node carries its evidence citations, redaction state, and provenance.
	Nodes []traceableprose.TraceableNode

	// Citation is the deterministic evidence citation string derived from all
	// EvidenceRefs across all nodes (produced by evidencecitation.Cite).
	Citation string

	// ConcernFlags is the set of policy or quality concerns raised against
	// the generated output. An empty slice means no concerns were found.
	ConcernFlags []concernflags.ConcernFlag

	// GateResult is the eval-gate outcome; Result.GateResult.Passed must be
	// true before the output is considered publishable.
	GateResult evalgate.Result

	// RawText is the raw LLM output before it was parsed into nodes, kept for
	// debugging and audit.
	RawText string
}

// CorrectionInput is the argument to Pipeline.Regenerate. It carries a
// correction that was recorded in the CorrectionLedger and a reference to
// the nodes that need to be regenerated.
type CorrectionInput struct {
	// CorrectionID is a stable identifier for this correction event; used as
	// the TriggeredBy value in the RegenerationLog.
	CorrectionID string

	// AffectedNodeIDs are the node IDs that are invalidated by the correction
	// and must be re-rendered.
	AffectedNodeIDs []string

	// NewText is the corrected text to substitute for affected nodes.
	// For simplicity the kit re-uses a single replacement text for all affected
	// nodes; richer callers can extend this by using multiple CorrectionInputs.
	NewText string
}

// RegenerationResult is the output of Pipeline.Regenerate.
type RegenerationResult struct {
	// Nodes is the updated node slice after regeneration.
	// Unaffected nodes are returned unchanged; affected nodes carry the new text.
	Nodes []traceableprose.TraceableNode

	// MarkedCount is the number of nodes that were found and re-rendered.
	MarkedCount int
}
