package main

// doctor_brief.go — HP-31: `fathom doctor --brief`, the payload of the
// harness's Setup event. Setup fires once, when a project is initialised
// or maintained, and it is the one moment an adopter is actually looking
// at wiring. A full doctor table there is 40 lines nobody reads; the
// useful answer is the short one — what is BROKEN, at most five lines,
// and exit 0 whatever it finds.
//
// Exit 0 is the contract, not an oversight: this runs as a hook, and a
// setup diagnosis that fails setup is worse than no diagnosis (§5.3,
// I9). `fathom doctor` without --brief keeps the 0/1/2 exit contract.

import (
	"fmt"
	"io"
	"strings"
)

// doctorBriefMaxLines caps the brief at five findings. A repo with more
// than five failing rows has one problem, not six (usually: nothing is
// wired yet), and the last line says how many were elided.
const doctorBriefMaxLines = 5

// doctorBrief renders the FAIL rows of checks as at most
// doctorBriefMaxLines short lines — name, reason, and the remedy where
// there is one. "" when nothing failed: silence is the correct output of
// a clean diagnosis, and a hook that prints "all good" on every setup is
// noise the adopter learns to skip.
func doctorBrief(checks []doctorCheck) string {
	var fails []doctorCheck
	for _, c := range checks {
		if c.Status == doctorFail {
			fails = append(fails, c)
		}
	}
	if len(fails) == 0 {
		return ""
	}
	shown := fails
	elided := 0
	if len(shown) > doctorBriefMaxLines {
		// One line of the budget is spent saying how many were dropped, so
		// the count is never the thing that got truncated away.
		shown = fails[:doctorBriefMaxLines-1]
		elided = len(fails) - len(shown)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "fathom doctor: %d check(s) FAIL\n", len(fails))
	for _, c := range shown {
		line := fmt.Sprintf("- %s: %s", c.Name, oneLine(c.Reason))
		if c.Remedy != "" {
			line += " → " + oneLine(c.Remedy)
		}
		sb.WriteString(line + "\n")
	}
	if elided > 0 {
		fmt.Fprintf(&sb, "- …and %d more — run `fathom doctor` for the full table\n", elided)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// oneLine flattens a reason to a single line and bounds it, so five rows
// stay five lines however verbose a check's prose is.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 160
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// printDoctorBrief is the --brief output path. It always returns 0.
func printDoctorBrief(stdout io.Writer, checks []doctorCheck) int {
	if brief := doctorBrief(checks); brief != "" {
		fmt.Fprintln(stdout, brief)
	}
	return 0
}
