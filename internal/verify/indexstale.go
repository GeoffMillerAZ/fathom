package verify

// indexstale.go — O1160: a strict shortfall whose every unproven claimed
// row is a gone: row held back only because the index is not at HEAD (an
// uncommitted tree, or a commit after the ingest) is an environment
// failure, not a failing claim. The rows stay Unknown (I44); the gate
// reads the shortfall as exit 2, the documented operational code.

import "strings"

// IndexStale is the one aggregate for a failed gone: precondition.
type IndexStale struct {
	Reason    string `json:"reason"`
	FirstPath string `json:"first_dirty_path,omitempty"`
	Rows      int    `json:"rows"`
	Remedy    string `json:"remedy"`
}

// IndexStaleRemedy is the fix for every IndexStale.
const IndexStaleRemedy = "commit or stash, then re-ingest"

const goneUnprovenPrefix = "gone: not proven — "

// IsIndexStaleCriterion reports whether c is a gone: Unknown held back only
// by the index-at-HEAD precondition.
func IsIndexStaleCriterion(c Criterion) bool {
	return c.Name == goneCriterion && c.Status == Unknown && strings.HasSuffix(c.Detail, " "+IndexStaleMarker)
}

// indexStaleRow reports whether tv is a claimed row held back only by that
// precondition (no failing criterion, an index-stale gone criterion).
func indexStaleRow(tv TaskVerdict) (Criterion, bool) {
	if !tv.Claimed || tv.Skipped || tv.Status != Unknown {
		return Criterion{}, false
	}
	var found Criterion
	ok := false
	for _, c := range tv.Criteria {
		if c.Status == Fail {
			return Criterion{}, false
		}
		if IsIndexStaleCriterion(c) {
			found, ok = c, true
		}
	}
	return found, ok
}

// IndexStaleOf aggregates the reports' index-stale rows: the first row's
// reason, its first dirty path when it names one, and how many rows. ok is
// false when no row is held back that way.
func IndexStaleOf(reports []Report) (IndexStale, bool) {
	var agg IndexStale
	for _, rep := range reports {
		for _, tv := range rep.Tasks {
			c, ok := indexStaleRow(tv)
			if !ok {
				continue
			}
			if agg.Rows == 0 {
				reason := strings.TrimSuffix(strings.TrimPrefix(c.Detail, goneUnprovenPrefix), " "+IndexStaleMarker)
				if i := strings.Index(reason, DirtyPathMarker); i >= 0 {
					rest := reason[i+len(DirtyPathMarker):]
					agg.FirstPath = strings.TrimSuffix(rest, ")")
					reason = reason[:i]
				}
				agg.Reason, agg.Remedy = reason, IndexStaleRemedy
			}
			agg.Rows++
		}
	}
	return agg, agg.Rows > 0
}
