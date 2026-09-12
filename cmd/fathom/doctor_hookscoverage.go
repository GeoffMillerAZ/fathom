package main

// doctor_hookscoverage.go — HP-32: the `hooks_coverage` row. Every other
// hook row answers "is what fathom wires actually wired and firing?".
// This one answers the question an adopter asks next and nothing could
// answer before: "what does my harness offer that fathom is NOT using,
// and why not?"
//
// Advisory by construction — it NEVER returns doctorFail. An unwired
// event is a decision recorded in the descriptor's event catalog, not a
// defect; a row that could fail the gate would turn every future harness
// release into a red build the day its changelog adds an event.

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// hooksCoverageListed bounds how many unwired events the row names
// inline. The rest are counted and pointed at `fathom harness
// capabilities --json`, which carries the whole catalog with reasons.
const hooksCoverageListed = 4

// checkHooksCoverage reports, for the detected harness, how many of the
// events it offers fathom wires — and names the unwired ones with the
// reason the descriptor records for each. PASS when the descriptor
// carries a catalog (the coverage is known and stated); SKIPPED when it
// carries none (fathom has not catalogued that harness's event surface —
// an honest "unknown", never a guessed completeness claim).
func checkHooksCoverage(harnessLabel string) doctorCheck {
	const name = "hooks_coverage"
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no registry descriptor for %q — no event surface to compare against", harnessLabel)}
	}
	if len(d.Events) == 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("%s's event surface is not catalogued in this build — fathom wires %d event(s) here, but what the harness offers beyond them is unknown, and an uncounted surface must not be printed as full coverage", d.Display, len(d.WiredEventNames())),
			Remedy: "catalogue it: add Events to the descriptor from the harness's own hooks reference (internal/harness/eventcatalog.go)",
		}
	}
	unwired := d.UnwiredEvents()
	wired := len(d.Events) - len(unwired)

	reason := fmt.Sprintf("%s offers %d event(s); fathom wires %d", d.Display, len(d.Events), wired)
	if len(unwired) == 0 {
		reason += " — every event is wired"
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}
	listed := unwired
	if len(listed) > hooksCoverageListed {
		listed = listed[:hooksCoverageListed]
	}
	var parts []string
	for _, e := range listed {
		parts = append(parts, fmt.Sprintf("%s (%s)", e.Event, oneLine(e.Reason)))
	}
	reason += fmt.Sprintf(" and leaves %d unwired, each by decision: %s", len(unwired), strings.Join(parts, "; "))
	if n := len(unwired) - len(listed); n > 0 {
		reason += fmt.Sprintf("; and %d more", n)
	}
	reason += " — advisory: an unwired event is a choice, never a defect"
	return doctorCheck{
		Name: name, Status: doctorPass, Reason: reason,
		Remedy: "the whole catalog with reasons: fathom harness capabilities --harness " + d.ID + " --json",
	}
}
