package main

// doctor_tenancy.go — doctor's tenancy rows (RT-50, design §13.3):
// daemon_lifecycle, host_pressure, scratch_daemons and tenancy_scope.
// The first three read the resources projection (resources.go) doctor
// builds once; tenancy_scope reads the config layers. Each WARNs, never
// FAILs (doctor.go's honesty discipline: an operator signal, not a
// wiring-broken verdict), and each remedy names a verb, never "restart".

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// checkDaemonLifecycle is daemon_lifecycle: starts and exits from the
// lifecycle ledger, the switch files, quarantine.
func checkDaemonLifecycle(rep resourcesReport) doctorCheck {
	const name = "daemon_lifecycle"
	l := rep.Lifecycle
	summary := fmt.Sprintf("%s · %d over-budget exits/24h · switch %s · %d quarantined [%s]",
		lifecycleStartsText(l), l.OverBudget24h, l.Switch, len(rep.Quarantined), l.Ledger)
	if l.NotRecorded != "" {
		summary += " — " + l.NotRecorded
	}
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
	tr := rep.Triggers
	fb := tenancy.FormatBytes
	reason := fmt.Sprintf("pressure %s · swap %s/%s · swap volume %s free [%s sample]", h.Pressure, fb(h.SwapUsed), fb(h.SwapTotal), fb(h.SwapVolumeFree), rep.Tenancy.Source)
	if rep.Footprint.Stale {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no current host reading: the ring's last sample is %s old (the daemon is not sampling); it read %s", rep.Footprint.SampleAge, reason)}
	}
	at := func(p string) string {
		if p == "off" {
			return "never"
		}
		return "at " + p
	}
	var problems, remedies []string
	if h.Pressure == tenancy.PressureCritical.String() || h.Pressure == tenancy.PressureWarn.String() {
		sheds := at(tr.ShedOnPressure)
		if tr.ShedOnPressure != "off" && tr.Allowance > 0 {
			sheds += " once its footprint reaches " + fb(tr.Allowance)
		}
		problems = append(problems, fmt.Sprintf("the machine is short of memory; fathom sheds %s and refuses heavy work %s", sheds, at(tr.RefuseOnPressure)))
	}
	if h.SwapVolumeFree >= 0 && tr.MinSwapVolumeFree > 0 && h.SwapVolumeFree < tr.MinSwapVolumeFree {
		problems = append(problems, fmt.Sprintf("under %s free on the swap volume, fathom's refuse trigger (min_swap_volume_free)", fb(tr.MinSwapVolumeFree)))
		remedies = append(remedies, "free space on the swap volume")
	}
	if len(problems) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: fmt.Sprintf("%s — %s [triggers: %s]", reason, strings.Join(problems, "; "), tr.Source),
			Remedy: strings.Join(append(remedies, "fathom resources"), "; ")}
	}
	switch {
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
	if s.Kept > 0 {
		reason += fmt.Sprintf(" · %d kept by reap (no record under this HOME, parent alive)", s.Kept)
	}
	if s.Leaked == 0 && s.Stale == 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}
	if len(s.Rows) > 0 {
		reason += ": " + strings.Join(s.Rows, "; ")
	}
	return doctorCheck{Name: name, Status: doctorWarn, Reason: reason, Remedy: "fathom daemon reap --apply (kills exactly the leaked daemons counted here — never the managed one, nor one whose record lives under another HOME while its parent is alive; prunes stale records)"}
}

// checkTenancyScope is tenancy_scope: [tenancy] is read at user scope
// only (§1); a repository's table is ignored and named, and an env
// override is disclosed. userPath is the user config (config.DefaultUserPath).
func checkTenancyScope(root, userPath string, getenv func(string) string) doctorCheck {
	const name = "tenancy_scope"
	projPath := projectdir.Config.Path(root)
	loaded, err := config.Load(userPath, projPath)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "config does not load: " + err.Error(), Remedy: "see config_valid"}
	}
	_, notes, err := config.ResolveTenancyConfigNotes(loaded, getenv)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "[tenancy] does not resolve: " + err.Error(), Remedy: "fix the named key in " + userPath}
	}
	// Judged by the note's kind (design §1): an ignored repository table,
	// admission off and a memory_limit/gc_hint conflict each WARN; an env
	// override or the migration is disclosed.
	var warns, other, remedies []string
	for _, n := range notes {
		switch n.Kind {
		case config.NoteProjectIgnored:
			warns = append(warns, n.Text)
			remedies = append(remedies, "move the budget to ~/.fathom/config.toml [tenancy] (a machine setting) and delete the repository's table")
		case config.NoteAdmissionOff:
			warns = append(warns, n.Text)
			remedies = append(remedies, "unset FATHOM_TENANCY_ADMISSION (or [tenancy].admission) once the emergency is over")
		case config.NoteConflict:
			warns = append(warns, n.Text)
			remedies = append(remedies, "remove [daemon].memory_limit / FATHOM_MEMORY_LIMIT")
		default:
			other = append(other, n.Text)
		}
	}
	reason := "[tenancy] is read at user scope only (" + userPath + ") and env"
	if len(other) > 0 {
		reason += "; " + strings.Join(other, "; ")
	}
	if len(warns) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: strings.Join(warns, "; ") + " — " + reason, Remedy: strings.Join(remedies, "; ")}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
