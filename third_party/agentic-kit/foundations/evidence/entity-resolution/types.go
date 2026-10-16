// Package entityresolution proposes entity-match candidates for human review.
// It never auto-merges; every candidate goes through a human-review workflow
// before any downstream system (e.g. entity-crosswalk) records the merge.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema. Similarity primitives live in similarity.go.
//
// Concurrency contract: all exported types are plain value types with no
// mutable state; Propose is a pure function safe to call concurrently from
// multiple goroutines.
package entityresolution

import (
	"fmt"
	"time"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Entity is a named, attributed real-world entity to be matched against others.
//
// CUE mirror: not directly modelled in schema.cue (input type only).
type Entity struct {
	// ID uniquely identifies this entity. Must be non-empty.
	ID string

	// Name is the human-readable label used for similarity matching.
	// Must be non-empty.
	Name string

	// Attributes holds additional metadata. Optional; may be nil.
	Attributes map[string]string
}

// Validate returns an error if the Entity is not well-formed.
// Rules: ID and Name must be non-empty.
func (e Entity) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("entityresolution: Entity.ID must not be empty")
	}
	if e.Name == "" {
		return fmt.Errorf("entityresolution: Entity.Name must not be empty")
	}
	return nil
}

// Candidate is a proposed match between two entities, scored by a named method
// and attributed to a specific author with full provenance.
//
// Canonical form: LeftID is always the lexicographically smaller entity ID.
// GenerateCandidates enforces this; Validate() rejects violations so that
// hand-constructed Candidates cannot break the de-duplication guarantee.
//
// CUE mirror: #Candidate in schema.cue.
type Candidate struct {
	// LeftID is the lexicographically smaller entity ID. Must be non-empty and
	// must be strictly less than RightID (canonical ordering).
	LeftID string

	// RightID is the other entity ID. Must be non-empty and differ from LeftID.
	RightID string

	// Score is the similarity in [0, 1]. 0 = no similarity, 1 = identical.
	Score float64

	// Method names the algorithm that produced this candidate. Must be non-empty.
	Method string

	// Author records who or what produced this candidate (human reviewer or AI
	// model). Mandatory — every candidate must have a traceable author.
	Author aiprovenance.Author

	// Grade is the epistemic strength of this candidate. It reflects the
	// weakest tier of the evidence spans that were used to generate the
	// candidate, in accordance with Throughline invariant 4.
	Grade evidencegrade.Grade

	// SourceSpanIDs are the IDs of the evidence spans from which the two
	// entity IDs were extracted. May be empty when the candidate is produced
	// from a source that predates span-level tracking, but should be populated
	// whenever spans are available. Callers receive a defensive copy.
	SourceSpanIDs []string
}

// SourceSpanIDsCopy returns a defensive copy of the SourceSpanIDs slice.
// Mutating the returned slice does not affect the Candidate.
func (c Candidate) SourceSpanIDsCopy() []string {
	if len(c.SourceSpanIDs) == 0 {
		return nil
	}
	out := make([]string, len(c.SourceSpanIDs))
	copy(out, c.SourceSpanIDs)
	return out
}

// Validate returns an error if the Candidate is not well-formed.
// Rules:
//   - LeftID and RightID must be non-empty and must differ.
//   - LeftID must be strictly less than RightID (canonical ordering invariant —
//     prevents the same pair appearing as both (a,b) and (b,a)).
//   - Score must be in [0, 1].
//   - Method must be non-empty.
//   - Author must be valid (see aiprovenance.Author.Validate).
//   - Grade must be valid (see evidencegrade.Grade.Validate).
func (c Candidate) Validate() error {
	if c.LeftID == "" {
		return fmt.Errorf("entityresolution: Candidate.LeftID must not be empty")
	}
	if c.RightID == "" {
		return fmt.Errorf("entityresolution: Candidate.RightID must not be empty")
	}
	if c.LeftID == c.RightID {
		return fmt.Errorf("entityresolution: Candidate.LeftID and RightID must differ, got %q", c.LeftID)
	}
	// Canonical ordering: LeftID must be lexicographically less than RightID.
	// GenerateCandidates enforces this automatically. Validate() also enforces it
	// so that hand-constructed Candidates cannot violate the de-duplication guarantee.
	if c.LeftID > c.RightID {
		return fmt.Errorf("entityresolution: Candidate.LeftID %q must be lexicographically less than RightID %q (canonical ordering)", c.LeftID, c.RightID)
	}
	if c.Score < 0 || c.Score > 1 {
		return fmt.Errorf("entityresolution: Candidate.Score %g is outside [0, 1]", c.Score)
	}
	if c.Method == "" {
		return fmt.Errorf("entityresolution: Candidate.Method must not be empty")
	}
	if err := c.Author.Validate(); err != nil {
		return fmt.Errorf("entityresolution: Candidate.Author invalid: %w", err)
	}
	if err := c.Grade.Validate(); err != nil {
		return fmt.Errorf("entityresolution: Candidate.Grade invalid: %w", err)
	}
	return nil
}

