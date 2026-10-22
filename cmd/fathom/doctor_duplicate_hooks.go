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
	present, notes, _, _ := harnessHookEntriesByScope(root, d, specs, parents...)
	var duplicated []string
	for _, s := range specs {
		var user, project bool
		for _, scope := range present[hookSpecKeyOf(s)] {
			user = user || strings.HasPrefix(scope, "user ")
			project = project || strings.HasPrefix(scope, "project ") || strings.HasPrefix(scope, "project-local ")
		}
		if user && project {
			duplicated = append(duplicated, s.event+" ("+s.matcher+", "+s.marker+")")
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
