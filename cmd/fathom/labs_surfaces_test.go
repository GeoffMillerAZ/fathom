package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	for _, key := range []string{
		"mcp:find_symbol", "skill:fathom-plan", "agent:fathom-scout",
		"hook:edit-hook --pre [pre-tool/subagent-prime PreToolUse matcher=^(Agent|Task)$]",
		"hook:readout-hook --event pre-read [pre-tool/read-outline-hint PreToolUse matcher=Read]",
		"stanza:agents-md", "decide:route",
	} {
		if !seen[key] {
			t.Errorf("missing known shipped surface %s", key)
		}
	}
	claudeHooks, allHooks := 0, 0
	for _, key := range keys {
		if strings.HasPrefix(key, "hook:") {
			allHooks++
			if !strings.Contains(key, "--harness ") {
				claudeHooks++
			}
		}
	}
	if claudeHooks != 26 || allHooks != 54 {
		t.Fatalf("hook bindings: Claude=%d all=%d, want 26 and 54", claudeHooks, allHooks)
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

// A new matcher is a new delivered binding even when its verb already ships.
func TestLabClosureRefusesNewHookBinding(t *testing.T) {
	d, ok := harness.Lookup(harness.ClaudeCode)
	if !ok {
		t.Fatal("Claude descriptor missing")
	}
	keys, err := shippedLabSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	old := d.Hooks[3]
	defer func() { d.Hooks[3] = old }()
	d.Hooks[3].Matcher = "FutureLabMatcher"
	next, err := shippedLabSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	keys = append(keys, next...)
	err = labs.CheckLabClosure(filepath.Join("..", "..", "calibration", "labs.d"), keys)
	if err == nil || !strings.Contains(err.Error(), "FutureLabMatcher") {
		t.Fatalf("new binding on an existing verb must fail closure: %v", err)
	}
}

func TestLabCardMetadata(t *testing.T) {
	keys, err := shippedLabSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	cards, err := labs.LoadLabs(filepath.Join("..", "..", "calibration", "labs.d"), keys)
	if err != nil {
		t.Fatal(err)
	}
	locations := map[string][]string{
		"lab-stanza":      {"cmd/fathom/init.go:fathomStanzaBody"},
		"lab-guards":      {".fathom/guard.cue:guards[].note", "cmd/fathom/edithook_pre.go:renderPreEdit"},
		"lab-obligations": {"cmd/fathom/edithook.go"},
		"lab-verify-plan": {"cmd/fathom/stophook.go"},
	}
	for _, card := range cards {
		for _, want := range locations[card.ID] {
			found := false
			for _, knob := range card.Knobs {
				if knob.Location == want {
					found = true
				}
			}
			if !found {
				t.Errorf("%s lacks real text location %s", card.ID, want)
			}
		}
		if card.ID == "lab-search" || card.ID == "lab-symbol-context" {
			for _, owner := range card.Owners {
				if strings.Contains(owner, "EW-76") {
					t.Errorf("%s lists quality instrument as stage owner", card.ID)
				}
			}
			found := false
			for _, reading := range card.Quality {
				if reading.Instrument == "EW-76" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s lost EW-76 quality instrument", card.ID)
			}
		}
	}
}
