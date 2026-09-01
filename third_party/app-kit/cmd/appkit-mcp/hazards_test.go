package main

import (
	"os"
	"strings"
	"testing"
)

// hazardDoc is the full reviewer checklist this guide condenses.
const hazardDoc = "../../docs/theming-hazards.md"

func TestThemingHazardsCoversEveryHazard(t *testing.T) {
	h := themingHazards()
	for _, want := range []string{
		"data-theme=\"light\"", // 1: the gating hazard, by its selector
		"synthwave",            // 1: why it already applies to this kit
		":not(.is-used)",       // 2: the immediate fix
		"GradedRole",           // 2: the structural fix, in kit terms
		"tokens.ContrastOver",  // 3: the composited measurement
		"never written",        // 3: the cautionary fact
		"fallback",             // 4: what a tolerant reference must emit
		"neon-pink",            // 4: the kit's live instance
		"3:1",                  // 5: the floor
		"viz.Heatmap",          // 5: the kit's own opacity ramp
	} {
		if !strings.Contains(h, want) {
			t.Errorf("hazard checklist is missing %q", want)
		}
	}
	if !strings.Contains(themingGuide(), "Hazards — the ones that shipped") {
		t.Error("get_kit_theming's guide no longer carries the hazard checklist")
	}
}

// TestHazardNumbersMatchTheDoc keeps the agent-facing summary and the reviewer
// checklist telling the same story. Both cite measurements; a measurement that
// drifts between two documents is worse than one that appears in neither,
// because it makes the wrong number look corroborated.
func TestHazardNumbersMatchTheDoc(t *testing.T) {
	body, err := os.ReadFile(hazardDoc)
	if err != nil {
		t.Skipf("checklist not readable from this working directory: %v", err)
	}
	doc, h := string(body), themingHazards()
	for _, n := range []string{
		"8.51:1", "3.39:1", // H1 — the inverted figure/ground
		"1.30:1", "1.66:1", // H2/H3 — the marks that shipped unreadable
		"5.97:1", "1.19:1", // H5 — the kit's own composited ramp
		"2.60", // the kit's one role below the floor
	} {
		if !strings.Contains(h, n) {
			t.Errorf("hazard checklist does not cite %s", n)
		}
		if !strings.Contains(doc, n) {
			t.Errorf("%s does not cite %s — the two surfaces have drifted", hazardDoc, n)
		}
	}
}
