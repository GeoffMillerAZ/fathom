package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func TestConsoleLearn_HookSeamsRenderStateAndCommand(t *testing.T) {
	facts := learnFacts{capVerb: "fathom capabilities --json", capModel: capabilityModel{
		HookSeams: []hookSeamRow{{ID: "claim_check", State: "off by choice", Reason: "default off", Cost: "local reads", Command: "fathom capabilities set claim_check on"}},
	}}
	page := renderLearnSwitches(learnChoice{Service: "fixture"}, console.Pipeline{}, facts, observatoryModel{})
	for _, want := range []string{"learn-hook-seams", "claim_check", "off by choice", "default off", "local reads", "fathom capabilities set claim_check on"} {
		if !strings.Contains(page, want) {
			t.Errorf("hook seams table missing %q", want)
		}
	}
	assertRoleUtilitiesOnly(t, page)
}

func TestConsoleLearn_UserHookConfigInvalidatesCapabilities(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	root := t.TempDir()
	before := learnProbeFilesKey(root)
	mustWrite(t, config.DefaultUserPath(), "[hooks]\nclaim_check = true\n")
	if before == learnProbeFilesKey(root) {
		t.Error("user hook configuration changed without invalidating the capabilities read")
	}
}
