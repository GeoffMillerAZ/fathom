package main

// doctor_concerns.go — CC-6 (docs/plans/custodian.md): doctor's config-
// custodian row, on the capability row's discipline: it is ONE check, and
// zero registered concerns is a passing row that names the absence — a
// repo that never wrote contracts is normal, not broken. Red is reserved
// for the two states that are unambiguously defects: a concerns file that
// does not load, and a concern that is violated (an instruction in the
// config that is no longer true).

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

func checkConcerns(root string) doctorCheck {
	const name = "concerns"

	concerns, loadErrs := loadConcernsBoth(root)
	if len(loadErrs) > 0 {
		return doctorCheck{
			Name:   name,
			Status: doctorFail,
			Reason: loadErrs[0].Error(),
			Remedy: "fix the named concerns.cue, then re-run doctor",
		}
	}
	if len(concerns) == 0 {
		return doctorCheck{
			Name:   name,
			Status: doctorPass,
			Reason: "no concerns registered — write ~/.fathom/concerns.cue (user default) or .fathom/concerns.cue (repo governs) to hold this repo's config to contracts",
		}
	}

	findings := custodian.Audit(root, concerns)
	var holds, unable, off int
	var violated []custodian.Finding
	for _, f := range findings {
		switch f.Status {
		case custodian.StatusViolated:
			violated = append(violated, f)
		case custodian.StatusHolds:
			holds++
		case custodian.StatusUnable:
			unable++
		case custodian.StatusOff:
			off++
		}
	}

	if len(violated) > 0 {
		first := violated[0]
		reason := fmt.Sprintf("%d concern(s) violated — %s: %s", len(violated), first.Concern.ID, first.Detail)
		return doctorCheck{
			Name:   name,
			Status: doctorFail,
			Reason: reason,
			Remedy: "fathom config audit — evidence per concern; fathom reports, you decide what to edit",
		}
	}

	parts := []string{fmt.Sprintf("%d hold", holds)}
	if off > 0 {
		parts = append(parts, fmt.Sprintf("%d off", off))
	}
	if unable > 0 {
		parts = append(parts, fmt.Sprintf("%d unable to check", unable))
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: strings.Join(parts, "; ")}
}
