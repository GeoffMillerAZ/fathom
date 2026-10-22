package main

// doctor_capabilities.go — CR-6: doctor's capability section, the answer to
// the most common adoption question ("why isn't X happening here?").
//
// Two design decisions worth defending:
//
// It is ONE check, not one per capability. Doctor's existing rows diagnose
// WIRING — things that are broken and should be fixed. A capability that is
// off is usually a choice, and turning nine "you could also enable this"
// rows into nine failures would make doctor cry wolf and train people to
// ignore it. The row passes whenever nothing is broken, and reports what is
// available in its reason.
//
// It never FAILS on an off capability. Only a capability that is switched
// ON and yet not operating is a defect — that is the config-says-yes,
// reality-says-no case a configuration check cannot see, and it is the only
// state here that deserves a red mark.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

func checkCapabilities(baseURL, root, explicitSvc string, parents ...context.Context) doctorCheck {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	const name = "capabilities"

	reg := capability.Default()
	res, cfgErrs := loadCapabilityConfig(reg, root)
	if len(cfgErrs) > 0 {
		// A config file that does not load is unambiguously broken, and
		// silently ignoring it is the defect class this whole phase exists
		// to remove.
		return doctorCheck{
			Name:   name,
			Status: doctorFail,
			Reason: cfgErrs[0].Error(),
			Remedy: "fix .fathom/capabilities.cue, then re-run doctor",
		}
	}

	probe := buildCapabilityProbeContext(ctx, baseURL, root, explicitSvc, time.Now())
	reports := reg.DetectAll(probe, res)

	var (
		operating []string
		broken    []capability.Report // inputs exist, delivery is failing
		off       []capability.Report // switched off, or its inputs are absent here
	)
	for _, r := range reports {
		switch {
		case r.State.Operating:
			operating = append(operating, r.Capability.ID)
		case r.Enabled && r.State.Broken:
			// Enabled, its inputs exist, and something is failing to
			// deliver them. This is the only state worth a red mark.
			broken = append(broken, r)
		default:
			off = append(off, r)
		}
	}

	if len(broken) > 0 {
		first := broken[0]
		reason := capabilityCount(len(broken)) + " switched on but not operating — " +
			first.Capability.ID + ": " + first.State.Reason
		remedy := first.Remedy.Cmd
		if remedy == "" {
			remedy = first.Remedy.Note
		}
		if len(broken) > 1 {
			remedy += "  (then re-run: " + capabilityCount(len(broken)-1) + " more in the same state)"
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}
	}

	reason := capabilityCount(len(operating)) + " operating"
	if len(off) > 0 {
		ids := make([]string, 0, len(off))
		for _, r := range off {
			ids = append(ids, r.Capability.ID)
		}
		reason += "; off by choice or absence: " + strings.Join(ids, ", ")
	}
	if conflicts := res.Conflicts(); len(conflicts) > 0 {
		parts := make([]string, 0, len(conflicts))
		for _, c := range conflicts {
			parts = append(parts, fmt.Sprintf("%s (capabilities.cue %s, config.toml %s)", c.ID, onOff(c.Cue), onOff(c.Config)))
		}
		return doctorCheck{
			Name:   name,
			Status: doctorWarn,
			Reason: reason + "; capabilities.cue wins over config.toml for " + strings.Join(parts, ", "),
			Remedy: "set each of those once: remove the capabilities.cue entry or the config.toml key",
		}
	}
	return doctorCheck{
		Name:   name,
		Status: doctorPass,
		Reason: reason,
		Remedy: "", // nothing is broken; `fathom capabilities` shows the detail
	}
}

// capabilityCount exists because the shared plural() helper appends a bare
// "s", and "2 capabilitys" is exactly the kind of detail that makes a tool
// read as unfinished.
func capabilityCount(n int) string {
	if n == 1 {
		return "1 capability"
	}
	return fmtThousands(n) + " capabilities"
}
