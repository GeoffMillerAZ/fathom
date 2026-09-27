package main

// doctor_tenancy.go — doctor's tenancy rows (RT-50, design §13.3):
// daemon_lifecycle, host_pressure, scratch_daemons and tenancy_scope.
// The first three read the resources projection (resources.go) doctor
// builds once; tenancy_scope reads the config layers. Each WARNs, never
// FAILs (doctor.go's honesty discipline: an operator signal, not a
// wiring-broken verdict), and each remedy names a verb, never "restart".

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// checkDaemonLifecycle is daemon_lifecycle: starts and exits from the
// lifecycle ledger, the switch files, quarantine.
func checkDaemonLifecycle(rep resourcesReport) doctorCheck {
	const name = "daemon_lifecycle"
	l := rep.Lifecycle
	summary := fmt.Sprintf("%d starts/24h · %d unclean · %d over-budget exits/24h · switch %s · %d quarantined [%s]",
		l.Starts24h, l.Unclean24h, l.OverBudget24h, l.Switch, len(rep.Quarantined), l.Ledger)
	if l.Uptime != "" {
		summary = "up " + l.Uptime + " · " + summary
	}
	var problems, remedies []string
	for _, u := range rep.Unreadable {
		if strings.HasPrefix(u, "lifecycle ledger") {
			problems = append(problems, "unreadable: "+u)
		}
	}
	if l.CrashLooping {
		problems = append(problems, fmt.Sprintf("crash-looping: %d counted exits in 30 min (last: %s)", l.CountedExits30m, l.LastExit))
	}
	if l.BreakerStopped {
		problems = append(problems, "stopped by the breaker")
		remedies = append(remedies, "read `fathom resources`, then (operator) `fathom daemon start`")
	} else if strings.HasPrefix(l.Switch, "stopped") || strings.HasPrefix(l.Switch, "paused") {
		problems = append(problems, l.Switch)
		remedies = append(remedies, "`fathom daemon start` / `fathom daemon resume` when intended")
	}
	if l.Unclean24h > 0 && !l.CrashLooping {
		problems = append(problems, fmt.Sprintf("%d unclean exit(s) in 24 h", l.Unclean24h))
	}
	if l.OverBudget24h > 0 && !l.CrashLooping {
		problems = append(problems, fmt.Sprintf("%d over-budget exit(s) in 24 h", l.OverBudget24h))
	}
	for _, q := range rep.Quarantined {
		problems = append(problems, q)
	}
	if len(rep.Quarantined) > 0 {
		remedies = append(remedies, "call the daemon's `ingest` tool for each quarantined service once the cause is understood")
	}
	if len(problems) == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: summary}
	}
	remedies = append(remedies, "fathom resources")
	return doctorCheck{Name: name, Status: doctorWarn, Reason: strings.Join(problems, "; ") + " — " + summary, Remedy: strings.Join(remedies, "; ")}
}

// checkHostPressure is host_pressure: the host's memory pressure, swap
// and swap-volume free space in the newest sample. An unknown reading is
// disclosed; the swap-used fraction alone is never a warning (§2.1).
func checkHostPressure(rep resourcesReport) doctorCheck {
	const name = "host_pressure"
	if rep.Tenancy == nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no sample: the daemon did not answer and no resource ring was readable (see daemon_footprint)"}
	}
	h := rep.Host
	fb := tenancy.FormatBytes
	reason := fmt.Sprintf("pressure %s · swap %s/%s · swap volume %s free [%s sample]", h.Pressure, fb(h.SwapUsed), fb(h.SwapTotal), fb(h.SwapVolumeFree), rep.Tenancy.Source)
	switch {
	case h.Pressure == tenancy.PressureCritical.String() || h.Pressure == tenancy.PressureWarn.String():
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason + " — the machine is short of memory; fathom sheds at warn and refuses heavy work at critical", Remedy: "fathom resources"}
	case h.SwapVolumeFree >= 0 && h.SwapVolumeFree < 5*tenancy.GiB:
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason + " — under 5 GiB free on the swap volume, fathom's refuse trigger", Remedy: "free space on the swap volume; fathom resources"}
	case h.Pressure == tenancy.PressureUnknown.String():
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: reason + " — pressure is not readable on this platform (disclosed, never a trigger)"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// checkScratchDaemons is scratch_daemons: the census's unmanaged daemons
// whose verdict is not ok, and stale records.
func checkScratchDaemons(rep resourcesReport) doctorCheck {
	const name = "scratch_daemons"
	s := rep.Scratch
	switch {
	case !s.Taken:
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "the daemon census was not taken"}
	case s.Error != "":
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "daemon census failed: " + s.Error, Remedy: "fathom daemon ps"}
	}
	reason := fmt.Sprintf("%d unmanaged serve-http daemon(s), %d leaked, %d stale record(s) [ps + daemon records]", s.Unmanaged, s.Leaked, s.Stale)
	if s.Leaked == 0 && s.Stale == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}
	if len(s.Rows) > 0 {
		reason += ": " + strings.Join(s.Rows, "; ")
	}
	return doctorCheck{Name: name, Status: doctorWarn, Reason: reason, Remedy: "fathom daemon reap --apply (kills only unmanaged daemons whose verdict is not ok; prunes stale records)"}
}

// checkTenancyScope is tenancy_scope: [tenancy] is read at user scope
// only (§1); a repository's table is ignored and named, and an env
// override is disclosed. userPath is the user config (config.DefaultUserPath).
func checkTenancyScope(root, userPath string, getenv func(string) string) doctorCheck {
	const name = "tenancy_scope"
	projPath := filepath.Join(root, ".fathom", "config.toml")
	loaded, err := config.Load(userPath, projPath)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "config does not load: " + err.Error(), Remedy: "see config_valid"}
	}
	_, notes, err := config.ResolveTenancyConfig(loaded, getenv)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "[tenancy] does not resolve: " + err.Error(), Remedy: "fix the named key in " + userPath}
	}
	var ignored, other []string
	for _, n := range notes {
		if strings.Contains(n, "ignored") {
			ignored = append(ignored, n)
		} else {
			other = append(other, n)
		}
	}
	reason := "[tenancy] is read at user scope only (" + userPath + ") and env"
	if len(other) > 0 {
		reason += "; " + strings.Join(other, "; ")
	}
	if len(ignored) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: strings.Join(ignored, "; ") + " — " + reason,
			Remedy: "move the budget to ~/.fathom/config.toml [tenancy] (a machine setting) and delete the repository's table"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
