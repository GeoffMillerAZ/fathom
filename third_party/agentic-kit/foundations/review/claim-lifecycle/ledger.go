package claimlifecycle

import (
	"encoding/json"
	"fmt"
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	eventstore "github.com/geoffmilleraz/agentic-kit/foundations/provenance/event-store"
)

// ClaimLedger wraps an event-store Store to provide an append-only,
// hash-chained ledger for claim lifecycle events. Clock is injected via
// NewClaimLedgerWithClock so tests can produce reproducible hashes without any
// wall-clock calls inside this package.
type ClaimLedger struct {
	store *eventstore.Store
}

// NewClaimLedger returns a ClaimLedger that uses time.Now().UTC() as its clock.
func NewClaimLedger() *ClaimLedger {
	return &ClaimLedger{store: eventstore.NewStore()}
}

// NewClaimLedgerWithClock returns a ClaimLedger that uses the supplied clock.
// Inject a deterministic clock in tests to produce reproducible entry hashes.
func NewClaimLedgerWithClock(clock func() time.Time) *ClaimLedger {
	return &ClaimLedger{store: eventstore.NewStoreWithClock(clock)}
}

// claimEventEnvelope is the canonical JSON representation stored in the
// event-store payload. The event Kind is stored as the store entry Kind
// discriminator, not repeated here.
type claimEventEnvelope struct {
	ClaimID         string                  `json:"claim_id"`
	AuthorKind      string                  `json:"author_kind"`
	AuthorHumanID   string                  `json:"author_human_id,omitempty"`
	AuthorAIModelID string                  `json:"author_ai_model_id,omitempty"`
	Proposed        *ProposedPayload        `json:"proposed,omitempty"`
	AssertionLinked *AssertionLinkedPayload `json:"assertion_linked,omitempty"`
	EvidenceLinked  *EvidenceLinkedPayload  `json:"evidence_linked,omitempty"`
}

func envelopeFromEvent(ev ClaimEvent) claimEventEnvelope {
	env := claimEventEnvelope{
		ClaimID:         ev.ClaimID,
		AuthorKind:      string(ev.Author.Kind),
		Proposed:        ev.Proposed,
		AssertionLinked: ev.AssertionLinked,
		EvidenceLinked:  ev.EvidenceLinked,
	}
	if ev.Author.Human != nil {
		env.AuthorHumanID = ev.Author.Human.ID
	}
	if ev.Author.AI != nil {
		env.AuthorAIModelID = ev.Author.AI.ModelID
	}
	return env
}

func eventFromEnvelope(kind ClaimEventKind, env claimEventEnvelope) ClaimEvent {
	ev := ClaimEvent{
		Kind:            kind,
		ClaimID:         env.ClaimID,
		Proposed:        env.Proposed,
		AssertionLinked: env.AssertionLinked,
		EvidenceLinked:  env.EvidenceLinked,
	}
	// Reconstruct Author from the minimal fields stored in the envelope.
	// Full AI reproduction keys are not re-stored here (they belong to the
	// AI-provenance record, which is the canonical source).
	switch aiprovenance.AuthorKind(env.AuthorKind) {
	case aiprovenance.KindHuman:
		ev.Author = aiprovenance.NewHuman(env.AuthorHumanID)
	case aiprovenance.KindAI:
		// Reconstruct a minimal AI author sufficient for identity. Callers that
		// need the full AI-provenance record should look it up by ReproKey.
		ev.Author = aiprovenance.Author{
			Kind: aiprovenance.KindAI,
			AI:   &aiprovenance.AIAuthor{ModelID: env.AuthorAIModelID},
		}
	}
	return ev
}

// Append serialises ev deterministically (encoding/json of the canonical
// envelope) and appends it to the underlying hash-chained store under the
// event Kind. Returns an error if serialisation fails.
//
// Append never mutates past events.
func (l *ClaimLedger) Append(ev ClaimEvent) error {
	env := envelopeFromEvent(ev)
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("claimlifecycle: Append: marshal event: %w", err)
	}
	if _, err := l.store.Append(string(ev.Kind), payload); err != nil {
		return fmt.Errorf("claimlifecycle: Append: store: %w", err)
	}
	return nil
}

// Verify delegates to the underlying event-store to verify the hash chain is
// intact. Returns a descriptive error if any entry has been tampered with.
func (l *ClaimLedger) Verify() error {
	return l.store.Verify()
}

// Events replays the underlying store and decodes every entry back into a
// ClaimEvent slice, in append order. Returns an error if any entry cannot be
// decoded.
func (l *ClaimLedger) Events() ([]ClaimEvent, error) {
	var events []ClaimEvent
	replayErr := l.store.Replay(func(entry eventstore.Entry) error {
		var env claimEventEnvelope
		if err := json.Unmarshal(entry.Payload, &env); err != nil {
			return fmt.Errorf("claimlifecycle: Events: decode entry seq=%d: %w", entry.Seq, err)
		}
		ev := eventFromEnvelope(ClaimEventKind(entry.Kind), env)
		ev.RecordedAt = entry.RecordedAt
		events = append(events, ev)
		return nil
	})
	if replayErr != nil {
		return nil, replayErr
	}
	return events, nil
}

// Len returns the total number of events stored in the ledger.
func (l *ClaimLedger) Len() int {
	return l.store.Len()
}
