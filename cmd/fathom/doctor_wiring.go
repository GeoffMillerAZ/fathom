package main

// doctor_wiring.go — PC-29 (docs/plans/proof-credibility.md, vision 26
// §1.4): one doctor row, `wiring_drift`, over everything an agent could be
// made to change and fathom baselines — hooks, stanza regions, MCP server
// entries, owned skills. PASS at zero drift, FAIL naming each drift by kind
// and path, SKIP with the reason when a file could not be read (I38: an
// unreadable file is never reported current).

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func checkWiringDrift(root, harnessLabel string) doctorCheck {
	const name = "wiring_drift"
	var ids []string
	if id, ok := harnessIDFor(harnessLabel); ok {
		ids = []string{id}
	}
	rep := wiringDriftFor(root, "", ids, wiringDriftHome())
	unreadable := ""
	if n := len(rep.Unreadable); n > 0 {
		unreadable = fmt.Sprintf("%d file(s) could NOT BE READ and may carry drift: %s", n, strings.Join(rep.Unreadable, "; "))
	}
	if len(rep.Drifts) > 0 {
		reason := fmt.Sprintf("%d drift(s): %s", len(rep.Drifts), strings.Join(rep.Lines(), "; "))
		if unreadable != "" {
			reason += "; " + unreadable
		}
		remedy := "read the named file — fathom will not merge a changed entry (law 15); if the change is yours, `fathom fleet reconcile --repo " + root + " --apply --rebaseline` records the new baseline"
		for _, d := range rep.Drifts {
			if d.Kind == wiring.DriftKindSkill {
				// FA-24: rebaseline records a skill only as a render fathom shipped.
				remedy += "; a changed skill file is recorded only when its content is a render fathom shipped — delete its fathom:owned line to keep your edit, or remove the file to take fathom's"
				break
			}
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}
	}
	if unreadable != "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: unreadable + " — cannot conclude the wiring is current", Remedy: "make the file readable and re-run; on an external volume this is usually a wedged TCC verdict"}
	}
	reason := fmt.Sprintf("no drift across %d checked entr%s", rep.Checked, plural2(rep.Checked))
	if n := len(rep.Unrecorded); n > 0 {
		reason += fmt.Sprintf("; %d entr%s with no recorded baseline yet (%s) — `fathom fleet reconcile --repo %s --apply` records them", n, plural2(n), strings.Join(rep.Unrecorded, ", "), root)
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

func plural2(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// harnessIDFor maps doctor's label ("claude", "codex") back to a registry
// id; "" / unknown means every descriptor's files are checked.
func harnessIDFor(label string) (string, bool) {
	switch label {
	case "claude":
		return "claude-code", true
	case "codex":
		return "codex", true
	}
	return "", false
}