// GenerateCandidates compares all unordered pairs of entities by name
// similarity and returns candidates whose score is >= threshold.
//
// Properties:
//   - All pairs i<j (by index) are evaluated; no self-pairs.
//   - Pairs where both entities share the same ID are silently skipped — such
//     pairs would produce LeftID==RightID which is structurally invalid.
//   - LeftID is always the lexicographically smaller ID (canonical ordering).
//   - Output is sorted by descending Score; ties broken by (LeftID, RightID).
//   - Method is set to "name-similarity".
//   - Author is set to the supplied author (required).
//   - Grade is set to the supplied grade (required).
//   - Deterministic: same input → same output regardless of map/goroutine order.
//
// The returned slice is a fresh allocation; the caller owns it.
func GenerateCandidates(entities []Entity, threshold float64, author aiprovenance.Author, grade evidencegrade.Grade) []Candidate {
	var out []Candidate
	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			a, b := entities[i], entities[j]
			// Skip self-pairs produced when two entities share the same ID.
			// Such a pair would violate Candidate.Validate() (LeftID == RightID).
			if a.ID == b.ID {
				continue
			}
			score := Similarity(a.Name, b.Name)
			if score < threshold {
				continue
			}
			left, right := a.ID, b.ID
			if left > right {
				left, right = right, left
			}
			out = append(out, Candidate{
				LeftID:  left,
				RightID: right,
				Score:   score,
				Method:  "name-similarity",
				Author:  author,
				Grade:   grade,
			})
		}
	}
	sortCandidates(out)
	return out
}

// sortCandidates sorts candidates by descending Score, then ascending LeftID,
// then ascending RightID — fully deterministic.
func sortCandidates(cs []Candidate) {
	// insertion sort: stable, allocation-free, fine for small candidate sets
	for i := 1; i < len(cs); i++ {
		key := cs[i]
		j := i - 1
		for j >= 0 && less(key, cs[j]) {
			cs[j+1] = cs[j]
			j--
		}
		cs[j+1] = key
	}
}

// less reports whether a should sort before b:
// higher Score first; ties broken lexicographically by LeftID then RightID.
func less(a, b Candidate) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.LeftID != b.LeftID {
		return a.LeftID < b.LeftID
	}
	return a.RightID < b.RightID
}

// ReviewState represents the human-review lifecycle of a Match.
//
// CUE mirror: #ReviewState in schema.cue.
type ReviewState string

const (
	// StateProposed is the initial state of every Match.
	StateProposed ReviewState = "proposed"
	// StateAccepted means a human reviewer confirmed the match is correct.
	StateAccepted ReviewState = "accepted"
	// StateRejected means a human reviewer determined the match is wrong.
	StateRejected ReviewState = "rejected"
)

// Match wraps a Candidate with human-review metadata.
//
// Timestamps must come from the caller — never from time.Now() inside this
// package. Inject a deterministic clock in tests (same pattern as
// provenance/event-store NewStoreWithClock).
//
// CUE mirror: #Match in schema.cue.
type Match struct {
	// Candidate is the underlying proposed match.
	Candidate Candidate

	// State is the current review lifecycle state.
	State ReviewState

	// ReviewedBy is the reviewer's identifier. Non-empty only after
	// Accept or Reject. Required when State is accepted or rejected.
	ReviewedBy string

	// Note is an optional free-text rationale from the reviewer.
	Note string

	// CreatedAt is the timestamp at which this Match was proposed (Propose was
	// called). Must be supplied by the caller; never set by this package.
	CreatedAt time.Time

	// ReviewedAt is the timestamp at which the reviewer called Accept or Reject.
	// Zero when State is proposed. Must be supplied by the caller; never set by
	// this package.
	ReviewedAt time.Time

	// Author records who authored the review decision. For a proposed match this
	// is the same as Candidate.Author (the proposer). After Accept/Reject it is
	// the reviewer's identity. Optional on proposed matches; enforced by Accept
	// and Reject.
	Author aiprovenance.Author
}

