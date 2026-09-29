package claimlifecycle

import (
	"fmt"
	"time"

	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
)

// ClaimView is the projected state of a claim at a point in time, derived by
// folding its event stream. It is a read-only snapshot — never stored as a
// mutable field.
type ClaimView struct {
	// ClaimID is the stable identifier of the claim.
	ClaimID string

	// Text is the most recently recorded claim text (from the KindProposed event).
	Text string

	// State is the current lifecycle state derived from the event fold.
	State ClaimState

	// Grade is the computed epistemic grade derived from linked evidence spans.
	// It stores a *ClaimGrade (not a bare *Grade) so that FloorTier, Promoted,
	// PromotionReason, DroppedSpanIDs, and AIContributedConfidence are preserved
	// for legal audit purposes (docs/practices.md §2).
	// Nil if no evidence has been linked yet.
	Grade *evidencegrade.ClaimGrade

	// LinkedAssertions contains the IDs of all assertions linked to the claim.
	LinkedAssertions []string

	// LinkedEvidence contains the SpanIDs of all evidence spans linked to the claim.
	LinkedEvidence []string

	// History records the sequence of event kinds in append order.
	History []ClaimEventKind
}

// Project folds all events for claimID into a ClaimView. Returns an error if
// no KindProposed event exists for the claim (claim not found) or if grade
// computation fails.
func Project(claimID string, events []ClaimEvent) (ClaimView, error) {
	return projectAsOf(claimID, events, time.Time{} /* zero = no time filter */)
}

// ProjectAsOf folds only events with RecordedAt <= asOf, reproducing the
// case's belief about the claim as of that instant. The events slice must
// carry RecordedAt values (populated from the store entry RecordedAt).
func ProjectAsOf(claimID string, events []ClaimEvent, asOf time.Time) (ClaimView, error) {
	return projectAsOf(claimID, events, asOf)
}

// projectAsOf is the shared implementation. If asOf is the zero time, no
// time filter is applied and all events are folded.
func projectAsOf(claimID string, events []ClaimEvent, asOf time.Time) (ClaimView, error) {
	view := ClaimView{
		ClaimID:          claimID,
		LinkedAssertions: []string{},
		LinkedEvidence:   []string{},
		History:          []ClaimEventKind{},
	}

	found := false
	var spans []evidencegrade.GradedSpan

	for _, ev := range events {
		if ev.ClaimID != claimID {
			continue
		}
		// Apply time filter when asOf is non-zero.
		if !asOf.IsZero() && ev.RecordedAt.After(asOf) {
			continue
		}

		found = true
		view.History = append(view.History, ev.Kind)

		switch ev.Kind {
		case KindProposed:
			if ev.Proposed != nil {
				view.Text = ev.Proposed.Text
			}
			view.State = StateProposed

		case KindAssertionLinked:
			if ev.AssertionLinked != nil {
				view.LinkedAssertions = append(view.LinkedAssertions, ev.AssertionLinked.AssertionID)
			}

		case KindEvidenceLinked:
			if ev.EvidenceLinked != nil {
				view.LinkedEvidence = append(view.LinkedEvidence, ev.EvidenceLinked.SpanID)
				spans = append(spans, evidencegrade.GradedSpan{
					SpanID:      ev.EvidenceLinked.SpanID,
					Tier:        evidencegrade.Tier(ev.EvidenceLinked.Tier),
					Confidence:  ev.EvidenceLinked.Confidence,
					Necessary:   ev.EvidenceLinked.Necessary,
					Independent: ev.EvidenceLinked.Independent,
				})
			}

		case KindSupported:
			view.State = StateSupported

		case KindRefuted:
			view.State = StateRefuted

		case KindMarkedUncertain:
			view.State = StateUncertain

		case KindReopened:
			view.State = StateProposed

		case KindSuperseded:
			view.State = StateSuperseded

		case KindRecanted:
			view.State = StateRecanted
		}
	}

	if !found {
		return ClaimView{}, fmt.Errorf("claimlifecycle: Project: claim %q not found", claimID)
	}

	// Compute grade from linked spans, if any.
	if len(spans) > 0 {
		cg, err := evidencegrade.GradeClaim(spans, evidencegrade.PromotionRule{AllowTierPromotion: false})
		if err != nil {
			return ClaimView{}, fmt.Errorf("claimlifecycle: Project: GradeClaim: %w", err)
		}
		view.Grade = &cg
	}

	return view, nil
}
