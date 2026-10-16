package main

// doctor_admission.go — design §1: `[tenancy].admission = "off"` (or
// FATHOM_TENANCY_ADMISSION=off) is the escape hatch, and it must never be
// quiet: serve-http prints a loud startup line and `admission_mode` WARNs
// on every doctor run while the daemon serves with it. Read from the
// daemon's own /metrics gauge (fathom_admission_mode{mode}), so doctor
// reports the mode the daemon actually runs, not what a config file says.

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func checkAdmissionMode(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
	const name = "admission_mode"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot read /metrics (see daemon_reachable)"}
	}
	m, err := fetchMetricsText(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not read %s/metrics: %v", baseURL, err)}
	}
	var modes []string
	for k, v := range m {
		if v == 1 && strings.HasPrefix(k, `fathom_admission_mode{mode="`) {
			modes = append(modes, strings.TrimSuffix(strings.TrimPrefix(k, `fathom_admission_mode{mode="`), `"}`))
		}
	}
	sort.Strings(modes)
	switch {
	case len(modes) == 0:
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "the daemon's /metrics carries no fathom_admission_mode — it predates the admission gate"}
	case modes[0] == "off":
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: "admission is off — no call is decided or refused by class (only the class deadlines apply); the daemon admits everything",
			Remedy: `remove FATHOM_TENANCY_ADMISSION=off / set [tenancy].admission = "observe" (or "enforce") in ~/.fathom/config.toml, then restart the daemon (task deploy)`}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "admission " + modes[0]}
}
