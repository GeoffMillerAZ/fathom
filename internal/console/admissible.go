package console

// admissible.go — PE-17 (docs/design/progress-and-economics.md §5, §6):
// fathom-on versus off is shown only when G2's admissibility test passes
// for the compared units — otherwise the panel names why it is withheld,
// never a silent absence (I38). G2's threshold (docs/intent/scope.md,
// MI-5) is at least ArmAdmissibleMinUnits arm-homogeneous units in EACH
// arm; below that, "fathom flywheel" itself reports the A/B as
// inadmissible and a savings figure exists only as a range, never a
// point (law 12) — here it does not exist at all until both arms clear
// the bar.

import "fmt"

// ArmAdmissibleMinUnits is G2's own threshold (docs/intent/scope.md):
// "fathom flywheel reports ≥20 arm-homogeneous units per arm (MI-5)".
const ArmAdmissibleMinUnits = 20

// AdmissibleComparison is admissibleComparison's verdict: either the
// comparison may be shown (Admissible; the caller renders its own On/Off
// values), or it may not, and Withheld says why — the two are mutually
// exclusive by construction.
type AdmissibleComparison struct {
	Admissible bool
	OnUnits    int
	OffUnits   int
	Withheld   string
}

// AdmissibleArms is G2's admissibility test for every caller that shows
// fathom-on versus off (PE-17w): `fathom rate`'s registered criterion, the
// observatory's pilot note and its readers (the brief, the impact view)
// read the verdict only when it passes, and name Withheld otherwise.
func AdmissibleArms(onUnits, offUnits int) AdmissibleComparison {
	return admissibleComparison(onUnits, offUnits)
}

// admissibleComparison applies G2's admissibility test to onUnits and
// offUnits — the arm-homogeneous unit counts a caller (ArmInputs.On/Off
// today; a cost-per-proven-row arm once PE-15/PE-16 feed one) already
// computed. Below threshold on either side, Withheld names the short arm
// and the threshold it fell short of, so the panel can say why rather
// than showing nothing unexplained.
func admissibleComparison(onUnits, offUnits int) AdmissibleComparison {
	c := AdmissibleComparison{OnUnits: onUnits, OffUnits: offUnits}
	onShort, offShort := onUnits < ArmAdmissibleMinUnits, offUnits < ArmAdmissibleMinUnits
	switch {
	case !onShort && !offShort:
		c.Admissible = true
	case onShort && offShort:
		c.Withheld = fmt.Sprintf("G2 not yet admissible (docs/intent/scope.md): %d ON-arm and %d OFF-arm unit(s), both short of the %d arm-homogeneous units/arm threshold (MI-5)",
			onUnits, offUnits, ArmAdmissibleMinUnits)
	case onShort:
		c.Withheld = fmt.Sprintf("G2 not yet admissible (docs/intent/scope.md): %d ON-arm unit(s), short of the %d arm-homogeneous units/arm threshold (OFF has %d)",
			onUnits, ArmAdmissibleMinUnits, offUnits)
	default:
		c.Withheld = fmt.Sprintf("G2 not yet admissible (docs/intent/scope.md): %d OFF-arm unit(s), short of the %d arm-homogeneous units/arm threshold (ON has %d)",
			offUnits, ArmAdmissibleMinUnits, onUnits)
	}
	return c
}
