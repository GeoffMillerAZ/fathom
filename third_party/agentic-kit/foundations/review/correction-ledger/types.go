// Package correctionledger provides an append-only ledger for recording
// finding corrections and dismissals, with a finite state machine that
// governs the lifecycle of each finding through human review.
//
// Concurrency: Ledger is safe for concurrent use via sync.RWMutex.
package correctionledger

import "time"

// AuthorKind discriminates between a human reviewer and an AI model as the
// producer of a Correction or Dismissal.
type AuthorKind string

const (
	// AuthorKindHuman identifies a human-produced correction or dismissal.
	AuthorKindHuman AuthorKind = "human"
	// AuthorKindAI identifies an AI-produced correction or dismissal.
	AuthorKindAI AuthorKind = "ai"
)

// Author carries the provenance of the entity that produced a Correction or
// Dismissal. Throughline invariant 1 requires that every evidence span be
// traceable to its exact producer.
//
// For human authors, IdentityID is the reviewer's identity (e.g. email or UUID).
// For AI authors, IdentityID is the model identifier and AIModelID, AIPromptHash,
// AIParamsHash, and AIInputDigest record the full reproduction key.
type Author struct {
	// Kind discriminates between AuthorKindHuman and AuthorKindAI.
	Kind AuthorKind

	// IdentityID is the identity of the human reviewer or the model ID for AI.
	// Must be non-empty when Author is used.
	IdentityID string

	// AIModelID is the canonical model identifier (e.g. "claude-opus-4-8").
	// Populated only when Kind == AuthorKindAI.
	AIModelID string

	// AIPromptHash is the hash of the fully-rendered prompt sent to the model.
	// Optional for AI authors.
	AIPromptHash string

	// AIParamsHash is the hash of sampling parameters.
	// Optional for AI authors.
	AIParamsHash string

	// AIInputDigest is the hash of the evidence or content fed as input.
	// Optional for AI authors.
	AIInputDigest string
}

// State represents the lifecycle state of a Finding.
type State string

const (
	// StateOpen is the initial state: the finding has been surfaced and awaits review.
	StateOpen State = "open"

	// StateReviewing means a reviewer has claimed the finding and is actively evaluating it.
	StateReviewing State = "reviewing"

	// StateCorrected is a forward-terminal state: the underlying fact was wrong and a
	// correction has been applied. Once corrected, the finding cannot return to open.
	StateCorrected State = "corrected"

	// StateDismissed is a forward-terminal state: the finding was a false positive or
	// acceptable risk. Dismissal does NOT mutate the underlying fact.
	StateDismissed State = "dismissed"

	// StateDeferred means the concern is valid but not actionable now. A deferred
	// finding can be reopened, transitioning back to reviewing.
	StateDeferred State = "deferred"
)

// Event represents an action that triggers a state transition in the Finding FSM.
type Event string

const (
	// EventClaim transitions open → reviewing.
	EventClaim Event = "claim"

	// EventCorrect transitions reviewing → corrected.
	EventCorrect Event = "correct"

	// EventDismiss transitions reviewing → dismissed.
	EventDismiss Event = "dismiss"

	// EventDefer transitions reviewing → deferred.
	EventDefer Event = "defer"

	// EventReopen transitions deferred → reviewing.
	EventReopen Event = "reopen"
)

// FindingID is the unique identifier for a finding.
type FindingID = string

// Finding is a surfaced claim or concern requiring human review.
// It moves through the FSM via Transition.
type Finding struct {
	// ID is a stable, unique identifier for this finding.
	ID FindingID

	// State is the current lifecycle state.
	State State

	// Description is a human-readable summary of the finding.
	Description string

	// CreatedAt is when the finding was first surfaced.
	CreatedAt time.Time

	// UpdatedAt is the last time the finding state was changed.
	UpdatedAt time.Time
}

// CorrectionTarget carries a stable versioned identifier for the entity
// being corrected, enabling replay semantics: given the same StableTarget
// and the same correction event sequence, the current fact can be
// reconstructed without re-reading the original source.
type CorrectionTarget struct {
	// StableTarget is the versioned identifier for the entity being corrected
	// (e.g., a table field, document section, or claim ID).
	StableTarget string
}

