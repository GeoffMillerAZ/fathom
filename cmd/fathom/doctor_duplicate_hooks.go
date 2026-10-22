package main

import (
	"context"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func checkDuplicateHookRegistration(root, label string, parents ...context.Context) doctorCheck {
	const name = "duplicate_hook_registration"
	d, ok := harness.Lookup(doctorHarnessID(label))
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "unknown harness"}
	}
	specs := hookSpecsFor(d.ID, hookOptInKeys()...)
	specs = slices.DeleteFunc(specs, func(s hookSpec) bool {
		for _, binding := range d.Hooks {
			if binding.Harness == s.event && binding.Matcher == s.matcher && binding.Marker == s.marker {
				return binding.Vehicle == harness.VehicleHookSilent
			}
		}
		return true
	})
	present, notes, _, _ := harnessHookEntriesByScopeMode(root, d, specs, true, parents...)
	var duplicated []string
	for _, s := range specs {
		user, project := map[string]bool{}, map[string]bool{}
		for _, entry := range present[hookSpecKeyOf(s)] {
			scope, command, _ := strings.Cut(entry, "\x00")
			if strings.HasPrefix(scope, "user ") {
				user[command] = true
			}
			if strings.HasPrefix(scope, "project ") || strings.HasPrefix(scope, "project-local ") {
				project[command] = true
			}
		}
		for command := range user {
			if project[command] {
				duplicated = append(duplicated, s.event+" ("+s.matcher+", "+command+")")
			}
		}
	}
	if len(duplicated) > 0 {
		slices.Sort(duplicated)
		duplicated = slices.Compact(duplicated)
		return doctorCheck{Name: name, Status: doctorWarn, Reason: "same fathom hook registered at user and project scope; duplicate injections: " + strings.Join(duplicated, "; "), Remedy: "remove the duplicate fathom rows from one scope (keep either user or project registration), then re-run doctor"}
	}
	if len(notes) > 0 {
		return doctorCheck{Name: name, Status: doctorUnknown, Reason: strings.Join(notes, "; "), Remedy: "make the named hook files readable, then re-run doctor"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "no fathom event/matcher/command is registered at both user and project scope"}
}
