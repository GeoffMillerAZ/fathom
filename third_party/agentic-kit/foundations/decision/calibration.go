package decision

import "math"

// CalibrationBar is the bar a confidence band's reading must clear before
// answers in that band may be acted on, stated per band (docs/fast-judgment.md
// section 3, step 6). A band passes when it has at least MinN samples, its
// accuracy (Correct / N) is at least MinAccuracy, and the lower bound of its
// Wilson 95% interval is at least MinWilsonLo.
//
// A bar must state an accuracy: MinAccuracy or MinWilsonLo above zero. The
// zero bar, or one with only MinN, states nothing and calibrates nothing.
// HintBar and GateBar are the guide's two bars.
type CalibrationBar struct {
	// MinN is the fewest samples the band needs; 0 means no count bar of
	// its own (a Wilson bar still implies one).
	MinN int
	// MinAccuracy is the point-estimate accuracy the band must reach, in
	// [0, 1]; 0 means no point-estimate bar.
	MinAccuracy float64
	// MinWilsonLo is the Wilson 95% lower bound the band's accuracy must
	// reach, in [0, 1]; 0 means no interval bar.
	MinWilsonLo float64
}

// HintBar is the bar for a hint shown as calibrated: at least 90 of 100
// right at the confidence shown (a point estimate, n at least 100).
func HintBar() CalibrationBar { return CalibrationBar{MinN: 100, MinAccuracy: 0.90} }

// GateBar is the bar for an answer that prunes, gates or reorders anything
// served by default: a Wilson 95% lower bound of at least 0.90. No band
// under 35 samples can reach it, and 30 of 30 bounds at 0.8865.
func GateBar() CalibrationBar { return CalibrationBar{MinWilsonLo: 0.90} }

// CalibrationStatus says whether, and why not, a confidence's band is
// calibrated for a site. Every status but CalibrationPass reads as unsure.
type CalibrationStatus string

// The calibration statuses. The first three follow the calibration record
// template's verdict (docs/fast-judgment.md section 3).
const (
	// CalibrationPass means the band's reading clears the bar.
	CalibrationPass CalibrationStatus = "pass"
	// CalibrationFail means the band has enough samples to be judged and
	// falls below the bar.
	CalibrationFail CalibrationStatus = "fail"
	// CalibrationInsufficientN means the band has too few samples to clear
	// the bar even if every one were right: none, fewer than MinN, or too
	// few for the Wilson lower bound of n of n to reach MinWilsonLo.
	CalibrationInsufficientN CalibrationStatus = "insufficient-n"
	// CalibrationRecalibrationDue means the reading is of another site,
	// provider, resolved model or template version (Reading.RecalibrationDue)
	// and says nothing about this one.
	CalibrationRecalibrationDue CalibrationStatus = "recalibration-due"
	// CalibrationInvalid means the confidence, the bar or the band's reading
	// is malformed: a confidence outside [0, 1] or NaN, a bar that states no
	// accuracy or holds a value out of range, or a band whose counts cannot
	// be (Correct above N, a negative count, the band listed twice).
	CalibrationInvalid CalibrationStatus = "invalid"
)

// BandCalibration is the result of CalibratedBand: the band a raw provider
// confidence may be acted on in at one site, and why.
type BandCalibration struct {
	// Raw is BandOf(confidence), the uncalibrated starting band; BandUnsure
	// when the confidence is not a number in [0, 1].
	Raw Band
	// Band is Raw when Status is CalibrationPass and BandUnsure otherwise.
	// It is never above Raw: a reading may confirm a band, never promote a
	// confidence into a higher one.
	Band Band
	// Status says whether the band is calibrated, and if not, why.
	Status CalibrationStatus
}

// Calibrated reports whether Band came from a reading that clears the bar.
func (c BandCalibration) Calibrated() bool { return c.Status == CalibrationPass }

// CalibratedBand maps a raw provider confidence to the band it may be acted
// on in at one site, using that site's Reading. The band is BandOf's, used
// only where the reading of that band clears bar; anywhere else the answer
// reads as BandUnsure and is never promoted:
//
//   - a malformed confidence, bar or band reading is CalibrationInvalid;
//   - a reading whose key differs from current (another site, or a moved
//     provider, resolved model or template version) is
//     CalibrationRecalibrationDue, however well the band did;
//   - a band with too few samples to clear the bar at all is
//     CalibrationInsufficientN;
//   - a band below the bar is CalibrationFail.
//
// The band's accuracy and Wilson bound are recomputed from its N and
// Correct; the bounds stored in the reading are never trusted, so a reading
// decoded from JSON cannot flatter itself.
//
// CalibratedBand is pure: no I/O, no clock, no table, no cache. BandOf,
// UnsureBelow and ProceedAtOrAbove are unchanged; they stay the
// uncalibrated starting bands every reading is split by.
func CalibratedBand(confidence float64, r Reading, current ReadingKey, bar CalibrationBar) BandCalibration {
	unsure := func(raw Band, s CalibrationStatus) BandCalibration {
		return BandCalibration{Raw: raw, Band: BandUnsure, Status: s}
	}
	if !validBar(bar) || math.IsNaN(confidence) || confidence < 0 || confidence > 1 {
		raw := BandUnsure
		if !math.IsNaN(confidence) && confidence >= 0 && confidence <= 1 {
			raw = BandOf(confidence)
		}
		return unsure(raw, CalibrationInvalid)
	}
	raw := BandOf(confidence)
	if r.RecalibrationDue(current) {
		return unsure(raw, CalibrationRecalibrationDue)
	}

	var band BandReading
	found := 0
	for _, b := range r.ByBand {
		if b.Band == raw {
			band = b
			found++
		}
	}
	switch {
	case found > 1, band.N < 0, band.Correct < 0, band.Correct > band.N:
		return unsure(raw, CalibrationInvalid)
	case band.N == 0 || band.N < bar.MinN:
		return unsure(raw, CalibrationInsufficientN)
	}
	if best, _ := wilson95(band.N, band.N); best < bar.MinWilsonLo {
		return unsure(raw, CalibrationInsufficientN)
	}
	lo, _ := wilson95(band.Correct, band.N)
	if float64(band.Correct)/float64(band.N) < bar.MinAccuracy || lo < bar.MinWilsonLo {
		return unsure(raw, CalibrationFail)
	}
	return BandCalibration{Raw: raw, Band: raw, Status: CalibrationPass}
}

// validBar reports whether bar states an accuracy and holds no value out of
// range.
func validBar(bar CalibrationBar) bool {
	in01 := func(v float64) bool { return !math.IsNaN(v) && v >= 0 && v <= 1 }
	return bar.MinN >= 0 && in01(bar.MinAccuracy) && in01(bar.MinWilsonLo) &&
		(bar.MinAccuracy > 0 || bar.MinWilsonLo > 0)
}
