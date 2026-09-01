package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

func TestCSSSetupGuideContract(t *testing.T) {
	g := cssSetupGuide()
	// The non-negotiable facts a consumer needs to get rendering right.
	for _, want := range []string{
		"No Node",    // D2
		"appkit gen", // theme layer generation
		"appkit css", // pinned-binary build
		"@import \"tailwindcss\"",
		"theme.css",   // the generated layer
		"@source",     // scanning requirement (the #1 inert-utility cause)
		"data-theme",  // light/dark contract
		"--ak-color-", // the variable layer roles resolve through
		"rounded-",    // radius vocabulary
		"font-",       // font vocabulary
	} {
		if !strings.Contains(g, want) {
			t.Errorf("CSS setup guide missing %q", want)
		}
	}
}

// TestCSSSetupGuideCoversMotion — the CSS setup guide is the agent-facing "how
// do I wire the kit's CSS in" answer, and the motion layer wires in the same
// way (an @import after the theme) but with one crucial difference: it needs no
// @source entry. Without this section an agent adapting a component finds the
// role utilities and never learns the motion layer exists, or wastes time
// hunting for the @source glob it does not need.
func TestCSSSetupGuideCoversMotion(t *testing.T) {
	g := cssSetupGuide()
	for _, want := range []string{
		"motion.css", // the import line
		"ak-motion",  // the class vocabulary
		"@source",    // the "no @source needed" clarification is stated here
		"RootClass",  // the Go helpers
	} {
		if !strings.Contains(g, want) {
			t.Errorf("CSS setup guide has no motion content: missing %q", want)
		}
	}
}

// TestCSSSetupGuideAdvertisesRealVocabulary guards drift: every color role,
// radius name, and font name the guide enumerates must come from the generated
// token vocabulary, and all of them must appear. If a role is renamed in CUE,
// the guide changes with it (it reads tokens.* directly) — this asserts the
// wiring stays intact.
func TestCSSSetupGuideAdvertisesRealVocabulary(t *testing.T) {
	g := cssSetupGuide()
	for _, r := range tokens.ColorNames {
		if !strings.Contains(g, "`"+r+"`") {
			t.Errorf("guide omits color role %q", r)
		}
	}
	for _, r := range tokens.RadiusNames {
		if !strings.Contains(g, "`rounded-"+r+"`") {
			t.Errorf("guide omits radius utility rounded-%s", r)
		}
	}
	for _, f := range tokens.FontNames {
		if !strings.Contains(g, "`font-"+f+"`") {
			t.Errorf("guide omits font utility font-%s", f)
		}
	}
}