// Validate returns an error if the Match is not well-formed.
// Rules (in addition to Candidate.Validate):
//   - State must be a valid ReviewState.
//   - When State is accepted or rejected, ReviewedBy must be non-empty.
//   - When State is accepted or rejected, ReviewedAt must be non-zero.
//   - CreatedAt must be non-zero.
func (m Match) Validate() error {
	if err := m.Candidate.Validate(); err != nil {
		return fmt.Errorf("entityresolution: Match.Candidate invalid: %w", err)
	}
	switch m.State {
	case StateProposed, StateAccepted, StateRejected:
	default:
		return fmt.Errorf("entityresolution: Match.State %q is not a valid ReviewState", m.State)
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("entityresolution: Match.CreatedAt must not be zero — supply a timestamp from the caller")
	}
	if m.State == StateAccepted || m.State == StateRejected {
		if m.ReviewedBy == "" {
			return fmt.Errorf("entityresolution: Match.ReviewedBy must not be empty when State is %q", m.State)
		}
		if m.ReviewedAt.IsZero() {
			return fmt.Errorf("entityresolution: Match.ReviewedAt must not be zero when State is %q", m.State)
		}
	}
	return nil
}

// Propose wraps a Candidate in a new Match with State=proposed.
// createdAt must be the caller-supplied timestamp (do not pass time.Now()
// inside this package — inject clocks at the call site).
//
// This is the entry point for every new candidate — append it to your ledger;
// never mutate an existing Match in place.
func Propose(c Candidate, createdAt time.Time) Match {
	return Match{
		Candidate: c,
		State:     StateProposed,
		CreatedAt: createdAt,
	}
}

// Accept transitions this Match to accepted and records the reviewer.
// reviewedAt must be the caller-supplied timestamp (do not pass time.Now()
// inside this package — inject clocks at the call site).
//
// Returns a new Match (append-only spirit — never mutates the receiver).
// Errors:
//   - State is not proposed.
//   - by is empty.
//   - reviewedAt is zero.
func (m Match) Accept(by, note string, reviewedAt time.Time) (Match, error) {
	if by == "" {
		return Match{}, fmt.Errorf("entityresolution: Accept requires non-empty reviewer (by)")
	}
	if reviewedAt.IsZero() {
		return Match{}, fmt.Errorf("entityresolution: Accept requires non-zero reviewedAt timestamp")
	}
	if m.State != StateProposed {
		return Match{}, fmt.Errorf("entityresolution: Accept requires state %q, got %q", StateProposed, m.State)
	}
	out := m
	out.State = StateAccepted
	out.ReviewedBy = by
	out.Note = note
	out.ReviewedAt = reviewedAt
	return out, nil
}

// Reject transitions this Match to rejected and records the reviewer.
// reviewedAt must be the caller-supplied timestamp (do not pass time.Now()
// inside this package — inject clocks at the call site).
//
// Returns a new Match (append-only spirit — never mutates the receiver).
// Errors:
//   - State is not proposed.
//   - by is empty.
//   - reviewedAt is zero.
func (m Match) Reject(by, note string, reviewedAt time.Time) (Match, error) {
	if by == "" {
		return Match{}, fmt.Errorf("entityresolution: Reject requires non-empty reviewer (by)")
	}
	if reviewedAt.IsZero() {
		return Match{}, fmt.Errorf("entityresolution: Reject requires non-zero reviewedAt timestamp")
	}
	if m.State != StateProposed {
		return Match{}, fmt.Errorf("entityresolution: Reject requires state %q, got %q", StateProposed, m.State)
	}
	out := m
	out.State = StateRejected
	out.ReviewedBy = by
	out.Note = note
	out.ReviewedAt = reviewedAt
	return out, nil
}
