// Package correctionledgeranalytical provides a Parquet-export-eligible
// projection of the correction ledger. It produces immutable, timestamped,
// versioned AnalyticalEvent records from correction and dismissal activity.
//
// These records feed the analytical substrate for time-travel queries such as
// "what did we believe about claim X on date Y?"
//
// Concurrency: Project is a pure function and safe for concurrent use.
package correctionledgeranalytical

import "time"

// AnalyticalEventKind discriminates the two types of analytical events.
type AnalyticalEventKind string

const (
	// AnalyticalEventKindCorrection indicates the event arose from a
	// correction — the underlying fact was wrong and has been updated.
	AnalyticalEventKindCorrection AnalyticalEventKind = "correction"

	// AnalyticalEventKindDismissal indicates the event arose from a
	// dismissal — the finding was a false positive or acceptable risk.
	AnalyticalEventKindDismissal AnalyticalEventKind = "dismissal"
)

// AnalyticalEvent is an immutable, timestamped, versioned record emitted by
// Project. Each event corresponds to one correction or dismissal activity
// recorded in the correction ledger.
//
// AnalyticalEvent is deliberately narrow — it carries only the fields needed
// for the analytical substrate (Parquet, time-travel queries) and is
// independent of any org-specific schema.
type AnalyticalEvent struct {
	// ID is a stable, unique identifier for this event.
	// Format: "<kind>:<FindingID>:<Version>".
	ID string

	// Kind discriminates correction vs dismissal activity.
	Kind AnalyticalEventKind

	// Target is the stable identifier for the entity whose activity is
	// recorded. For corrections this is the CorrectionTarget.StableTarget;
	// for dismissals it is the FindingID.
	Target string

	// At is the UTC timestamp when the activity occurred
	// (CorrectedAt or DismissedAt).
	At time.Time

	// Version is a monotonically increasing sequence number among all
	// AnalyticalEvents for the same Target, starting at 1.
	Version int
}
