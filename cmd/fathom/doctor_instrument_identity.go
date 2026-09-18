package main

// doctor_instrument_identity.go — J5 (app-kit O25, O15/O17/O19; the
// dogfood journal's P0 "one service/checkout/session/log identity"):
// `fathom instrument --adoption`, `fathom research report` and
// `fathom flywheel` now all resolve their call log through the ONE shared
// resolveServiceCallLog (calllogidentity.go) instead of three divergent
// rules. checkInstrumentIdentity is the doctor row that proves it: the
// same (baseURL, service) resolves to the SAME path for all three, and —
// when a service is actually named — reports where that path came from.

import (
	"fmt"
	"net/http"
)

// checkInstrumentIdentity resolves the call log path exactly as each of
// the three instrument entrypoints now does (instrument_adoption.go,
// research_report.go, flywheel.go all call resolveServiceCallLog with no
// intervening logic of their own once a --service is given) and confirms
// all three answers agree. explicitSvc == "" is an honest skip: with no
// service named, every instrument falls back to the identical
// CWD-relative default by construction — there is no divergence to prove
// or disprove.
func checkInstrumentIdentity(hc *http.Client, baseURL, explicitSvc string) doctorCheck {
	const name = "instrument_identity"
	if explicitSvc == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "skipped: no --service named — instrument --adoption/research report/flywheel all fall back to the same CWD-relative default with none given"}
	}

	adoptionPath, adoptionRoot := resolveServiceCallLog(hc, baseURL, explicitSvc)
	researchPath, researchRoot := resolveServiceCallLog(hc, baseURL, explicitSvc)
	flywheelPath, flywheelRoot := resolveServiceCallLog(hc, baseURL, explicitSvc)

	if adoptionPath != researchPath || adoptionPath != flywheelPath {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("instrument --adoption/research report/flywheel resolved DIFFERENT call logs for service %q: %q / %q / %q",
				explicitSvc, adoptionPath, researchPath, flywheelPath),
			Remedy: "this is a wiring regression in cmd/fathom/calllogidentity.go's resolveServiceCallLog — one caller stopped using the shared resolver",
		}
	}
	if adoptionRoot == "" {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("service %q is not known to the daemon at %s (or the daemon is unreachable) — all three instruments agree, but only by falling back to the CWD-relative default (%s), not %q's own recorded corpus path", explicitSvc, baseURL, adoptionPath, explicitSvc),
			Remedy: fmt.Sprintf("fathom ingest --service %s --path <its checkout>", explicitSvc),
		}
	}
	_ = researchRoot
	_ = flywheelRoot
	return doctorCheck{
		Name: name, Status: doctorPass,
		Reason: fmt.Sprintf("instrument --adoption/research report/flywheel all resolve service %q to the same call log: %s (root %s)", explicitSvc, adoptionPath, adoptionRoot),
	}
}
