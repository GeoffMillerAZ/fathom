package decision_test

import (
	"math"
	"testing"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// siteSamples builds a fixture calibration set with the given (n, correct)
// per band, at one confidence inside each band, all human-labelled.
func siteSamples(unsureN, unsureOK, confirmN, confirmOK, proceedN, proceedOK int) []decision.LabeledSample {
	var out []decision.LabeledSample
	out = append(out, rows(unsureN, unsureOK, 0.30)...)
	out = append(out, rows(confirmN, confirmOK, 0.70)...)
	out = append(out, rows(proceedN, proceedOK, 0.95)...)
	return out
}

// siteReading is NewReading over siteSamples for testKey; it fails t on error.
func siteReading(t *testing.T, unsureN, unsureOK, confirmN, confirmOK, proceedN, proceedOK int) decision.Reading {
	t.Helper()
	r, err := decision.NewReading(testKey, siteSamples(unsureN, unsureOK, confirmN, confirmOK, proceedN, proceedOK))
	if err != nil {
		t.Fatalf("NewReading: %v", err)
	}
	return r
}

func TestCalibratedBandFromSiteReading(t *testing.T) {
	// The fixture site: the proceed band is 50 of 50 (Wilson lower bound
	// 0.9287, clears the gate bar), the confirm band 40 of 50 (0.8 point,
	// fails either bar), the unsure band 18 of 60.
	site := siteReading(t, 60, 18, 50, 40, 50, 50)

	cases := []struct {
		name       string
		confidence float64
		reading    decision.Reading
		current    decision.ReadingKey
		bar        decision.CalibrationBar
		want       decision.BandCalibration
	}{
		{
			name: "a passing proceed band is used", confidence: 0.95, reading: site, current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandProceed, Status: decision.CalibrationPass},
		},
		{
			name: "the band boundary is BandOf's: 0.9 is proceed", confidence: 0.9, reading: site, current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandProceed, Status: decision.CalibrationPass},
		},
		{
			name: "a failing confirm band reads as unsure", confidence: 0.7, reading: site, current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandConfirm, Band: decision.BandUnsure, Status: decision.CalibrationFail},
		},
		{
			name: "a failing unsure band stays unsure", confidence: 0.2, reading: site, current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandUnsure, Band: decision.BandUnsure, Status: decision.CalibrationFail},
		},
		{
			name: "30 of 30 cannot reach the gate bar: insufficient n, not a pass", confidence: 0.95,
			reading: siteReading(t, 0, 0, 0, 0, 30, 30), current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationInsufficientN},
		},
		{
			name: "a band with no samples is insufficient n", confidence: 0.7,
			reading: siteReading(t, 0, 0, 0, 0, 50, 50), current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandConfirm, Band: decision.BandUnsure, Status: decision.CalibrationInsufficientN},
		},
		{
			name: "the hint bar needs n of at least 100", confidence: 0.95, reading: site, current: testKey, bar: decision.HintBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationInsufficientN},
		},
		{
			name: "the hint bar passes 90 of 100 at the confidence shown", confidence: 0.95,
			reading: siteReading(t, 0, 0, 0, 0, 100, 90), current: testKey, bar: decision.HintBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandProceed, Status: decision.CalibrationPass},
		},
		{
			name: "the hint bar fails 89 of 100", confidence: 0.95,
			reading: siteReading(t, 0, 0, 0, 0, 100, 89), current: testKey, bar: decision.HintBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationFail},
		},
		{
			name: "90 of 100 passes the hint bar but not the gate bar", confidence: 0.95,
			reading: siteReading(t, 0, 0, 0, 0, 100, 90), current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationFail},
		},
		{
			name: "a passing confirm band is used but never promoted to proceed", confidence: 0.7,
			reading: siteReading(t, 0, 0, 50, 50, 0, 0), current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandConfirm, Band: decision.BandConfirm, Status: decision.CalibrationPass},
		},
		{
			name: "a passing unsure band is calibrated and still unsure", confidence: 0.3,
			reading: siteReading(t, 50, 50, 0, 0, 0, 0), current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandUnsure, Band: decision.BandUnsure, Status: decision.CalibrationPass},
		},
		{
			name: "a moved resolved model makes a passing band recalibration-due", confidence: 0.95, reading: site,
			current: withKey(testKey, func(k *decision.ReadingKey) { k.ResolvedModel = "model-1.14.0" }), bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationRecalibrationDue},
		},
		{
			name: "another site's reading is recalibration-due", confidence: 0.95, reading: site,
			current: withKey(testKey, func(k *decision.ReadingKey) { k.Site = "route_request" }), bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationRecalibrationDue},
		},
		{
			name: "the zero Reading is insufficient n for its own key", confidence: 0.95,
			reading: decision.Reading{Key: testKey}, current: testKey, bar: decision.GateBar(),
			want: decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationInsufficientN},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decision.CalibratedBand(c.confidence, c.reading, c.current, c.bar)
			if got != c.want {
				t.Errorf("CalibratedBand(%v) = %+v, want %+v", c.confidence, got, c.want)
			}
			if got.Calibrated() != (c.want.Status == decision.CalibrationPass) {
				t.Errorf("Calibrated() = %v for status %s", got.Calibrated(), got.Status)
			}
			if got.Raw != decision.BandOf(c.confidence) {
				t.Errorf("Raw = %s, want BandOf's %s", got.Raw, decision.BandOf(c.confidence))
			}
		})
	}
}