// Correction records that an underlying fact was wrong and has been updated.
// A Correction triggers targeted prose regeneration for all nodes that used
// the corrected fact. It carries full provenance for auditability.
type Correction struct {
	// FindingID identifies the finding this correction resolves.
	FindingID FindingID

	// Target identifies the versioned entity whose fact was corrected.
	Target CorrectionTarget

	// Original is the value of the fact before correction.
	Original string

	// Corrected is the new, correct value of the fact.
	Corrected string

	// CorrectedBy is the identity of the person or system that applied the correction.
	CorrectedBy string

	// CorrectedAt is when the correction was applied.
	CorrectedAt time.Time

	// Reason explains why the original value was wrong.
	Reason string

	// Confidence is the corrector's certainty about the new value, on a scale of 1–5.
	// 1 = low confidence, 5 = high confidence.
	Confidence int

	// Author optionally carries the full provenance of who or what produced this
	// correction (Throughline invariant 1). Nil means not recorded (permitted for
	// backwards compatibility; new callers should always supply Author).
	Author *Author

	// SourceHash is the content hash of the source document that supports this
	// correction. Optional; used for click-to-source traceability.
	SourceHash string

	// Locator is a URI/offset reference pointing to the exact location in the
	// source document. Optional (e.g. "doc://section/3#para2").
	Locator string
}

// Dismissal records that a finding was a false positive or an acceptable risk.
// A Dismissal NEVER mutates the underlying fact and does NOT trigger regeneration.
// It carries the reviewer's rationale for auditability.
type Dismissal struct {
	// FindingID identifies the finding being dismissed.
	FindingID FindingID

	// DismissedBy is the identity of the reviewer who dismissed the finding.
	DismissedBy string

	// Reason explains why the finding is considered a false positive or acceptable risk.
	Reason string

	// DismissedAt is when the dismissal was recorded.
	DismissedAt time.Time

	// Author optionally carries the full provenance of who or what produced this
	// dismissal (Throughline invariant 1). Nil means not recorded (permitted for
	// backwards compatibility; new callers should always supply Author).
	Author *Author
}

// LedgerEventKind discriminates the type of event stored in the ledger.
type LedgerEventKind string

const (
	// LedgerEventKindFindingOpened records that a new finding was opened.
	LedgerEventKindFindingOpened LedgerEventKind = "finding_opened"
	// LedgerEventKindTransition records a finding's status transition.
	LedgerEventKindTransition LedgerEventKind = "transition"
	// LedgerEventKindCorrectionAdded records that a correction was appended
	// to a finding.
	LedgerEventKindCorrectionAdded LedgerEventKind = "correction_added"
	// LedgerEventKindDismissalAdded records that a dismissal was appended to
	// a finding.
	LedgerEventKindDismissalAdded LedgerEventKind = "dismissal_added"
)

// LedgerEvent is the sealed union type stored in the append-only ledger.
// Exactly one of the payload fields will be non-zero depending on Kind.
type LedgerEvent struct {
	// Kind identifies which payload field is populated.
	Kind LedgerEventKind

	// OccurredAt is the wall-clock time the event was appended.
	OccurredAt time.Time

	// FindingOpened is populated when Kind == LedgerEventKindFindingOpened.
	FindingOpened *Finding

	// Transition is populated when Kind == LedgerEventKindTransition.
	Transition *TransitionEvent

	// Correction is populated when Kind == LedgerEventKindCorrectionAdded.
	Correction *Correction

	// Dismissal is populated when Kind == LedgerEventKindDismissalAdded.
	Dismissal *Dismissal
}

// TransitionEvent records an FSM state transition for a finding.
type TransitionEvent struct {
	// FindingID identifies the finding that transitioned.
	FindingID FindingID

	// Event is the FSM event that caused the transition.
	Event Event

	// FromState is the state before the transition.
	FromState State

	// ToState is the state after the transition.
	ToState State
}
