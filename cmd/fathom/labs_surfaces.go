package main

import (
	"sort"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// shippedLabSurfaces derives MCP, hook and rendered-skill keys from their
// shipping registries. It also names the shipped stanza and concrete decision
// sites so cards using those key families are checked against an inventory.
// Hook keys are command verbs (including flags); shared bindings deduplicate.
// Both default and opt-in tables count: enabling readouts folds the test-run
// bindings, but those bindings still ship for installations without readouts.
func shippedLabSurfaces() ([]string, error) {
	tools, err := registeredToolNames("")
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{"stanza:agents-md": true}
	for _, tool := range tools {
		keys["mcp:"+tool] = true
	}
	for _, d := range harness.All() {
		opts := []string{}
		for _, binding := range d.Hooks {
			if binding.OptIn != "" {
				opts = append(opts, binding.OptIn)
			}
		}
		specs := append(hookSpecsFor(d.ID), hookSpecsFor(d.ID, opts...)...)
		for _, spec := range specs {
			keys["hook:"+spec.sub] = true
		}
	}
	for _, name := range skillsdist.Names {
		keys["skill:"+name] = true
	}
	for _, site := range decide.AllSites {
		keys["decide:"+site] = true
	}
	for _, site := range decide.SessionSites {
		keys["decide:"+site] = true
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}