// Everything malformed fails closed: an unusable confidence, bar or reading
// is never calibrated and never promoted.
func TestCalibratedBandRefusesMalformedInputAsUnsure(t *testing.T) {
	site := siteReading(t, 0, 0, 0, 0, 50, 50)
	unsureInvalid := decision.BandCalibration{Raw: decision.BandUnsure, Band: decision.BandUnsure, Status: decision.CalibrationInvalid}
	proceedInvalid := decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationInvalid}

	for _, conf := range []float64{math.NaN(), -0.01, 1.01, math.Inf(1), math.Inf(-1)} {
		if got := decision.CalibratedBand(conf, site, testKey, decision.GateBar()); got != unsureInvalid {
			t.Errorf("CalibratedBand(%v) = %+v, want %+v", conf, got, unsureInvalid)
		}
	}

	bars := []struct {
		name string
		bar  decision.CalibrationBar
	}{
		{"the zero bar states nothing", decision.CalibrationBar{}},
		{"a MinN-only bar states nothing", decision.CalibrationBar{MinN: 100}},
		{"negative MinN", decision.CalibrationBar{MinN: -1, MinWilsonLo: 0.9}},
		{"MinWilsonLo above 1", decision.CalibrationBar{MinWilsonLo: 1.1}},
		{"MinAccuracy below 0", decision.CalibrationBar{MinAccuracy: -0.1, MinWilsonLo: 0.9}},
		{"NaN MinAccuracy", decision.CalibrationBar{MinAccuracy: math.NaN()}},
		{"NaN MinWilsonLo", decision.CalibrationBar{MinWilsonLo: math.NaN()}},
	}
	for _, b := range bars {
		t.Run(b.name, func(t *testing.T) {
			if got := decision.CalibratedBand(0.95, site, testKey, b.bar); got != proceedInvalid {
				t.Errorf("CalibratedBand(bar %+v) = %+v, want %+v", b.bar, got, proceedInvalid)
			}
		})
	}

	// A reading whose counts cannot be (hand-built or decoded from JSON) is
	// refused, never trusted for its stored bounds.
	lo, hi := 0.99, 1.0
	impossible := decision.Reading{Key: testKey, ByBand: []decision.BandReading{
		{Band: decision.BandProceed, N: 10, Correct: 50, WilsonLo: &lo, WilsonHi: &hi},
	}}
	if got := decision.CalibratedBand(0.95, impossible, testKey, decision.GateBar()); got != proceedInvalid {
		t.Errorf("CalibratedBand(Correct > N) = %+v, want %+v", got, proceedInvalid)
	}
	duplicated := site
	duplicated.ByBand = append(append([]decision.BandReading(nil), site.ByBand...), site.ByBand[2])
	if got := decision.CalibratedBand(0.95, duplicated, testKey, decision.GateBar()); got != proceedInvalid {
		t.Errorf("CalibratedBand(a band listed twice) = %+v, want %+v", got, proceedInvalid)
	}
}

// The bounds a band clears are recomputed from its counts: a stored
// WilsonLo that flatters the band is ignored.
func TestCalibratedBandRecomputesTheBoundFromCounts(t *testing.T) {
	flattering := 0.99
	r := decision.Reading{Key: testKey, ByBand: []decision.BandReading{
		{Band: decision.BandProceed, N: 41, Correct: 38, WilsonLo: &flattering, WilsonHi: &flattering},
	}}
	want := decision.BandCalibration{Raw: decision.BandProceed, Band: decision.BandUnsure, Status: decision.CalibrationFail}
	if got := decision.CalibratedBand(0.95, r, testKey, decision.GateBar()); got != want {
		t.Errorf("CalibratedBand(38 of 41 with a stored 0.99 bound) = %+v, want %+v (38/41 bounds at 0.806)", got, want)
	}
}

// The bars are the guide's (docs/fast-judgment.md section 3, step 6).
func TestCalibrationBarsAreTheGuides(t *testing.T) {
	if got, want := decision.HintBar(), (decision.CalibrationBar{MinN: 100, MinAccuracy: 0.90}); got != want {
		t.Errorf("HintBar() = %+v, want %+v", got, want)
	}
	if got, want := decision.GateBar(), (decision.CalibrationBar{MinWilsonLo: 0.90}); got != want {
		t.Errorf("GateBar() = %+v, want %+v", got, want)
	}
}
