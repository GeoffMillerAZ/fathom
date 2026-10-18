package verify

// operational.go — O1162: one predicate for "this claimed row is unproven
// because its evidence could not be read", not because the claim failed.
// Three causes today: the gone: precondition (index-stale, O1160), a graph
// fallback (the typed edges could not be read, so the reachability criteria
// ran on the syntax graph), and a verdict the daemon never answered. A
// strict shortfall made only of such rows is operational (exit 2); every
// row stays Unknown (I38, I44).

import (
	"fmt"
	"strings"
)

// GraphFallbackLabel labels a row held Unknown by the graph fallback.
const GraphFallbackLabel = "graph-fallback"

// VerdictElidedLabel labels a row whose verdict the daemon never answered.
const VerdictElidedLabel = "verdict-elided"

// GraphFallbackMarker ends the detail of a graph criterion held Unknown by
// the fallback. Like IndexStaleMarker it rides the detail so the daemon wire
// needs no new field.
const GraphFallbackMarker = "[graph-fallback]"

const graphFallbackCausePrefix = " (typed call graph unavailable: "

// GraphFallbackRemedy is the fix for every GraphFallback.
const GraphFallbackRemedy = "re-run when the daemon is below its shed mark, or verify with --store"

// GraphFallbackEvidence is implemented by an Evidence whose call graph is
// the syntax graph because the typed edges could not be read. The cause
// names the failure; "" means the typed graph is in use.
type GraphFallbackEvidence interface {
	GraphFallback() string
}

// GraphFallbackCause is ev's graph-fallback cause, "" when none.
func GraphFallbackCause(ev Evidence) string {
	if g, ok := ev.(GraphFallbackEvidence); ok {
		return g.GraphFallback()
	}
	return ""
}

func graphFallbackDetail(detail, cause string) string {
	return detail + graphFallbackCausePrefix + cause + ") " + GraphFallbackMarker
}

// IsGraphFallbackCriterion reports whether c is a graph criterion held
// Unknown by the fallback.
func IsGraphFallbackCriterion(c Criterion) bool {
	return c.Status == Unknown && strings.HasSuffix(c.Detail, " "+GraphFallbackMarker)
}

// OperationalRow reports whether tv is a claimed row unproven only because
// its evidence could not be read: no failing criterion, and an index-stale
// gone criterion, a graph-fallback criterion, or an unanswered verdict.
func OperationalRow(tv TaskVerdict) bool {
	if !tv.Claimed || tv.Skipped || tv.Status != Unknown {
		return false
	}
	operational := tv.Label == VerdictElidedLabel
	for _, c := range tv.Criteria {
		if c.Status == Fail {
			return false
		}
		if IsIndexStaleCriterion(c) || IsGraphFallbackCriterion(c) {
			operational = true
		}
	}
	return operational
}

// OperationalShortfall reports whether the gate's failures are ALL
// operational rows: failures is the gate's failure count, and each must be
// one OperationalRow. Any other failure (a failing claim, a --require-all
// incompleteness, an Unknown from another cause) keeps the gate at exit 1.
func OperationalShortfall(reports []Report, failures int, strict bool) bool {
	n := 0
	for _, rep := range reports {
		for _, tv := range rep.Tasks {
			if OperationalRow(tv) {
				n++
			}
		}
	}
	return strict && n > 0 && failures == n
}

// GraphFallback is the one aggregate for rows held back by a graph fallback.
type GraphFallback struct {
	Reason string `json:"reason"`
	Rows   int    `json:"rows"`
	Remedy string `json:"remedy"`
}

// GraphFallbackOf aggregates the reports' graph-fallback rows: the first
// row's cause and how many claimed rows. ok is false when none.
func GraphFallbackOf(reports []Report) (GraphFallback, bool) {
	var agg GraphFallback
	for _, rep := range reports {
		for _, tv := range rep.Tasks {
			if !tv.Claimed || tv.Skipped {
				continue
			}
			for _, c := range tv.Criteria {
				if !IsGraphFallbackCriterion(c) {
					continue
				}
				if agg.Rows == 0 {
					d := strings.TrimSuffix(c.Detail, " "+GraphFallbackMarker)
					if i := strings.LastIndex(d, graphFallbackCausePrefix); i >= 0 {
						agg.Reason = strings.TrimSuffix(d[i+len(graphFallbackCausePrefix):], ")")
					}
					agg.Remedy = GraphFallbackRemedy
				}
				agg.Rows++
				break
			}
		}
	}
	return agg, agg.Rows > 0
}

// GraphFallbackLine is the one aggregate line for a graph fallback.
func GraphFallbackLine(a GraphFallback) string {
	return fmt.Sprintf("graph-fallback: the typed call graph was unavailable (%s); %d claimed row(s) held back, still unknown (I44); %s", a.Reason, a.Rows, a.Remedy)
}
