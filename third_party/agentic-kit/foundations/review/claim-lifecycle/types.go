// Package claimlifecycle provides an event-sourced lifecycle FSM for claims
// under review. Every state change is recorded as an immutable ClaimEvent in
// a hash-chained ClaimLedger; the current view is always derived by projection.
//
// Provenance invariants (CLAUDE.md §1–4, §6):
//   - Every event carries an Author (human or AI with full provenance).
//   - The ledger is append-only; no event is ever mutated or deleted.
//   - Timestamps are set by the ledger's injected clock (never inline time.Now()).
//   - ClaimView.Grade is *evidencegrade.ClaimGrade so FloorTier and Promoted
//     provenance are never discarded (invariant §4, §6).
//
// Concurrency: ClaimLedger is safe for concurrent use; all access is serialised by the underlying event-store's sync.RWMutex.
package claimlifecycle

import (
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ── States ────────────────────────────────────────────────────────────────────

// ClaimState is the current lifecycle state of a Claim.
type ClaimState string

const (
	// StateProposed is the initial state: the claim has been put forward and
	// awaits evaluation.
	StateProposed ClaimState = "proposed"

	// StateSupported means sufficient evidence has been gathered to support
	// the claim.
	StateSupported ClaimState = "supported"

	// StateRefuted means evidence has been found that directly contradicts
	// the claim.
	StateRefuted ClaimState = "refuted"

	// StateUncertain means evaluation is complete but evidence is insufficient
	// to either support or refute the claim.
	StateUncertain ClaimState = "uncertain"

	// StateSuperseded means the claim has been replaced by a newer claim.
	// This is a forward-terminal state: no transitions out of it.
	StateSuperseded ClaimState = "superseded"

	// StateRecanted means the claim was withdrawn by its author.
	// This is a forward-terminal state: no transitions out of it.
	StateRecanted ClaimState = "recanted"
)

// ── Event kinds ───────────────────────────────────────────────────────────────

// ClaimEventKind is the discriminator for the type of event stored in a
// ClaimLedger entry.
type ClaimEventKind string

const (
	// KindProposed opens a new claim or amends its text while still open.
	KindProposed ClaimEventKind = "proposed"

	// KindSupported transitions proposed → supported.
	KindSupported ClaimEventKind = "supported"

	// KindRefuted transitions proposed → refuted.
	KindRefuted ClaimEventKind = "refuted"

	// KindMarkedUncertain transitions proposed → uncertain.
	KindMarkedUncertain ClaimEventKind = "marked_uncertain"

	// KindReopened transitions supported|refuted|uncertain → proposed.
	KindReopened ClaimEventKind = "reopened"

	// KindSuperseded transitions supported|refuted → superseded (terminal).
	KindSuperseded ClaimEventKind = "superseded"

	// KindRecanted transitions any open state → recanted (terminal).
	KindRecanted ClaimEventKind = "recanted"

	// KindAssertionLinked attaches an assertion ID to an open claim.
	KindAssertionLinked ClaimEventKind = "assertion_linked"

	// KindEvidenceLinked attaches a graded evidence span to an open claim.
	KindEvidenceLinked ClaimEventKind = "evidence_linked"
)

// ── Payloads ──────────────────────────────────────────────────────────────────

// ProposedPayload carries the claim text for a KindProposed event.
type ProposedPayload struct {
	// Text is the human-readable statement being proposed.
	Text string `json:"text"`
}

// AssertionLinkedPayload carries the assertion reference for a
// KindAssertionLinked event.
type AssertionLinkedPayload struct {
	// AssertionID is the stable identifier of the assertion being linked.
	AssertionID string `json:"assertion_id"`
}

// EvidenceLinkedPayload carries the graded span reference for a
// KindEvidenceLinked event. The tier and grading attributes are denormalised
// here so the projection can compute a ClaimGrade without a separate span
// lookup (provenance §1: every record is self-contained).
type EvidenceLinkedPayload struct {
	// SpanID is the stable identifier of the evidence span.
	SpanID string `json:"span_id"`

	// Tier is the epistemic tier of the span (mirrors evidencegrade.Tier).
	Tier string `json:"tier"`

	// Confidence is the certainty of this span's support in [0, 1].
	Confidence float64 `json:"confidence"`

	// Necessary is true when the claim logically depends on this span.
	Necessary bool `json:"necessary"`

	// Independent is true when this span corroborates the claim from a source
	// that is epistemically independent of all other spans.
	Independent bool `json:"independent"`
}

// ── Event ─────────────────────────────────────────────────────────────────────

// ClaimEvent is one immutable fact appended to a ClaimLedger. The Kind field
// discriminates the payload union; only the field matching Kind is set.
//
// RecordedAt is populated from the event-store entry when replaying events;
// it is not set by callers.
type ClaimEvent struct {
	// Kind discriminates the event type.
	Kind ClaimEventKind

	// ClaimID is the stable identifier of the claim this event belongs to.
	ClaimID string

	// Author records who (human or AI) produced this event.
	Author aiprovenance.Author

	// RecordedAt is the wall-clock time at which this event was appended to
	// the ledger. Set during replay from the event-store entry; zero before
	// append.
	RecordedAt time.Time

	// Proposed is set for KindProposed events.
	Proposed *ProposedPayload

	// AssertionLinked is set for KindAssertionLinked events.
	AssertionLinked *AssertionLinkedPayload

	// EvidenceLinked is set for KindEvidenceLinked events.
	EvidenceLinked *EvidenceLinkedPayload
}
