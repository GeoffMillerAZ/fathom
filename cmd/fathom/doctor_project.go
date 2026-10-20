package main

// doctor_project.go — the one `project` row (docs/design/fathom-dir.md
// section 5, ADR-061): every key of the .fathom/*.cue project value, with
// the file that sets it, its status and its entry count, and every key
// fathom does not know. A new project key adds no doctor row of its own;
// this row reports it. The guard_file, binding_file, capabilities and
// concerns rows keep their own names and verdicts.
//
// Red is reserved for what the product would not load: a file that does
// not parse or cannot be read, a key its reader refuses, two files setting
// one key. A key only the closed schema refuses (its reader still loads it)
// and a key fathom does not know are warnings, never errors.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
)

func checkProjectKeys(root string) doctorCheck {
	const name = "project"
	p, err := projectdir.Load(root)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: err.Error(), Remedy: "make .fathom/ readable, then re-run doctor"}
	}
	readers := projectReaderErrs(p)

	var failures, warnings []string
	for _, f := range p.Files() {
		if ferr := f.Err(); ferr != nil {
			failures = append(failures, ferr.Error())
		}
	}
	known := p.KnownKeys()
	parts := make([]string, 0, len(known))
	set := 0
	for _, key := range known {
		k, ok := p.Key(key)
		if !ok {
			if home := projectdir.Home(key); home != "" && p.File(home).Err() != nil {
				parts = append(parts, fmt.Sprintf("%s: %s unreadable", key, home))
			} else {
				parts = append(parts, key+": unset")
			}
			continue
		}
		set++
		status := "ok"
		var conflict *projectdir.ConflictError
		switch {
		case errors.As(k.Err, &conflict):
			status = "conflict"
			failures = append(failures, conflict.Error())
		case readers[key] != nil:
			status = "invalid"
			failures = append(failures, readers[key].Error())
		case k.Err != nil:
			status = "invalid"
			warnings = append(warnings, k.Err.Error())
		}
		parts = append(parts, fmt.Sprintf("%s: %s %s (%d)", key, k.File, status, k.Entries))
	}
	for _, k := range p.Keys() {
		if k.Known {
			continue
		}
		parts = append(parts, fmt.Sprintf("unknown %s: %s", k.Name, k.File))
		warnings = append(warnings, fmt.Sprintf("%s: key %q is not one fathom reads", k.Path, k.Name))
	}

	reason := fmt.Sprintf("%d of %d keys set: %s", set, len(known), strings.Join(parts, "; "))
	reason += archLayerRulesNote(root)
	switch {
	case len(failures) > 0:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: reason + " — " + firstOfN(failures),
			Remedy: "fix the named file or key, then re-run doctor (docs/configuration.md names each file)",
		}
	case len(warnings) > 0:
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: reason + " — " + firstOfN(warnings),
			Remedy: "remove or rename a key fathom does not read, or fix the value schema/project/schema.cue refuses",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// projectReaderErrs is each key reader's verdict over the loaded value: the
// readers decode their keys from p with no second parse.
func projectReaderErrs(p projectdir.Project) map[string]error {
	out := map[string]error{}
	_, out["guards"] = guardspec.GuardsFromProject(p)
	_, out["layer_rules"] = guardspec.LayerRulesFromProject(p)
	_, out["bindings"] = bindspec.BindingsFromProject(p)
	_, out["capabilities"] = capability.Default().SettingsFromProject(p)
	_, out["budgets"] = capability.BudgetsFromProject(p)
	_, concernsErr := custodian.ScopeFileFromProject(p)
	out["concerns"], out["packs"] = concernsErr, concernsErr
	return out
}

// firstOfN is the first problem, with a count of the rest.
func firstOfN(problems []string) string {
	if len(problems) == 1 {
		return problems[0]
	}
	return fmt.Sprintf("%s (and %d more)", problems[0], len(problems)-1)
}

// archLayerRulesNote prints, once, the layer_rules entries equivalent to
// the legacy rules/archconformance.cue pack ("" when the pack is absent or
// does not load). The pack is still read; the entries say how to move it.
func archLayerRulesNote(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	res, err := projectrules.Load(abs)
	if err != nil || !res.Found {
		return ""
	}
	if _, serr := os.Stat(res.Path); serr != nil {
		return ""
	}
	var legacy []archconformance.Rule
	for _, r := range res.Rules {
		if len(r.FromGlobs) == 0 && len(r.ToGlobs) == 0 {
			legacy = append(legacy, r)
		}
	}
	entries := projectrules.EquivalentLayerRules(legacy)
	if len(entries) == 0 {
		return ""
	}
	for i, e := range entries {
		if len(e) > 160 {
			entries[i] = e[:160] + "..."
		}
	}
	return "; rules/archconformance.cue as layer_rules: " + strings.Join(entries, " ")
}
