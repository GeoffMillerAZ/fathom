package decide

// decidedby.go — JG-1 (ADR-073 decision 6, DMP-3): the one renderer of the
// decided_by badge. Every surface a configured port can shape (the
// obligations envelope, POST /route, certify suggest) names how its answer
// was reached through DecidedBy, and nowhere else: computed, table,
// override, or judged(<site> · <provider>/<model> · conf <x> · calibration
// <status> <n>/100). With provider none the badge is absent, so I42's bytes
// hold. A badge is disclosure, never proof (I62): no surface styles it as a
// pass, and none reads it back as evidence.

import (
	"errors"
	"fmt"
	"sort"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// The decided_by tiers.
const (
	// DecidedComputed: the port was asked and the computed answer stood (it
	// answered unsure, invalid, or missed; a miss names its class).
	DecidedComputed = "computed"
	// DecidedTable: a fitted table answered (DMP-5).
	DecidedTable = "table"
	// DecidedOverride: the port's answer replaced the table's.
	DecidedOverride = "override"
	// DecidedJudged: the port's answer shaped what was served.
	DecidedJudged = "judged"
)

// uncalibrated is the calibration status of a site no reading speaks for
// (ADR-073 decision 4: every judged answer reads uncalibrated until its
// site's reading clears ADR-056 R3).
const uncalibrated = "uncalibrated"

// Calibration is a site's latest calibration reading as the badge shows it:
// Status is the kit's decision.CalibrationStatus ("" reads uncalibrated)
// and N the reading's sample count, shown against the n the hint bar needs.
type Calibration struct {
	Status string
	N      int
}

// Decision is how one served answer was reached: what DecidedBy renders.
type Decision struct {
	// Provider is the configured port's provider; "" or "none" renders no
	// badge.
	Provider string
	// Tier is one of the Decided* tiers.
	Tier string
	// Record is the judged answer's decision row (DecidedJudged).
	Record decision.Record
	// Calibration is Record's site's reading (DecidedJudged); the zero
	// value reads uncalibrated 0/100.
	Calibration Calibration
	// Table is the table's own answer an override replaced
	// (DecidedOverride), shown beside it.
	Table string
	// Miss is the degrade class when the port was asked and answered
	// nothing (DecidedComputed): MissClass of the ask's error.
	Miss string
}

// DecidedBy renders d's decided_by badge, "" for provider none or an
// unknown tier.
func DecidedBy(d Decision) string {
	if d.Provider == "" || d.Provider == "none" {
		return ""
	}
	switch d.Tier {
	case DecidedComputed:
		if d.Miss != "" {
			return DecidedComputed + " · not consulted (" + d.Miss + ")"
		}
		return DecidedComputed
	case DecidedTable:
		return DecidedTable
	case DecidedOverride:
		if d.Table != "" {
			return DecidedOverride + " · table " + d.Table
		}
		return DecidedOverride
	case DecidedJudged:
		return judgedBadge(d)
	}
	return ""
}

// judgedBadge is the judged tier's badge.
func judgedBadge(d Decision) string {
	r := d.Record
	provider := r.Provider
	if provider == "" {
		provider = d.Provider
	}
	who := provider
	if r.Model != "" {
		who += "/" + r.Model
	}
	status := d.Calibration.Status
	if status == "" {
		status = uncalibrated
	}
	n := max(d.Calibration.N, 0)
	return fmt.Sprintf("%s(%s · %s · conf %.2f · calibration %s %d/%d)", DecidedJudged, r.Site, who, r.Confidence, status, n, decision.HintBar().MinN)
}

// WeakestRecord is the row a surface that served many answers at once (the
// obligations re-rank) names: the lowest confidence, ties broken by
// question id. ok is false for no rows.
func WeakestRecord(recs []decision.Record) (decision.Record, bool) {
	if len(recs) == 0 {
		return decision.Record{}, false
	}
	sorted := append([]decision.Record(nil), recs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Confidence != sorted[j].Confidence {
			return sorted[i].Confidence < sorted[j].Confidence
		}
		return sorted[i].QuestionID < sorted[j].QuestionID
	})
	return sorted[0], true
}

// MissClass is the degrade class of a failed ask (the degrade line's
// class), "" for no error or a live kill: a killed port was not asked, and
// its surfaces carry no badge (I42's bytes).
func MissClass(err error) string {
	if err == nil || IsKilled(err) {
		return ""
	}
	var de *DegradedError
	if errors.As(err, &de) {
		return degradeClass(de.Err)
	}
	return degradeClass(err)
}

// IsKilled reports whether err is a live kill's (*KilledError).
func IsKilled(err error) bool {
	var ke *KilledError
	return errors.As(err, &ke)
}
