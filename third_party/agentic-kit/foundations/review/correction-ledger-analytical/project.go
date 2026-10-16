package correctionledgeranalytical

import (
	"fmt"
	"sort"
	"time"

	cl "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/correction-ledger"
)

// Project produces a deterministic, immutable slice of AnalyticalEvent records
// from a sequence of LedgerEvents sourced from the correction ledger.
//
// Only LedgerEventKindCorrectionAdded and LedgerEventKindDismissalAdded events
// are projected; all other event kinds are ignored.
//
// Events are processed in the order they appear in the input slice. Version
// numbers are assigned per-target in ascending order of occurrence (starting
// at 1), so the same input always yields the same output.
//
// Project is stateless and deterministic: given the same events slice, the
// returned slice is always identical.
func Project(events []cl.LedgerEvent) []AnalyticalEvent {
	// Track per-target version counters so Version is monotonically increasing
	// within each Target, regardless of event kind.
	versionByTarget := make(map[string]int)

	var out []AnalyticalEvent

	for _, ev := range events {
		switch ev.Kind {
		case cl.LedgerEventKindCorrectionAdded:
			if ev.Correction == nil {
				continue
			}
			c := ev.Correction
			target := c.Target.StableTarget
			if target == "" {
				// Malformed event — skip rather than panic.
				continue
			}
			at := c.CorrectedAt
			if at.IsZero() {
				at = ev.OccurredAt
			}

			versionByTarget[target]++
			v := versionByTarget[target]

			out = append(out, AnalyticalEvent{
				ID:      fmt.Sprintf("%s:%s:%d", AnalyticalEventKindCorrection, c.FindingID, v),
				Kind:    AnalyticalEventKindCorrection,
				Target:  target,
				At:      at.UTC().Truncate(time.Second),
				Version: v,
			})

		case cl.LedgerEventKindDismissalAdded:
			if ev.Dismissal == nil {
				continue
			}
			d := ev.Dismissal
			target := d.FindingID
			if target == "" {
				continue
			}
			at := d.DismissedAt
			if at.IsZero() {
				at = ev.OccurredAt
			}

			versionByTarget[target]++
			v := versionByTarget[target]

			out = append(out, AnalyticalEvent{
				ID:      fmt.Sprintf("%s:%s:%d", AnalyticalEventKindDismissal, d.FindingID, v),
				Kind:    AnalyticalEventKindDismissal,
				Target:  target,
				At:      at.UTC().Truncate(time.Second),
				Version: v,
			})
		}
	}

	// Sort by (At, ID) for fully deterministic output ordering even when
	// multiple events share the same timestamp.
	sort.Slice(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID < out[j].ID
		}
		return out[i].At.Before(out[j].At)
	})

	return out
}
