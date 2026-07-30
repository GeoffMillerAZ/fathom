package metrics

// reviewfindings.go — O20's fix (docs/dogfood-observations.md O20,
// docs/design/findings-as-evidence.md §3.1/§6 item 1): `research report`
// printed review_active_findings=251783 on a repo with ~250 real findings.
//
// Root cause: cmd/fathom/research_report.go's (and research_bundle.go's)
// window loop treated review_active_findings as an EVENT total — summing
// every "review" call-log entry's verdict_outcome map across every call in
// the window — when the field each call actually carries is a SNAPSHOT: the
// count of currently-active findings AS OF THAT ONE REVIEW RUN
// (review.reviewVerdictOutcome / Dimension.Count, cmd/fathom/review.go).
// `fathom review` runs on every push (lefthook's pre-push step), so a
// window with N pushes summed N snapshots of the same ~250-ish findings
// instead of reporting the most recent one — "re-counting per call" (O20's
// own hypothesis, docs/dogfood-observations.md). 251783 is not a bogus
// number from a join or a never-marked-inactive row; it is N snapshots
// legitimately added together, which is simply the wrong operation for a
// state metric.
//
// The fix stays inside this command's own documented data-source contract
// (research_report.go's header comment: "This command never queries the
// live daemon or opens a store: every input is a local file" — call-log
// only, by design). LatestReviewActiveFindings reports the snapshot from
// the MOST RECENT "review" call-log entry in the window, not a sum —
// pinned by a fixture-of-known-size test in reviewfindings_test.go per the
// design's honesty rule 5 (no count ships without one).

import "time"

// ReviewCallSnapshot is one "review" call-log entry's minimal shape: when it
// ran, and its per-dimension active-finding counts (the same
// map[string]int a call log's verdict_outcome field carries for tool
// "review" — cmd/fathom's reviewVerdictOutcome). Decoupled from
// cmd/fathom's adoptionCallEntry the same way CallVerdictOutcome/CallEntry
// already are.
type ReviewCallSnapshot struct {
	At      time.Time
	Outcome map[string]int
}

// LatestReviewActiveFindings reports the active-finding count as of the
// MOST RECENT review call in snapshots (ties at the max timestamp broken by
// input order — the last one wins, matching a caller that appends in file
// order). Returns ok=false for an empty input (no review calls with a
// verdict_outcome in the window) — the caller must render an honest note,
// never a zero.
//
// sampleSize is len(snapshots) — how many review-call snapshots were seen
// in the window, so a renderer can disclose "as of the latest of N calls"
// rather than just the number itself.
func LatestReviewActiveFindings(snapshots []ReviewCallSnapshot) (count, sampleSize int, ok bool) {
	if len(snapshots) == 0 {
		return 0, 0, false
	}
	var latest time.Time
	for _, s := range snapshots {
		if !s.At.Before(latest) {
			latest = s.At
			sum := 0
			for _, n := range s.Outcome {
				sum += n
			}
			count = sum
		}
	}
	return count, len(snapshots), true
}
