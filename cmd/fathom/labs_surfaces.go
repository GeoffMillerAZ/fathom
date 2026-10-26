package main

import (
	"path/filepath"
	"sort"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// shippedLabSurfaces derives MCP, hook, rendered-agent and skill keys from their
// shipping registries. It also names the shipped stanza and concrete decision
// sites so cards using those key families are checked against an inventory.
// Hook keys preserve event, capability, harness event and matcher identity.
// Both default and opt-in tables count: enabling readouts folds the test-run
// bindings, but those bindings still ship for installations without readouts.
func shippedLabSurfaces() ([]string, error) {
	tools, err := registeredToolNames("")
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{"stanza:agents-md": true, "agent:" + strings.TrimSuffix(filepath.Base(scoutAgentPath), ".md"): true}
	for _, tool := range tools {
		keys["mcp:"+tool] = true
	}
	for _, d := range harness.All() {
		for _, binding := range d.Hooks {
			keys[labHookSurface(binding)] = true
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

// labHookSurface includes both the canonical and harness event: CwdChanged
// and DirectoryAdded share a capability but are separate installed bindings.
func labHookSurface(b harness.EventBinding) string {
	return "hook:" + b.Verb + " [" + string(b.Event) + "/" + string(b.Capability) + " " + b.Harness + " matcher=" + b.Matcher + "]"
}
