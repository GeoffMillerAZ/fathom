package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/labs"
)

func TestShippedLabSurfaces(t *testing.T) {
	keys, err := shippedLabSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(keys) {
		t.Fatal("surface inventory is unsorted")
	}
	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			t.Errorf("duplicate surface %s", key)
		}
		seen[key] = true
	}
	tools, err := registeredToolNames("")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if !seen["mcp:"+tool] {
			t.Errorf("missing tool %s", tool)
		}
	}
	for _, name := range skillsdist.Names {
		if !seen["skill:"+name] {
			t.Errorf("missing rendered skill %s", name)
		}
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
			if !seen["hook:"+spec.sub] {
				t.Errorf("missing hook %s (%s)", spec.sub, d.ID)
			}
		}
	}
	for _, key := range []string{"hook:session-hook --event test-run", "hook:readout-hook --event post-tool", "hook:stop-hook --event subagent-stop", "stanza:agents-md", "decide:route"} {
		if !seen[key] {
			t.Errorf("missing conditional surface %s", key)
		}
	}
}

func TestCommittedLabsCloseOverShippedSurfaces(t *testing.T) {
	keys, err := shippedLabSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("empty shipped inventory")
	}
	dir := filepath.Join("..", "..", "calibration", "labs.d")
	if err := labs.CheckLabClosure(dir, keys); err != nil {
		t.Fatal(err)
	}
	keys = append(keys, "mcp:future_lab_closure_probe")
	if err := labs.CheckLabClosure(dir, keys); err == nil || !strings.Contains(err.Error(), "future_lab_closure_probe") {
		t.Fatalf("new shipped surface must fail closure: %v", err)
	}
}
